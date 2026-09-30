package piinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Archive extraction limits. They bound both hostile and accidental archives.
const (
	defaultMaxArchiveEntries = 200000
	defaultMaxArchiveFile    = int64(1) << 30 // 1 GiB per member
	defaultMaxArchiveTotal   = int64(4) << 30 // 4 GiB expanded total
)

var (
	errArchiveTraversal = errors.New("piinstall: archive entry escapes the extraction root")
	errArchiveCollision = errors.New("piinstall: archive entry collides with an existing path")
	errSymlinkComponent = errors.New("piinstall: refusing to write through a symlinked path component")
)

// ExtractOptions bounds one extraction.
type ExtractOptions struct {
	StripComponents int
	MaxEntries      int
	MaxFileSize     int64
	MaxTotalSize    int64
}

func (o ExtractOptions) withDefaults() ExtractOptions {
	if o.MaxEntries <= 0 {
		o.MaxEntries = defaultMaxArchiveEntries
	}
	if o.MaxFileSize <= 0 {
		o.MaxFileSize = defaultMaxArchiveFile
	}
	if o.MaxTotalSize <= 0 {
		o.MaxTotalSize = defaultMaxArchiveTotal
	}
	return o
}

type pendingLink struct {
	rel    string
	target string
}

// makeDirsSafe creates dir and every missing ancestor, refusing to traverse an
// existing symlink or non-directory component. It closes the symlink-ancestor
// gap without claiming race-proof sandboxing.
func makeDirsSafe(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs, volume)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	if rest == "" {
		return nil
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: %s", errSymlinkComponent, current)
			}
			if !info.IsDir() {
				return fmt.Errorf("piinstall: path component %s is not a directory", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// checkDestinationLeaf rejects an existing symlink or non-directory leaf.
func checkDestinationLeaf(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", errSymlinkComponent, target)
	}
	return nil
}

// rejectSymlinkedPath checks every path component of target below base and
// refuses to follow a symlink at any level, including the leaf. It is used
// before reading existing payload files so a symlinked ancestor or leaf cannot
// redirect a read outside the owned base. Missing components are not an error.
// base itself is trusted as the caller-owned root and is not required to be a
// real directory.
func rejectSymlinkedPath(base, target string) error {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("piinstall: %s escapes the owned root %s", target, base)
	}
	current := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", errSymlinkComponent, current)
		}
	}
	return nil
}

// verifyArchive checks the exact byte size when pinned and the SHA-256 digest.
func verifyArchive(archivePath, expectedSHA string, expectedSize int64) error {
	if strings.TrimSpace(expectedSHA) == "" {
		return errors.New("piinstall: refusing to verify an archive without a pinned SHA-256")
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return err
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return fmt.Errorf("piinstall: archive size %d does not match pinned size %d", info.Size(), expectedSize)
	}
	sum, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, expectedSHA) {
		return fmt.Errorf("piinstall: archive SHA-256 %s does not match pinned %s", sum, expectedSHA)
	}
	return nil
}

func sha256File(archivePath string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// cleanArchiveName normalizes a member name, rejects absolute/drive/traversal
// paths, and applies StripComponents.
func cleanArchiveName(name string, strip int) (string, error) {
	if name == "" {
		return "", nil
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: NUL in entry name", errArchiveTraversal)
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") || hasDrivePrefix(normalized) {
		return "", fmt.Errorf("%w: %s", errArchiveTraversal, name)
	}
	parts := strings.Split(normalized, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("%w: %s", errArchiveTraversal, name)
		}
		clean = append(clean, part)
	}
	if strip > 0 {
		if len(clean) <= strip {
			return "", nil
		}
		clean = clean[strip:]
	}
	if len(clean) == 0 {
		return "", nil
	}
	return strings.Join(clean, "/"), nil
}

func hasDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	first := name[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}

// validateLink resolves a relative link within the archive root and rejects
// absolute or escaping targets. In-root links such as Node's npm/npx are kept.
func validateLink(linkRel, target string) (string, error) {
	if target == "" || strings.ContainsRune(target, 0) {
		return "", fmt.Errorf("%w: empty link target for %s", errArchiveTraversal, linkRel)
	}
	normalized := strings.ReplaceAll(target, "\\", "/")
	if strings.HasPrefix(normalized, "/") || hasDrivePrefix(normalized) {
		return "", fmt.Errorf("%w: absolute link target %q", errArchiveTraversal, target)
	}
	joined := path.Clean(path.Join(path.Dir(linkRel), normalized))
	if joined == "." || joined == ".." || strings.HasPrefix(joined, "../") || strings.HasPrefix(joined, "/") {
		return "", fmt.Errorf("%w: link %s -> %s escapes the extraction root", errArchiveTraversal, linkRel, target)
	}
	return joined, nil
}

func withinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func writeRegularFile(target string, reader io.Reader, size int64, mode os.FileMode) error {
	if err := makeDirsSafe(filepath.Dir(target)); err != nil {
		return err
	}
	if anyExists(target) {
		return fmt.Errorf("%w: %s", errArchiveCollision, target)
	}
	mode = sanitizeFileMode(mode)
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if size > 0 {
		if _, err := io.CopyN(file, reader, size); err != nil {
			file.Close()
			return fmt.Errorf("piinstall: extract %s: %w", target, err)
		}
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func sanitizeFileMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0o644
	}
	return perm
}

func createDeferredLinks(root string, links []pendingLink) error {
	for _, link := range links {
		target := filepath.Join(root, filepath.FromSlash(link.rel))
		if err := makeDirsSafe(filepath.Dir(target)); err != nil {
			return err
		}
		if anyExists(target) {
			return fmt.Errorf("%w: symlink %s", errArchiveCollision, link.rel)
		}
		if err := os.Symlink(filepath.FromSlash(link.target), target); err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz extracts a gzip-compressed tar archive into a fresh directory at
// dst. Regular files are written first; in-root symlinks are deferred so their
// targets exist before the link is created.
func extractTarGz(dst, archivePath string, opts ExtractOptions) error {
	opts = opts.withDefaults()
	if err := makeDirsSafe(dst); err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("piinstall: open gzip archive: %w", err)
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	var links []pendingLink
	entries := 0
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("piinstall: read tar entry: %w", err)
		}
		entries++
		if entries > opts.MaxEntries {
			return fmt.Errorf("piinstall: archive exceeds %d entries", opts.MaxEntries)
		}
		rel, err := cleanArchiveName(header.Name, opts.StripComponents)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if !withinRoot(dst, target) {
			return fmt.Errorf("%w: %s", errArchiveTraversal, header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := makeDirsSafe(target); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > opts.MaxFileSize {
				return fmt.Errorf("piinstall: tar entry %s size %d exceeds limit", header.Name, header.Size)
			}
			total += header.Size
			if total > opts.MaxTotalSize {
				return fmt.Errorf("piinstall: archive expands beyond %d bytes", opts.MaxTotalSize)
			}
			if err := writeRegularFile(target, reader, header.Size, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if _, err := validateLink(rel, header.Linkname); err != nil {
				return err
			}
			links = append(links, pendingLink{rel: rel, target: header.Linkname})
		default:
			return fmt.Errorf("piinstall: unsupported tar entry type %q for %s", header.Typeflag, header.Name)
		}
	}
	return createDeferredLinks(dst, links)
}

// extractZip extracts a zip archive into a fresh directory at dst.
func extractZip(dst, archivePath string, opts ExtractOptions) error {
	opts = opts.withDefaults()
	if err := makeDirsSafe(dst); err != nil {
		return err
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("piinstall: open zip archive: %w", err)
	}
	defer reader.Close()

	var links []pendingLink
	entries := 0
	var total int64
	for _, member := range reader.File {
		entries++
		if entries > opts.MaxEntries {
			return fmt.Errorf("piinstall: archive exceeds %d entries", opts.MaxEntries)
		}
		rel, err := cleanArchiveName(member.Name, opts.StripComponents)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if !withinRoot(dst, target) {
			return fmt.Errorf("%w: %s", errArchiveTraversal, member.Name)
		}
		mode := member.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			linkTarget, err := readZipMember(member, opts.MaxFileSize)
			if err != nil {
				return err
			}
			if _, err := validateLink(rel, string(linkTarget)); err != nil {
				return err
			}
			links = append(links, pendingLink{rel: rel, target: string(linkTarget)})
		case member.FileInfo().IsDir() || strings.HasSuffix(member.Name, "/"):
			if err := makeDirsSafe(target); err != nil {
				return err
			}
		default:
			size := int64(member.UncompressedSize64)
			if size > opts.MaxFileSize {
				return fmt.Errorf("piinstall: zip entry %s size %d exceeds limit", member.Name, size)
			}
			total += size
			if total > opts.MaxTotalSize {
				return fmt.Errorf("piinstall: archive expands beyond %d bytes", opts.MaxTotalSize)
			}
			if err := extractZipMember(target, member, size, mode); err != nil {
				return err
			}
		}
	}
	return createDeferredLinks(dst, links)
}

func readZipMember(member *zip.File, max int64) ([]byte, error) {
	reader, err := member.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, max+1))
}

func extractZipMember(target string, member *zip.File, size int64, mode os.FileMode) error {
	reader, err := member.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return writeRegularFile(target, reader, size, mode)
}

// findRegularFile locates exactly one regular file named name under root,
// searching at most maxDepth directory levels.
func findRegularFile(root, name string, maxDepth int) (string, error) {
	root = filepath.Clean(root)
	var matches []string
	walkErr := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if current == root {
				return nil
			}
			rel, relErr := filepath.Rel(root, current)
			if relErr != nil {
				return relErr
			}
			if strings.Count(rel, string(filepath.Separator)) >= maxDepth {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && entry.Name() == name {
			matches = append(matches, current)
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("piinstall: expected exactly one regular file named %q under %s, found %d", name, root, len(matches))
	}
	return matches[0], nil
}

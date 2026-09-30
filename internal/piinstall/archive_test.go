package piinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type tarEntry struct {
	name     string
	mode     int64
	typeflag byte
	data     []byte
	link     string
}

func writeTarGz(t *testing.T, archivePath string, entries []tarEntry) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar fixture: %v", err)
	}
	gz := gzip.NewWriter(file)
	writer := tar.NewWriter(gz)
	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Typeflag: entry.typeflag,
			Linkname: entry.link,
		}
		if entry.typeflag == tar.TypeReg {
			header.Size = int64(len(entry.data))
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if entry.typeflag == tar.TypeReg {
			if _, err := writer.Write(entry.data); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

type zipEntry struct {
	name string
	mode os.FileMode
	data []byte
}

func writeZip(t *testing.T, archivePath string, entries []zipEntry) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip fixture: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		body, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip header: %v", err)
		}
		if _, err := body.Write(entry.data); err != nil {
			t.Fatalf("write zip body: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

func TestExtractTarGzPreservesNodeStyleInternalLinks(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node.tar.gz")
	writeTarGz(t, archivePath, []tarEntry{
		{name: "node-v24.21.0-linux-x64/bin/node", mode: 0o755, typeflag: tar.TypeReg, data: []byte("node")},
		{name: "node-v24.21.0-linux-x64/lib/node_modules/npm/bin/npm-cli.js", mode: 0o644, typeflag: tar.TypeReg, data: []byte("npm")},
		{name: "node-v24.21.0-linux-x64/bin/npm", mode: 0o777, typeflag: tar.TypeSymlink, link: "../lib/node_modules/npm/bin/npm-cli.js"},
	})

	dst := filepath.Join(t.TempDir(), "out")
	if err := extractTarGz(dst, archivePath, ExtractOptions{StripComponents: 1}); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	linkPath := filepath.Join(dst, "bin", "npm")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected npm symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink, mode=%v", linkPath, info.Mode())
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "../lib/node_modules/npm/bin/npm-cli.js" {
		t.Errorf("unexpected link target %q", target)
	}
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("resolve internal link: %v", err)
	}
	if !strings.HasPrefix(resolved, dst) {
		t.Errorf("internal link resolved outside extraction root: %s", resolved)
	}
	if _, err := os.Stat(filepath.Join(dst, "bin", "node")); err != nil {
		t.Errorf("expected extracted node binary: %v", err)
	}
}

func TestExtractTarGzRejectsTraversalAndAbsoluteNames(t *testing.T) {
	for _, name := range []string{"../evil", "/absolute/evil", "a/../../evil", `..\evil`} {
		t.Run(name, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "bad.tar.gz")
			writeTarGz(t, archivePath, []tarEntry{{name: name, mode: 0o644, typeflag: tar.TypeReg, data: []byte("x")}})
			dst := filepath.Join(t.TempDir(), "out")
			err := extractTarGz(dst, archivePath, ExtractOptions{})
			if !errors.Is(err, errArchiveTraversal) {
				t.Fatalf("expected traversal rejection, got %v", err)
			}
		})
	}
}

func TestExtractTarGzRejectsEscapingAndAbsoluteLinks(t *testing.T) {
	for _, link := range []string{"../../etc/passwd", "/etc/passwd", "C:\\Windows\\system32"} {
		t.Run(link, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "badlink.tar.gz")
			writeTarGz(t, archivePath, []tarEntry{
				{name: "bin/evil", mode: 0o777, typeflag: tar.TypeSymlink, link: link},
			})
			dst := filepath.Join(t.TempDir(), "out")
			err := extractTarGz(dst, archivePath, ExtractOptions{})
			if !errors.Is(err, errArchiveTraversal) {
				t.Fatalf("expected link rejection, got %v", err)
			}
		})
	}
}

func TestExtractRejectsDuplicateCollision(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "dup.tar.gz")
	writeTarGz(t, archivePath, []tarEntry{
		{name: "tool", mode: 0o755, typeflag: tar.TypeReg, data: []byte("first")},
		{name: "tool", mode: 0o755, typeflag: tar.TypeReg, data: []byte("second")},
	})
	dst := filepath.Join(t.TempDir(), "out")
	err := extractTarGz(dst, archivePath, ExtractOptions{})
	if !errors.Is(err, errArchiveCollision) {
		t.Fatalf("expected collision rejection, got %v", err)
	}
}

func TestMakeDirsSafeRejectsSymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	err := makeDirsSafe(filepath.Join(link, "nested"))
	if !errors.Is(err, errSymlinkComponent) {
		t.Fatalf("expected symlink-ancestor rejection, got %v", err)
	}
}

func TestInstallPayloadRejectsSymlinkLeaf(t *testing.T) {
	workspace := t.TempDir()
	destination := payloadDestination(workspace)
	if err := os.MkdirAll(filepath.Join(destination, "ogs-godot-change", "references"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.Symlink(outside, filepath.Join(destination, "ogs-godot-change", "SKILL.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := installPayload(workspace); err == nil {
		t.Fatalf("expected symlink-leaf payload rejection")
	}
	if _, err := os.Stat(outside); err == nil {
		t.Errorf("payload write must not have created the symlink target")
	}
}

// TestInstallPayloadRejectsSymlinkedGroupAndReference proves the shared root, a
// sibling group directory, and a reference leaf are all closed against symlink
// redirection without writing into the link target.
func TestInstallPayloadRejectsSymlinkedGroupAndReference(t *testing.T) {
	cases := []struct {
		name string
		link func(workspace, destination, outside string)
	}{
		{
			name: "symlinked new group directory",
			link: func(_, destination, outside string) {
				if err := os.MkdirAll(destination, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.Symlink(outside, filepath.Join(destination, "ogs-core")); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
		},
		{
			name: "symlinked reference leaf",
			link: func(_, destination, outside string) {
				refDir := filepath.Join(destination, "ogs-core", "references")
				if err := os.MkdirAll(refDir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.Symlink(outside, filepath.Join(refDir, "handoff-contract.md")); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
		},
		{
			name: "symlinked shared root",
			link: func(workspace, _, outside string) {
				if err := os.MkdirAll(filepath.Join(workspace, ".pi"), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.Symlink(outside, filepath.Join(workspace, ".pi", "skills")); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			destination := payloadDestination(workspace)
			outside := filepath.Join(t.TempDir(), "outside.md")
			tc.link(workspace, destination, outside)
			if _, err := installPayload(workspace); err == nil {
				t.Fatalf("expected symlinked payload path rejection")
			}
			if _, err := os.Stat(outside); err == nil {
				t.Errorf("payload write must not have created the symlink target")
			}
		})
	}
}

func TestExtractZipRejectsTraversalAndExtractsGodotStyleMember(t *testing.T) {
	badArchive := filepath.Join(t.TempDir(), "bad.zip")
	writeZip(t, badArchive, []zipEntry{{name: "../evil", mode: 0o644, data: []byte("x")}})
	if err := extractZip(filepath.Join(t.TempDir(), "out"), badArchive, ExtractOptions{}); !errors.Is(err, errArchiveTraversal) {
		t.Fatalf("expected zip traversal rejection, got %v", err)
	}

	goodArchive := filepath.Join(t.TempDir(), "godot.zip")
	writeZip(t, goodArchive, []zipEntry{{name: "Godot_v4.7.2-stable_linux.x86_64", mode: 0o755, data: []byte("godot")}})
	dst := filepath.Join(t.TempDir(), "out")
	if err := extractZip(dst, goodArchive, ExtractOptions{}); err != nil {
		t.Fatalf("extractZip failed: %v", err)
	}
	found, err := findRegularFile(dst, "Godot_v4.7.2-stable_linux.x86_64", 4)
	if err != nil {
		t.Fatalf("findRegularFile failed: %v", err)
	}
	if filepath.Dir(found) != dst {
		t.Errorf("expected member at archive root, got %s", found)
	}
}

func TestFindRegularFileRejectsAmbiguity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "engram"), []byte("a"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	nested := filepath.Join(dir, "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "engram"), []byte("b"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := findRegularFile(dir, "engram", 4); err == nil {
		t.Fatalf("expected ambiguity error for two matching files")
	}
}

func TestVerifyArchiveDigestAndSize(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "payload.bin")
	payload := []byte("verified payload")
	if err := os.WriteFile(archivePath, payload, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	if err := verifyArchive(archivePath, digest, int64(len(payload))); err != nil {
		t.Fatalf("verifyArchive rejected a valid archive: %v", err)
	}
	if err := verifyArchive(archivePath, strings.Repeat("0", 64), int64(len(payload))); err == nil {
		t.Fatalf("expected digest mismatch")
	}
	if err := verifyArchive(archivePath, digest, int64(len(payload)+1)); err == nil {
		t.Fatalf("expected size mismatch")
	}
	if err := verifyArchive(archivePath, "", 0); err == nil {
		t.Fatalf("expected refusal to verify without a pinned SHA-256")
	}
}

func TestHTTPDownloaderRejectsNonHTTPS(t *testing.T) {
	downloader := HTTPDownloader{}
	err := downloader.Download(context.Background(), "http://127.0.0.1/archive", filepath.Join(t.TempDir(), "f"), 1024)
	if err == nil || !strings.Contains(err.Error(), "non-https") {
		t.Fatalf("expected non-https rejection, got %v", err)
	}
}

func TestHTTPDownloaderLoopbackBounded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("archive-bytes"))
	}))
	defer server.Close()

	downloader := HTTPDownloader{Client: server.Client()}
	dest := filepath.Join(t.TempDir(), "download")
	if err := downloader.Download(context.Background(), server.URL, dest, 1024); err != nil {
		t.Fatalf("loopback download failed: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != "archive-bytes" {
		t.Errorf("unexpected download content %q", data)
	}

	tooSmall := filepath.Join(t.TempDir(), "too-small")
	if err := downloader.Download(context.Background(), server.URL, tooSmall, 3); err == nil {
		t.Fatalf("expected bounded download to reject oversized body")
	}
}

func TestHTTPDownloaderRejectsHTTPSDowngradeRedirect(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plaintext"))
	}))
	defer plain.Close()

	redirector := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client := redirector.Client()
	downloader := HTTPDownloader{Client: client}
	dest := filepath.Join(t.TempDir(), "downgrade")
	err := downloader.Download(context.Background(), redirector.URL, dest, 1024)
	if err == nil || !strings.Contains(err.Error(), "non-https redirect") {
		t.Fatalf("expected https->http redirect rejection, got %v", err)
	}
	if client.CheckRedirect != nil {
		t.Errorf("the supplied client's redirect policy was mutated")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("a rejected redirect must not leave a downloaded file")
	}
}

func TestHTTPDownloaderFollowsSafeHTTPSRedirect(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/payload", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("redirected-bytes"))
	}))
	defer server.Close()

	downloader := HTTPDownloader{Client: server.Client()}
	dest := filepath.Join(t.TempDir(), "redirected")
	if err := downloader.Download(context.Background(), server.URL+"/redirect", dest, 1024); err != nil {
		t.Fatalf("safe https redirect failed: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read redirected download: %v", err)
	}
	if string(data) != "redirected-bytes" {
		t.Errorf("unexpected redirected content %q", data)
	}
}

// TestHTTPDownloaderBoundsRedirectsDespitePermissiveCallerPolicy proves the
// hop cap is enforced even when a caller-supplied CheckRedirect would allow an
// unbounded redirect loop. The server eventually returns a payload, so a
// bypassed bound would produce a successful download instead of a hang.
func TestHTTPDownloaderBoundsRedirectsDespitePermissiveCallerPolicy(t *testing.T) {
	var hops int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hops, 1) > 30 {
			_, _ = w.Write([]byte("payload"))
			return
		}
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()

	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	downloader := HTTPDownloader{Client: client}
	dest := filepath.Join(t.TempDir(), "loop")
	err := downloader.Download(context.Background(), server.URL+"/loop", dest, 1024)
	if err == nil {
		t.Fatalf("a permissive caller redirect policy must not bypass the redirect bound")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("expected a redirect-bound error, got %v", err)
	}
	if got := atomic.LoadInt32(&hops); got > maxDownloadRedirects {
		t.Errorf("redirect bound not enforced: saw %d requests", got)
	}
	if client.CheckRedirect == nil {
		t.Errorf("the supplied client's redirect policy was dropped")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("a bounded redirect must not leave a downloaded file")
	}
}

// TestHTTPDownloaderHonorsCallerRedirectPolicy proves a caller-supplied policy
// is still consulted, and its refusal still fails the download.
func TestHTTPDownloaderHonorsCallerRedirectPolicy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/payload", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("redirected-bytes"))
	}))
	defer server.Close()

	sentinel := errors.New("caller refused the redirect")
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return sentinel }
	downloader := HTTPDownloader{Client: client}
	dest := filepath.Join(t.TempDir(), "refused")
	err := downloader.Download(context.Background(), server.URL+"/redirect", dest, 1024)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the caller redirect error to propagate, got %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("a caller-refused redirect must not leave a downloaded file")
	}
}

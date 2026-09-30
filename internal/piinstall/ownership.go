package piinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ownerResolver returns the numeric owner uid for an existing path. It is a
// package variable so tests can simulate a foreign owner without changing real
// filesystem ownership. On platforms without a supported implementation it
// returns ok=false, which makes every ownership check fail closed.
var ownerResolver = platformOwnerResolver

// currentUserID returns the effective numeric user id of this process. Like
// ownerResolver it fails closed when unsupported.
var currentUserID = platformCurrentUserID

// trustedCreationRoots returns the root-owned sticky shared temp bases under
// which creating a new private child is permitted (for example /tmp or /var/tmp).
// It is a package variable so tests can supply a hermetic fixture without
// touching the real platform temp roots.
var trustedCreationRoots = defaultTrustedCreationRoots

func defaultTrustedCreationRoots() []string {
	return append([]string(nil), tempRoots...)
}

func isTrustedTempRoot(path string) bool {
	clean := filepath.Clean(path)
	for _, root := range trustedCreationRoots() {
		if clean == root {
			return true
		}
	}
	return false
}

// validateOwnedRoot checks only local path metadata for the supplied managed
// root and its existing ancestors. It never scans host configuration, never
// resolves symlinks, never changes ownership or permissions, and makes no
// race-free claim: a path component can still be swapped after validation. The
// walk is initialized with the filesystem root itself, so a missing first
// component still has a concrete creation parent to validate. When a component
// is missing, the nearest existing creation parent is validated instead of being
// accepted unconditionally; the executor revalidates the created owned root
// before any managed probe or write.
func validateOwnedRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return ErrInvalidConfig
	}
	uid, ok := currentUserID()
	if !ok {
		return fmt.Errorf("%w: cannot determine the current user on this platform", ErrUnsafeRoot)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnsafeRoot, err)
	}
	abs = filepath.Clean(abs)
	volume := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs, volume)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	// Initialize and validate the filesystem-root metadata before walking any
	// component. If the first component is missing, the creation parent is the
	// filesystem root, which checkCreationParent refuses; empty or nil
	// creation-parent metadata never falls through as acceptable.
	lastExisting := current
	var lastInfo os.FileInfo
	if info, err := os.Lstat(current); err != nil {
		return fmt.Errorf("%w: cannot inspect %s: %v", ErrUnsafeRoot, current, err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrUnsafeRoot, current)
	} else if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUnsafeRoot, current)
	} else if err := checkDirMetadata(current, info, uid, false); err != nil {
		return err
	} else {
		lastInfo = info
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return checkCreationParent(lastExisting, lastInfo, uid)
			}
			return fmt.Errorf("%w: cannot inspect %s: %v", ErrUnsafeRoot, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a symlink", ErrUnsafeRoot, current)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: %s is not a directory", ErrUnsafeRoot, current)
		}
		if err := checkDirMetadata(current, info, uid, current == abs); err != nil {
			return err
		}
		lastExisting = current
		lastInfo = info
	}
	return nil
}

// checkCreationParent validates the nearest existing ancestor when the next path
// component is missing and would have to be created. Creating a child requires
// an existing parent owned by the current user, or a narrowly trusted root-owned
// sticky shared temp base (for example /tmp or /var/tmp) where the sticky bit
// protects new private children. The filesystem root is never an acceptable
// creation parent, even for a caller running as UID 0: a new per-user install
// directory must be created beneath an owned or narrowly trusted parent, not
// directly under /. A root-owned non-sticky ancestor such as /home cannot be
// written by the current user, so a missing component beneath it is refused
// instead of being created optimistically. Empty or nil parent metadata fails
// closed rather than being treated as acceptable. An already-owned user root
// does not reach this path because every component exists and is validated as an
// ancestor instead.
func checkCreationParent(path string, info os.FileInfo, uid uint32) error {
	if path == "" || info == nil {
		return fmt.Errorf("%w: cannot verify the creation parent for a new component", ErrUnsafeRoot)
	}
	if filepath.Clean(path) == filepath.VolumeName(path)+string(filepath.Separator) {
		return fmt.Errorf("%w: filesystem root %s is not an acceptable creation parent", ErrUnsafeRoot, path)
	}
	owner, ok := ownerResolver(path, info)
	if !ok {
		return fmt.Errorf("%w: cannot verify ownership of creation parent %s", ErrUnsafeRoot, path)
	}
	if owner == uid {
		if info.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("%w: creation parent %s is group/world-writable (mode %#o)", ErrUnsafeRoot, path, info.Mode().Perm())
		}
		return nil
	}
	if owner == 0 && info.Mode()&os.ModeSticky != 0 && isTrustedTempRoot(path) {
		return nil
	}
	return fmt.Errorf("%w: cannot create a new component under %s: it is owned by uid %d and is not a current-user or trusted sticky temp parent", ErrUnsafeRoot, path, owner)
}

// checkDirMetadata applies the ownership and permission policy to one existing
// directory component. The managed root itself must be owned by the current
// user and not group/world-writable. An ancestor owned by the current user must
// not be group/world-writable; a root-owned ancestor is allowed when it is not
// writable or is guarded by the sticky bit (for example a shared temp root). A
// foreign-owned (non-root, non-current) ancestor is rejected regardless of its
// mode: that owner controls its children even when it is not group/world-writable.
func checkDirMetadata(path string, info os.FileInfo, uid uint32, final bool) error {
	owner, ok := ownerResolver(path, info)
	if !ok {
		return fmt.Errorf("%w: cannot verify ownership of %s", ErrUnsafeRoot, path)
	}
	mode := info.Mode().Perm()
	writable := mode&0o022 != 0
	sticky := info.Mode()&os.ModeSticky != 0
	if final {
		if owner != uid {
			return fmt.Errorf("%w: managed root %s is owned by uid %d, not the current user uid %d", ErrUnsafeRoot, path, owner, uid)
		}
		if writable {
			return fmt.Errorf("%w: managed root %s is group/world-writable (mode %#o)", ErrUnsafeRoot, path, mode)
		}
		return nil
	}
	switch {
	case owner == uid:
		if writable {
			return fmt.Errorf("%w: ancestor %s is group/world-writable (mode %#o)", ErrUnsafeRoot, path, mode)
		}
		return nil
	case owner == 0:
		if writable && !sticky {
			return fmt.Errorf("%w: ancestor %s is group/world-writable and not sticky", ErrUnsafeRoot, path)
		}
		return nil
	default:
		return fmt.Errorf("%w: ancestor %s is owned by uid %d, not the current user uid %d", ErrUnsafeRoot, path, owner, uid)
	}
}

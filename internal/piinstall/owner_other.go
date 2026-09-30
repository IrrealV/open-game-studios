//go:build !linux

package piinstall

import "os"

// platformOwnerResolver fails closed on platforms where numeric ownership is
// not implemented here. This package targets Linux/WSL first; the guarded
// syscall.Stat_t helper must not be referenced unguarded elsewhere.
func platformOwnerResolver(_ string, _ os.FileInfo) (uint32, bool) {
	return 0, false
}

// platformCurrentUserID fails closed on unsupported platforms.
func platformCurrentUserID() (uint32, bool) {
	return 0, false
}

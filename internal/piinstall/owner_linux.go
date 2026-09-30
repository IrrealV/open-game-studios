//go:build linux

package piinstall

import (
	"os"
	"syscall"
)

// platformOwnerResolver reads the numeric owner uid from Linux stat metadata.
func platformOwnerResolver(_ string, info os.FileInfo) (uint32, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return stat.Uid, true
}

// platformCurrentUserID returns the effective uid of this process.
func platformCurrentUserID() (uint32, bool) {
	return uint32(os.Geteuid()), true
}

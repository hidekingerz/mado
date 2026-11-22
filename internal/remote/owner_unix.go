//go:build !windows

package remote

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// processRunning reports whether a process with this pid exists.
// Signal 0 checks without delivering anything; only "no such process"
// says it is gone — a process we may not signal is still running.
func processRunning(pid int) bool {
	return !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

// ownedByUs reports whether info describes something owned by the user
// running this process. Ownership is what keeps another local user from
// preparing the socket directory, or planting a socket in it, before we
// get there.
func ownedByUs(info fs.FileInfo) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("cannot read the owner of %s", info.Name())
	}
	return int(stat.Uid) == os.Getuid(), nil
}

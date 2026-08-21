//go:build !windows

package remote

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

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

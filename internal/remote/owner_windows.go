//go:build windows

package remote

import "io/fs"

// ownedByUs is a Unix ownership check. Windows keeps its temp directory
// per-user and guards it with ACLs rather than a uid, so there is
// nothing to compare here.
func ownedByUs(fs.FileInfo) (bool, error) { return true, nil }

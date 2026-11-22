//go:build !windows

package remote

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// leaveSocket binds a socket at path and closes it without removing
// the file, which is what an instance killed before it could clean
// up leaves behind.
func leaveSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
}

// deadPid returns the pid of a process that has exited.
func deadPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestPruneRemovesSocketsOfExitedInstances(t *testing.T) {
	dir := t.TempDir()
	dead := filepath.Join(dir, fmt.Sprintf("%d.sock", deadPid(t)))
	leaveSocket(t, dead)
	alive := filepath.Join(dir, fmt.Sprintf("%d.sock", os.Getpid()))
	listen(t, alive)
	// Not named after a pid: set by MADO_SOCKET, not ours to judge.
	named := filepath.Join(dir, "herdr.sock")
	leaveSocket(t, named)
	// Named like a socket but not one.
	plain := filepath.Join(dir, fmt.Sprintf("%d.sock", deadPid(t)))
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	Prune(dir)

	if exists(dead) {
		t.Error("the socket of an exited instance is still there")
	}
	if !exists(alive) {
		t.Error("the socket of a running instance was removed")
	}
	if !exists(named) {
		t.Error("a socket not named after a pid was removed")
	}
	if !exists(plain) {
		t.Error("a file that is not a socket was removed")
	}
}

func TestPruneMissingDir(t *testing.T) {
	Prune(filepath.Join(t.TempDir(), "absent")) // must not panic or create it
}

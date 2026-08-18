package remote

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testWait = 5 * time.Second

func listen(t *testing.T, path string) *Server {
	t.Helper()
	s, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen(%s): %v", path, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// answer serves one request with the given outcome and hands the
// request back so the test can inspect what arrived.
func answer(t *testing.T, s *Server, outcome error) <-chan *Request {
	t.Helper()
	got := make(chan *Request, 1)
	go func() {
		req, ok := s.Next()
		if !ok {
			close(got)
			return
		}
		req.Respond(outcome)
		got <- req
	}()
	return got
}

func TestSendDeliversTheRequest(t *testing.T) {
	dir := t.TempDir()
	s := listen(t, filepath.Join(dir, "1.sock"))
	got := answer(t, s, nil)

	if err := Send(dir, CmdOpen, "/tmp/a.md"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case req := <-got:
		if req == nil {
			t.Fatal("server closed before the request arrived")
		}
		if req.Cmd != CmdOpen || req.Path != "/tmp/a.md" {
			t.Errorf("request = %+v, want open /tmp/a.md", req)
		}
	case <-time.After(testWait):
		t.Fatal("request never reached the server")
	}
}

func TestSendReturnsTheHandlerError(t *testing.T) {
	dir := t.TempDir()
	s := listen(t, filepath.Join(dir, "1.sock"))
	answer(t, s, errors.New("/tmp/a.md is not open"))

	err := Send(dir, CmdFocus, "/tmp/a.md")
	if err == nil {
		t.Fatal("expected the handler's error")
	}
	if err.Error() != "/tmp/a.md is not open" {
		t.Errorf("error = %q, want the handler's message", err)
	}
}

func TestSendWithoutAnInstance(t *testing.T) {
	if err := Send(t.TempDir(), CmdOpen, "/tmp/a.md"); !errors.Is(err, ErrNoInstance) {
		t.Errorf("Send = %v, want ErrNoInstance", err)
	}
}

func TestSendWithoutASocketDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "never-created")
	if err := Send(dir, CmdOpen, "/tmp/a.md"); !errors.Is(err, ErrNoInstance) {
		t.Errorf("Send = %v, want ErrNoInstance", err)
	}
}

// A crashed instance leaves its socket file behind. It must not stop
// the search, and it should not be left to slow down the next one.
func TestStaleSocketIsSkippedAndRemoved(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "999.sock")
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := listen(t, filepath.Join(dir, "1.sock"))
	answer(t, s, nil)
	// The stale entry sorts first, so it is tried first.
	if err := os.Chtimes(stale, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := Send(dir, CmdOpen, "/tmp/a.md"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale socket still present (stat err = %v)", err)
	}
}

func TestNewestInstanceWins(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "1.sock")
	newPath := filepath.Join(dir, "2.sock")
	old := listen(t, oldPath)
	recent := listen(t, newPath)

	now := time.Now()
	if err := os.Chtimes(oldPath, now, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, now, now); err != nil {
		t.Fatal(err)
	}

	gotOld := answer(t, old, nil)
	gotNew := answer(t, recent, nil)
	if err := Send(dir, CmdOpen, "/tmp/a.md"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case <-gotNew:
	case <-gotOld:
		t.Fatal("the older instance answered")
	case <-time.After(testWait):
		t.Fatal("nobody answered")
	}
}

func TestEnvSocketPinsTheInstance(t *testing.T) {
	dir := t.TempDir()
	other := listen(t, filepath.Join(dir, "1.sock"))
	pinnedPath := filepath.Join(t.TempDir(), "pinned.sock")
	pinned := listen(t, pinnedPath)
	t.Setenv(EnvSocket, pinnedPath)

	gotOther := answer(t, other, nil)
	gotPinned := answer(t, pinned, nil)
	if err := Send(dir, CmdOpen, "/tmp/a.md"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case <-gotPinned:
	case <-gotOther:
		t.Fatal("MADO_SOCKET was ignored")
	case <-time.After(testWait):
		t.Fatal("nobody answered")
	}
}

func TestListenReplacesAnAbandonedSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := listen(t, path)
	answer(t, s, nil)
	if err := Send(filepath.Dir(path), CmdOpen, "/tmp/a.md"); err != nil {
		t.Fatalf("Send after replacing the abandoned socket: %v", err)
	}
}

func TestCloseRemovesTheSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1.sock")
	s := listen(t, path)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("socket not created: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("socket left behind (stat err = %v)", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNextReportsClosed(t *testing.T) {
	s := listen(t, filepath.Join(t.TempDir(), "1.sock"))
	done := make(chan bool, 1)
	go func() {
		_, ok := s.Next()
		done <- ok
	}()
	s.Close()
	select {
	case ok := <-done:
		if ok {
			t.Error("Next reported a request after Close")
		}
	case <-time.After(testWait):
		t.Fatal("Next did not return after Close")
	}
}

func TestDefaultPathFollowsEnvSocket(t *testing.T) {
	t.Setenv(EnvSocket, "/run/custom/mado.sock")
	if got := DefaultPath(); got != "/run/custom/mado.sock" {
		t.Errorf("DefaultPath = %q, want the MADO_SOCKET value", got)
	}
}

func TestDefaultDirPrefersXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4242")
	if got, want := DefaultDir(), filepath.Join("/run/user/4242", "mado"); got != want {
		t.Errorf("DefaultDir = %q, want %q", got, want)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := DefaultDir(); !filepath.IsAbs(got) || got == "mado" {
		t.Errorf("DefaultDir without XDG_RUNTIME_DIR = %q, want a path under the temp dir", got)
	}
}

func TestDefaultPathIsPerProcess(t *testing.T) {
	t.Setenv(EnvSocket, "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	got := DefaultPath()
	if filepath.Dir(got) != DefaultDir() {
		t.Errorf("DefaultPath %q is not inside DefaultDir %q", got, DefaultDir())
	}
	if filepath.Ext(got) != ".sock" {
		t.Errorf("DefaultPath = %q, want a .sock file", got)
	}
}

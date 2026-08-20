package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// debounce kept short so tests stay fast; waits are generous so that a
// slow CI machine does not turn timing into flakiness.
const (
	testDebounce = 20 * time.Millisecond
	waitFor      = 3 * time.Second
	quietFor     = 300 * time.Millisecond
)

func newTestWatcher(t *testing.T, dirs ...string) *Watcher {
	t.Helper()
	w, err := New(testDebounce)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	w.SetDirs(dirs)
	return w
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectEvent(t *testing.T, w *Watcher, msg string) {
	t.Helper()
	select {
	case _, ok := <-w.Events():
		if !ok {
			t.Fatalf("%s: events channel closed", msg)
		}
	case <-time.After(waitFor):
		t.Fatalf("%s: no event within %s", msg, waitFor)
	}
}

func expectQuiet(t *testing.T, w *Watcher, msg string) {
	t.Helper()
	select {
	case <-w.Events():
		t.Fatalf("%s: unexpected event", msg)
	case <-time.After(quietFor):
	}
}

func TestReportsWriteInWatchedDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	write(t, path, "# before")

	w := newTestWatcher(t, dir)
	write(t, path, "# after")
	expectEvent(t, w, "write to watched file")
}

func TestReportsNewFileInWatchedDir(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)
	write(t, filepath.Join(dir, "new.md"), "# new")
	expectEvent(t, w, "file created in watched dir")
}

// A save that writes a temporary file and renames it over the original
// is how editors — and agents — usually write. Watching the directory,
// not the file, is what makes this visible.
func TestReportsAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	write(t, path, "# before")

	w := newTestWatcher(t, dir)
	tmp := filepath.Join(dir, ".a.md.tmp")
	write(t, tmp, "# after")
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	expectEvent(t, w, "atomic replace")
}

func TestCoalescesBurst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	write(t, path, "0")

	w := newTestWatcher(t, dir)
	for i := 0; i < 20; i++ {
		write(t, path, string(rune('a'+i)))
	}
	expectEvent(t, w, "burst")
	// The whole burst should have arrived within one debounce window,
	// leaving nothing behind it.
	expectQuiet(t, w, "burst should report once")
}

func TestUnwatchedDirIsIgnored(t *testing.T) {
	watched := t.TempDir()
	other := t.TempDir()
	w := newTestWatcher(t, watched)
	write(t, filepath.Join(other, "a.md"), "# a")
	expectQuiet(t, w, "change outside the watched set")
}

func TestSetDirsReplacesTheWatchedSet(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	w := newTestWatcher(t, first)

	w.SetDirs([]string{second})
	write(t, filepath.Join(first, "a.md"), "# a")
	expectQuiet(t, w, "dropped directory")

	write(t, filepath.Join(second, "b.md"), "# b")
	expectEvent(t, w, "newly added directory")
}

func TestSetDirsSkipsUnwatchableDirs(t *testing.T) {
	dir := t.TempDir()
	// A path that does not exist must not stop the real one from being
	// watched: a partial watch beats none.
	w := newTestWatcher(t, filepath.Join(dir, "gone"), dir)
	write(t, filepath.Join(dir, "a.md"), "# a")
	expectEvent(t, w, "watchable dir alongside an unwatchable one")
}

func TestCloseClosesEvents(t *testing.T) {
	w, err := New(testDebounce)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case _, ok := <-w.Events():
		if ok {
			t.Fatal("expected the channel to be closed, got a value")
		}
	case <-time.After(waitFor):
		t.Fatal("events channel was not closed")
	}
}

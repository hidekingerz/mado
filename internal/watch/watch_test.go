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
	// testDebounce is long enough that a burst of writes lands inside
	// one window even on a slow CI runner under the race detector,
	// where 20 writes have been seen to take more than 20 ms.
	testDebounce = 100 * time.Millisecond
	waitFor      = 3 * time.Second
	quietFor     = 500 * time.Millisecond
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

// A burst of writes is reported after it goes quiet, not during it:
// the writes come closer together than the debounce, so no report may
// arrive until the last one is in. How many reports follow depends on
// how the filesystem delivers its events — on a loaded CI runner they
// can arrive in two clumps with a pause between — so the test asserts
// nothing about the count.
//
// The test can only say "during the burst" while its own writes keep
// to that spacing. A CI runner sometimes freezes the whole process for
// longer than the debounce; the watcher then rightly sees the quiet it
// waits for and reports, between two writes of what was meant to be
// one burst. So the gaps between writes are measured, and a report
// that follows a stalled gap discards the attempt instead of failing
// the test.
func TestCoalescesBurst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	write(t, path, "0")

	w := newTestWatcher(t, dir)
	// Spread the burst over a few debounce windows, each write well
	// inside the window after the previous one.
	const (
		step     = testDebounce / 4
		stalled  = testDebounce / 2
		attempts = 5
	)
	for attempt := 1; ; attempt++ {
		reportedAt, longest := writeBurst(t, w, path, step)
		if reportedAt < 0 {
			break
		}
		if longest < stalled {
			t.Fatalf("reported during the burst, after write %d (longest gap between writes %v, debounce %v)",
				reportedAt, longest, testDebounce)
		}
		if attempt == attempts {
			t.Fatalf("no burst in %d attempts kept its writes within %v of each other; last gap %v",
				attempts, stalled, longest)
		}
		t.Logf("attempt %d: writes stalled for %v, longer than the burst allows; trying again", attempt, longest)
		// Let the discarded burst report and go quiet before the next.
		drain(w)
	}
	expectEvent(t, w, "burst")
}

// writeBurst writes to path 12 times, step apart. It returns the write
// after which the watcher reported, or -1 when it stayed quiet through
// the burst, and the longest time the test let pass between two writes
// up to that point.
func writeBurst(t *testing.T, w *Watcher, path string, step time.Duration) (reportedAt int, longest time.Duration) {
	t.Helper()
	var last time.Time
	gap := func() {
		if d := time.Since(last); !last.IsZero() && d > longest {
			longest = d
		}
	}
	for i := 0; i < 12; i++ {
		gap()
		last = time.Now()
		write(t, path, string(rune('a'+i)))
		select {
		case <-w.Events():
			gap()
			return i, longest
		case <-time.After(step):
		}
	}
	return -1, longest
}

// drain waits until the watcher has been quiet for quietFor, taking
// whatever it reports meanwhile.
func drain(w *Watcher) {
	for {
		select {
		case <-w.Events():
		case <-time.After(quietFor):
			return
		}
	}
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

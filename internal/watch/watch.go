// Package watch reports debounced filesystem changes for the files and
// directories mado is currently displaying.
package watch

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is how long changes are coalesced before a single
// event is reported. Editors and agents write files in bursts — a save
// is often a truncate, a write and a rename — and without this every
// one of them would trigger its own reload.
const DefaultDebounce = 200 * time.Millisecond

// Watcher reports "something under the watched directories changed" on
// a single channel, coalescing bursts of filesystem events.
//
// Directories are watched rather than individual files: editors and
// agents commonly save by writing a temporary file and renaming it over
// the original, which detaches a watch registered on the file itself.
type Watcher struct {
	fsw      *fsnotify.Watcher
	events   chan struct{}
	debounce time.Duration

	mu   sync.Mutex
	dirs map[string]bool
}

// New starts a watcher with no directories registered. Call SetDirs to
// tell it what to watch. Close stops it.
func New(debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if debounce <= 0 {
		debounce = DefaultDebounce
	}
	w := &Watcher{
		fsw:      fsw,
		events:   make(chan struct{}, 1),
		debounce: debounce,
		dirs:     map[string]bool{},
	}
	go w.run()
	return w, nil
}

// Events yields one value per burst of filesystem activity. It is
// closed when the watcher is closed.
func (w *Watcher) Events() <-chan struct{} { return w.events }

// SetDirs replaces the watched set with dirs. Directories that cannot
// be watched (removed, or over the per-user inotify limit) are skipped:
// a partial watch is more useful than none.
func (w *Watcher) SetDirs(dirs []string) {
	want := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		want[d] = true
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	for d := range w.dirs {
		if !want[d] {
			_ = w.fsw.Remove(d)
			delete(w.dirs, d)
		}
	}
	for d := range want {
		if w.dirs[d] {
			continue
		}
		if err := w.fsw.Add(d); err != nil {
			continue
		}
		w.dirs[d] = true
	}
}

// Close stops watching and closes the event channel.
func (w *Watcher) Close() error { return w.fsw.Close() }

func (w *Watcher) run() {
	defer close(w.events)

	// timer is nil until the first event; from then on it is reset by
	// every event so that a burst reports once, once it goes quiet.
	// Go 1.23+ timer channels never hold a stale value after Reset.
	var timer *time.Timer
	var pending <-chan time.Time

	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod {
				// Permission and timestamp touches change no content.
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.debounce)
			} else {
				timer.Reset(w.debounce)
			}
			pending = timer.C
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// A dropped event just means one missed auto-reload; the
			// manual reload key is still there.
		case <-pending:
			pending = nil
			select {
			case w.events <- struct{}{}:
			default:
				// A change is already queued; the reload it triggers
				// re-reads everything anyway.
			}
		}
	}
}

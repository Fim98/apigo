package collection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testDebounce is short enough to keep tests fast and long enough to coalesce
// the create+write burst an editor produces.
const testDebounce = 20 * time.Millisecond

// settleWindow is how long a test waits to prove that no further event arrives.
// Absence can only be tested against a window; it stays well above the debounce
// so a duplicate would have been delivered.
const settleWindow = 150 * time.Millisecond

func newTestWatcher(t *testing.T, root string, opts WatcherOptions) *Watcher {
	t.Helper()
	if opts.Debounce == 0 {
		opts.Debounce = testDebounce
	}
	w, err := NewWatcher(context.Background(), root, opts)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return w
}

// nextEvent waits for the next event, failing the test on timeout.
func nextEvent(t *testing.T, w *Watcher, timeout time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-w.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a watch event")
		return Event{}
	}
}

// eventsUntil collects events until want is satisfied or the timeout expires.
// It is used instead of a fixed sleep so tests wait for the condition, not the
// clock.
func eventsUntil(t *testing.T, w *Watcher, timeout time.Duration, want func(Event) bool) (Event, bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-w.Events():
			if !ok {
				return Event{}, false
			}
			if want(ev) {
				return ev, true
			}
		case <-deadline:
			return Event{}, false
		}
	}
}

// collect gathers events for d after triggering a change.
func collect(t *testing.T, w *Watcher, d time.Duration) []Event {
	t.Helper()
	deadline := time.After(d)
	var events []Event
	for {
		select {
		case ev, ok := <-w.Events():
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			return events
		}
	}
}

func TestWatcherCreateUpdateRemove(t *testing.T) {
	root := t.TempDir()
	w := newTestWatcher(t, root, WatcherOptions{})

	path := filepath.Join(root, "request.bru")
	if err := os.WriteFile(path, []byte("get {\n  url: https://a\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := nextEvent(t, w, 3*time.Second)
	if ev.Op != OpCreate || ev.RelPath != "request.bru" || ev.IsDir {
		t.Fatalf("create event = %+v", ev)
	}
	if ev.Path != path {
		t.Fatalf("path = %q, want %q", ev.Path, path)
	}

	if err := os.WriteFile(path, []byte("get {\n  url: https://b\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev = nextEvent(t, w, 3*time.Second)
	if ev.Op != OpUpdate || ev.RelPath != "request.bru" {
		t.Fatalf("update event = %+v", ev)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ev = nextEvent(t, w, 3*time.Second)
	if ev.Op != OpRemove || ev.RelPath != "request.bru" {
		t.Fatalf("remove event = %+v", ev)
	}
}

func TestWatcherCoalescesBursts(t *testing.T) {
	root := t.TempDir()
	w := newTestWatcher(t, root, WatcherOptions{})

	path := filepath.Join(root, "burst.bru")
	// An editor's save is often create + write + chmod in quick succession.
	for i := range 5 {
		if err := os.WriteFile(path, []byte{byte('0' + i)}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	events := collect(t, w, settleWindow)
	if len(events) != 1 {
		t.Fatalf("events = %d (%+v), want exactly 1", len(events), events)
	}
	if events[0].RelPath != "burst.bru" || events[0].Op != OpCreate {
		t.Fatalf("event = %+v", events[0])
	}
}

func TestWatcherSkipsHiddenIgnoredAndNodeModules(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".hidden", "node_modules", "vendor"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := newTestWatcher(t, root, WatcherOptions{Ignore: []string{"vendor"}})

	for _, name := range []string{".hidden/a.bru", "node_modules/b.bru", "vendor/c.bru"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if events := collect(t, w, settleWindow); len(events) != 0 {
		t.Fatalf("ignored paths produced events: %+v", events)
	}

	// A real file in the same window is still reported, so the test cannot pass
	// just because the watcher is silent.
	if err := os.WriteFile(filepath.Join(root, "ok.bru"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool { return ev.RelPath == "ok.bru" })
	if !ok || ev.Op != OpCreate {
		t.Fatalf("event = %+v ok=%v", ev, ok)
	}
}

func TestWatcherSeesNewSubdirectories(t *testing.T) {
	root := t.TempDir()
	w := newTestWatcher(t, root, WatcherOptions{})

	nested := filepath.Join(root, "users", "admin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool {
		return ev.RelPath == "users" && ev.IsDir && ev.Op == OpCreate
	}); !ok {
		t.Fatal("new directory was not reported")
	}

	path := filepath.Join(nested, "list.bru")
	if err := os.WriteFile(path, []byte("get {\n  url: https://a\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool {
		return ev.RelPath == "users/admin/list.bru" && ev.Op == OpCreate
	}); !ok {
		t.Fatal("file inside a new subdirectory was never reported")
	}
}

func TestWatcherReportsRemovedDirectoryAsDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "users")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "list.bru"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := newTestWatcher(t, root, WatcherOptions{})

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	ev, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool {
		return ev.RelPath == "users" && ev.Op == OpRemove
	})
	if !ok {
		t.Fatal("directory removal was not reported")
	}
	if !ev.IsDir {
		t.Fatalf("removed directory reported as a file: %+v", ev)
	}
}

func TestWatcherReportsMovedFileAsRemove(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(root, "old.bru")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := newTestWatcher(t, root, WatcherOptions{})

	if err := os.Rename(src, filepath.Join(outside, "new.bru")); err != nil {
		t.Fatal(err)
	}
	if _, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool {
		return ev.RelPath == "old.bru" && ev.Op == OpRemove
	}); !ok {
		t.Fatal("moving a file out of the collection was not reported as a removal")
	}
}

// TestWatcherTreatsRenameIntoPlaceAsUpdate covers the common editor save: write a
// temp file, then rename it over the target. The target still exists afterwards,
// so reporting it as removed would make the app drop a live request.
func TestWatcherTreatsRenameIntoPlaceAsUpdate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "request.bru")
	w := newTestWatcher(t, root, WatcherOptions{})

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := eventsUntil(t, w, 3*time.Second, func(ev Event) bool { return ev.RelPath == "request.bru" }); !ok {
		t.Fatal("initial create was not reported")
	}

	tmp := filepath.Join(root, ".request.bru.tmp")
	if err := os.WriteFile(tmp, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	events := collect(t, w, 2*settleWindow)
	var sawTarget bool
	for _, ev := range events {
		if ev.RelPath != "request.bru" {
			continue
		}
		sawTarget = true
		if ev.Op == OpRemove {
			t.Fatalf("rename-into-place reported as removal: %+v", ev)
		}
	}
	if !sawTarget {
		t.Fatalf("rename-into-place was not reported: %+v", events)
	}
}

func TestWatcherFlagsOversizedFiles(t *testing.T) {
	root := t.TempDir()
	w := newTestWatcher(t, root, WatcherOptions{})

	big := strings.Repeat("#", MaxFileSize+1)
	if err := os.WriteFile(filepath.Join(root, "big.bru"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	ev, ok := eventsUntil(t, w, 5*time.Second, func(ev Event) bool { return ev.RelPath == "big.bru" })
	if !ok {
		t.Fatal("oversized file was not reported")
	}
	if !ev.TooLarge {
		t.Fatalf("oversized file not flagged: %+v", ev)
	}

	if err := os.WriteFile(filepath.Join(root, "small.bru"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev, ok = eventsUntil(t, w, 3*time.Second, func(ev Event) bool { return ev.RelPath == "small.bru" })
	if !ok {
		t.Fatal("small file was not reported")
	}
	if ev.TooLarge {
		t.Fatalf("small file flagged as oversized: %+v", ev)
	}
}

func TestWatcherCloseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	w, err := NewWatcher(context.Background(), root, WatcherOptions{Debounce: testDebounce})
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	if _, ok := <-w.Events(); ok {
		t.Fatal("events channel must be closed")
	}
	if _, ok := <-w.Errors(); ok {
		t.Fatal("errors channel must be closed")
	}
}

func TestWatcherRunStopsOnContextCancel(t *testing.T) {
	root := t.TempDir()
	w, err := NewWatcher(context.Background(), root, WatcherOptions{Debounce: testDebounce})
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}

	if _, ok := <-w.Events(); ok {
		t.Fatal("events channel must be closed after Run returns")
	}
}

func TestNewWatcherRejectsBadRoots(t *testing.T) {
	if _, err := NewWatcher(context.Background(), filepath.Join(t.TempDir(), "missing"), WatcherOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing root: %v", err)
	}

	file := filepath.Join(t.TempDir(), "file.bru")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWatcher(context.Background(), file, WatcherOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a file is not a collection root: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewWatcher(ctx, t.TempDir(), WatcherOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
}

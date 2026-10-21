package collection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testDebounce is short enough to keep tests fast and long enough to coalesce
// the create+write burst an editor produces.
const testDebounce = 20 * time.Millisecond

func newTestWatcher(t *testing.T, root string, opts WatcherOptions) *Watcher {
	t.Helper()
	if opts.Debounce == 0 {
		opts.Debounce = testDebounce
	}
	w, err := NewWatcher(root, opts)
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
	case err := <-w.Errors():
		if err != nil {
			t.Fatalf("watcher error: %v", err)
		}
		return nextEvent(t, w, timeout)
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a watch event")
		return Event{}
	}
}

// collect gathers events for d after triggering a change.
func collect(t *testing.T, w *Watcher, d time.Duration) []Event {
	t.Helper()
	deadline := time.After(d)
	var events []Event
	for {
		select {
		case ev := <-w.Events():
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
	events := collect(t, w, 300*time.Millisecond)
	if len(events) != 1 {
		t.Fatalf("events = %d (%+v), want exactly 1", len(events), events)
	}
	if events[0].RelPath != "burst.bru" {
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
	if events := collect(t, w, 250*time.Millisecond); len(events) != 0 {
		t.Fatalf("ignored paths produced events: %+v", events)
	}

	// A real file in the same window is still reported, so the test cannot pass
	// just because the watcher is silent.
	if err := os.WriteFile(filepath.Join(root, "ok.bru"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := nextEvent(t, w, 3*time.Second)
	if ev.RelPath != "ok.bru" || ev.Op != OpCreate {
		t.Fatalf("event = %+v", ev)
	}
}

func TestWatcherSeesNewSubdirectories(t *testing.T) {
	root := t.TempDir()
	w := newTestWatcher(t, root, WatcherOptions{})

	nested := filepath.Join(root, "users", "admin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// The directory creation itself is reported.
	var sawUsers bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !sawUsers {
		ev := nextEvent(t, w, 3*time.Second)
		if ev.RelPath == "users" && ev.IsDir && ev.Op == OpCreate {
			sawUsers = true
		}
	}

	path := filepath.Join(nested, "list.bru")
	if err := os.WriteFile(path, []byte("get {\n  url: https://a\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The watcher must have added a watch for the new directory before the write
	// lands. Give it a moment, then assert the file is reported.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ev := nextEvent(t, w, 3*time.Second)
		if ev.RelPath == "users/admin/list.bru" && ev.Op == OpCreate {
			return
		}
	}
	t.Fatal("file inside a new subdirectory was never reported")
}

func TestWatcherCloseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	w, err := NewWatcher(root, WatcherOptions{Debounce: testDebounce})
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
	w, err := NewWatcher(root, WatcherOptions{Debounce: testDebounce})
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
	if _, err := NewWatcher(filepath.Join(t.TempDir(), "missing"), WatcherOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing root: %v", err)
	}

	file := filepath.Join(t.TempDir(), "file.bru")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewWatcher(file, WatcherOptions{}); err == nil {
		t.Fatal("a file is not a collection root")
	}
}

package collection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Op is the kind of change a watcher reports.
type Op uint8

// Watcher operations.
const (
	// OpCreate means the path appeared.
	OpCreate Op = iota + 1
	// OpUpdate means the path's content changed.
	OpUpdate
	// OpRemove means the path disappeared.
	OpRemove
)

// String returns the operation name.
func (o Op) String() string {
	switch o {
	case OpCreate:
		return "create"
	case OpUpdate:
		return "update"
	case OpRemove:
		return "remove"
	default:
		return "unknown"
	}
}

// Event is one debounced change under the watched root.
type Event struct {
	// Path is the absolute path that changed.
	Path string
	// RelPath is the slash-separated path relative to the root.
	RelPath string
	Op      Op
	// IsDir reports whether the path is a directory.
	IsDir bool
}

// DefaultDebounce is the default window that coalesces bursts of file system
// events, such as an editor's write-then-rename.
const DefaultDebounce = 150 * time.Millisecond

// WatcherOptions configures a Watcher.
type WatcherOptions struct {
	// Debounce is the coalescing window. Zero uses DefaultDebounce.
	Debounce time.Duration
	// Ignore lists directory names to skip in addition to hidden directories
	// and node_modules. It is normally Config.Ignore.
	Ignore []string
}

// Watcher reports external changes under a collection root.
//
// A Watcher owns one goroutine. Events are delivered on Events; a consumer that
// stops reading blocks the watcher, so drain the channel or cancel the context.
// Close stops watching and closes both channels exactly once.
type Watcher struct {
	root     string
	debounce time.Duration
	ignore   map[string]bool

	fsw   *fsnotify.Watcher
	due   chan string
	event chan Event
	errs  chan error

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// pendingEvent is a change waiting for its debounce window to expire.
type pendingEvent struct {
	path  string
	op    Op
	isDir bool
}

// NewWatcher starts watching root and its subdirectories.
func NewWatcher(root string, opts WatcherOptions) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("collection watcher: root is not a directory")
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = DefaultDebounce
	}
	ignore := map[string]bool{}
	for _, name := range opts.Ignore {
		ignore[name] = true
	}

	w := &Watcher{
		root:     abs,
		debounce: debounce,
		ignore:   ignore,
		fsw:      fsw,
		due:      make(chan string, 64),
		event:    make(chan Event, 64),
		errs:     make(chan error, 8),
		done:     make(chan struct{}),
	}

	if err := w.watchTree(abs); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w.wg.Add(1)
	go w.loop()
	return w, nil
}

// Root returns the absolute watched root.
func (w *Watcher) Root() string { return w.root }

// Events returns the debounced change stream. It is closed by Close.
func (w *Watcher) Events() <-chan Event { return w.event }

// Errors returns watcher-level failures. It is closed by Close.
func (w *Watcher) Errors() <-chan error { return w.errs }

// Close stops watching and closes the event and error channels.
func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.fsw.Close()
	})
	w.wg.Wait()
	return err
}

// Run blocks until ctx is cancelled, then closes the watcher.
func (w *Watcher) Run(ctx context.Context) error {
	<-ctx.Done()
	return w.Close()
}

// watchTree adds a recursive watch for every collection directory under root.
func (w *Watcher) watchTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && w.skipDir(entry.Name()) {
			return filepath.SkipDir
		}
		return w.fsw.Add(path)
	})
}

// skipDir reports whether a directory is outside the collection.
func (w *Watcher) skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || w.ignore[name]
}

// relative converts an absolute path into a slash-separated relative path and
// reports whether it belongs to the collection.
func (w *Watcher) relative(path string) (string, bool) {
	rel, err := filepath.Rel(w.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", false
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") || part == "node_modules" || w.ignore[part] {
			return "", false
		}
	}
	return rel, true
}

// loop multiplexes raw events, debounce expiries and shutdown. All maps are
// owned by this goroutine; timer callbacks only send a path.
func (w *Watcher) loop() {
	defer w.wg.Done()
	defer close(w.event)
	defer close(w.errs)

	timers := map[string]*time.Timer{}
	pending := map[string]pendingEvent{}
	defer func() {
		for _, timer := range timers {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-w.done:
			return

		case raw, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleRaw(raw, pending, timers)

		case raw, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			if raw == nil {
				continue
			}
			select {
			case w.errs <- raw:
			case <-w.done:
				return
			}

		case path := <-w.due:
			due, ok := pending[path]
			if !ok {
				continue
			}
			delete(pending, path)
			delete(timers, path)
			rel, ok := w.relative(due.path)
			if !ok {
				continue
			}
			select {
			case w.event <- Event{Path: due.path, RelPath: rel, Op: due.op, IsDir: due.isDir}:
			case <-w.done:
				return
			}
		}
	}
}

// handleRaw maps a raw fsnotify event to a pending change and arms its timer.
func (w *Watcher) handleRaw(raw fsnotify.Event, pending map[string]pendingEvent, timers map[string]*time.Timer) {
	if raw.Name == "" {
		return
	}
	if _, ok := w.relative(raw.Name); !ok {
		return
	}

	info, statErr := os.Stat(raw.Name)
	isDir := statErr == nil && info.IsDir()

	op := OpUpdate
	switch {
	case raw.Op.Has(fsnotify.Remove):
		op = OpRemove
	case raw.Op.Has(fsnotify.Rename):
		// Editors save by renaming a temp file over the target; the create
		// follows as a separate event.
		op = OpRemove
	case raw.Op.Has(fsnotify.Create):
		op = OpCreate
	case raw.Op.Has(fsnotify.Write):
		op = OpUpdate
	case raw.Op.Has(fsnotify.Chmod):
		// Permission-only churn is noise for the collection model.
		return
	}

	if isDir {
		switch op {
		case OpCreate:
			if err := w.watchTree(raw.Name); err != nil {
				select {
				case w.errs <- err:
				case <-w.done:
				}
			}
		case OpRemove:
			// fsnotify drops the watch automatically when the directory dies.
			_ = w.fsw.Remove(raw.Name)
		}
	}

	merged := pending[raw.Name]
	merged.path = raw.Name
	merged.isDir = isDir
	switch {
	case op == OpRemove:
		merged.op = OpRemove
	case merged.op == 0:
		merged.op = op
	case merged.op == OpUpdate && op == OpCreate:
		merged.op = OpCreate
	}
	pending[raw.Name] = merged

	path := raw.Name
	if timer, ok := timers[path]; ok {
		timer.Reset(w.debounce)
		return
	}
	timers[path] = time.AfterFunc(w.debounce, func() {
		select {
		case w.due <- path:
		case <-w.done:
		}
	})
}

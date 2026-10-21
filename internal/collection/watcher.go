package collection

import (
	"context"
	"errors"
	"fmt"
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
	// OpUpdate means the path's content changed or was replaced in place.
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
	// IsDir reports whether the path is a directory. It is preserved for a
	// removed directory, so a consumer can invalidate the whole subtree.
	IsDir bool
	// TooLarge reports that the file exceeds MaxFileSize and must not be parsed
	// automatically (Bruno's watcher cap).
	TooLarge bool
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
// A Watcher owns one goroutine. Events and errors are buffered, but a consumer
// that stops reading eventually blocks the watcher, so drain both or close the
// watcher. Close stops watching and closes both channels exactly once.
type Watcher struct {
	root     string
	debounce time.Duration
	ignore   map[string]bool

	fsw   *fsnotify.Watcher
	due   chan dueKey
	event chan Event
	errs  chan error

	// dirs holds the absolute paths currently watched as directories. It is
	// built during setup, before the loop starts, and owned by the loop after.
	dirs map[string]bool

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// dueKey identifies one debounce expiry. The generation guards against a timer
// that fired just as a newer change arrived.
type dueKey struct {
	path string
	gen  uint64
}

// pendingEvent is a change waiting for its debounce window to expire.
type pendingEvent struct {
	path     string
	op       Op
	isDir    bool
	tooLarge bool
	gen      uint64
}

// NewWatcher starts watching root and its subdirectories. The context bounds the
// initial scan; cancelling it later does not stop the watcher, Close or Run do.
func NewWatcher(ctx context.Context, root string, opts WatcherOptions) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, abs)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrNotFound, abs)
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
		due:      make(chan dueKey, 64),
		event:    make(chan Event, 64),
		errs:     make(chan error, 8),
		dirs:     map[string]bool{},
		done:     make(chan struct{}),
	}

	if err := w.watchTree(ctx, abs); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w.wg.Add(1)
	go w.loop()
	return w, nil
}

// Events returns the debounced change stream. It is closed by Close.
func (w *Watcher) Events() <-chan Event { return w.event }

// Errors returns watcher-level failures, for example a directory that could not
// be watched. It is closed by Close.
func (w *Watcher) Errors() <-chan error { return w.errs }

// Close stops watching and closes the event and error channels. It is safe to
// call more than once.
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

// errWatcherClosed stops a directory walk once the watcher is closing.
var errWatcherClosed = errors.New("collection watcher: closed")

// watchTree adds a recursive watch for every collection directory under root.
func (w *Watcher) watchTree(ctx context.Context, root string) error {
	return w.addTree(root, ctx.Err)
}

// addTree walks root and watches every collection directory, stopping when stop
// returns an error or the watcher closes.
func (w *Watcher) addTree(root string, stop func() error) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-w.done:
			return errWatcherClosed
		default:
		}
		if stop != nil {
			if serr := stop(); serr != nil {
				return serr
			}
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && w.skipDir(entry.Name()) {
			return filepath.SkipDir
		}
		if err := w.fsw.Add(path); err != nil {
			return err
		}
		w.dirs[path] = true
		return nil
	})
}

// skipDir reports whether a directory is outside the collection.
func (w *Watcher) skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || w.ignore[name]
}

// relative converts an absolute path into a slash-separated relative path and
// reports whether it belongs to the collection. Directory components follow the
// loader's ignore rules; the final component is always kept, because the loader
// ignores directories, not files.
func (w *Watcher) relative(path string) (string, bool) {
	rel, err := filepath.Rel(w.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", false
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts[:len(parts)-1] {
		if strings.HasPrefix(part, ".") || part == "node_modules" || w.ignore[part] {
			return "", false
		}
	}
	return rel, true
}

// loop multiplexes raw events, debounce expiries and shutdown. It owns the
// pending and timer maps; timer callbacks only send a due key.
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

		case key := <-w.due:
			due, ok := pending[key.path]
			if !ok || due.gen != key.gen {
				// A newer change already re-armed the timer for this path.
				continue
			}
			delete(pending, key.path)
			if timer, ok := timers[key.path]; ok {
				timer.Stop()
				delete(timers, key.path)
			}
			rel, ok := w.relative(due.path)
			if !ok {
				continue
			}
			select {
			case w.event <- Event{
				Path:     due.path,
				RelPath:  rel,
				Op:       due.op,
				IsDir:    due.isDir,
				TooLarge: due.tooLarge,
			}:
			case <-w.done:
				return
			}
		}
	}
}

// handleRaw maps one raw fsnotify event to a pending change and arms its timer.
func (w *Watcher) handleRaw(raw fsnotify.Event, pending map[string]pendingEvent, timers map[string]*time.Timer) {
	if raw.Name == "" {
		return
	}
	if _, ok := w.relative(raw.Name); !ok {
		return
	}

	info, statErr := os.Stat(raw.Name)
	exists := statErr == nil
	isDir := exists && info.IsDir()

	op := OpUpdate
	switch {
	case raw.Op.Has(fsnotify.Remove):
		op = OpRemove
	case raw.Op.Has(fsnotify.Rename):
		// A rename of the watched name usually means it moved away, but editors
		// also save by renaming a temp file over the target. Stat decides which.
		if exists {
			op = OpUpdate
		} else {
			op = OpRemove
		}
	case raw.Op.Has(fsnotify.Create):
		op = OpCreate
	case raw.Op.Has(fsnotify.Write):
		op = OpUpdate
	case raw.Op.Has(fsnotify.Chmod):
		// Permission-only churn is noise for the collection model.
		return
	}

	// A removed directory cannot be stat'ed; the watch list still knows it was
	// one, so the consumer can invalidate the subtree.
	if op == OpRemove && w.dirs[raw.Name] {
		isDir = true
		delete(w.dirs, raw.Name)
	}

	if isDir && op == OpCreate {
		w.dirs[raw.Name] = true
		if err := w.addTree(raw.Name, nil); err != nil {
			select {
			case w.errs <- err:
			case <-w.done:
			}
		}
	}

	prev, hadPrev := pending[raw.Name]
	gen := prev.gen + 1
	if !hadPrev {
		gen = 1
	}
	pending[raw.Name] = pendingEvent{
		path:     raw.Name,
		op:       mergeOp(prev.op, op, hadPrev),
		isDir:    isDir || prev.isDir,
		tooLarge: exists && info.Size() > MaxFileSize,
		gen:      gen,
	}

	if timer, ok := timers[raw.Name]; ok {
		timer.Stop()
	}
	timers[raw.Name] = time.AfterFunc(w.debounce, func() {
		select {
		case w.due <- dueKey{path: raw.Name, gen: gen}:
		case <-w.done:
		}
	})
}

// mergeOp folds a burst of raw operations into the one that best describes the
// net effect: a removal wins unless the path came back, and a creation keeps its
// identity through the writes that follow.
func mergeOp(prev, next Op, hadPrev bool) Op {
	if !hadPrev {
		return next
	}
	switch {
	case prev == OpRemove && next == OpCreate:
		return OpCreate
	case next == OpRemove:
		return OpRemove
	case prev == OpCreate:
		return OpCreate
	default:
		return next
	}
}

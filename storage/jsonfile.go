package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"jabberwocky238/jw238dns/types"

	"github.com/fsnotify/fsnotify"
)

// JSONFileLoader loads DNS records from a JSON file and watches it for
// changes. It supports bidirectional sync: file changes are applied to
// the MemoryStorage, and storage changes can be persisted back to the file.
type JSONFileLoader struct {
	path  string
	store *MemoryStorage

	mu      sync.Mutex
	syncing bool // guards against echo loops during bidirectional sync
}

// NewJSONFileLoader creates a JSONFileLoader for the given file path.
func NewJSONFileLoader(path string, store *MemoryStorage) *JSONFileLoader {
	return &JSONFileLoader{
		path:  path,
		store: store,
	}
}

// Load reads the JSON file and returns the parsed DNS records. If the
// file does not exist it returns an empty slice and no error.
func (l *JSONFileLoader) Load() ([]*types.DNSRecord, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("json file not found, starting empty", "path", l.path)
			return nil, nil
		}
		return nil, fmt.Errorf("read json file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	return records, nil
}

// LoadAndApply reads the JSON file and applies the records to the store
// using CalculateChanges + PartialReload.
func (l *JSONFileLoader) LoadAndApply(ctx context.Context) error {
	records, err := l.Load()
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.syncing = true
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.syncing = false
		l.mu.Unlock()
	}()

	changes := l.store.CalculateChanges(records)
	if len(changes.Added) == 0 && len(changes.Updated) == 0 && len(changes.Deleted) == 0 {
		return nil
	}

	return l.store.PartialReload(ctx, changes)
}

// Watch uses fsnotify to watch the JSON file for changes. On each write
// event it reloads the file and applies changes to the store. It blocks
// until the context is cancelled.
func (l *JSONFileLoader) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	// Watch the directory so we catch atomic rename-based writes.
	dir := filepath.Dir(l.path)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch directory %s: %w", dir, err)
	}

	// Debounce timer to coalesce rapid writes.
	var debounce *time.Timer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Only react to writes/creates for our specific file.
			absEvent, _ := filepath.Abs(event.Name)
			absPath, _ := filepath.Abs(l.path)
			if absEvent != absPath {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce: wait a short period before reloading.
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(100*time.Millisecond, func() {
				if err := l.LoadAndApply(ctx); err != nil {
					slog.Error("reload json file", "err", err)
				}
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("fsnotify error", "err", err)
		}
	}
}

// Save writes the current storage contents to the JSON file using an
// atomic write (write to temp file, then rename).
func (l *JSONFileLoader) Save(ctx context.Context) error {
	l.mu.Lock()
	if l.syncing {
		l.mu.Unlock()
		return nil // skip echo
	}
	l.mu.Unlock()

	records, err := l.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	// If records is nil, marshal as empty array.
	if records == nil {
		records = []*types.DNSRecord{}
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: write to temp file in the same directory, then rename.
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".jw238dns-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, l.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}

	slog.Info("persisted records to json file", "path", l.path, "records", len(records))
	return nil
}

// WatchAndSync starts both the file watcher and a goroutine that listens
// on the storage Watch channel to persist changes back to the JSON file.
// It blocks until ctx is cancelled.
func (l *JSONFileLoader) WatchAndSync(ctx context.Context) error {
	ch, err := l.store.Watch(ctx)
	if err != nil {
		return fmt.Errorf("watch storage: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				if err := l.Save(ctx); err != nil {
					slog.Error("persist to json file", "err", err)
				}
			}
		}
	}()

	return l.Watch(ctx)
}

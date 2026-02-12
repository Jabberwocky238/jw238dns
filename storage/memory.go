package storage

import (
	"context"
	"log/slog"
	"sync"

	"jabberwocky238/jw238dns/types"
)

// MemoryStorage is a thread-safe in-memory implementation of CoreStorage.
type MemoryStorage struct {
	mu       sync.RWMutex
	records  map[string]map[types.RecordType][]*types.DNSRecord // domain -> type -> records
	version  uint64
	watchers []chan types.StorageEvent
	watchMu  sync.Mutex
}

// NewMemoryStorage creates a new empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		records: make(map[string]map[types.RecordType][]*types.DNSRecord),
	}
}

// Get returns all records matching the given name and type.
func (s *MemoryStorage) Get(_ context.Context, name string, recordType types.RecordType) ([]*types.DNSRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byType, ok := s.records[name]
	if !ok {
		return nil, types.ErrRecordNotFound
	}

	recs, ok := byType[recordType]
	if !ok || len(recs) == 0 {
		return nil, types.ErrRecordNotFound
	}

	// Return a copy to avoid data races on the slice.
	out := make([]*types.DNSRecord, len(recs))
	copy(out, recs)
	return out, nil
}

// List returns all stored DNS records.
func (s *MemoryStorage) List(_ context.Context) ([]*types.DNSRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*types.DNSRecord
	for _, byType := range s.records {
		for _, recs := range byType {
			all = append(all, recs...)
		}
	}
	return all, nil
}

// Create adds a new DNS record to storage. Returns ErrRecordExists if a
// record with the same name and type already exists.
func (s *MemoryStorage) Create(_ context.Context, record *types.DNSRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	byType, ok := s.records[record.Name]
	if ok {
		if _, exists := byType[record.Type]; exists {
			return types.ErrRecordExists
		}
	}

	s.addRecordLocked(record)
	s.version++

	s.emit(types.StorageEvent{Type: types.EventAdded, Record: record})
	return nil
}

// Update replaces an existing DNS record. Returns ErrRecordNotFound if the
// record does not exist.
func (s *MemoryStorage) Update(_ context.Context, record *types.DNSRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	byType, ok := s.records[record.Name]
	if !ok {
		return types.ErrRecordNotFound
	}
	if _, exists := byType[record.Type]; !exists {
		return types.ErrRecordNotFound
	}

	s.updateRecordLocked(record)
	s.version++

	s.emit(types.StorageEvent{Type: types.EventUpdated, Record: record})
	return nil
}

// Delete removes a DNS record identified by name and type.
func (s *MemoryStorage) Delete(_ context.Context, name string, recordType types.RecordType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	byType, ok := s.records[name]
	if !ok {
		return types.ErrRecordNotFound
	}
	if _, exists := byType[recordType]; !exists {
		return types.ErrRecordNotFound
	}

	s.deleteRecordLocked(name, recordType)
	s.version++

	s.emit(types.StorageEvent{Type: types.EventDeleted, Record: &types.DNSRecord{Name: name, Type: recordType}})
	return nil
}

// HotReload replaces all records atomically with the provided set.
func (s *MemoryStorage) HotReload(_ context.Context, records []*types.DNSRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = make(map[string]map[types.RecordType][]*types.DNSRecord)
	for _, r := range records {
		s.addRecordLocked(r)
	}
	s.version++

	slog.Info("hot reload complete", "records", len(records), "version", s.version)
	s.emit(types.StorageEvent{Type: types.EventReloaded})
	return nil
}

// PartialReload applies only the changed records atomically.
func (s *MemoryStorage) PartialReload(_ context.Context, changes *types.RecordChanges) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range changes.Added {
		s.addRecordLocked(r)
	}
	for _, r := range changes.Updated {
		s.updateRecordLocked(r)
	}
	for _, key := range changes.Deleted {
		s.deleteRecordLocked(key.Name, key.Type)
	}
	s.version++

	slog.Info("partial reload complete",
		"added", len(changes.Added),
		"updated", len(changes.Updated),
		"deleted", len(changes.Deleted),
		"version", s.version,
	)
	s.emit(types.StorageEvent{Type: types.EventReloaded})
	return nil
}

// Watch returns a channel that receives storage change events. The channel
// is closed when the provided context is cancelled.
func (s *MemoryStorage) Watch(ctx context.Context) (<-chan types.StorageEvent, error) {
	ch := make(chan types.StorageEvent, 64)

	s.watchMu.Lock()
	s.watchers = append(s.watchers, ch)
	s.watchMu.Unlock()

	go func() {
		<-ctx.Done()
		s.watchMu.Lock()
		defer s.watchMu.Unlock()
		for i, w := range s.watchers {
			if w == ch {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}()

	return ch, nil
}

// Version returns the current storage version counter.
func (s *MemoryStorage) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// --- internal helpers (caller must hold s.mu write lock) ---

func (s *MemoryStorage) addRecordLocked(record *types.DNSRecord) {
	if s.records[record.Name] == nil {
		s.records[record.Name] = make(map[types.RecordType][]*types.DNSRecord)
	}
	s.records[record.Name][record.Type] = []*types.DNSRecord{record}
}

func (s *MemoryStorage) updateRecordLocked(record *types.DNSRecord) {
	if s.records[record.Name] == nil {
		s.records[record.Name] = make(map[types.RecordType][]*types.DNSRecord)
	}
	s.records[record.Name][record.Type] = []*types.DNSRecord{record}
}

func (s *MemoryStorage) deleteRecordLocked(name string, recordType types.RecordType) {
	byType, ok := s.records[name]
	if !ok {
		return
	}
	delete(byType, recordType)
	if len(byType) == 0 {
		delete(s.records, name)
	}
}

// emit sends an event to all active watchers without blocking.
func (s *MemoryStorage) emit(event types.StorageEvent) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	for _, ch := range s.watchers {
		select {
		case ch <- event:
		default:
			// Drop event if watcher is not keeping up.
		}
	}
}

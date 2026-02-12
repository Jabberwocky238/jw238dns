package storage

import (
	"jabberwocky238/jw238dns/types"
)

// CalculateChanges compares a new set of records against the current contents
// of the MemoryStorage and returns the diff as a RecordChanges value.
// The caller can then pass the result to PartialReload for an atomic update.
func (s *MemoryStorage) CalculateChanges(newRecords []*types.DNSRecord) *types.RecordChanges {
	s.mu.RLock()
	defer s.mu.RUnlock()

	changes := &types.RecordChanges{
		Added:   []*types.DNSRecord{},
		Updated: []*types.DNSRecord{},
		Deleted: []types.RecordKey{},
	}

	oldMap := s.buildRecordMapLocked()
	newMap := buildRecordMapFromSlice(newRecords)

	// Find added and updated records.
	for key, newRec := range newMap {
		if oldRec, exists := oldMap[key]; exists {
			if !recordsEqual(oldRec, newRec) {
				changes.Updated = append(changes.Updated, newRec)
			}
		} else {
			changes.Added = append(changes.Added, newRec)
		}
	}

	// Find deleted records.
	for key := range oldMap {
		if _, exists := newMap[key]; !exists {
			changes.Deleted = append(changes.Deleted, key)
		}
	}

	return changes
}

// buildRecordMapLocked returns a flat map of RecordKey -> DNSRecord from the
// current storage contents. Caller must hold at least s.mu.RLock.
func (s *MemoryStorage) buildRecordMapLocked() map[types.RecordKey]*types.DNSRecord {
	m := make(map[types.RecordKey]*types.DNSRecord)
	for name, byType := range s.records {
		for rt, recs := range byType {
			if len(recs) > 0 {
				m[types.RecordKey{Name: name, Type: rt}] = recs[0]
			}
		}
	}
	return m
}

// buildRecordMapFromSlice builds a RecordKey -> DNSRecord map from a slice.
func buildRecordMapFromSlice(records []*types.DNSRecord) map[types.RecordKey]*types.DNSRecord {
	m := make(map[types.RecordKey]*types.DNSRecord, len(records))
	for _, r := range records {
		m[types.RecordKey{Name: r.Name, Type: r.Type}] = r
	}
	return m
}

// recordsEqual reports whether two DNS records have the same TTL and values.
func recordsEqual(a, b *types.DNSRecord) bool {
	if a.TTL != b.TTL {
		return false
	}
	if len(a.Value) != len(b.Value) {
		return false
	}
	for i := range a.Value {
		if a.Value[i] != b.Value[i] {
			return false
		}
	}
	return true
}

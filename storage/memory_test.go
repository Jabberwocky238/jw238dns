package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"jabberwocky238/jw238dns/types"
)

func setupTestStorage(t *testing.T) *MemoryStorage {
	t.Helper()
	store := NewMemoryStorage()

	err := store.Create(context.Background(), &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.1.1"},
	})
	if err != nil {
		t.Fatalf("setup: failed to create A record: %v", err)
	}

	err = store.Create(context.Background(), &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeAAAA,
		TTL:   300,
		Value: []string{"2001:db8::1"},
	})
	if err != nil {
		t.Fatalf("setup: failed to create AAAA record: %v", err)
	}

	return store
}

func TestMemoryStorage_Create(t *testing.T) {
	tests := []struct {
		name    string
		record  *types.DNSRecord
		wantErr error
	}{
		{
			name: "create new A record",
			record: &types.DNSRecord{
				Name:  "new.example.com.",
				Type:  types.RecordTypeA,
				TTL:   300,
				Value: []string{"10.0.0.1"},
			},
			wantErr: nil,
		},
		{
			name: "create duplicate A record",
			record: &types.DNSRecord{
				Name:  "example.com.",
				Type:  types.RecordTypeA,
				TTL:   300,
				Value: []string{"10.0.0.2"},
			},
			wantErr: types.ErrRecordExists,
		},
		{
			name: "create TXT record for existing domain",
			record: &types.DNSRecord{
				Name:  "example.com.",
				Type:  types.RecordTypeTXT,
				TTL:   60,
				Value: []string{"v=spf1 include:example.com ~all"},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := setupTestStorage(t)
			err := store.Create(context.Background(), tt.record)
			if err != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryStorage_Get(t *testing.T) {
	tests := []struct {
		name       string
		queryName  string
		queryType  types.RecordType
		wantErr    error
		wantValues []string
	}{
		{
			name:       "get existing A record",
			queryName:  "example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"192.168.1.1"},
		},
		{
			name:       "get existing AAAA record",
			queryName:  "example.com.",
			queryType:  types.RecordTypeAAAA,
			wantErr:    nil,
			wantValues: []string{"2001:db8::1"},
		},
		{
			name:      "get non-existent domain",
			queryName: "notfound.com.",
			queryType: types.RecordTypeA,
			wantErr:   types.ErrRecordNotFound,
		},
		{
			name:      "get non-existent type for existing domain",
			queryName: "example.com.",
			queryType: types.RecordTypeMX,
			wantErr:   types.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := setupTestStorage(t)
			recs, err := store.Get(context.Background(), tt.queryName, tt.queryType)
			if err != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil {
				if len(recs) == 0 {
					t.Fatal("Get() returned empty slice, want records")
				}
				for i, v := range tt.wantValues {
					if recs[0].Value[i] != v {
						t.Errorf("Get() value[%d] = %q, want %q", i, recs[0].Value[i], v)
					}
				}
			}
		})
	}
}

func TestMemoryStorage_Update(t *testing.T) {
	tests := []struct {
		name    string
		record  *types.DNSRecord
		wantErr error
	}{
		{
			name: "update existing record",
			record: &types.DNSRecord{
				Name:  "example.com.",
				Type:  types.RecordTypeA,
				TTL:   600,
				Value: []string{"10.0.0.1"},
			},
			wantErr: nil,
		},
		{
			name: "update non-existent domain",
			record: &types.DNSRecord{
				Name:  "notfound.com.",
				Type:  types.RecordTypeA,
				TTL:   300,
				Value: []string{"10.0.0.1"},
			},
			wantErr: types.ErrRecordNotFound,
		},
		{
			name: "update non-existent type",
			record: &types.DNSRecord{
				Name:  "example.com.",
				Type:  types.RecordTypeMX,
				TTL:   300,
				Value: []string{"mail.example.com."},
			},
			wantErr: types.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := setupTestStorage(t)
			err := store.Update(context.Background(), tt.record)
			if err != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil {
				recs, err := store.Get(context.Background(), tt.record.Name, tt.record.Type)
				if err != nil {
					t.Fatalf("Get() after Update() error = %v", err)
				}
				if recs[0].Value[0] != tt.record.Value[0] {
					t.Errorf("value after update = %q, want %q", recs[0].Value[0], tt.record.Value[0])
				}
				if recs[0].TTL != tt.record.TTL {
					t.Errorf("TTL after update = %d, want %d", recs[0].TTL, tt.record.TTL)
				}
			}
		})
	}
}

func TestMemoryStorage_Delete(t *testing.T) {
	tests := []struct {
		name       string
		deleteName string
		deleteType types.RecordType
		wantErr    error
	}{
		{
			name:       "delete existing record",
			deleteName: "example.com.",
			deleteType: types.RecordTypeA,
			wantErr:    nil,
		},
		{
			name:       "delete non-existent domain",
			deleteName: "notfound.com.",
			deleteType: types.RecordTypeA,
			wantErr:    types.ErrRecordNotFound,
		},
		{
			name:       "delete non-existent type",
			deleteName: "example.com.",
			deleteType: types.RecordTypeMX,
			wantErr:    types.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := setupTestStorage(t)
			err := store.Delete(context.Background(), tt.deleteName, tt.deleteType)
			if err != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil {
				_, err := store.Get(context.Background(), tt.deleteName, tt.deleteType)
				if err != types.ErrRecordNotFound {
					t.Errorf("Get() after Delete() error = %v, want ErrRecordNotFound", err)
				}
			}
		})
	}
}

func TestMemoryStorage_List(t *testing.T) {
	store := setupTestStorage(t)

	recs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("List() returned %d records, want 2", len(recs))
	}
}

func TestMemoryStorage_List_Empty(t *testing.T) {
	store := NewMemoryStorage()

	recs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("List() returned %d records, want 0", len(recs))
	}
}

func TestMemoryStorage_Version(t *testing.T) {
	store := NewMemoryStorage()

	if v := store.Version(); v != 0 {
		t.Errorf("initial version = %d, want 0", v)
	}

	_ = store.Create(context.Background(), &types.DNSRecord{
		Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})
	if v := store.Version(); v != 1 {
		t.Errorf("version after create = %d, want 1", v)
	}

	_ = store.Update(context.Background(), &types.DNSRecord{
		Name: "a.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"5.6.7.8"},
	})
	if v := store.Version(); v != 2 {
		t.Errorf("version after update = %d, want 2", v)
	}

	_ = store.Delete(context.Background(), "a.com.", types.RecordTypeA)
	if v := store.Version(); v != 3 {
		t.Errorf("version after delete = %d, want 3", v)
	}
}

func TestMemoryStorage_Watch(t *testing.T) {
	store := NewMemoryStorage()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := store.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Create a record and verify event is received.
	_ = store.Create(context.Background(), &types.DNSRecord{
		Name: "watch.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	select {
	case ev := <-ch:
		if ev.Type != types.EventAdded {
			t.Errorf("event type = %q, want %q", ev.Type, types.EventAdded)
		}
		if ev.Record.Name != "watch.com." {
			t.Errorf("event record name = %q, want %q", ev.Record.Name, "watch.com.")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch event")
	}

	// Cancel context and verify channel is closed.
	cancel()
	time.Sleep(50 * time.Millisecond)

	_, ok := <-ch
	if ok {
		t.Error("watch channel should be closed after context cancel")
	}
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent creates with unique keys.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "concurrent" + string(rune('A'+n)) + ".com."
			_ = store.Create(ctx, &types.DNSRecord{
				Name: name, Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
			})
		}(i)
	}

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.List(ctx)
		}()
	}

	wg.Wait()

	recs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recs) != goroutines {
		t.Errorf("List() returned %d records, want %d", len(recs), goroutines)
	}
}

func TestMemoryStorage_Delete_CleansUpEmptyDomain(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "single.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})

	_ = store.Delete(ctx, "single.com.", types.RecordTypeA)

	recs, _ := store.List(ctx)
	if len(recs) != 0 {
		t.Errorf("List() after deleting only record = %d, want 0", len(recs))
	}
}

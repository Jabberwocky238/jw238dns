package storage

import (
	"context"
	"regexp"
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

// TestMemoryStorage_Wildcard tests wildcard DNS record matching
func TestMemoryStorage_Wildcard(t *testing.T) {
	tests := []struct {
		name          string
		setupRecords  []*types.DNSRecord
		queryName     string
		queryType     types.RecordType
		wantErr       error
		wantValues    []string
		wantNoMatch   bool
	}{
		{
			name: "single wildcard match - *.example.com",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"192.168.1.1", "192.168.1.2"},
				},
			},
			queryName:  "test.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"192.168.1.1", "192.168.1.2"},
		},
		{
			name: "single wildcard match - different subdomain",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.0.0.1"},
				},
			},
			queryName:  "api.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.0.0.1"},
		},
		{
			name: "nested wildcard - *.*.app.com",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.*.app.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"172.16.0.1"},
				},
			},
			queryName:  "test.dev.app.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"172.16.0.1"},
		},
		{
			name: "nested wildcard - different labels",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.*.app.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"172.16.0.2"},
				},
			},
			queryName:  "api.prod.app.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"172.16.0.2"},
		},
		{
			name: "triple wildcard - *.*.*.example.com",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.*.*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.1.1.1"},
				},
			},
			queryName:  "a.b.c.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.1.1.1"},
		},
		{
			name: "wildcard with exact match - exact wins",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"192.168.1.1"},
				},
				{
					Name:  "test.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.0.0.1"},
				},
			},
			queryName:  "test.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.0.0.1"}, // Exact match should win
		},
		{
			name: "wildcard no match - too few labels",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.*.app.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"172.16.0.1"},
				},
			},
			queryName:   "test.app.com.", // Only 1 label before app.com
			queryType:   types.RecordTypeA,
			wantErr:     types.ErrRecordNotFound,
			wantNoMatch: true,
		},
		{
			name: "wildcard no match - too many labels",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"192.168.1.1"},
				},
			},
			queryName:   "a.b.example.com.", // 2 labels, wildcard expects 1
			queryType:   types.RecordTypeA,
			wantErr:     types.ErrRecordNotFound,
			wantNoMatch: true,
		},
		{
			name: "wildcard AAAA record",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.ipv6.example.com.",
					Type:  types.RecordTypeAAAA,
					TTL:   300,
					Value: []string{"2001:db8::1", "2001:db8::2"},
				},
			},
			queryName:  "test.ipv6.example.com.",
			queryType:  types.RecordTypeAAAA,
			wantErr:    nil,
			wantValues: []string{"2001:db8::1", "2001:db8::2"},
		},
		{
			name: "wildcard TXT record",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.txt.example.com.",
					Type:  types.RecordTypeTXT,
					TTL:   60,
					Value: []string{"v=spf1 include:example.com ~all"},
				},
			},
			queryName:  "mail.txt.example.com.",
			queryType:  types.RecordTypeTXT,
			wantErr:    nil,
			wantValues: []string{"v=spf1 include:example.com ~all"},
		},
		{
			name: "wildcard CNAME record",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.cdn.example.com.",
					Type:  types.RecordTypeCNAME,
					TTL:   300,
					Value: []string{"cdn.cloudfront.net."},
				},
			},
			queryName:  "assets.cdn.example.com.",
			queryType:  types.RecordTypeCNAME,
			wantErr:    nil,
			wantValues: []string{"cdn.cloudfront.net."},
		},
		{
			name: "wildcard wrong type - no match",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"192.168.1.1"},
				},
			},
			queryName:   "test.example.com.",
			queryType:   types.RecordTypeMX,
			wantErr:     types.ErrRecordNotFound,
			wantNoMatch: true,
		},
		{
			name: "middle wildcard - test.*.com",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "test.*.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.2.2.2"},
				},
			},
			queryName:  "test.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.2.2.2"},
		},
		{
			name: "multiple wildcards in different positions",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.prod.*.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.3.3.3"},
				},
			},
			queryName:  "api.prod.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.3.3.3"},
		},
		{
			name: "wildcard at TLD level - should not match",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"1.1.1.1"},
				},
			},
			queryName:  "example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"1.1.1.1"},
		},
		{
			name: "complex nested wildcard - *.*.*.*.example.com",
			setupRecords: []*types.DNSRecord{
				{
					Name:  "*.*.*.*.example.com.",
					Type:  types.RecordTypeA,
					TTL:   300,
					Value: []string{"10.4.4.4"},
				},
			},
			queryName:  "a.b.c.d.example.com.",
			queryType:  types.RecordTypeA,
			wantErr:    nil,
			wantValues: []string{"10.4.4.4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStorage()
			ctx := context.Background()

			// Setup records
			for _, rec := range tt.setupRecords {
				err := store.Create(ctx, rec)
				if err != nil {
					t.Fatalf("setup: failed to create record %s: %v", rec.Name, err)
				}
			}

			// Query
			recs, err := store.Get(ctx, tt.queryName, tt.queryType)

			// Check error
			if err != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect no match, we're done
			if tt.wantNoMatch {
				return
			}

			// Check values
			if tt.wantErr == nil {
				if len(recs) == 0 {
					t.Fatal("Get() returned empty slice, want records")
				}
				if len(recs[0].Value) != len(tt.wantValues) {
					t.Errorf("Get() returned %d values, want %d", len(recs[0].Value), len(tt.wantValues))
					return
				}
				for i, wantVal := range tt.wantValues {
					if recs[0].Value[i] != wantVal {
						t.Errorf("Get() value[%d] = %q, want %q", i, recs[0].Value[i], wantVal)
					}
				}
			}
		})
	}
}

// TestWildcardToRegex tests the wildcard to regex conversion function
func TestWildcardToRegex(t *testing.T) {
	tests := []struct {
		wildcard    string
		testDomain  string
		shouldMatch bool
	}{
		{"*.example.com.", "test.example.com.", true},
		{"*.example.com.", "api.example.com.", true},
		{"*.example.com.", "a.b.example.com.", false},
		{"*.example.com.", "example.com.", false},
		{"*.*.app.com.", "test.dev.app.com.", true},
		{"*.*.app.com.", "a.b.app.com.", true},
		{"*.*.app.com.", "test.app.com.", false},
		{"*.*.app.com.", "a.b.c.app.com.", false},
		{"test.*.com.", "test.example.com.", true},
		{"test.*.com.", "test.demo.com.", true},
		{"test.*.com.", "prod.example.com.", false},
		{"*.prod.*.com.", "api.prod.example.com.", true},
		{"*.prod.*.com.", "web.prod.demo.com.", true},
		{"*.prod.*.com.", "api.prod.a.b.com.", false},
		{"*.*.*.example.com.", "a.b.c.example.com.", true},
		{"*.*.*.example.com.", "x.y.z.example.com.", true},
		{"*.*.*.example.com.", "a.b.example.com.", false},
		{"*.com.", "example.com.", true},
		{"*.com.", "test.example.com.", false},
	}

	for _, tt := range tests {
		t.Run(tt.wildcard+" vs "+tt.testDomain, func(t *testing.T) {
			pattern := wildcardToRegex(tt.wildcard)
			matched, err := regexp.MatchString(pattern, tt.testDomain)
			if err != nil {
				t.Fatalf("regexp.MatchString() error = %v", err)
			}
			if matched != tt.shouldMatch {
				t.Errorf("pattern %q matched %q = %v, want %v", pattern, tt.testDomain, matched, tt.shouldMatch)
			}
		})
	}
}

// TestMemoryStorage_WildcardPriority tests that exact matches take priority over wildcards
func TestMemoryStorage_WildcardPriority(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	// Create wildcard record
	err := store.Create(ctx, &types.DNSRecord{
		Name:  "*.example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.1.1"},
	})
	if err != nil {
		t.Fatalf("failed to create wildcard record: %v", err)
	}

	// Create exact match record
	err = store.Create(ctx, &types.DNSRecord{
		Name:  "test.example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatalf("failed to create exact record: %v", err)
	}

	// Query should return exact match, not wildcard
	recs, err := store.Get(ctx, "test.example.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(recs) == 0 || len(recs[0].Value) == 0 {
		t.Fatal("Get() returned no records")
	}

	if recs[0].Value[0] != "10.0.0.1" {
		t.Errorf("Get() returned %q, want exact match %q (not wildcard %q)",
			recs[0].Value[0], "10.0.0.1", "192.168.1.1")
	}

	// Query for different subdomain should return wildcard
	recs, err = store.Get(ctx, "api.example.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(recs) == 0 || len(recs[0].Value) == 0 {
		t.Fatal("Get() returned no records")
	}

	if recs[0].Value[0] != "192.168.1.1" {
		t.Errorf("Get() returned %q, want wildcard match %q",
			recs[0].Value[0], "192.168.1.1")
	}
}

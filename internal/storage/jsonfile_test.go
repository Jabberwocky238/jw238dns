package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"jabberwocky238/jw238dns/internal/types"
)

func TestJSONFileLoader_Load(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		create    bool
		wantCount int
		wantErr   bool
	}{
		{
			name:      "valid single record",
			content:   `[{"name":"a.com.","type":"A","ttl":300,"value":["1.2.3.4"]}]`,
			create:    true,
			wantCount: 1,
		},
		{
			name:      "valid multiple records",
			content:   `[{"name":"a.com.","type":"A","ttl":300,"value":["1.2.3.4"]},{"name":"b.com.","type":"AAAA","ttl":600,"value":["2001:db8::1"]}]`,
			create:    true,
			wantCount: 2,
		},
		{
			name:   "file not found returns nil",
			create: false,
		},
		{
			name:    "invalid json",
			content: `{not valid json`,
			create:  true,
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			create:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "records.json")

			if tt.create {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("write test file: %v", err)
				}
			}

			store := NewMemoryStorage()
			loader := NewJSONFileLoader(path, store)

			records, err := loader.Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(records) != tt.wantCount {
				t.Errorf("Load() returned %d records, want %d", len(records), tt.wantCount)
			}
		})
	}
}

func TestJSONFileLoader_LoadAndApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	records := []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
		{Name: "b.com.", Type: types.RecordTypeAAAA, TTL: 600, Value: []string{"2001:db8::1"}},
	}
	data, _ := json.Marshal(records)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	ctx := context.Background()
	if err := loader.LoadAndApply(ctx); err != nil {
		t.Fatalf("LoadAndApply() error = %v", err)
	}

	recs, err := store.Get(ctx, "a.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(a.com.) error = %v", err)
	}
	if recs[0].Value[0] != "1.2.3.4" {
		t.Errorf("a.com. value = %q, want %q", recs[0].Value[0], "1.2.3.4")
	}

	recs, err = store.Get(ctx, "b.com.", types.RecordTypeAAAA)
	if err != nil {
		t.Fatalf("Get(b.com.) error = %v", err)
	}
	if recs[0].Value[0] != "2001:db8::1" {
		t.Errorf("b.com. value = %q, want %q", recs[0].Value[0], "2001:db8::1")
	}
}

func TestJSONFileLoader_LoadAndApply_NoChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	// Write the same record to the file.
	records := []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
	}
	data, _ := json.Marshal(records)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	loader := NewJSONFileLoader(path, store)
	vBefore := store.Version()

	if err := loader.LoadAndApply(ctx); err != nil {
		t.Fatalf("LoadAndApply() error = %v", err)
	}

	if store.Version() != vBefore {
		t.Errorf("version changed from %d to %d on no-op apply", vBefore, store.Version())
	}
}

func TestJSONFileLoader_LoadAndApply_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	// Should not error; just no records loaded.
	if err := loader.LoadAndApply(context.Background()); err != nil {
		t.Fatalf("LoadAndApply() error = %v, want nil for missing file", err)
	}
}

func TestJSONFileLoader_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "save.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"9.9.9.9"},
	})

	loader := NewJSONFileLoader(path, store)

	if err := loader.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read back and verify.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Name != "save.com." {
		t.Errorf("record name = %q, want %q", records[0].Name, "save.com.")
	}
	if records[0].Value[0] != "9.9.9.9" {
		t.Errorf("record value = %q, want %q", records[0].Value[0], "9.9.9.9")
	}
}

func TestJSONFileLoader_Save_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "atomic.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	loader := NewJSONFileLoader(path, store)

	if err := loader.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify no temp files remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file %q still exists after Save()", e.Name())
		}
	}

	// Verify the file is valid JSON.
	data, _ := os.ReadFile(path)
	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
}

func TestJSONFileLoader_Save_EmptyStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	if err := loader.Save(context.Background()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestJSONFileLoader_Save_SkipsDuringSyncing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	loader.mu.Lock()
	loader.syncing = true
	loader.mu.Unlock()

	err := loader.Save(context.Background())
	if err != nil {
		t.Errorf("Save() during sync error = %v, want nil", err)
	}

	// File should not have been created.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to not exist when syncing is true")
	}
}

func TestJSONFileLoader_Save_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "records.json")

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	if err := loader.Save(context.Background()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created in new subdirectory")
	}
}

func TestJSONFileLoader_Watch_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	// Write initial file.
	initial := []*types.DNSRecord{
		{Name: "initial.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	// Load initial records.
	if err := loader.LoadAndApply(context.Background()); err != nil {
		t.Fatalf("LoadAndApply() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchErr := make(chan error, 1)
	go func() {
		watchErr <- loader.Watch(ctx)
	}()

	// Give the watcher time to start.
	time.Sleep(200 * time.Millisecond)

	// Write updated records.
	updated := []*types.DNSRecord{
		{Name: "initial.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
		{Name: "added.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"2.2.2.2"}},
	}
	data, _ = json.Marshal(updated)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write updated file: %v", err)
	}

	// Wait for debounce + reload.
	time.Sleep(500 * time.Millisecond)

	recs, err := store.Get(ctx, "added.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(added.com.) after file change error = %v", err)
	}
	if len(recs) == 0 || recs[0].Value[0] != "2.2.2.2" {
		t.Errorf("expected added.com. -> 2.2.2.2, got %v", recs)
	}

	cancel()
	select {
	case err := <-watchErr:
		if err != nil && err != context.Canceled {
			t.Errorf("Watch() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not return after context cancel")
	}
}

func TestJSONFileLoader_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	store := NewMemoryStorage()
	ctx := context.Background()

	// Create records in store.
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "rt.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "rt.com.", Type: types.RecordTypeTXT, TTL: 60, Value: []string{"v=spf1 ~all"},
	})

	loader := NewJSONFileLoader(path, store)

	// Save to file.
	if err := loader.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create a new store and loader, load from file.
	store2 := NewMemoryStorage()
	loader2 := NewJSONFileLoader(path, store2)

	if err := loader2.LoadAndApply(ctx); err != nil {
		t.Fatalf("LoadAndApply() error = %v", err)
	}

	recs, err := store2.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("round-trip: expected 2 records, got %d", len(recs))
	}
}

func TestJSONFileLoader_WatchAndSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	// Write initial empty array.
	if err := os.WriteFile(path, []byte("[]"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	store := NewMemoryStorage()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loader := NewJSONFileLoader(path, store)

	syncErr := make(chan error, 1)
	go func() {
		syncErr <- loader.WatchAndSync(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Add a record via the store API (not via file).
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "sync.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"7.7.7.7"},
	})

	// Give the sync goroutine time to persist.
	time.Sleep(300 * time.Millisecond)

	// Verify the file was updated.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) == 0 {
		t.Error("expected records in file after sync, got 0")
	}

	cancel()
	select {
	case err := <-syncErr:
		if err != nil && err != context.Canceled {
			t.Errorf("WatchAndSync() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("WatchAndSync() did not return after cancel")
	}
}

func TestJSONFileLoader_Watch_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	if err := os.WriteFile(path, []byte("[]"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewMemoryStorage()
	loader := NewJSONFileLoader(path, store)

	ctx, cancel := context.WithCancel(context.Background())

	watchErr := make(chan error, 1)
	go func() {
		watchErr <- loader.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-watchErr:
		if err != nil && err != context.Canceled {
			t.Errorf("Watch() error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not return after cancel")
	}
}

func TestJSONFileLoader_Save_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.json")

	// Write initial content.
	if err := os.WriteFile(path, []byte(`[{"name":"old.com.","type":"A","ttl":300,"value":["1.1.1.1"]}]`), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "new.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"2.2.2.2"},
	})

	loader := NewJSONFileLoader(path, store)
	if err := loader.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	var records []*types.DNSRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 1 || records[0].Name != "new.com." {
		t.Errorf("expected [new.com.], got %v", records)
	}
}

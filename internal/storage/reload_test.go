package storage

import (
	"context"
	"testing"

	"jabberwocky238/jw238dns/internal/types"
)

func TestMemoryStorage_CalculateChanges(t *testing.T) {
	tests := []struct {
		name        string
		initial     []*types.DNSRecord
		newRecords  []*types.DNSRecord
		wantAdded   int
		wantUpdated int
		wantDeleted int
	}{
		{
			name: "no changes",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			wantAdded:   0,
			wantUpdated: 0,
			wantDeleted: 0,
		},
		{
			name: "one added",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
				{Name: "b.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"5.6.7.8"}},
			},
			wantAdded:   1,
			wantUpdated: 0,
			wantDeleted: 0,
		},
		{
			name: "one updated TTL",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"1.2.3.4"}},
			},
			wantAdded:   0,
			wantUpdated: 1,
			wantDeleted: 0,
		},
		{
			name: "one updated value",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"9.9.9.9"}},
			},
			wantAdded:   0,
			wantUpdated: 1,
			wantDeleted: 0,
		},
		{
			name: "one deleted",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
				{Name: "b.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"5.6.7.8"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			wantAdded:   0,
			wantUpdated: 0,
			wantDeleted: 1,
		},
		{
			name:    "all added from empty",
			initial: []*types.DNSRecord{},
			newRecords: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
				{Name: "b.com.", Type: types.RecordTypeAAAA, TTL: 300, Value: []string{"::1"}},
			},
			wantAdded:   2,
			wantUpdated: 0,
			wantDeleted: 0,
		},
		{
			name: "all deleted to empty",
			initial: []*types.DNSRecord{
				{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"}},
			},
			newRecords:  []*types.DNSRecord{},
			wantAdded:   0,
			wantUpdated: 0,
			wantDeleted: 1,
		},
		{
			name: "mixed add update delete",
			initial: []*types.DNSRecord{
				{Name: "keep.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
				{Name: "update.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"2.2.2.2"}},
				{Name: "remove.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"3.3.3.3"}},
			},
			newRecords: []*types.DNSRecord{
				{Name: "keep.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
				{Name: "update.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"2.2.2.2"}},
				{Name: "new.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"4.4.4.4"}},
			},
			wantAdded:   1,
			wantUpdated: 1,
			wantDeleted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStorage()
			ctx := context.Background()
			for _, r := range tt.initial {
				if err := store.Create(ctx, r); err != nil {
					t.Fatalf("setup Create() error = %v", err)
				}
			}

			changes := store.CalculateChanges(tt.newRecords)

			if len(changes.Added) != tt.wantAdded {
				t.Errorf("Added = %d, want %d", len(changes.Added), tt.wantAdded)
			}
			if len(changes.Updated) != tt.wantUpdated {
				t.Errorf("Updated = %d, want %d", len(changes.Updated), tt.wantUpdated)
			}
			if len(changes.Deleted) != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", len(changes.Deleted), tt.wantDeleted)
			}
		})
	}
}

func TestMemoryStorage_HotReload(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "old.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	newRecords := []*types.DNSRecord{
		{Name: "new1.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"2.2.2.2"}},
		{Name: "new2.com.", Type: types.RecordTypeAAAA, TTL: 600, Value: []string{"::1"}},
	}

	err := store.HotReload(ctx, newRecords)
	if err != nil {
		t.Fatalf("HotReload() error = %v", err)
	}

	// Old record should be gone.
	_, err = store.Get(ctx, "old.com.", types.RecordTypeA)
	if err != types.ErrRecordNotFound {
		t.Errorf("Get(old.com.) after HotReload error = %v, want ErrRecordNotFound", err)
	}

	// New records should exist.
	recs, err := store.Get(ctx, "new1.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(new1.com.) error = %v", err)
	}
	if recs[0].Value[0] != "2.2.2.2" {
		t.Errorf("new1.com. value = %q, want %q", recs[0].Value[0], "2.2.2.2")
	}

	recs, err = store.Get(ctx, "new2.com.", types.RecordTypeAAAA)
	if err != nil {
		t.Fatalf("Get(new2.com.) error = %v", err)
	}
	if recs[0].Value[0] != "::1" {
		t.Errorf("new2.com. value = %q, want %q", recs[0].Value[0], "::1")
	}
}

func TestMemoryStorage_PartialReload(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "keep.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "update.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"2.2.2.2"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "remove.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"3.3.3.3"},
	})

	changes := &types.RecordChanges{
		Added: []*types.DNSRecord{
			{Name: "new.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"4.4.4.4"}},
		},
		Updated: []*types.DNSRecord{
			{Name: "update.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"9.9.9.9"}},
		},
		Deleted: []types.RecordKey{
			{Name: "remove.com.", Type: types.RecordTypeA},
		},
	}

	versionBefore := store.Version()
	err := store.PartialReload(ctx, changes)
	if err != nil {
		t.Fatalf("PartialReload() error = %v", err)
	}

	if store.Version() != versionBefore+1 {
		t.Errorf("version = %d, want %d", store.Version(), versionBefore+1)
	}

	// Kept record unchanged.
	recs, err := store.Get(ctx, "keep.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(keep.com.) error = %v", err)
	}
	if recs[0].Value[0] != "1.1.1.1" {
		t.Errorf("keep.com. value = %q, want %q", recs[0].Value[0], "1.1.1.1")
	}

	// Updated record has new values.
	recs, err = store.Get(ctx, "update.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(update.com.) error = %v", err)
	}
	if recs[0].Value[0] != "9.9.9.9" {
		t.Errorf("update.com. value = %q, want %q", recs[0].Value[0], "9.9.9.9")
	}
	if recs[0].TTL != 600 {
		t.Errorf("update.com. TTL = %d, want %d", recs[0].TTL, 600)
	}

	// Deleted record is gone.
	_, err = store.Get(ctx, "remove.com.", types.RecordTypeA)
	if err != types.ErrRecordNotFound {
		t.Errorf("Get(remove.com.) error = %v, want ErrRecordNotFound", err)
	}

	// Added record exists.
	recs, err = store.Get(ctx, "new.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get(new.com.) error = %v", err)
	}
	if recs[0].Value[0] != "4.4.4.4" {
		t.Errorf("new.com. value = %q, want %q", recs[0].Value[0], "4.4.4.4")
	}
}

func TestRecordsEqual(t *testing.T) {
	tests := []struct {
		name string
		a    *types.DNSRecord
		b    *types.DNSRecord
		want bool
	}{
		{
			name: "identical records",
			a:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4"}},
			b:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4"}},
			want: true,
		},
		{
			name: "different TTL",
			a:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4"}},
			b:    &types.DNSRecord{TTL: 600, Value: []string{"1.2.3.4"}},
			want: false,
		},
		{
			name: "different value",
			a:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4"}},
			b:    &types.DNSRecord{TTL: 300, Value: []string{"5.6.7.8"}},
			want: false,
		},
		{
			name: "different value count",
			a:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4"}},
			b:    &types.DNSRecord{TTL: 300, Value: []string{"1.2.3.4", "5.6.7.8"}},
			want: false,
		},
		{
			name: "both empty values",
			a:    &types.DNSRecord{TTL: 300, Value: []string{}},
			b:    &types.DNSRecord{TTL: 300, Value: []string{}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recordsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("recordsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryStorage_HotReload_WatchEvent(t *testing.T) {
	store := NewMemoryStorage()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := store.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	_ = store.HotReload(context.Background(), []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
	})

	select {
	case ev := <-ch:
		if ev.Type != types.EventReloaded {
			t.Errorf("event type = %q, want %q", ev.Type, types.EventReloaded)
		}
	default:
		t.Error("expected reload event on watch channel")
	}
}

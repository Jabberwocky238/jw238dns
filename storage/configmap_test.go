package storage

import (
	"context"
	"testing"
	"time"

	"jabberwocky238/jw238dns/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestParseConfigMap(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]string
		dataKey   string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid single record",
			data: map[string]string{
				"config.yaml": "records:\n  - name: example.com.\n    type: A\n    ttl: 300\n    value:\n      - 192.168.1.1\n",
			},
			dataKey:   "config.yaml",
			wantCount: 1,
		},
		{
			name: "valid multiple records",
			data: map[string]string{
				"config.yaml": "records:\n  - name: a.com.\n    type: A\n    ttl: 300\n    value:\n      - 1.2.3.4\n  - name: b.com.\n    type: AAAA\n    ttl: 600\n    value:\n      - \"2001:db8::1\"\n",
			},
			dataKey:   "config.yaml",
			wantCount: 2,
		},
		{
			name:    "missing key",
			data:    map[string]string{"other.yaml": "foo"},
			dataKey: "config.yaml",
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			data:    map[string]string{"config.yaml": "not: [valid: yaml: {{"},
			dataKey: "config.yaml",
			wantErr: true,
		},
		{
			name:    "empty records",
			data:    map[string]string{"config.yaml": "records: []\n"},
			dataKey: "config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Data:       tt.data,
			}
			records, err := parseConfigMap(cm, tt.dataKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfigMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(records) != tt.wantCount {
				t.Errorf("parseConfigMap() returned %d records, want %d", len(records), tt.wantCount)
			}
		})
	}
}

func TestConfigMapWatcher_WatchAppliesRecords(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the watcher first, then create the ConfigMap so the fake
	// client's watch channel picks up the ADDED event.
	watchErr := make(chan error, 1)
	go func() {
		watchErr <- w.Watch(ctx)
	}()

	// Give the watcher goroutine time to register the K8s watch.
	time.Sleep(100 * time.Millisecond)

	// Now create the ConfigMap -- the watcher should see the ADDED event.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "jw238dns-config", Namespace: "default"},
		Data: map[string]string{
			"config.yaml": "records:\n  - name: watch.com.\n    type: A\n    ttl: 300\n    value:\n      - 10.0.0.1\n",
		},
	}
	_, err := fakeClient.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	// Give the watcher time to process the event.
	time.Sleep(200 * time.Millisecond)

	recs, err := store.Get(ctx, "watch.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get() after watch error = %v", err)
	}
	if len(recs) == 0 || recs[0].Value[0] != "10.0.0.1" {
		t.Errorf("expected watch.com. -> 10.0.0.1, got %v", recs)
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

func TestConfigMapWatcher_PersistToConfigMap(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	ctx := context.Background()

	// Create an empty ConfigMap first.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "jw238dns-config", Namespace: "default"},
		Data:       map[string]string{"config.yaml": "records: []\n"},
	}
	_, err := fakeClient.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	// Add a record to the store.
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "persist.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})

	// Persist to ConfigMap.
	if err := w.PersistToConfigMap(ctx); err != nil {
		t.Fatalf("PersistToConfigMap() error = %v", err)
	}

	// Read back the ConfigMap and verify.
	updated, err := fakeClient.CoreV1().ConfigMaps("default").Get(ctx, "jw238dns-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}

	records, err := parseConfigMap(updated, "config.yaml")
	if err != nil {
		t.Fatalf("parseConfigMap() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Name != "persist.com." {
		t.Errorf("record name = %q, want %q", records[0].Name, "persist.com.")
	}
}

func TestConfigMapWatcher_PersistSkipsDuringSyncing(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	// Simulate syncing state.
	w.mu.Lock()
	w.syncing = true
	w.mu.Unlock()

	// PersistToConfigMap should return nil without doing anything.
	err := w.PersistToConfigMap(context.Background())
	if err != nil {
		t.Errorf("PersistToConfigMap() during sync error = %v, want nil", err)
	}
}

func TestConfigMapWatcher_PersistErrorOnMissingConfigMap(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "nonexistent", "config.yaml", store)

	err := w.PersistToConfigMap(context.Background())
	if err == nil {
		t.Error("PersistToConfigMap() expected error for missing configmap, got nil")
	}
}

func TestConfigMapWatcher_ApplyRecordsNoChanges(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	fakeClient := fake.NewSimpleClientset()
	w := NewConfigMapWatcher(fakeClient, "default", "test", "config.yaml", store)

	vBefore := store.Version()
	w.applyRecords(ctx, []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"}},
	})

	if store.Version() != vBefore {
		t.Errorf("version changed from %d to %d on no-op apply", vBefore, store.Version())
	}
}

func TestConfigMapWatcher_ApplyRecordsWithChanges(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.1.1.1"},
	})

	fakeClient := fake.NewSimpleClientset()
	w := NewConfigMapWatcher(fakeClient, "default", "test", "config.yaml", store)

	vBefore := store.Version()
	w.applyRecords(ctx, []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"2.2.2.2"}},
	})

	if store.Version() == vBefore {
		t.Error("version should have changed after apply with updates")
	}

	recs, err := store.Get(ctx, "a.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if recs[0].Value[0] != "2.2.2.2" {
		t.Errorf("value = %q, want %q", recs[0].Value[0], "2.2.2.2")
	}
}

func TestConfigMapWatcher_WatchModifiedEvent(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchErr := make(chan error, 1)
	go func() {
		watchErr <- w.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create initial ConfigMap.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "jw238dns-config", Namespace: "default"},
		Data: map[string]string{
			"config.yaml": "records:\n  - name: mod.com.\n    type: A\n    ttl: 300\n    value:\n      - 1.1.1.1\n",
		},
	}
	_, err := fakeClient.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Now update the ConfigMap (Modified event).
	cm.Data["config.yaml"] = "records:\n  - name: mod.com.\n    type: A\n    ttl: 600\n    value:\n      - 9.9.9.9\n"
	_, err = fakeClient.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("update configmap: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	recs, err := store.Get(ctx, "mod.com.", types.RecordTypeA)
	if err != nil {
		t.Fatalf("Get() after modify error = %v", err)
	}
	if recs[0].TTL != 600 {
		t.Errorf("TTL = %d, want 600", recs[0].TTL)
	}
	if recs[0].Value[0] != "9.9.9.9" {
		t.Errorf("value = %q, want %q", recs[0].Value[0], "9.9.9.9")
	}

	cancel()
}

func TestConfigMapWatcher_WatchAndSync(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create ConfigMap so PersistToConfigMap can find it.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "jw238dns-config", Namespace: "default"},
		Data:       map[string]string{"config.yaml": "records: []\n"},
	}
	_, err := fakeClient.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	syncErr := make(chan error, 1)
	go func() {
		syncErr <- w.WatchAndSync(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	// Add a record via the store API (not via ConfigMap).
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "sync.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"5.5.5.5"},
	})

	// Give the sync goroutine time to persist.
	time.Sleep(300 * time.Millisecond)

	// Verify the ConfigMap was updated.
	updated, err := fakeClient.CoreV1().ConfigMaps("default").Get(ctx, "jw238dns-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	records, err := parseConfigMap(updated, "config.yaml")
	if err != nil {
		t.Fatalf("parseConfigMap: %v", err)
	}
	if len(records) == 0 {
		t.Error("expected records in ConfigMap after sync, got 0")
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

func TestConfigMapWatcher_PersistToConfigMap_NilData(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	store := NewMemoryStorage()
	w := NewConfigMapWatcher(fakeClient, "default", "jw238dns-config", "config.yaml", store)

	ctx := context.Background()

	// Create ConfigMap with nil Data.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "jw238dns-config", Namespace: "default"},
	}
	_, err := fakeClient.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	if err := w.PersistToConfigMap(ctx); err != nil {
		t.Fatalf("PersistToConfigMap() error = %v", err)
	}

	updated, err := fakeClient.CoreV1().ConfigMaps("default").Get(ctx, "jw238dns-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	if updated.Data == nil {
		t.Error("expected Data to be initialized after persist")
	}
}

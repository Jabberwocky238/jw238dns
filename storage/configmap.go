package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"jabberwocky238/jw238dns/types"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// configYAML is the top-level structure inside the ConfigMap's config.yaml key.
type configYAML struct {
	Records []*types.DNSRecord `yaml:"records"`
}

// ConfigMapWatcher watches a Kubernetes ConfigMap for DNS record changes
// and syncs them into a MemoryStorage via CalculateChanges + PartialReload.
type ConfigMapWatcher struct {
	client    kubernetes.Interface
	namespace string
	name      string
	dataKey   string
	store     *MemoryStorage

	mu      sync.Mutex
	syncing bool // guards against echo loops during bidirectional sync
}

// NewConfigMapWatcher creates a ConfigMapWatcher that watches the named
// ConfigMap in the given namespace. The dataKey parameter specifies which
// key inside the ConfigMap's Data map holds the YAML records (typically
// "config.yaml").
func NewConfigMapWatcher(client kubernetes.Interface, namespace, name, dataKey string, store *MemoryStorage) *ConfigMapWatcher {
	return &ConfigMapWatcher{
		client:    client,
		namespace: namespace,
		name:      name,
		dataKey:   dataKey,
		store:     store,
	}
}

// Watch starts watching the ConfigMap for changes. It blocks until the
// context is cancelled. On each change it parses the YAML, computes a
// diff, and applies a partial reload.
func (w *ConfigMapWatcher) Watch(ctx context.Context) error {
	for {
		if err := w.watchOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("configmap watch error, retrying", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		// watcher closed cleanly; restart unless cancelled.
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// watchOnce runs a single watch session. It returns when the watch channel
// closes or an error occurs.
func (w *ConfigMapWatcher) watchOnce(ctx context.Context) error {
	watcher, err := w.client.CoreV1().ConfigMaps(w.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", w.name),
	})
	if err != nil {
		return fmt.Errorf("watch configmap: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil // channel closed, caller will restart
			}
			if event.Type == watch.Added || event.Type == watch.Modified {
				cm, ok := event.Object.(*corev1.ConfigMap)
				if !ok {
					continue
				}
				records, err := parseConfigMap(cm, w.dataKey)
				if err != nil {
					slog.Error("parse configmap", "err", err)
					continue
				}
				w.applyRecords(ctx, records)
			}
		}
	}
}

// applyRecords computes a diff and applies a partial reload.
func (w *ConfigMapWatcher) applyRecords(ctx context.Context, records []*types.DNSRecord) {
	w.mu.Lock()
	w.syncing = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.syncing = false
		w.mu.Unlock()
	}()

	changes := w.store.CalculateChanges(records)
	if len(changes.Added) == 0 && len(changes.Updated) == 0 && len(changes.Deleted) == 0 {
		return
	}
	if err := w.store.PartialReload(ctx, changes); err != nil {
		slog.Error("partial reload from configmap", "err", err)
	}
}

// PersistToConfigMap writes the current storage contents back to the
// ConfigMap (Storage -> ConfigMap direction of bidirectional sync).
func (w *ConfigMapWatcher) PersistToConfigMap(ctx context.Context) error {
	w.mu.Lock()
	if w.syncing {
		w.mu.Unlock()
		return nil // skip echo
	}
	w.mu.Unlock()

	records, err := w.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	cfg := configYAML{Records: records}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	cm, err := w.client.CoreV1().ConfigMaps(w.namespace).Get(ctx, w.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get configmap: %w", err)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[w.dataKey] = string(data)

	_, err = w.client.CoreV1().ConfigMaps(w.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}

	slog.Info("persisted records to configmap", "records", len(records))
	return nil
}

// WatchAndSync starts both the ConfigMap watcher and a goroutine that
// listens on the storage Watch channel to persist changes back to the
// ConfigMap. It blocks until ctx is cancelled.
func (w *ConfigMapWatcher) WatchAndSync(ctx context.Context) error {
	ch, err := w.store.Watch(ctx)
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
				if err := w.PersistToConfigMap(ctx); err != nil {
					slog.Error("persist to configmap", "err", err)
				}
			}
		}
	}()

	return w.Watch(ctx)
}

// parseConfigMap extracts DNS records from the given ConfigMap.
func parseConfigMap(cm *corev1.ConfigMap, dataKey string) ([]*types.DNSRecord, error) {
	raw, ok := cm.Data[dataKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in configmap %s/%s", dataKey, cm.Namespace, cm.Name)
	}

	var cfg configYAML
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	return cfg.Records, nil
}

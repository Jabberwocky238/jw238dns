# ConfigMap and JSON File Integration

## Goal
Implement ConfigMap integration for Kubernetes and JSON file integration for DNS records. Support hot reload, bidirectional sync, and provide example configuration files in /assets directory.

## Requirements

### 1. ConfigMap Integration (`internal/storage/configmap.go`)
- Create `ConfigMapWatcher` struct
- Watch Kubernetes ConfigMap changes using K8s client-go
- Parse ConfigMap data to extract DNS records
- Trigger hot reload on ConfigMap changes (ConfigMap → Storage)
- Implement bidirectional sync (Storage → ConfigMap)
- Use in-cluster Kubernetes configuration
- Handle ConfigMap not found gracefully

### 2. JSON File Integration (`internal/storage/jsonfile.go`)
- Create `JSONFileLoader` struct
- Load DNS records from JSON file
- Watch file changes using fsnotify
- Trigger hot reload on file changes (JSON → Storage)
- Implement bidirectional sync (Storage → JSON file)
- Handle file not found gracefully
- Support atomic file writes

### 3. Example Assets (`assets/`)
- Create `example-configmap.yaml` with sample DNS records
- Create `example-records.json` with sample DNS records
- Include all supported record types (A, AAAA, CNAME, MX, TXT, etc.)
- Follow the format defined in DNS core spec

### 4. Integration with Core Storage
- Use existing `CalculateChanges()` for diff computation
- Call `PartialReload()` for incremental updates
- Call `HotReload()` for full replacement
- Subscribe to `Watch()` channel for bidirectional sync

## Acceptance Criteria

- [ ] `internal/storage/configmap.go` created with ConfigMap watcher
- [ ] `internal/storage/configmap_test.go` with fake K8s client tests
- [ ] `internal/storage/jsonfile.go` created with JSON file loader
- [ ] `internal/storage/jsonfile_test.go` with file watch tests
- [ ] `assets/example-configmap.yaml` created with sample records
- [ ] `assets/example-records.json` created with sample records
- [ ] ConfigMap changes trigger hot reload
- [ ] JSON file changes trigger hot reload
- [ ] Bidirectional sync works (Storage → ConfigMap/JSON)
- [ ] All tests pass with `go test ./...`
- [ ] Test coverage >85% for new code
- [ ] Graceful error handling for missing resources

## Technical Notes

### ConfigMap Format (from spec)
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jw238dns-config
  namespace: default
data:
  config.yaml: |
    records:
      - name: example.com.
        type: A
        ttl: 300
        value:
          - 192.168.1.1
```

### JSON Format
```json
[
  {
    "name": "example.com.",
    "type": "A",
    "ttl": 300,
    "value": ["192.168.1.1"]
  }
]
```

### Libraries
- `k8s.io/client-go` - Kubernetes API client
- `k8s.io/api/core/v1` - ConfigMap types
- `k8s.io/client-go/kubernetes/fake` - Testing
- `github.com/fsnotify/fsnotify` - File watching

### Integration Pattern
1. External source (ConfigMap/JSON) changes
2. Parse to `[]*types.DNSRecord`
3. Call `store.CalculateChanges(newRecords)`
4. Call `store.PartialReload(ctx, changes)`
5. Storage emits events via `Watch()` channel
6. Other watchers receive events and sync back

### Bidirectional Sync
- ConfigMap watcher subscribes to Storage `Watch()` channel
- On `EventTypeAdded/Updated/Deleted` from API, persist to ConfigMap
- JSON file loader subscribes to Storage `Watch()` channel
- On events from API, write to JSON file atomically

### Error Handling
- ConfigMap not found: log warning, continue without ConfigMap sync
- JSON file not found: create new file on first write
- Parse errors: log error, skip reload
- K8s API errors: retry with exponential backoff

## Out of Scope
- RBAC manifest creation (will be in deployment task)
- Application entry point wiring (will be in main.go task)
- ConfigMap validation webhook
- Multi-namespace support

## Dependencies
- Depends on: DNS Core (commit 8861029)
- Depends on: Core Storage hot reload (already implemented)

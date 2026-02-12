# Storage Layer Guidelines

> Standards for implementing and using the storage abstraction layer

---

## Overview

The storage layer provides a unified interface for persisting DNS records across different backends (ConfigMap, JSON file, etc.).

---

## Core Interface

```go
package storage

import "context"

type Storage interface {
    // Get retrieves a single DNS record
    Get(ctx context.Context, domain, recordType string) (*DNSRecord, error)

    // List retrieves multiple DNS records with optional filtering
    List(ctx context.Context, filter Filter) ([]*DNSRecord, error)

    // Create adds a new DNS record
    Create(ctx context.Context, record *DNSRecord) error

    // Update modifies an existing DNS record
    Update(ctx context.Context, record *DNSRecord) error

    // Delete removes a DNS record
    Delete(ctx context.Context, domain, recordType string) error

    // Watch returns a channel for real-time updates
    Watch(ctx context.Context) (<-chan Event, error)
}
```

---

## Data Structures

### DNSRecord

```go
type DNSRecord struct {
    Domain   string            `json:"domain"`   // e.g., "example.com" or "*.example.com"
    Type     string            `json:"type"`     // A, AAAA, CNAME, TXT, SRV, MX, NS
    Value    string            `json:"value"`    // Record value
    TTL      int               `json:"ttl"`      // Time to live in seconds
    Priority *int              `json:"priority,omitempty"` // For MX, SRV records
    Weight   *int              `json:"weight,omitempty"`   // For SRV records
    Port     *int              `json:"port,omitempty"`     // For SRV records
    Metadata map[string]string `json:"metadata,omitempty"` // Additional metadata

    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Filter

```go
type Filter struct {
    Domain     string   // Exact match or wildcard
    Types      []string // Filter by record types
    Limit      int      // Max results
    Offset     int      // Pagination offset
}
```

### Event

```go
type Event struct {
    Type   EventType  // Created, Updated, Deleted
    Record *DNSRecord
}

type EventType string

const (
    EventCreated EventType = "created"
    EventUpdated EventType = "updated"
    EventDeleted EventType = "deleted"
)
```

---

## Implementation Guidelines

### 1. ConfigMap Storage

**Key Points**:
- Store all records in a single ConfigMap as JSON
- Use `ResourceVersion` for optimistic locking
- Implement retry logic for conflicts
- Use K8s watch API for real-time updates

**Example Structure**:
```go
type ConfigMapStorage struct {
    client    kubernetes.Interface
    namespace string
    name      string
    cache     *recordCache // Optional local cache
}

func (s *ConfigMapStorage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("get configmap: %w", err)
    }

    // Parse records from cm.Data
    records, err := parseRecords(cm.Data["records"])
    if err != nil {
        return nil, err
    }

    // Find matching record
    for _, r := range records {
        if r.Domain == domain && r.Type == recordType {
            return r, nil
        }
    }

    return nil, ErrNotFound
}
```

**Conflict Handling**:
```go
func (s *ConfigMapStorage) Update(ctx context.Context, record *DNSRecord) error {
    for retry := 0; retry < 3; retry++ {
        cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
        if err != nil {
            return err
        }

        // Modify records
        records := parseRecords(cm.Data["records"])
        updateRecord(records, record)
        cm.Data["records"] = serializeRecords(records)

        // Try to update with ResourceVersion
        _, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
        if err == nil {
            return nil
        }

        // Retry on conflict
        if !errors.IsConflict(err) {
            return err
        }
    }
    return ErrConflict
}
```

---

### 2. JSON File Storage

**Key Points**:
- Use file locking to prevent concurrent writes
- Implement atomic writes (write to temp file, then rename)
- Support hot reload via file watching
- Provide backup mechanism

**Example Structure**:
```go
type JSONFileStorage struct {
    path     string
    mu       sync.RWMutex
    records  map[string]*DNSRecord // key: domain:type
    watcher  *fsnotify.Watcher
}

func (s *JSONFileStorage) Create(ctx context.Context, record *DNSRecord) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    key := recordKey(record.Domain, record.Type)
    if _, exists := s.records[key]; exists {
        return ErrAlreadyExists
    }

    record.CreatedAt = time.Now()
    record.UpdatedAt = time.Now()
    s.records[key] = record

    return s.persist()
}

func (s *JSONFileStorage) persist() error {
    // Marshal records
    data, err := json.MarshalIndent(s.records, "", "  ")
    if err != nil {
        return err
    }

    // Atomic write: write to temp file, then rename
    tmpPath := s.path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }

    // Backup old file
    if _, err := os.Stat(s.path); err == nil {
        backupPath := s.path + ".bak"
        os.Rename(s.path, backupPath)
    }

    return os.Rename(tmpPath, s.path)
}
```

---

## Error Handling

### Standard Errors

```go
var (
    ErrNotFound       = errors.New("record not found")
    ErrAlreadyExists  = errors.New("record already exists")
    ErrInvalidRecord  = errors.New("invalid record")
    ErrConflict       = errors.New("update conflict")
    ErrStorageUnavailable = errors.New("storage unavailable")
)
```

### Error Wrapping

Always wrap errors with context:

```go
// Good
return nil, fmt.Errorf("get configmap %s/%s: %w", namespace, name, err)

// Bad
return nil, err
```

---

## Validation

### Record Validation

```go
func ValidateRecord(r *DNSRecord) error {
    if r.Domain == "" {
        return fmt.Errorf("%w: domain is required", ErrInvalidRecord)
    }

    if !isValidDomain(r.Domain) {
        return fmt.Errorf("%w: invalid domain format", ErrInvalidRecord)
    }

    if !isValidRecordType(r.Type) {
        return fmt.Errorf("%w: unsupported record type %s", ErrInvalidRecord, r.Type)
    }

    switch r.Type {
    case "A":
        if !isValidIPv4(r.Value) {
            return fmt.Errorf("%w: invalid IPv4 address", ErrInvalidRecord)
        }
    case "AAAA":
        if !isValidIPv6(r.Value) {
            return fmt.Errorf("%w: invalid IPv6 address", ErrInvalidRecord)
        }
    case "MX":
        if r.Priority == nil {
            return fmt.Errorf("%w: MX record requires priority", ErrInvalidRecord)
        }
    case "SRV":
        if r.Priority == nil || r.Weight == nil || r.Port == nil {
            return fmt.Errorf("%w: SRV record requires priority, weight, and port", ErrInvalidRecord)
        }
    }

    if r.TTL < 0 {
        return fmt.Errorf("%w: TTL must be non-negative", ErrInvalidRecord)
    }

    return nil
}
```

---

## Testing

### Unit Tests

```go
func TestConfigMapStorage_Create(t *testing.T) {
    client := fake.NewSimpleClientset()
    storage := NewConfigMapStorage(client, "default", "dns-records")

    record := &DNSRecord{
        Domain: "example.com",
        Type:   "A",
        Value:  "1.2.3.4",
        TTL:    300,
    }

    err := storage.Create(context.Background(), record)
    assert.NoError(t, err)

    // Verify record was created
    retrieved, err := storage.Get(context.Background(), "example.com", "A")
    assert.NoError(t, err)
    assert.Equal(t, record.Value, retrieved.Value)
}
```

### Integration Tests

```go
func TestStorageIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Test with real K8s cluster or kind
    // ...
}
```

---

## Performance Considerations

### Caching

Implement local cache for frequently accessed records:

```go
type cachedStorage struct {
    backend Storage
    cache   *lru.Cache
    ttl     time.Duration
}

func (s *cachedStorage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    key := domain + ":" + recordType

    // Check cache first
    if val, ok := s.cache.Get(key); ok {
        return val.(*DNSRecord), nil
    }

    // Fetch from backend
    record, err := s.backend.Get(ctx, domain, recordType)
    if err != nil {
        return nil, err
    }

    // Cache result
    s.cache.Add(key, record)
    return record, nil
}
```

### Batch Operations

For bulk updates, use batch operations:

```go
func (s *Storage) CreateBatch(ctx context.Context, records []*DNSRecord) error {
    // Single transaction for all records
    // More efficient than individual Creates
}
```

---

## Common Mistakes

### ❌ Don't: Ignore Context Cancellation

```go
// Bad
func (s *Storage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    // Ignores ctx
    return s.fetchFromBackend(domain, recordType)
}
```

```go
// Good
func (s *Storage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        return s.fetchFromBackend(ctx, domain, recordType)
    }
}
```

### ❌ Don't: Forget to Close Watchers

```go
// Bad
func main() {
    events, _ := storage.Watch(ctx)
    for event := range events {
        // Process event
    }
    // Watcher never closed
}
```

```go
// Good
func main() {
    events, err := storage.Watch(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close() // Implement Close() method

    for event := range events {
        // Process event
    }
}
```

### ❌ Don't: Store Sensitive Data in ConfigMap

```go
// Bad - ConfigMaps are not encrypted
record := &DNSRecord{
    Domain: "api.example.com",
    Type:   "TXT",
    Value:  "secret-api-key-12345", // Exposed!
}
```

```go
// Good - Use Secrets for sensitive data
// Or don't store sensitive data in DNS records at all
```

---

## Best Practices

1. **Always validate records before storage**
2. **Use context for cancellation and timeouts**
3. **Implement proper error wrapping**
4. **Add metrics for storage operations**
5. **Test with both storage backends**
6. **Document storage format for manual editing**
7. **Implement graceful degradation on storage failure**

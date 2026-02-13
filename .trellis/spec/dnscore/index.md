# DNS Core Requirements - jw238dns

## Overview

This document defines the core requirements and specifications for the jw238dns DNS module.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         jw238dns Core                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Frontend Layer                         │  │
│  │  - Receive all DNS query types                           │  │
│  │  - Parse DNS requests (A, AAAA, CNAME, MX, TXT, etc.)   │  │
│  │  - Validate query format                                 │  │
│  └────────────────────┬─────────────────────────────────────┘  │
│                       │                                          │
│                       ▼                                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Core Storage                           │  │
│  │  - In-memory record storage                              │  │
│  │  - Thread-safe operations                                │  │
│  │  - Hot reload support (partial reload)                   │  │
│  │  - Record indexing and lookup                            │  │
│  └────────┬─────────────────────────────────┬───────────────┘  │
│           │                                  │                   │
│           ▼                                  ▼                   │
│  ┌────────────────────┐          ┌─────────────────────────┐   │
│  │  Backend Layer     │          │  Management Interfaces  │   │
│  │  - Read config     │          │  - HTTP API             │   │
│  │  - Return results  │          │  - ConfigMap Watcher    │   │
│  │  - Apply rules     │          │  - Direct storage ops   │   │
│  └────────────────────┘          └─────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Frontend Layer - DNS Query Receiver

**Responsibility**: Receive and parse ALL DNS query types

**Supported Query Types**:
- **A**: IPv4 address records
- **AAAA**: IPv6 address records
- **CNAME**: Canonical name records
- **MX**: Mail exchange records
- **TXT**: Text records
- **NS**: Name server records
- **SRV**: Service records
- **PTR**: Pointer records
- **SOA**: Start of authority records
- **CAA**: Certification Authority Authorization
- **ANY**: All available records

**Requirements**:
```go
// Frontend must handle all query types
type DNSFrontend interface {
    // ReceiveQuery accepts any DNS query type
    ReceiveQuery(ctx context.Context, query *dns.Msg) (*dns.Msg, error)

    // ParseQuery validates and extracts query information
    ParseQuery(query *dns.Msg) (*QueryInfo, error)
}

type QueryInfo struct {
    Domain string
    Type   uint16  // DNS query type
    Class  uint16  // Usually IN (Internet)
}
```

**Implementation Notes**:
- Use `github.com/miekg/dns` library
- Support both UDP (port 53) and TCP (port 53)
- Validate query format before processing
- Log all incoming queries with structured logging

### 2. Core Storage - Central Data Store

**Responsibility**: Thread-safe in-memory storage with hot reload capability

**Key Features**:
- **Thread-Safe**: Support concurrent reads and writes
- **Hot Reload**: Partial reload without full restart
- **Fast Lookup**: O(1) record lookup by domain name
- **Indexing**: Efficient search by record type

**Storage Interface**:
```go
type CoreStorage interface {
    // Basic CRUD operations
    Get(ctx context.Context, name string, recordType RecordType) ([]*DNSRecord, error)
    List(ctx context.Context) ([]*DNSRecord, error)
    Create(ctx context.Context, record *DNSRecord) error
    Update(ctx context.Context, record *DNSRecord) error
    Delete(ctx context.Context, name string, recordType RecordType) error

    // Hot reload support
    HotReload(ctx context.Context, records []*DNSRecord) error
    PartialReload(ctx context.Context, changes *RecordChanges) error

    // Watch for changes
    Watch(ctx context.Context) (<-chan StorageEvent, error)
}

type RecordChanges struct {
    Added   []*DNSRecord
    Updated []*DNSRecord
    Deleted []RecordKey
}

type RecordKey struct {
    Name string
    Type RecordType
}

type StorageEvent struct {
    Type   EventType   // Added, Updated, Deleted, Reloaded
    Record *DNSRecord
}
```

**Hot Reload Requirements**:
1. **Partial Reload**: Only reload changed records, not entire dataset
2. **Zero Downtime**: DNS queries continue during reload
3. **Atomic Updates**: Changes are applied atomically
4. **Rollback Support**: Can rollback on error
5. **Event Notification**: Notify watchers of changes

**Implementation**:
```go
type MemoryStorage struct {
    mu      sync.RWMutex
    records map[string]map[RecordType][]*DNSRecord  // domain -> type -> records
    version uint64  // For optimistic locking
}

// Hot reload implementation
func (s *MemoryStorage) PartialReload(ctx context.Context, changes *RecordChanges) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Apply changes atomically
    for _, record := range changes.Added {
        s.addRecord(record)
    }

    for _, record := range changes.Updated {
        s.updateRecord(record)
    }

    for _, key := range changes.Deleted {
        s.deleteRecord(key.Name, key.Type)
    }

    s.version++

    // Notify watchers
    s.notifyWatchers(changes)

    return nil
}
```

### 3. Backend Layer - Response Generator

**Responsibility**: Read configuration and return DNS responses based on stored records

**Requirements**:
```go
type DNSBackend interface {
    // Resolve returns DNS records for a query
    Resolve(ctx context.Context, query *QueryInfo) ([]*DNSRecord, error)

    // ApplyRules applies any transformation rules
    ApplyRules(ctx context.Context, records []*DNSRecord) ([]*DNSRecord, error)
}
```

**Response Generation**:
1. Lookup records from Core Storage
2. Apply any configured rules (e.g., filtering, transformation)
3. If not found locally, forward to upstream DNS (if enabled)
4. Format response according to DNS protocol
5. Handle special cases (NXDOMAIN, CNAME chains, etc.)

**Configuration-Based Behavior**:
```yaml
# Backend configuration
backend:
  # Default TTL if not specified
  default_ttl: 300

  # Enable CNAME chain resolution
  resolve_cname_chain: true

  # Maximum CNAME chain depth
  max_cname_depth: 10

  # Return SOA on NXDOMAIN
  return_soa_on_nxdomain: true

  # Upstream DNS forwarding (for recursive queries)
  upstream:
    enabled: true
    servers:
      - "1.1.1.1:53"
      - "8.8.8.8:53"  # fallback
    timeout: "5s"
```

### 3.1 Upstream DNS Forwarding

**Purpose**: Enable recursive DNS resolution by forwarding unknown queries to upstream DNS servers

**Architecture**:
```go
type Forwarder struct {
    config ForwarderConfig
    client *dns.Client
}

type ForwarderConfig struct {
    Enabled bool          // Enable upstream forwarding
    Servers []string      // Upstream DNS server addresses
    Timeout time.Duration // Query timeout
}
```

**Resolution Flow**:
1. Query local storage first
2. If not found and upstream enabled:
   - Forward query to first upstream server
   - On timeout/network error: try next server
   - On NXDOMAIN/SERVFAIL: return immediately (no retry)
3. Convert upstream response to internal DNSRecord format
4. Return to frontend

**Fallback Logic**:
- Try each upstream server in order
- Network errors trigger fallback to next server
- Authoritative negative responses (NXDOMAIN, SERVFAIL) are final
- If all servers fail, return ErrRecordNotFound

**Implementation**:
```go
func (f *Forwarder) Forward(ctx context.Context, domain string, qtype uint16) ([]*DNSRecord, error) {
    query := new(dns.Msg)
    query.SetQuestion(domain, qtype)
    query.RecursionDesired = true

    for _, server := range f.config.Servers {
        resp, _, err := f.client.ExchangeContext(ctx, query, server)
        if err != nil {
            // Network error, try next server
            continue
        }

        if resp.Rcode == dns.RcodeNameError || resp.Rcode == dns.RcodeServerFailure {
            // Authoritative negative response, don't retry
            return nil, fmt.Errorf("upstream %s: %s", server, dns.RcodeToString[resp.Rcode])
        }

        // Convert dns.RR to DNSRecord
        return f.rrToRecords(resp.Answer), nil
    }

    return nil, ErrRecordNotFound
}
```

**Use Cases**:
- **Hybrid DNS**: Authoritative for managed domains, recursive for others
- **Split-horizon DNS**: Internal domains from storage, external from upstream
- **Development**: Local overrides with fallback to public DNS

### 4. Management Interfaces

**Direct Storage Access**: Both HTTP API and ConfigMap watcher directly access Core Storage

#### HTTP Management API

**Endpoints**:
```
GET    /api/v1/records           # List all DNS records
GET    /api/v1/records/:name     # Get specific record
POST   /api/v1/records           # Create new record
PUT    /api/v1/records/:name     # Update record
DELETE /api/v1/records/:name     # Delete record
POST   /api/v1/reload            # Trigger hot reload

GET    /health                   # Health check
GET    /ready                    # Readiness check
```

**Direct Storage Integration**:
```go
type HTTPHandler struct {
    storage CoreStorage  // Direct access to core storage
}

func (h *HTTPHandler) CreateRecord(w http.ResponseWriter, r *http.Request) {
    var record DNSRecord
    json.NewDecoder(r.Body).Decode(&record)

    // Directly write to core storage
    err := h.storage.Create(r.Context(), &record)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(record)
}
```

#### ConfigMap Integration

**Direct Storage Sync**:
```go
type ConfigMapWatcher struct {
    storage  CoreStorage  // Direct access to core storage
    client   kubernetes.Interface
    namespace string
    name     string
}

func (w *ConfigMapWatcher) Watch(ctx context.Context) error {
    watcher, err := w.client.CoreV1().ConfigMaps(w.namespace).Watch(ctx, metav1.ListOptions{
        FieldSelector: fmt.Sprintf("metadata.name=%s", w.name),
    })

    for event := range watcher.ResultChan() {
        cm := event.Object.(*corev1.ConfigMap)

        // Parse records from ConfigMap
        records, err := w.parseConfigMap(cm)
        if err != nil {
            continue
        }

        // Calculate changes
        changes := w.calculateChanges(records)

        // Directly update core storage with hot reload
        err = w.storage.PartialReload(ctx, changes)
        if err != nil {
            slog.Error("failed to reload from ConfigMap", "error", err)
        }
    }

    return nil
}
```

**Bidirectional Sync**:
- **ConfigMap → Storage**: Watch ConfigMap changes, hot reload storage
- **Storage → ConfigMap**: Persist API changes back to ConfigMap

```go
func (w *ConfigMapWatcher) PersistToConfigMap(ctx context.Context, record *DNSRecord) error {
    // Get current ConfigMap
    cm, err := w.client.CoreV1().ConfigMaps(w.namespace).Get(ctx, w.name, metav1.GetOptions{})
    if err != nil {
        return err
    }

    // Update ConfigMap data
    records := w.parseConfigMap(cm)
    records = append(records, record)
    cm.Data["config.yaml"] = w.serializeRecords(records)

    // Update ConfigMap
    _, err = w.client.CoreV1().ConfigMaps(w.namespace).Update(ctx, cm, metav1.UpdateOptions{})
    return err
}
```

## Hot Reload Implementation

### Partial Hot Reload

**Requirements**:
1. Only reload changed records
2. No service interruption
3. Atomic updates
4. Event-driven notifications

**Change Detection**:
```go
func (s *MemoryStorage) calculateChanges(newRecords []*DNSRecord) *RecordChanges {
    s.mu.RLock()
    defer s.mu.RUnlock()

    changes := &RecordChanges{
        Added:   []*DNSRecord{},
        Updated: []*DNSRecord{},
        Deleted: []RecordKey{},
    }

    // Build maps for comparison
    oldMap := s.buildRecordMap()
    newMap := buildRecordMapFromSlice(newRecords)

    // Find added and updated
    for key, newRecord := range newMap {
        if oldRecord, exists := oldMap[key]; exists {
            if !recordsEqual(oldRecord, newRecord) {
                changes.Updated = append(changes.Updated, newRecord)
            }
        } else {
            changes.Added = append(changes.Added, newRecord)
        }
    }

    // Find deleted
    for key := range oldMap {
        if _, exists := newMap[key]; !exists {
            changes.Deleted = append(changes.Deleted, key)
        }
    }

    return changes
}
```

### Reload Triggers

1. **ConfigMap Update**: Automatic hot reload when ConfigMap changes
2. **API Trigger**: Manual reload via `/api/v1/reload` endpoint
3. **Signal**: SIGHUP signal triggers reload

```go
func (s *Server) setupReloadSignal() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGHUP)

    go func() {
        for range sigCh {
            slog.Info("SIGHUP received, triggering hot reload")
            if err := s.triggerReload(context.Background()); err != nil {
                slog.Error("hot reload failed", "error", err)
            }
        }
    }()
}
```

## ACME Challenge Support

### DNS-01 Challenge

**Integration with Core Storage**:
```go
type DNS01Handler struct {
    storage CoreStorage  // Direct access to core storage
}

func (h *DNS01Handler) SetChallenge(ctx context.Context, domain, token string) error {
    record := &DNSRecord{
        Name:  fmt.Sprintf("_acme-challenge.%s", domain),
        Type:  RecordTypeTXT,
        TTL:   60,
        Value: []string{token},
    }

    // Directly create in core storage
    return h.storage.Create(ctx, record)
}

func (h *DNS01Handler) ClearChallenge(ctx context.Context, domain string) error {
    name := fmt.Sprintf("_acme-challenge.%s", domain)
    return h.storage.Delete(ctx, name, RecordTypeTXT)
}
```

### HTTP-01 Challenge

**Integration with HTTP Server**:
```go
type HTTP01Handler struct {
    challenges sync.Map  // token -> keyAuth
}

func (h *HTTP01Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract token from path: /.well-known/acme-challenge/:token
    token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")

    if keyAuth, ok := h.challenges.Load(token); ok {
        w.Header().Set("Content-Type", "text/plain")
        w.Write([]byte(keyAuth.(string)))
        return
    }

    http.NotFound(w, r)
}
```

### Cert-Manager Compatibility

**Webhook Server for DNS-01**:
```go
// Implement cert-manager webhook interface
type CertManagerWebhook struct {
    storage CoreStorage
}

func (w *CertManagerWebhook) Present(ctx context.Context, ch *v1alpha1.ChallengeRequest) error {
    record := &DNSRecord{
        Name:  ch.ResolvedFQDN,
        Type:  RecordTypeTXT,
        TTL:   60,
        Value: []string{ch.Key},
    }

    return w.storage.Create(ctx, record)
}

func (w *CertManagerWebhook) CleanUp(ctx context.Context, ch *v1alpha1.ChallengeRequest) error {
    return w.storage.Delete(ctx, ch.ResolvedFQDN, RecordTypeTXT)
}
```

**HTTP-01 Ingress Integration**:
- Expose HTTP-01 endpoint via Kubernetes Service
- Cert-manager creates Ingress pointing to jw238dns service
- jw238dns serves challenge responses

## DNS Record Structure

```go
type DNSRecord struct {
    Name  string     `json:"name" yaml:"name"`     // FQDN (e.g., "example.com.")
    Type  RecordType `json:"type" yaml:"type"`     // Record type (A, AAAA, CNAME, etc.)
    TTL   uint32     `json:"ttl" yaml:"ttl"`       // Time to live in seconds
    Value []string   `json:"value" yaml:"value"`   // Record values (can be multiple)
}

type RecordType string

const (
    RecordTypeA     RecordType = "A"
    RecordTypeAAAA  RecordType = "AAAA"
    RecordTypeCNAME RecordType = "CNAME"
    RecordTypeMX    RecordType = "MX"
    RecordTypeTXT   RecordType = "TXT"
    RecordTypeNS    RecordType = "NS"
    RecordTypeSRV   RecordType = "SRV"
    RecordTypePTR   RecordType = "PTR"
    RecordTypeSOA   RecordType = "SOA"
    RecordTypeCAA   RecordType = "CAA"
)
```

## ConfigMap Format

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jw238dns-config
  namespace: default
data:
  config.yaml: |
    dns_port: 53
    http_port: 8080
    log_level: info

    records:
      - name: example.com.
        type: A
        ttl: 300
        value:
          - 192.168.1.1

      - name: www.example.com.
        type: CNAME
        ttl: 300
        value:
          - example.com.

      - name: _acme-challenge.example.com.
        type: TXT
        ttl: 60
        value:
          - "challenge-token-here"
```

## Performance Requirements

- **DNS Query Response**: < 10ms for cached records
- **Hot Reload Time**: < 100ms for partial reload
- **Throughput**: Support 1000+ queries per second
- **Memory Efficiency**: O(n) memory usage where n = number of records
- **Concurrent Operations**: Support 100+ concurrent API requests

## Error Handling

```go
var (
    ErrRecordNotFound    = errors.New("DNS record not found")
    ErrRecordExists      = errors.New("DNS record already exists")
    ErrInvalidRecordType = errors.New("invalid DNS record type")
    ErrInvalidTTL        = errors.New("TTL must be between 60 and 86400")
    ErrInvalidName       = errors.New("invalid domain name")
    ErrReloadFailed      = errors.New("hot reload failed")
    ErrStorageLocked     = errors.New("storage is locked during update")
)
```

## Module Structure

```
dns/
├── frontend.go          # DNS query receiver and parser
├── frontend_test.go     # Frontend tests
├── backend.go           # Response generator with storage integration
├── backend_test.go      # Backend tests
├── forward.go           # Upstream DNS forwarder (NEW)
├── forward_test.go      # Forwarder tests (NEW)
├── helpers.go           # Shared utilities
└── helpers_test.go      # Helper tests
```

**New Module: forward.go**
- Encapsulates upstream DNS forwarding logic
- Independent from backend core logic
- Fully tested with comprehensive test coverage
- Supports multiple upstream servers with fallback
- Converts upstream responses to internal format

---

**Last Updated**: 2026-02-13
**Project**: jw238dns - Cloud-Native DNS Module

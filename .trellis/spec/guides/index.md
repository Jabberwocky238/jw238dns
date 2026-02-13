# Development Guidelines - jw238dns

## Overview

This document defines the development standards for the jw238dns cloud-native DNS module.

## Core Principles

1. **Read Before Write** - Understand context and requirements before coding
2. **Follow Standards** - Adhere to Go best practices and project conventions
3. **Type Safety** - Leverage Go's type system for compile-time safety
4. **Error Handling** - Always handle errors explicitly with context
5. **Testing** - Write tests for all critical functionality
6. **Documentation** - Document all exported functions and types

## Technology Stack

- **Language**: Go 1.21+
- **Deployment**: Kubernetes (cloud-native, runs in-cluster)
- **Configuration**: ConfigMap persistence
- **Protocols**: DNS, HTTP
- **Certificate Management**: Delegated to control plane (not handled by this service)

## Project Structure

**IMPORTANT**: `main.go` must be placed in the `cmd/` folder

```
jw238dns/
├── cmd/
│   └── jw238dns/          # Main application entry [REQUIRED]
│       └── main.go        # Application entry point
├── internal/
│   ├── dns/               # DNS resolution logic
│   │   ├── frontend.go    # DNS query receiver (all query types)
│   │   ├── frontend_test.go
│   │   ├── backend.go     # DNS response generator
│   │   ├── backend_test.go
│   │   ├── server.go      # DNS server
│   │   ├── server_test.go
│   │   └── handler.go     # DNS query handler
│   │       └── handler_test.go
│   ├── storage/           # Core storage
│   │   ├── storage.go     # Storage interface
│   │   ├── storage_test.go
│   │   ├── memory.go      # In-memory implementation
│   │   ├── memory_test.go
│   │   └── reload.go      # Hot reload logic
│   │       └── reload_test.go
│   ├── api/               # HTTP management API
│   │   ├── server.go      # HTTP server
│   │   ├── server_test.go
│   │   ├── handlers.go    # API handlers
│   │   ├── handlers_test.go
│   │   └── routes.go      # Route definitions
│   │       └── routes_test.go
│   ├── config/            # Configuration management
│   │   ├── config.go      # Config structures
│   │   ├── config_test.go
│   │   ├── loader.go      # Config loading
│   │   ├── loader_test.go
│   │   └── k8s.go         # ConfigMap integration
│   │       └── k8s_test.go
│   └── types/             # Shared types
│       ├── record.go      # DNS record types
│       └── record_test.go
├── pkg/                   # Public libraries (if needed)
├── deployments/
│   └── kubernetes/        # K8s manifests
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── configmap.yaml
│       └── rbac.yaml
├── test/
│   ├── integration/       # Integration tests
│   └── e2e/               # End-to-end tests
├── go.mod
├── go.sum
└── README.md
```

## Entry Point Requirements

### cmd/jw238dns/main.go

**MUST** be the application entry point:

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/jw238dns/internal/api"
    "github.com/yourusername/jw238dns/internal/config"
    "github.com/yourusername/jw238dns/internal/dns"
    "github.com/yourusername/jw238dns/internal/storage"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        slog.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    // Setup logger
    setupLogger(cfg)

    // Initialize core storage
    store := storage.NewMemoryStorage()

    // Start DNS server
    dnsServer := dns.NewServer(store, cfg)
    go dnsServer.Start()

    // Start HTTP API server
    apiServer := api.NewServer(store, cfg)
    go apiServer.Start()

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    <-sigCh

    slog.Info("shutting down...")
}
```

## Kubernetes Naming Constraints

### DNS-1123 Subdomain Rules

When creating Kubernetes resources (Secrets, ConfigMaps, Services, etc.), names must follow DNS-1123 subdomain rules:

**Allowed**:
- Lowercase alphanumeric characters: `a-z`, `0-9`
- Hyphens: `-`
- Dots: `.` (for some resources)

**Not Allowed**:
- Underscores: `_`
- Uppercase letters
- Special characters

**Length Limits**:
- Maximum 253 characters
- Must start and end with alphanumeric character

**Example Error**:
```
Secret "tls-wildcard--__mesh-worker_cloud" is invalid:
metadata.name: Invalid value: "tls-wildcard--__mesh-worker_cloud":
a lowercase RFC 1123 subdomain must consist of lower case alphanumeric
characters, '-' or '.', and must start and end with an alphanumeric character
```

**Common Mistakes**:

1. ❌ Using underscores in generated names:
   ```go
   // Bad: generates names with underscores
   secretName := "tls-" + strings.ReplaceAll(domain, ".", "_")
   // Result: "tls-example_com" (invalid!)
   ```

2. ✅ Use hyphens instead:
   ```go
   // Good: use hyphens
   secretName := "tls-" + strings.ReplaceAll(domain, ".", "-")
   // Result: "tls-example-com" (valid)
   ```

**Domain-to-Resource Name Mapping Challenges**:

When mapping domain names to Kubernetes resource names, be aware of these challenges:

1. **Domains can contain hyphens**: `mesh-worker.cloud`
2. **Bidirectional mapping is complex**: Hard to distinguish between hyphens that represent dots vs. original hyphens
3. **International domains**: Punycode adds complexity
4. **Wildcard certificates**: `*.example.com` needs special handling

**Recommendation**: For complex naming schemes (like certificate management), consider delegating to specialized control plane components (cert-manager, external-dns) rather than implementing custom mapping logic.

## Kubernetes Deployment Requirements

### In-Cluster Execution

The application **MUST** run inside a Kubernetes cluster:

1. **In-Cluster Configuration**: Use `rest.InClusterConfig()`
2. **ServiceAccount**: Require ServiceAccount with ConfigMap permissions
3. **RBAC**: Define Role and RoleBinding for ConfigMap access
4. **Environment Variables**: Use downward API for namespace detection

```go
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func NewK8sClient() (kubernetes.Interface, error) {
    // MUST use in-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create clientset: %w", err)
    }

    return clientset, nil
}
```

### RBAC Configuration

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: jw238dns
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: jw238dns
  namespace: default
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: jw238dns
  namespace: default
subjects:
  - kind: ServiceAccount
    name: jw238dns
    namespace: default
roleRef:
  kind: Role
  name: jw238dns
  apiGroup: rbac.authorization.k8s.io
```

## Coding Standards

### 1. Error Handling

Always wrap errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

Use custom error types for domain errors:

```go
type DNSRecordNotFoundError struct {
    Name string
}

func (e *DNSRecordNotFoundError) Error() string {
    return fmt.Sprintf("DNS record not found: %s", e.Name)
}
```

### 2. Logging

Use Go's standard `log/slog` package for structured logging:

```go
import "log/slog"

slog.Info("DNS query received",
    "domain", domain,
    "type", queryType,
    "client", clientIP)

slog.Error("failed to resolve DNS",
    "domain", domain,
    "error", err)
```

**Log Levels**:
- **INFO**: Application lifecycle, successful operations
- **WARN**: Recoverable errors, invalid but handled requests
- **ERROR**: Failed operations, unrecoverable errors
- **DEBUG**: Detailed request/response data, internal state

### 3. Configuration

Use struct tags for configuration:

```go
type Config struct {
    DNSPort       int    `yaml:"dns_port" env:"DNS_PORT" default:"53"`
    HTTPPort      int    `yaml:"http_port" env:"HTTP_PORT" default:"8080"`
    ConfigMapName string `yaml:"configmap_name" env:"CONFIGMAP_NAME"`
    Namespace     string `yaml:"namespace" env:"NAMESPACE" default:"default"`
}
```

### 4. Context Usage

Always pass context for cancellation and timeout:

```go
func (s *DNSServer) HandleQuery(ctx context.Context, query *dns.Msg) (*dns.Msg, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        // Handle query
    }
}
```

### 5. Type Safety

Define strong types for domain concepts:

```go
type RecordType string

const (
    RecordTypeA     RecordType = "A"
    RecordTypeAAAA  RecordType = "AAAA"
    RecordTypeCNAME RecordType = "CNAME"
    RecordTypeMX    RecordType = "MX"
    RecordTypeTXT   RecordType = "TXT"
)

func (rt RecordType) IsValid() bool {
    switch rt {
    case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeMX, RecordTypeTXT:
        return true
    default:
        return false
    }
}
```

### 6. Interface Design

Keep interfaces small and focused:

```go
type RecordStorage interface {
    Get(ctx context.Context, name string, recordType RecordType) ([]*DNSRecord, error)
    List(ctx context.Context) ([]*DNSRecord, error)
    Create(ctx context.Context, record *DNSRecord) error
    Update(ctx context.Context, record *DNSRecord) error
    Delete(ctx context.Context, name string, recordType RecordType) error
}
```

## Testing Requirements

**CRITICAL**: Every module and sub-module MUST have a corresponding `*_test.go` file

### Test File Naming Convention

```
internal/dns/frontend.go       → internal/dns/frontend_test.go
internal/dns/backend.go        → internal/dns/backend_test.go
internal/storage/memory.go     → internal/storage/memory_test.go
internal/acme/dns01.go         → internal/acme/dns01_test.go
internal/api/handlers.go       → internal/api/handlers_test.go
```

### Test Coverage Requirements

- **Core modules**: >90% coverage
- **Storage layer**: >95% coverage (critical path)
- **DNS handlers**: >85% coverage
- **API handlers**: >80% coverage
- **ACME handlers**: >90% coverage

### Example Test Structure

```go
func TestMemoryStorage_Create(t *testing.T) {
    tests := []struct {
        name    string
        record  *DNSRecord
        wantErr bool
    }{
        {
            name: "create A record",
            record: &DNSRecord{
                Name:  "example.com.",
                Type:  RecordTypeA,
                TTL:   300,
                Value: []string{"192.168.1.1"},
            },
            wantErr: false,
        },
        {
            name: "duplicate record",
            record: &DNSRecord{
                Name:  "example.com.",
                Type:  RecordTypeA,
                TTL:   300,
                Value: []string{"192.168.1.1"},
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := NewMemoryStorage()
            err := store.Create(context.Background(), tt.record)

            if (err != nil) != tt.wantErr {
                t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Security Considerations

1. **Input Validation**: Validate all DNS queries and API inputs
2. **Rate Limiting**: Implement rate limiting for both DNS and HTTP
3. **RBAC**: Use Kubernetes RBAC for ConfigMap access
4. **TLS**: Support TLS for management API
5. **DNS Amplification**: Prevent DNS amplification attacks

## Performance Guidelines

1. **Caching**: Implement DNS response caching
2. **Connection Pooling**: Reuse connections where possible
3. **Goroutines**: Use goroutines for concurrent request handling
4. **Resource Limits**: Set appropriate memory and CPU limits

## Architecture Decisions

### Separation of Concerns: When to Delegate to Control Plane

**Principle**: Not all functionality should be implemented in every component. Some features are better handled by specialized control plane components.

**When to Delegate**:

1. **Certificate Management**
   - ❌ Don't: Implement ACME protocol in DNS service
   - ✅ Do: Use cert-manager or external certificate management
   - **Why**: Certificate management involves complex state management, renewal logic, and multiple CA integrations. Specialized tools handle this better.

2. **Complex Resource Naming/Mapping**
   - ❌ Don't: Implement bidirectional domain-to-resource name mapping with edge cases
   - ✅ Do: Use standard naming conventions or delegate to control plane
   - **Why**: Edge cases (domains with hyphens, international domains, wildcards) make custom mapping schemes fragile and hard to maintain.

3. **Multi-Tenant Resource Management**
   - ❌ Don't: Implement custom resource isolation and RBAC
   - ✅ Do: Use Kubernetes native RBAC and namespace isolation
   - **Why**: Kubernetes already provides robust multi-tenancy features.

**Decision Criteria**:

Ask these questions when considering adding a feature:

1. **Complexity**: Does this feature have many edge cases or require complex state management?
2. **Specialization**: Are there existing tools that specialize in this functionality?
3. **Maintenance**: Will this feature require ongoing updates to handle new scenarios?
4. **Scope**: Is this feature outside the core responsibility of this service?

If you answer "yes" to 2 or more questions, consider delegating to control plane or specialized components.

**Example from this project**:

We initially attempted to implement ACME certificate management with domain-to-Secret name mapping. We discovered:
- Kubernetes DNS-1123 naming rules are restrictive (no underscores)
- Bidirectional mapping is complex (domains can contain hyphens)
- International domains (Punycode) add more complexity
- Certificate renewal, storage, and CA integration require significant code

**Decision**: Remove ACME implementation, delegate to cert-manager. This simplified the codebase and leveraged a mature, well-tested solution.

## Documentation Requirements

1. **Code Comments**: Document all exported functions and types
2. **README.md**: Project overview, quick start, deployment guide
3. **API.md**: Complete API documentation with examples
4. **DEPLOYMENT.md**: Kubernetes deployment instructions

## Related Documents

- `../dnscore/index.md` - DNS core requirements and specifications
- `../tests/index.md` - Testing standards and guidelines

---

**Last Updated**: 2026-02-12
**Project**: jw238dns - Cloud-Native DNS Module

# Backend Development Guidelines

> Go DNS Server for Kubernetes - Backend Specifications

---

## Overview

This is a Go-based DNS server designed for Kubernetes environments. The project provides:
- DNS server with support for A, AAAA, CNAME, TXT, SRV, MX, NS records
- Pluggable storage layer (ConfigMap, JSON file)
- ACME DNS-01 certificate automation
- HTTP management API (Gin-based, non-RESTful)

---

## Quick Start

Before writing any backend code, read these documents in order:

1. **[Project Architecture](./architecture.md)** - System design and component relationships
2. **[Storage Layer Guidelines](./storage-guidelines.md)** - Storage interface and implementations
3. **[DNS Server Guidelines](./dns-guidelines.md)** - DNS server implementation standards
4. **[Error Handling](./error-handling.md)** - Error handling patterns
5. **[Type Safety](./type-safety.md)** - Go type safety and validation
6. **[Logging Guidelines](./logging-guidelines.md)** - Structured logging standards
7. **[Testing Guidelines](./testing-guidelines.md)** - Unit and integration testing

---

## Document Status

| Document | Status | Last Updated |
|----------|--------|--------------|
| architecture.md | ✅ Complete | 2026-02-12 |
| storage-guidelines.md | ✅ Complete | 2026-02-12 |
| dns-guidelines.md | ✅ Complete | 2026-02-12 |
| error-handling.md | ✅ Complete | 2026-02-12 |
| type-safety.md | ✅ Complete | 2026-02-12 |
| logging-guidelines.md | ✅ Complete | 2026-02-12 |
| testing-guidelines.md | ✅ Complete | 2026-02-12 |

---

## Technology Stack

### Core Libraries
- **DNS**: `github.com/miekg/dns` - DNS protocol implementation
- **ACME**: `github.com/go-acme/lego/v4` - ACME client for Let's Encrypt
- **HTTP**: `github.com/gin-gonic/gin` - HTTP framework for management API
- **K8s**: `k8s.io/client-go` - Kubernetes client library

### Supporting Libraries
- **Logging**: `go.uber.org/zap` or standard `log/slog`
- **Metrics**: `github.com/prometheus/client_golang`
- **Testing**: Standard `testing` package + `github.com/stretchr/testify`

---

## Project Structure

```
.
├── cmd/
│   └── dnsd/              # Main entry point
│       └── main.go
├── pkg/
│   ├── storage/           # Storage layer abstraction
│   │   ├── interface.go   # Storage interface
│   │   ├── types.go       # DNS record types
│   │   ├── configmap.go   # K8s ConfigMap implementation
│   │   └── jsonfile.go    # JSON file implementation
│   ├── dns/               # DNS server
│   │   ├── server.go      # DNS server core
│   │   ├── handler.go     # Query handler
│   │   └── metrics.go     # Prometheus metrics
│   ├── acme/              # ACME certificate management
│   │   ├── client.go      # ACME client wrapper
│   │   ├── dns01.go       # DNS-01 provider
│   │   └── manager.go     # Certificate manager
│   └── http/              # HTTP management API
│       ├── server.go      # HTTP server
│       ├── handler_dns.go # DNS record handlers
│       └── handler_cert.go# Certificate handlers
├── config/
│   └── config.yaml        # Configuration example
└── go.mod
```

---

## Code Quality Standards

### Linting
```bash
golangci-lint run ./...
```

### Testing
```bash
go test -v -race -cover ./...
```

### Build
```bash
go build -o bin/dnsd ./cmd/dnsd
```

---

## Development Workflow

1. Read relevant spec documents
2. Implement feature following guidelines
3. Write unit tests (minimum 80% coverage)
4. Run linter and fix issues
5. Manual testing
6. Commit with conventional commit message

---

## Commit Convention

```
type(scope): description

Types: feat, fix, refactor, test, docs, chore
Scope: storage, dns, acme, http, config
```

Examples:
- `feat(storage): add ConfigMap storage implementation`
- `fix(dns): handle NXDOMAIN correctly`
- `refactor(acme): simplify DNS-01 provider`

---

## Related Documents

- [Cross-Layer Thinking Guide](../guides/cross-layer-thinking-guide.md)
- [Debugging Guide](../guides/debugging-guide.md)

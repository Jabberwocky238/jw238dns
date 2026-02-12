# DNS Core Implementation

## Goal
Implement the core DNS functionality including Frontend (query receiver), Backend (response generator), and Core Storage (in-memory storage with hot reload capability).

## Requirements

### 1. Core Types (`internal/types/`)
- Define `DNSRecord` struct with Name, Type, TTL, Value fields
- Define `RecordType` constants (A, AAAA, CNAME, MX, TXT, NS, SRV, PTR, SOA, CAA)
- Define `QueryInfo`, `RecordKey`, `RecordChanges`, `StorageEvent` types
- Define sentinel errors (ErrRecordNotFound, ErrRecordExists, etc.)
- Add validation methods (IsValid for RecordType)

### 2. Core Storage (`internal/storage/`)
- Implement `CoreStorage` interface with CRUD operations
- Implement `MemoryStorage` with thread-safe map-based storage
- Support concurrent reads/writes using `sync.RWMutex`
- Implement hot reload functionality:
  - `HotReload`: Full reload
  - `PartialReload`: Only reload changed records
  - Change detection with `calculateChanges`
  - Atomic updates with zero downtime
- Implement `Watch` for event notifications

### 3. DNS Frontend (`internal/dns/`)
- Implement `DNSFrontend` interface
- Receive and parse all DNS query types (A, AAAA, CNAME, MX, TXT, etc.)
- Validate query format
- Use `github.com/miekg/dns` library
- Support both UDP and TCP on port 53

### 4. DNS Backend (`internal/dns/`)
- Implement `DNSBackend` interface
- Resolve queries from Core Storage
- Handle CNAME chain resolution (max depth: 10)
- Return SOA on NXDOMAIN
- Apply configurable rules

## Acceptance Criteria

- [ ] All types defined in `internal/types/record.go` with tests
- [ ] `CoreStorage` interface and `MemoryStorage` implementation complete
- [ ] Hot reload works with partial updates (only changed records)
- [ ] Thread-safe concurrent operations verified with tests
- [ ] DNS Frontend can parse all supported query types
- [ ] DNS Backend resolves queries correctly
- [ ] CNAME chain resolution works with depth limit
- [ ] All modules have corresponding `*_test.go` files
- [ ] Test coverage >90% for storage, >85% for DNS handlers
- [ ] All tests pass with `go test ./...`
- [ ] Code passes `go vet` and `golangci-lint`

## Technical Notes

- Module path: `jabberwocky238/jw238dns`
- Go version: 1.24.2
- Required dependency: `github.com/miekg/dns`
- Entry point will be at `cmd/jw238dns/main.go` (not part of this task)
- Use structured logging with `log/slog`
- Follow table-driven test pattern from spec
- All exported functions must have godoc comments

## Out of Scope (Future Tasks)
- HTTP Management API
- ConfigMap integration
- ACME challenge handlers
- Kubernetes deployment
- Main application entry point

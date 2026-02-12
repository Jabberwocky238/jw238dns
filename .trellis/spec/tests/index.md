# Testing Standards - jw238dns

## Overview

This document defines the testing standards and guidelines for the jw238dns DNS module.

## Testing Principles

1. **Test Coverage**: Aim for >80% code coverage for critical paths
2. **Test Isolation**: Each test should be independent and repeatable
3. **Fast Tests**: Unit tests should run in milliseconds
4. **Clear Assertions**: Test failures should clearly indicate what went wrong
5. **Table-Driven Tests**: Use table-driven tests for multiple scenarios

## Test Structure

```
jw238dns/
├── internal/
│   ├── dns/
│   │   ├── server.go
│   │   ├── server_test.go      # Unit tests
│   │   ├── handler.go
│   │   └── handler_test.go
│   ├── acme/
│   │   ├── dns01.go
│   │   ├── dns01_test.go
│   │   ├── http01.go
│   │   └── http01_test.go
│   └── storage/
│       ├── configmap.go
│       └── configmap_test.go
└── test/
    ├── integration/            # Integration tests
    │   ├── dns_test.go
    │   └── api_test.go
    └── e2e/                    # End-to-end tests
        └── full_test.go
```

## Unit Testing

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestDNSHandler_HandleQuery(t *testing.T) {
    tests := []struct {
        name     string
        query    string
        qtype    uint16
        expected string
        wantErr  bool
    }{
        {
            name:     "A record query",
            query:    "example.com.",
            qtype:    dns.TypeA,
            expected: "192.168.1.1",
            wantErr:  false,
        },
        {
            name:     "AAAA record query",
            query:    "example.com.",
            qtype:    dns.TypeAAAA,
            expected: "2001:db8::1",
            wantErr:  false,
        },
        {
            name:     "non-existent domain",
            query:    "notfound.com.",
            qtype:    dns.TypeA,
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := NewDNSHandler(mockStorage)
            result, err := handler.HandleQuery(tt.query, tt.qtype)

            if (err != nil) != tt.wantErr {
                t.Errorf("HandleQuery() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !tt.wantErr && result != tt.expected {
                t.Errorf("HandleQuery() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Test Helpers

Create helper functions for common test setup:

```go
func setupTestStorage(t *testing.T) *storage.MemoryStorage {
    t.Helper()

    store := storage.NewMemoryStorage()

    // Add test records
    store.Create(context.Background(), &DNSRecord{
        Name:  "example.com.",
        Type:  RecordTypeA,
        TTL:   300,
        Value: []string{"192.168.1.1"},
    })

    return store
}

func TestWithStorage(t *testing.T) {
    store := setupTestStorage(t)

    // Use store in test
    record, err := store.Get(context.Background(), "example.com.")
    if err != nil {
        t.Fatalf("failed to get record: %v", err)
    }

    if record.Name != "example.com." {
        t.Errorf("got %s, want example.com.", record.Name)
    }
}
```

### Mocking

Use interfaces for mocking dependencies:

```go
// Mock storage for testing
type MockStorage struct {
    GetFunc    func(ctx context.Context, name string) (*DNSRecord, error)
    ListFunc   func(ctx context.Context) ([]*DNSRecord, error)
    CreateFunc func(ctx context.Context, record *DNSRecord) error
}

func (m *MockStorage) Get(ctx context.Context, name string) (*DNSRecord, error) {
    if m.GetFunc != nil {
        return m.GetFunc(ctx, name)
    }
    return nil, errors.New("not implemented")
}

// Usage in tests
func TestDNSServer_WithMock(t *testing.T) {
    mockStore := &MockStorage{
        GetFunc: func(ctx context.Context, name string) (*DNSRecord, error) {
            return &DNSRecord{
                Name:  name,
                Type:  RecordTypeA,
                TTL:   300,
                Value: []string{"192.168.1.1"},
            }, nil
        },
    }

    server := NewDNSServer(mockStore)
    // Test server with mock
}
```

## Integration Testing

### DNS Server Integration Tests

```go
// +build integration

func TestDNSServer_Integration(t *testing.T) {
    // Start test DNS server
    store := storage.NewMemoryStorage()
    server := dns.NewServer(store, ":15353")

    go server.Start()
    defer server.Stop()

    time.Sleep(100 * time.Millisecond) // Wait for server to start

    // Create DNS client
    client := new(dns.Client)
    msg := new(dns.Msg)
    msg.SetQuestion("example.com.", dns.TypeA)

    // Send query
    resp, _, err := client.Exchange(msg, "127.0.0.1:15353")
    if err != nil {
        t.Fatalf("DNS query failed: %v", err)
    }

    // Verify response
    if len(resp.Answer) == 0 {
        t.Error("expected DNS answer, got none")
    }
}
```

### HTTP API Integration Tests

```go
// +build integration

func TestHTTPAPI_Integration(t *testing.T) {
    // Start test HTTP server
    store := storage.NewMemoryStorage()
    server := api.NewServer(store, ":18080")

    go server.Start()
    defer server.Stop()

    time.Sleep(100 * time.Millisecond)

    // Test create record
    record := DNSRecord{
        Name:  "test.com.",
        Type:  RecordTypeA,
        TTL:   300,
        Value: []string{"192.168.1.1"},
    }

    body, _ := json.Marshal(record)
    resp, err := http.Post("http://localhost:18080/api/v1/records",
        "application/json",
        bytes.NewBuffer(body))

    if err != nil {
        t.Fatalf("HTTP request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        t.Errorf("expected status 201, got %d", resp.StatusCode)
    }
}
```

## Kubernetes Testing

### Mock Kubernetes Client

```go
import (
    "k8s.io/client-go/kubernetes/fake"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigMapStorage(t *testing.T) {
    // Create fake Kubernetes client
    fakeClient := fake.NewSimpleClientset()

    // Create test ConfigMap
    cm := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-config",
            Namespace: "default",
        },
        Data: map[string]string{
            "config.yaml": `
records:
  - name: example.com.
    type: A
    ttl: 300
    value:
      - 192.168.1.1
`,
        },
    }

    _, err := fakeClient.CoreV1().ConfigMaps("default").Create(
        context.Background(),
        cm,
        metav1.CreateOptions{},
    )
    if err != nil {
        t.Fatalf("failed to create ConfigMap: %v", err)
    }

    // Test ConfigMap storage
    store := storage.NewConfigMapStorage(fakeClient, "default", "test-config")
    records, err := store.List(context.Background())

    if err != nil {
        t.Fatalf("failed to list records: %v", err)
    }

    if len(records) != 1 {
        t.Errorf("expected 1 record, got %d", len(records))
    }
}
```

## ACME Challenge Testing

### DNS-01 Challenge Tests

```go
func TestDNS01Handler(t *testing.T) {
    store := storage.NewMemoryStorage()
    handler := acme.NewDNS01Handler(store)

    // Set challenge
    err := handler.SetChallenge(context.Background(), "example.com", "test-token")
    if err != nil {
        t.Fatalf("failed to set challenge: %v", err)
    }

    // Verify TXT record created
    record, err := store.Get(context.Background(), "_acme-challenge.example.com.")
    if err != nil {
        t.Fatalf("failed to get challenge record: %v", err)
    }

    if record.Type != RecordTypeTXT {
        t.Errorf("expected TXT record, got %s", record.Type)
    }

    if record.Value[0] != "test-token" {
        t.Errorf("expected token 'test-token', got %s", record.Value[0])
    }

    // Clear challenge
    err = handler.ClearChallenge(context.Background(), "example.com")
    if err != nil {
        t.Fatalf("failed to clear challenge: %v", err)
    }

    // Verify record removed
    _, err = store.Get(context.Background(), "_acme-challenge.example.com.")
    if err == nil {
        t.Error("expected record to be removed")
    }
}
```

### HTTP-01 Challenge Tests

```go
func TestHTTP01Handler(t *testing.T) {
    handler := acme.NewHTTP01Handler()

    // Set challenge
    token := "test-token"
    keyAuth := "test-key-auth"

    err := handler.SetChallenge(context.Background(), token, keyAuth)
    if err != nil {
        t.Fatalf("failed to set challenge: %v", err)
    }

    // Get challenge
    result, err := handler.GetChallenge(context.Background(), token)
    if err != nil {
        t.Fatalf("failed to get challenge: %v", err)
    }

    if result != keyAuth {
        t.Errorf("expected %s, got %s", keyAuth, result)
    }

    // Clear challenge
    err = handler.ClearChallenge(context.Background(), token)
    if err != nil {
        t.Fatalf("failed to clear challenge: %v", err)
    }

    // Verify removed
    _, err = handler.GetChallenge(context.Background(), token)
    if err == nil {
        t.Error("expected challenge to be removed")
    }
}
```

## Benchmarking

### DNS Query Benchmarks

```go
func BenchmarkDNSHandler_HandleQuery(b *testing.B) {
    store := setupTestStorage(b)
    handler := NewDNSHandler(store)

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := handler.HandleQuery("example.com.", dns.TypeA)
        if err != nil {
            b.Fatalf("query failed: %v", err)
        }
    }
}

func BenchmarkDNSCache_Get(b *testing.B) {
    cache := NewDNSCache()
    cache.Set("example.com.", &DNSRecord{
        Name:  "example.com.",
        Type:  RecordTypeA,
        TTL:   300,
        Value: []string{"192.168.1.1"},
    })

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, _ = cache.Get("example.com.")
    }
}
```

## Test Commands

### Run All Tests

```bash
go test ./...
```

### Run with Coverage

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Integration Tests

```bash
go test -tags=integration ./test/integration/...
```

### Run Benchmarks

```bash
go test -bench=. ./...
go test -bench=. -benchmem ./...
```

### Run Specific Test

```bash
go test -run TestDNSHandler_HandleQuery ./internal/dns/
```

## Test Quality Checklist

- [ ] All critical paths have unit tests
- [ ] Edge cases are covered
- [ ] Error conditions are tested
- [ ] Integration tests verify component interaction
- [ ] Mocks are used for external dependencies
- [ ] Tests are fast and independent
- [ ] Test names clearly describe what is being tested
- [ ] Assertions have clear error messages

## Continuous Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: go test -v -cover ./...

      - name: Run integration tests
        run: go test -v -tags=integration ./test/integration/...
```

---

**Last Updated**: 2026-02-12
**Project**: jw238dns - Cloud-Native DNS Module

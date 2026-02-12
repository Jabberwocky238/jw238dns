# Error Handling Guidelines

> Standards for error handling in Go DNS Server

---

## Error Handling Principles

1. **Wrap errors with context** - Always add context when propagating errors
2. **Use sentinel errors** - Define package-level errors for common cases
3. **Distinguish error types** - Separate temporary vs permanent errors
4. **Log appropriately** - Error vs Warning vs Info
5. **Return early** - Fail fast, don't nest error handling

---

## Sentinel Errors

Define package-level errors for common cases:

```go
package storage

import "errors"

var (
    ErrNotFound          = errors.New("record not found")
    ErrAlreadyExists     = errors.New("record already exists")
    ErrInvalidRecord     = errors.New("invalid record")
    ErrConflict          = errors.New("update conflict")
    ErrStorageUnavailable = errors.New("storage unavailable")
)
```

---

## Error Wrapping

Always wrap errors with context using `fmt.Errorf` with `%w`:

```go
// Good
func (s *ConfigMapStorage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("get configmap %s/%s: %w", s.namespace, s.name, err)
    }
    // ...
}

// Bad - No context
func (s *ConfigMapStorage) Get(ctx context.Context, domain, recordType string) (*DNSRecord, error) {
    cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
    if err != nil {
        return nil, err // Lost context!
    }
    // ...
}
```

---

## Error Checking

Use `errors.Is()` and `errors.As()` for error comparison:

```go
// Good - Check sentinel error
record, err := storage.Get(ctx, domain, recordType)
if errors.Is(err, storage.ErrNotFound) {
    // Try wildcard match
    record, err = storage.Get(ctx, wildcardDomain, recordType)
}

// Bad - String comparison
if err != nil && err.Error() == "record not found" {
    // Fragile!
}
```

```go
// Good - Extract error type
var netErr *net.OpError
if errors.As(err, &netErr) {
    if netErr.Temporary() {
        // Retry
    }
}
```

---

## HTTP Error Responses

Unified error response format:

```go
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

func handleError(c *gin.Context, err error) {
    var code int
    var message string

    switch {
    case errors.Is(err, storage.ErrNotFound):
        code = 404
        message = "Record not found"
    case errors.Is(err, storage.ErrAlreadyExists):
        code = 409
        message = "Record already exists"
    case errors.Is(err, storage.ErrInvalidRecord):
        code = 400
        message = "Invalid record"
    default:
        code = 500
        message = "Internal server error"
    }

    c.JSON(code, ErrorResponse{
        Code:    code,
        Message: message,
        Details: err.Error(),
    })
}
```

---

## Retry Logic

Implement retry for temporary errors:

```go
func retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
    backoff := time.Second

    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }

        // Check if error is retryable
        if !isRetryable(err) {
            return err
        }

        // Check context
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
            backoff *= 2 // Exponential backoff
        }
    }

    return fmt.Errorf("max retries exceeded")
}

func isRetryable(err error) bool {
    // K8s conflict errors are retryable
    if errors.IsConflict(err) {
        return true
    }

    // Network temporary errors are retryable
    var netErr *net.OpError
    if errors.As(err, &netErr) && netErr.Temporary() {
        return true
    }

    return false
}
```

---

## Logging Errors

Use structured logging with appropriate levels:

```go
import "log/slog"

// Error - Unexpected errors that need attention
func (s *Server) handleQuery(ctx context.Context, q *dns.Question) error {
    record, err := s.storage.Get(ctx, q.Name, dns.TypeToString[q.Qtype])
    if err != nil {
        if errors.Is(err, storage.ErrNotFound) {
            // Expected, log at debug level
            slog.Debug("record not found",
                "domain", q.Name,
                "type", dns.TypeToString[q.Qtype])
            return nil
        }
        // Unexpected error, log at error level
        slog.Error("failed to get record",
            "domain", q.Name,
            "error", err)
        return err
    }
    return nil
}

// Warning - Recoverable issues
func (s *ACMEManager) renewCertificate(domain string) error {
    cert, err := s.getCertificate(domain)
    if err != nil {
        slog.Warn("failed to get certificate for renewal",
            "domain", domain,
            "error", err)
        return err
    }
    // ...
}
```

---

## Panic Recovery

Recover from panics in goroutines:

```go
func (s *Server) Start() error {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                slog.Error("panic in dns server",
                    "panic", r,
                    "stack", string(debug.Stack()))
            }
        }()

        if err := s.udpServer.ListenAndServe(); err != nil {
            slog.Error("udp server error", "error", err)
        }
    }()

    return nil
}
```

---

## Validation Errors

Provide detailed validation errors:

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

func ValidateRecord(r *DNSRecord) error {
    var errs []error

    if r.Domain == "" {
        errs = append(errs, &ValidationError{
            Field:   "domain",
            Message: "domain is required",
        })
    }

    if !isValidRecordType(r.Type) {
        errs = append(errs, &ValidationError{
            Field:   "type",
            Message: fmt.Sprintf("unsupported record type: %s", r.Type),
        })
    }

    if len(errs) > 0 {
        return fmt.Errorf("validation failed: %w", errors.Join(errs...))
    }

    return nil
}
```

---

## Common Mistakes

### ❌ Don't: Ignore Errors

```go
// Bad
storage.Create(ctx, record)

// Good
if err := storage.Create(ctx, record); err != nil {
    return fmt.Errorf("create record: %w", err)
}
```

### ❌ Don't: Use Panic for Expected Errors

```go
// Bad
func (s *Storage) Get(domain string) *DNSRecord {
    record, err := s.fetch(domain)
    if err != nil {
        panic(err) // Don't panic!
    }
    return record
}

// Good
func (s *Storage) Get(domain string) (*DNSRecord, error) {
    record, err := s.fetch(domain)
    if err != nil {
        return nil, fmt.Errorf("fetch record: %w", err)
    }
    return record, nil
}
```

### ❌ Don't: Swallow Errors

```go
// Bad
func processRecords(records []*DNSRecord) {
    for _, r := range records {
        if err := validate(r); err != nil {
            continue // Silently skip!
        }
        // ...
    }
}

// Good
func processRecords(records []*DNSRecord) error {
    var errs []error
    for _, r := range records {
        if err := validate(r); err != nil {
            errs = append(errs, fmt.Errorf("validate %s: %w", r.Domain, err))
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

---

## Best Practices

1. **Return errors, don't panic** (except for programmer errors)
2. **Wrap errors with context** using `fmt.Errorf` with `%w`
3. **Use sentinel errors** for common cases
4. **Check errors with `errors.Is()` and `errors.As()`**
5. **Log errors at appropriate levels** (Error/Warn/Debug)
6. **Provide actionable error messages** (what went wrong, why, how to fix)
7. **Implement retry logic** for temporary errors
8. **Validate input early** and return detailed validation errors
9. **Recover from panics** in goroutines
10. **Test error paths** in unit tests

# DNS Server Guidelines

> Standards for implementing the DNS server component

---

## Overview

The DNS server handles DNS queries using the `miekg/dns` library and resolves records from the storage layer.

---

## Supported Record Types

Must support these 7 record types:

| Type | Description | Example Value |
|------|-------------|---------------|
| A | IPv4 address | `1.2.3.4` |
| AAAA | IPv6 address | `2001:db8::1` |
| CNAME | Canonical name | `target.example.com.` |
| TXT | Text record | `"v=spf1 include:_spf.example.com ~all"` |
| SRV | Service record | `10 60 5060 sipserver.example.com.` |
| MX | Mail exchange | `10 mail.example.com.` |
| NS | Name server | `ns1.example.com.` |

---

## Server Structure

```go
package dns

import (
    "context"
    "github.com/miekg/dns"
    "your-project/pkg/storage"
)

type Server struct {
    storage    storage.Storage
    udpServer  *dns.Server
    tcpServer  *dns.Server
    config     *Config
    metrics    *Metrics
}

type Config struct {
    ListenAddr string
    UDPEnabled bool
    TCPEnabled bool
    CacheEnabled bool
    CacheTTL   int
}

func NewServer(storage storage.Storage, config *Config) *Server {
    s := &Server{
        storage: storage,
        config:  config,
        metrics: NewMetrics(),
    }

    // Setup UDP server
    if config.UDPEnabled {
        s.udpServer = &dns.Server{
            Addr: config.ListenAddr,
            Net:  "udp",
            Handler: dns.HandlerFunc(s.ServeDNS),
        }
    }

    // Setup TCP server
    if config.TCPEnabled {
        s.tcpServer = &dns.Server{
            Addr: config.ListenAddr,
            Net:  "tcp",
            Handler: dns.HandlerFunc(s.ServeDNS),
        }
    }

    return s
}
```

---

## Query Handler

```go
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative = true

    // Process each question
    for _, q := range r.Question {
        s.handleQuestion(m, &q)
    }

    // Record metrics
    s.metrics.RecordQuery(r.Question[0].Qtype, len(m.Answer) > 0)

    w.WriteMsg(m)
}

func (s *Server) handleQuestion(m *dns.Msg, q *dns.Question) {
    domain := q.Name
    qtype := dns.TypeToString[q.Qtype]

    // Query storage
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    record, err := s.storage.Get(ctx, normalizeDomain(domain), qtype)
    if err != nil {
        if errors.Is(err, storage.ErrNotFound) {
            // Try wildcard match
            record, err = s.findWildcardMatch(ctx, domain, qtype)
        }
        if err != nil {
            // No record found, return empty answer
            return
        }
    }

    // Convert storage record to DNS RR
    rr := s.recordToRR(record, q.Name)
    if rr != nil {
        m.Answer = append(m.Answer, rr)
    }
}
```

---

## Record Type Handlers

### A Record

```go
func (s *Server) handleA(record *storage.DNSRecord, qname string) dns.RR {
    return &dns.A{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeA,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        A: net.ParseIP(record.Value),
    }
}
```

### AAAA Record

```go
func (s *Server) handleAAAA(record *storage.DNSRecord, qname string) dns.RR {
    return &dns.AAAA{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeAAAA,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        AAAA: net.ParseIP(record.Value),
    }
}
```

### CNAME Record

```go
func (s *Server) handleCNAME(record *storage.DNSRecord, qname string) dns.RR {
    return &dns.CNAME{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeCNAME,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        Target: dns.Fqdn(record.Value),
    }
}
```

### TXT Record

```go
func (s *Server) handleTXT(record *storage.DNSRecord, qname string) dns.RR {
    return &dns.TXT{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeTXT,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        Txt: []string{record.Value},
    }
}
```

### MX Record

```go
func (s *Server) handleMX(record *storage.DNSRecord, qname string) dns.RR {
    priority := uint16(10) // Default
    if record.Priority != nil {
        priority = uint16(*record.Priority)
    }

    return &dns.MX{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeMX,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        Preference: priority,
        Mx:         dns.Fqdn(record.Value),
    }
}
```

### SRV Record

```go
func (s *Server) handleSRV(record *storage.DNSRecord, qname string) dns.RR {
    // SRV format: priority weight port target
    priority := uint16(10)
    weight := uint16(10)
    port := uint16(80)

    if record.Priority != nil {
        priority = uint16(*record.Priority)
    }
    if record.Weight != nil {
        weight = uint16(*record.Weight)
    }
    if record.Port != nil {
        port = uint16(*record.Port)
    }

    return &dns.SRV{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeSRV,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        Priority: priority,
        Weight:   weight,
        Port:     port,
        Target:   dns.Fqdn(record.Value),
    }
}
```

### NS Record

```go
func (s *Server) handleNS(record *storage.DNSRecord, qname string) dns.RR {
    return &dns.NS{
        Hdr: dns.RR_Header{
            Name:   qname,
            Rrtype: dns.TypeNS,
            Class:  dns.ClassINET,
            Ttl:    uint32(record.TTL),
        },
        Ns: dns.Fqdn(record.Value),
    }
}
```

---

## Wildcard Matching

```go
func (s *Server) findWildcardMatch(ctx context.Context, domain, qtype string) (*storage.DNSRecord, error) {
    // Try matching *.example.com for subdomain.example.com
    parts := strings.Split(domain, ".")
    if len(parts) < 2 {
        return nil, storage.ErrNotFound
    }

    // Build wildcard pattern
    wildcard := "*." + strings.Join(parts[1:], ".")

    return s.storage.Get(ctx, wildcard, qtype)
}
```

---

## Server Lifecycle

### Start

```go
func (s *Server) Start() error {
    errChan := make(chan error, 2)

    if s.udpServer != nil {
        go func() {
            if err := s.udpServer.ListenAndServe(); err != nil {
                errChan <- fmt.Errorf("udp server: %w", err)
            }
        }()
    }

    if s.tcpServer != nil {
        go func() {
            if err := s.tcpServer.ListenAndServe(); err != nil {
                errChan <- fmt.Errorf("tcp server: %w", err)
            }
        }()
    }

    // Wait for first error
    return <-errChan
}
```

### Graceful Shutdown

```go
func (s *Server) Shutdown(ctx context.Context) error {
    var errs []error

    if s.udpServer != nil {
        if err := s.udpServer.ShutdownContext(ctx); err != nil {
            errs = append(errs, fmt.Errorf("udp shutdown: %w", err))
        }
    }

    if s.tcpServer != nil {
        if err := s.tcpServer.ShutdownContext(ctx); err != nil {
            errs = append(errs, fmt.Errorf("tcp shutdown: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    return nil
}
```

---

## Metrics

```go
type Metrics struct {
    queriesTotal   *prometheus.CounterVec
    queryDuration  *prometheus.HistogramVec
    cacheHits      prometheus.Counter
    cacheMisses    prometheus.Counter
}

func (m *Metrics) RecordQuery(qtype uint16, success bool) {
    status := "success"
    if !success {
        status = "nxdomain"
    }

    m.queriesTotal.WithLabelValues(
        dns.TypeToString[qtype],
        status,
    ).Inc()
}
```

---

## Testing

### Unit Test Example

```go
func TestServer_HandleA(t *testing.T) {
    mockStorage := &mockStorage{
        records: map[string]*storage.DNSRecord{
            "example.com:A": {
                Domain: "example.com",
                Type:   "A",
                Value:  "1.2.3.4",
                TTL:    300,
            },
        },
    }

    server := NewServer(mockStorage, &Config{})

    // Create DNS query
    m := new(dns.Msg)
    m.SetQuestion("example.com.", dns.TypeA)

    // Create response writer
    w := &testResponseWriter{}

    // Handle query
    server.ServeDNS(w, m)

    // Verify response
    assert.Equal(t, 1, len(w.msg.Answer))
    assert.Equal(t, "1.2.3.4", w.msg.Answer[0].(*dns.A).A.String())
}
```

---

## Common Mistakes

### ❌ Don't: Forget FQDN Trailing Dot

```go
// Bad - Missing trailing dot
Target: "example.com"

// Good - FQDN with trailing dot
Target: dns.Fqdn("example.com") // Returns "example.com."
```

### ❌ Don't: Block on Storage Calls

```go
// Bad - No timeout
record, err := s.storage.Get(context.Background(), domain, qtype)

// Good - With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
record, err := s.storage.Get(ctx, domain, qtype)
```

### ❌ Don't: Ignore Query Class

```go
// Bad - Assumes INET class
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    // Process without checking class
}

// Good - Check class
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    for _, q := range r.Question {
        if q.Qclass != dns.ClassINET {
            continue // Skip non-INET queries
        }
        s.handleQuestion(m, &q)
    }
}
```

---

## Best Practices

1. **Always use dns.Fqdn() for domain names in responses**
2. **Set appropriate TTL values (default: 300s)**
3. **Implement query logging for debugging**
4. **Add Prometheus metrics for monitoring**
5. **Support both UDP and TCP protocols**
6. **Handle NXDOMAIN gracefully (empty answer section)**
7. **Implement wildcard matching for flexibility**
8. **Use context with timeout for storage calls**
9. **Test with real DNS clients (dig, nslookup)**
10. **Document supported record types clearly**

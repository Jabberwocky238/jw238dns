# Add Upstream DNS Forwarding (1.1.1.1)

## Goal
Add recursive DNS query support to the DNS server. When a domain is not found in local storage, forward the query to upstream DNS server (1.1.1.1) instead of returning NXDOMAIN.

## Background
- Current DNS server is authoritative-only: returns NXDOMAIN for unknown domains
- This causes issues when the DNS server is set as the system resolver
- Users expect recursive behavior: resolve configured domains locally, forward others upstream

## Requirements

### 1. Add Upstream Configuration
- Add `UpstreamConfig` struct to `cmd/jw238dns/main.go`:
  - `Enabled bool` - enable/disable upstream forwarding
  - `Servers []string` - list of upstream DNS servers (default: `["1.1.1.1:53"]`)
  - `Timeout string` - query timeout (default: `"5s"`)
- Add `upstream` section to YAML config structure

### 2. Extend BackendConfig
- Add to `dns.BackendConfig`:
  - `EnableUpstreamForwarding bool`
  - `UpstreamServers []string`
  - `UpstreamTimeout time.Duration`
- Update `DefaultBackendConfig()` with sensible defaults

### 3. Implement Upstream Forwarding in Backend
- Add `forwardToUpstream()` method to `dns.Backend`:
  - Use `dns.Client` from miekg/dns library
  - Try each upstream server in order until success
  - Convert upstream `*dns.Msg` response to `[]*types.DNSRecord`
  - Handle timeouts and errors gracefully
- Modify `Resolve()` method:
  - After local storage miss and CNAME chain miss
  - Before returning `ErrRecordNotFound`
  - Call `forwardToUpstream()` if enabled

### 4. Wire Configuration
- In `cmd/jw238dns/main.go`, load upstream config from YAML
- Pass upstream settings to `BackendConfig` during initialization
- Parse timeout string to `time.Duration`

### 5. Update K8s Deployment
- Add upstream config to `assets/k8s-deployment.yaml` ConfigMap
- Set default upstream to `1.1.1.1:53`

### 6. Add Tests
- Test upstream forwarding on local miss
- Test upstream disabled returns NXDOMAIN
- Test upstream timeout handling
- Test multiple upstream servers (fallback)
- Test upstream response parsing (A, AAAA, CNAME, etc.)

## Acceptance Criteria
- [ ] `BackendConfig` has upstream forwarding fields
- [ ] `Backend.Resolve()` forwards to upstream when local record not found
- [ ] Upstream uses `1.1.1.1:53` by default
- [ ] Config supports multiple upstream servers with fallback
- [ ] Timeout is configurable (default 5s)
- [ ] Upstream forwarding can be disabled via config
- [ ] K8s deployment YAML includes upstream config
- [ ] Tests pass for upstream forwarding scenarios
- [ ] Lint and typecheck pass

## Technical Notes

### miekg/dns Client Usage
```go
client := &dns.Client{
    Net:     "udp",
    Timeout: 5 * time.Second,
}

// Build query message
query := new(dns.Msg)
query.SetQuestion(domain, qtype)
query.RecursionDesired = true

// Forward to upstream
resp, _, err := client.ExchangeContext(ctx, query, "1.1.1.1:53")
```

### Response Conversion
Convert `dns.Msg.Answer` RRs back to `types.DNSRecord`:
- Extract Name, Type, TTL from RR header
- Parse RData based on type (A, AAAA, CNAME, etc.)
- Return slice of `DNSRecord` structs

### Config Example
```yaml
dns:
  listen: "0.0.0.0:53"
  tcp_enabled: true
  udp_enabled: true
  upstream:
    enabled: true
    servers:
      - "1.1.1.1:53"
      - "8.8.8.8:53"  # fallback
    timeout: "5s"
```

### Fallback Logic
Try each upstream server in order:
1. Query first server
2. If timeout or network error, try next server
3. If NXDOMAIN or SERVFAIL, return immediately (don't retry)
4. If all servers fail, return `ErrRecordNotFound`

## Dependencies
- None (standalone feature)

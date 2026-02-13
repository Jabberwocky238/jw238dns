# DNS Records Configuration Examples

This document provides examples of DNS record configurations for jw238dns.

---

## Record Format

DNS records in jw238dns use the following structure:

```yaml
records:
  - name: "example.com."      # Fully qualified domain name (must end with .)
    type: "A"                 # Record type
    ttl: 300                  # Time to live in seconds
    value:                    # Record values (array)
      - "192.168.1.1"
```

**Important:** Domain names MUST end with a dot (`.`) to be fully qualified.

---

## Basic Record Types

### A Record (IPv4 Address)

```yaml
records:
  # Single IP address
  - name: "example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.1"

  # Multiple IP addresses (load balancing)
  - name: "app.example.com."
    type: "A"
    ttl: 300
    value:
      - "10.0.0.1"
      - "10.0.0.2"
      - "10.0.0.3"
```

### AAAA Record (IPv6 Address)

```yaml
records:
  - name: "example.com."
    type: "AAAA"
    ttl: 300
    value:
      - "2001:db8::1"

  - name: "ipv6.example.com."
    type: "AAAA"
    ttl: 300
    value:
      - "2001:db8::1"
      - "2001:db8::2"
```

### CNAME Record (Canonical Name)

```yaml
records:
  # Point www to apex domain
  - name: "www.example.com."
    type: "CNAME"
    ttl: 300
    value:
      - "example.com."

  # Point subdomain to external service
  - name: "blog.example.com."
    type: "CNAME"
    ttl: 300
    value:
      - "myblog.wordpress.com."
```

### TXT Record (Text)

```yaml
records:
  # SPF record
  - name: "example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=spf1 include:_spf.google.com ~all"

  # Domain verification
  - name: "example.com."
    type: "TXT"
    ttl: 300
    value:
      - "google-site-verification=abc123xyz"

  # DKIM record
  - name: "default._domainkey.example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQ..."

  # Multiple TXT records for same domain
  - name: "example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=spf1 include:_spf.google.com ~all"
      - "google-site-verification=abc123"
```

### MX Record (Mail Exchange)

```yaml
records:
  # Single mail server
  - name: "example.com."
    type: "MX"
    ttl: 300
    value:
      - "10 mail.example.com."

  # Multiple mail servers with priority
  - name: "example.com."
    type: "MX"
    ttl: 300
    value:
      - "10 mail1.example.com."
      - "20 mail2.example.com."
      - "30 mail3.example.com."
```

### NS Record (Name Server)

```yaml
records:
  - name: "example.com."
    type: "NS"
    ttl: 86400
    value:
      - "ns1.example.com."
      - "ns2.example.com."

  # Subdomain delegation
  - name: "subdomain.example.com."
    type: "NS"
    ttl: 3600
    value:
      - "ns1.subdomain.example.com."
      - "ns2.subdomain.example.com."
```

### SRV Record (Service)

```yaml
records:
  # SIP service
  - name: "_sip._tcp.example.com."
    type: "SRV"
    ttl: 300
    value:
      - "10 60 5060 sipserver.example.com."
      # Format: priority weight port target

  # XMPP service
  - name: "_xmpp-client._tcp.example.com."
    type: "SRV"
    ttl: 300
    value:
      - "5 0 5222 xmpp.example.com."

  # Minecraft server
  - name: "_minecraft._tcp.example.com."
    type: "SRV"
    ttl: 300
    value:
      - "0 5 25565 mc.example.com."
```

### CAA Record (Certificate Authority Authorization)

```yaml
records:
  # Allow Let's Encrypt to issue certificates
  - name: "example.com."
    type: "CAA"
    ttl: 300
    value:
      - "0 issue letsencrypt.org"

  # Allow multiple CAs
  - name: "example.com."
    type: "CAA"
    ttl: 300
    value:
      - "0 issue letsencrypt.org"
      - "0 issue digicert.com"

  # Wildcard certificate authorization
  - name: "example.com."
    type: "CAA"
    ttl: 300
    value:
      - "0 issuewild letsencrypt.org"

  # Report violations
  - name: "example.com."
    type: "CAA"
    ttl: 300
    value:
      - "0 issue letsencrypt.org"
      - "0 iodef mailto:security@example.com"
```

---

## Complete Domain Example

```yaml
records:
  # Apex domain A records (with GeoIP load balancing)
  - name: "example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.1"
      - "192.168.1.2"
      - "192.168.1.3"

  # Apex domain AAAA record
  - name: "example.com."
    type: "AAAA"
    ttl: 300
    value:
      - "2001:db8::1"

  # WWW subdomain
  - name: "www.example.com."
    type: "CNAME"
    ttl: 300
    value:
      - "example.com."

  # API subdomain
  - name: "api.example.com."
    type: "A"
    ttl: 300
    value:
      - "10.0.0.10"

  # Mail servers
  - name: "example.com."
    type: "MX"
    ttl: 300
    value:
      - "10 mail1.example.com."
      - "20 mail2.example.com."

  - name: "mail1.example.com."
    type: "A"
    ttl: 300
    value:
      - "10.0.0.20"

  - name: "mail2.example.com."
    type: "A"
    ttl: 300
    value:
      - "10.0.0.21"

  # SPF record
  - name: "example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=spf1 mx include:_spf.google.com ~all"

  # DMARC record
  - name: "_dmarc.example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"

  # CAA records
  - name: "example.com."
    type: "CAA"
    ttl: 300
    value:
      - "0 issue letsencrypt.org"
      - "0 issuewild letsencrypt.org"
```

---

## Wildcard Records

```yaml
records:
  # Wildcard A record (matches any subdomain)
  - name: "*.example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.100"

  # Wildcard CNAME
  - name: "*.app.example.com."
    type: "CNAME"
    ttl: 300
    value:
      - "app.example.com."

  # Note: Wildcard does NOT match the apex domain
  # *.example.com matches foo.example.com but NOT example.com
```

---

## ACME DNS-01 Challenge Records

```yaml
records:
  # ACME challenge for Let's Encrypt
  - name: "_acme-challenge.example.com."
    type: "TXT"
    ttl: 60
    value:
      - "challenge-token-here"

  # Wildcard certificate challenge
  - name: "_acme-challenge.example.com."
    type: "TXT"
    ttl: 60
    value:
      - "wildcard-challenge-token"
```

**Note:** These records are typically managed automatically by the ACME client.

---

## Multi-Region Setup (GeoIP)

```yaml
records:
  # Global load balancing with GeoIP
  # Closest IP will be returned first based on client location
  - name: "global.example.com."
    type: "A"
    ttl: 60
    value:
      - "192.168.1.1"   # US East
      - "192.168.2.1"   # US West
      - "192.168.3.1"   # Europe
      - "192.168.4.1"   # Asia
```

---

## Development/Testing Records

```yaml
records:
  # Local development
  - name: "dev.example.com."
    type: "A"
    ttl: 60
    value:
      - "127.0.0.1"

  # Staging environment
  - name: "staging.example.com."
    type: "A"
    ttl: 300
    value:
      - "10.0.0.100"

  # Testing subdomain
  - name: "test.example.com."
    type: "CNAME"
    ttl: 60
    value:
      - "staging.example.com."
```

---

## Kubernetes Service Records

```yaml
records:
  # Kubernetes service
  - name: "k8s-app.example.com."
    type: "A"
    ttl: 60
    value:
      - "10.96.0.10"

  # LoadBalancer external IP
  - name: "lb.example.com."
    type: "A"
    ttl: 300
    value:
      - "203.0.113.10"

  # Multiple cluster nodes
  - name: "cluster.example.com."
    type: "A"
    ttl: 60
    value:
      - "10.0.1.10"
      - "10.0.1.11"
      - "10.0.1.12"
```

---

## Common Patterns

### Redirect WWW to Apex

```yaml
records:
  # Apex domain
  - name: "example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.1"

  # WWW points to apex
  - name: "www.example.com."
    type: "CNAME"
    ttl: 300
    value:
      - "example.com."
```

### CDN Configuration

```yaml
records:
  # Main site
  - name: "example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.1"

  # CDN subdomain
  - name: "cdn.example.com."
    type: "CNAME"
    ttl: 3600
    value:
      - "d111111abcdef8.cloudfront.net."

  # Static assets
  - name: "static.example.com."
    type: "CNAME"
    ttl: 3600
    value:
      - "cdn.example.com."
```

### Email Configuration

```yaml
records:
  # MX records
  - name: "example.com."
    type: "MX"
    ttl: 300
    value:
      - "10 mail.example.com."

  # Mail server A record
  - name: "mail.example.com."
    type: "A"
    ttl: 300
    value:
      - "192.168.1.10"

  # SPF
  - name: "example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=spf1 mx a:mail.example.com ~all"

  # DKIM
  - name: "default._domainkey.example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA..."

  # DMARC
  - name: "_dmarc.example.com."
    type: "TXT"
    ttl: 300
    value:
      - "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"
```

---

## TTL Guidelines

| Use Case | Recommended TTL | Reason |
|----------|----------------|--------|
| Static records | 3600-86400 (1-24 hours) | Rarely change, can cache longer |
| Dynamic records | 60-300 (1-5 minutes) | May change frequently |
| Load balanced | 60-300 (1-5 minutes) | Allow quick failover |
| ACME challenges | 60 (1 minute) | Need quick propagation |
| Development | 60 (1 minute) | Frequent changes during testing |
| Production | 300-3600 (5-60 minutes) | Balance between cache and flexibility |

---

## File Storage Format (JSON)

For local file storage, use JSON format:

```json
{
  "records": [
    {
      "name": "example.com.",
      "type": "A",
      "ttl": 300,
      "value": ["192.168.1.1"]
    },
    {
      "name": "www.example.com.",
      "type": "CNAME",
      "ttl": 300,
      "value": ["example.com."]
    }
  ]
}
```

---

## ConfigMap Format (Kubernetes)

For Kubernetes ConfigMap storage:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jw238dns-records
  namespace: jw238dns
data:
  records.yaml: |
    records:
      - name: "example.com."
        type: "A"
        ttl: 300
        value:
          - "192.168.1.1"

      - name: "www.example.com."
        type: "CNAME"
        ttl: 300
        value:
          - "example.com."
```

---

## Validation Rules

1. **Domain names must end with a dot (.)**
   - ✓ `example.com.`
   - ✗ `example.com`

2. **Record types must be uppercase**
   - ✓ `A`, `AAAA`, `CNAME`
   - ✗ `a`, `aaaa`, `cname`

3. **TTL must be between 60 and 86400 seconds**
   - ✓ `300` (5 minutes)
   - ✗ `30` (too short)
   - ✗ `100000` (too long)

4. **Values must be arrays**
   - ✓ `value: ["192.168.1.1"]`
   - ✗ `value: "192.168.1.1"`

5. **CNAME targets must be FQDN**
   - ✓ `value: ["example.com."]`
   - ✗ `value: ["example.com"]`

6. **MX records must include priority**
   - ✓ `value: ["10 mail.example.com."]`
   - ✗ `value: ["mail.example.com."]`

---

## Best Practices

1. **Always use FQDN** - End domain names with a dot
2. **Use appropriate TTL** - Balance between cache and flexibility
3. **Test before production** - Use low TTL during testing
4. **Document your records** - Add comments in YAML
5. **Use version control** - Track changes to DNS records
6. **Monitor DNS queries** - Check logs for issues
7. **Plan for failover** - Use multiple A records for redundancy
8. **Secure with CAA** - Restrict certificate issuance
9. **Validate email** - Use SPF, DKIM, and DMARC
10. **Regular backups** - Keep copies of DNS configurations

---

**Last Updated:** 2026-02-13
**Version:** 1.0.0

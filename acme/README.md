# ACME DNS-01 Challenge Support

This package implements DNS-01 ACME challenge support for automatic TLS certificate management using Let's Encrypt and other ACME-compatible Certificate Authorities.

## Features

- ✅ DNS-01 challenge provider integrated with storage layer
- ✅ Automatic certificate issuance and renewal
- ✅ Wildcard certificate support (`*.example.com`)
- ✅ Multi-domain (SAN) certificates
- ✅ Kubernetes Secret storage backend
- ✅ File system storage backend
- ✅ Configurable renewal policies
- ✅ Let's Encrypt production and staging support
- ✅ ZeroSSL support with External Account Binding (EAB)
- ✅ Mode switching between Let's Encrypt and ZeroSSL

## Architecture

```
┌─────────────┐
│   Manager   │  Certificate lifecycle management
└──────┬──────┘
       │
       ├──────────┐
       │          │
┌──────▼──────┐  ┌▼────────────┐
│   Client    │  │   Storage   │
│  (lego)     │  │  (K8s/File) │
└──────┬──────┘  └─────────────┘
       │
┌──────▼──────┐
│ DNS01       │  Integrates with DNS storage
│ Provider    │  to create TXT records
└─────────────┘
```

## Usage

### Basic Setup

```go
import (
    "context"
    "jabberwocky238/jw238dns/internal/acme"
    "jabberwocky238/jw238dns/internal/storage"
)

// Create DNS storage (for DNS-01 challenges)
dnsStore := storage.NewMemoryStorage()

// Create certificate storage (Kubernetes)
certStorage := acme.NewKubernetesSecretStorage(k8sClient, "default")

// Configure ACME
config := &acme.Config{
    ServerURL:       acme.LetsEncryptProduction(),
    Email:           "admin@example.com",
    KeyType:         "RSA2048",
    AutoRenew:       true,
    CheckInterval:   24 * time.Hour,
    RenewBefore:     30 * 24 * time.Hour,
    PropagationWait: 60 * time.Second,
}

// Create manager
manager, err := acme.NewManager(config, dnsStore, certStorage)
if err != nil {
    log.Fatal(err)
}

// Obtain certificate
ctx := context.Background()
err = manager.ObtainCertificate(ctx, []string{"example.com", "www.example.com"})
if err != nil {
    log.Fatal(err)
}

// Start automatic renewal
manager.StartAutoRenewal(ctx)
```

### Wildcard Certificates

```go
// Wildcard certificates require DNS-01 challenge
domains := []string{"*.example.com", "example.com"}
err := manager.ObtainCertificate(ctx, domains)
```

### ZeroSSL with EAB

```go
// ZeroSSL requires External Account Binding credentials
config := &acme.Config{
    ServerURL: acme.ZeroSSLProduction(),
    Email:     "admin@example.com",
    KeyType:   "RSA2048",
    EAB: acme.EABConfig{
        KID:     os.Getenv("ACME_EAB_KID"),
        HMACKey: os.Getenv("ACME_EAB_HMAC"),
    },
}
```

### File Storage Backend

```go
// Use file system instead of Kubernetes
certStorage := acme.NewFileStorage("/var/lib/jw238dns/certs")
```

## DNS-01 Challenge Flow

1. **Challenge Request**: ACME CA requests DNS-01 challenge
2. **TXT Record Creation**: Provider creates `_acme-challenge.example.com` TXT record
3. **Propagation Wait**: Wait for DNS propagation (configurable)
4. **Validation**: ACME CA validates the TXT record
5. **Certificate Issuance**: CA issues the certificate
6. **Cleanup**: Provider removes the TXT record

## Certificate Storage

### Kubernetes Secrets

Certificates are stored as TLS secrets:

```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: tls-example-com
  namespace: default
  labels:
    app: jw238dns
    domain: example.com
data:
  tls.crt: <base64-encoded-certificate>
  tls.key: <base64-encoded-private-key>
  ca.crt: <base64-encoded-issuer-certificate>
```

### File System

Certificates are stored in domain-specific directories:

```
/var/lib/jw238dns/certs/
├── example-com/
│   ├── certificate.crt
│   ├── private.key
│   └── issuer.crt
└── wildcard-example-com/
    ├── certificate.crt
    ├── private.key
    └── issuer.crt
```

## Automatic Renewal

The manager automatically renews certificates before expiration:

- **Check Interval**: How often to check (default: 24 hours)
- **Renew Before**: Renew this long before expiry (default: 30 days)
- **Process**: Checks all managed certificates and renews those expiring soon

```go
// Start auto-renewal (runs in background)
manager.StartAutoRenewal(ctx)

// Stop auto-renewal
manager.StopAutoRenewal()
```

## Configuration

See `assets/example-acme-config.yaml` for a complete configuration example.

### Key Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `ServerURL` | ACME directory URL | Staging |
| `Email` | Account email | Required |
| `KeyType` | Certificate key type | RSA2048 |
| `AutoRenew` | Enable auto-renewal | true |
| `CheckInterval` | Renewal check frequency | 24h |
| `RenewBefore` | Renew before expiry | 30 days |
| `PropagationWait` | DNS propagation wait | 60s |
| `EAB.KID` | EAB Key Identifier (for ZeroSSL) | Empty |
| `EAB.HMACKey` | EAB HMAC key (for ZeroSSL) | Empty |

### Supported Key Types

- `RSA2048` - RSA 2048-bit (default, widely compatible)
- `RSA4096` - RSA 4096-bit (more secure, larger)
- `EC256` - ECDSA P-256 (recommended, smaller & faster)
- `EC384` - ECDSA P-384 (more secure)

## Testing

Use Let's Encrypt staging for testing to avoid rate limits:

```go
config.ServerURL = acme.LetsEncryptStaging()
```

**Rate Limits (Production)**:
- 50 certificates per registered domain per week
- 5 duplicate certificates per week

## Error Handling

Common errors and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| DNS propagation timeout | DNS not propagated | Increase `PropagationWait` |
| Rate limit exceeded | Too many requests | Use staging or wait |
| Invalid domain | Domain not owned | Verify DNS control |
| TXT record conflict | Old record exists | Clean up manually |

## Security Considerations

1. **Private Key Protection**: Keys stored with 0600 permissions (file storage)
2. **RBAC**: Kubernetes storage requires Secret read/write permissions
3. **Email Privacy**: Use a dedicated email for ACME notifications
4. **Staging First**: Always test with staging before production

## Dependencies

- `github.com/go-acme/lego/v4` - ACME client library
- `k8s.io/client-go` - Kubernetes client (for Secret storage)
- Internal storage layer - For DNS TXT record management

## References

- [ACME Protocol (RFC 8555)](https://tools.ietf.org/html/rfc8555)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)
- [Lego Documentation](https://go-acme.github.io/lego/)

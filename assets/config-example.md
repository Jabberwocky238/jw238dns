# jw238dns Configuration Example

This file provides a complete configuration example for jw238dns with detailed explanations.

---

## Complete Configuration Example

```yaml
# DNS Server Configuration
dns:
  # Listen address for DNS server
  listen: "0.0.0.0:53"

  # Enable TCP DNS queries
  tcp_enabled: true

  # Enable UDP DNS queries
  udp_enabled: true

  # Upstream DNS forwarding (for recursive queries)
  upstream:
    # Enable forwarding to upstream DNS servers
    enabled: true

    # List of upstream DNS servers (tried in order)
    servers:
      - "1.1.1.1:53"      # Cloudflare DNS (primary)
      - "8.8.8.8:53"      # Google DNS (fallback)

    # Timeout for upstream queries
    timeout: "5s"

# GeoIP Configuration (for distance-based DNS responses)
geoip:
  # Enable GeoIP-based sorting of A records
  enabled: true

  # Path to MaxMind GeoIP2 database file
  mmdb_path: "/app/assets/GeoLite2-City.mmdb"

# Storage Configuration
storage:
  # Storage type: "configmap" (Kubernetes) or "file" (local file)
  type: "configmap"

  # ConfigMap storage settings (for Kubernetes)
  configmap:
    namespace: "jw238dns"
    name: "jw238dns-records"
    data_key: "records.yaml"

  # File storage settings (for local development)
  file:
    path: "/app/data/records.json"

# HTTP Management API Configuration
http:
  # Enable HTTP management API
  enabled: true

  # Listen address for HTTP API
  listen: "0.0.0.0:8080"

  # Authentication settings
  auth:
    # Enable Bearer token authentication
    enabled: true

    # Environment variable containing the auth token
    # The actual token value is read from this env var at runtime
    token_env: "DNS_HTTP_AUTH_TOKEN"
```

---

## Configuration for Different Environments

### Development (Local)

```yaml
dns:
  listen: "127.0.0.1:5353"  # Non-privileged port
  tcp_enabled: true
  udp_enabled: true
  upstream:
    enabled: true
    servers:
      - "1.1.1.1:53"
    timeout: "5s"

geoip:
  enabled: false  # Disable GeoIP in development

storage:
  type: "file"
  file:
    path: "./data/records.json"

http:
  enabled: true
  listen: "127.0.0.1:8080"
  auth:
    enabled: true
    token_env: "DNS_HTTP_AUTH_TOKEN"
```

### Production (Kubernetes)

```yaml
dns:
  listen: "0.0.0.0:53"
  tcp_enabled: true
  udp_enabled: true
  upstream:
    enabled: true
    servers:
      - "1.1.1.1:53"
      - "8.8.8.8:53"
    timeout: "5s"

geoip:
  enabled: true
  mmdb_path: "/app/assets/GeoLite2-City.mmdb"

storage:
  type: "configmap"
  configmap:
    namespace: "jw238dns"
    name: "jw238dns-records"
    data_key: "records.yaml"

http:
  enabled: true
  listen: "0.0.0.0:8080"
  auth:
    enabled: true
    token_env: "DNS_HTTP_AUTH_TOKEN"
```

---

## Environment Variables

The following environment variables are used by jw238dns:

### Required

```bash
# Configuration file path
CONFIG_PATH="/app/config/app.yaml"
```

### Optional (HTTP API)

```bash
# HTTP API authentication token
DNS_HTTP_AUTH_TOKEN="your-secret-token-here"
```

---

## Configuration Options Reference

### DNS Section

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen` | string | `"0.0.0.0:53"` | DNS server listen address |
| `tcp_enabled` | bool | `true` | Enable TCP DNS queries |
| `udp_enabled` | bool | `true` | Enable UDP DNS queries |
| `upstream.enabled` | bool | `false` | Enable upstream DNS forwarding |
| `upstream.servers` | []string | `["1.1.1.1:53"]` | List of upstream DNS servers |
| `upstream.timeout` | string | `"5s"` | Timeout for upstream queries |

### GeoIP Section

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable GeoIP-based sorting |
| `mmdb_path` | string | `""` | Path to MaxMind GeoIP2 database |

### Storage Section

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `type` | string | `"configmap"` | Storage type: `configmap` or `file` |
| `configmap.namespace` | string | `"default"` | Kubernetes namespace |
| `configmap.name` | string | `""` | ConfigMap name |
| `configmap.data_key` | string | `"records.yaml"` | ConfigMap data key |
| `file.path` | string | `""` | File path for local storage |

### HTTP Section

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable HTTP management API |
| `listen` | string | `"0.0.0.0:8080"` | HTTP server listen address |
| `auth.enabled` | bool | `false` | Enable authentication |
| `auth.token_env` | string | `""` | Env var containing auth token |

---

## Upstream DNS Servers

Common upstream DNS servers:

```yaml
dns:
  upstream:
    servers:
      # Cloudflare DNS
      - "1.1.1.1:53"
      - "1.0.0.1:53"

      # Google DNS
      - "8.8.8.8:53"
      - "8.8.4.4:53"

      # Quad9 DNS
      - "9.9.9.9:53"
      - "149.112.112.112:53"
```

---

## Security Best Practices

1. **Never commit secrets to version control**
   - Use environment variables for tokens and credentials
   - Use Kubernetes Secrets for sensitive data

2. **Use strong authentication tokens**
   ```bash
   # Generate a secure random token
   openssl rand -base64 32
   ```

3. **Restrict HTTP API access**
   - Use firewall rules to limit access
   - Consider using a reverse proxy with additional security

4. **Use Let's Encrypt staging for testing**
   - Avoid rate limits during development
   - Switch to production only when ready

5. **Enable GeoIP only if needed**
   - Requires additional memory for MMDB database
   - May add latency to DNS queries

---

## Troubleshooting

### DNS Server Won't Start

**Port 53 already in use:**
```bash
# Check what's using port 53
sudo lsof -i :53

# Use a different port for testing
dns:
  listen: "0.0.0.0:5353"
```

### HTTP API Returns 401

**Check authentication:**
```bash
# Verify token is set
echo $DNS_HTTP_AUTH_TOKEN

# Test with correct token
curl -H "Authorization: Bearer $DNS_HTTP_AUTH_TOKEN" http://localhost:8080/dns/list
```

### Upstream DNS Not Working

**Check connectivity:**
```bash
# Test upstream DNS directly
dig @1.1.1.1 example.com

# Check timeout setting
dns:
  upstream:
    timeout: "10s"  # Increase if needed
```

---

**Last Updated:** 2026-02-13
**Version:** 1.0.0

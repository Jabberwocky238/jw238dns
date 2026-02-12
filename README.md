# jw238dns - Cloud-Native DNS Module

A cloud-native DNS module written in Go that provides DNS resolution, ACME challenge handling (DNS-01 and HTTP-01), and HTTP management API with Kubernetes ConfigMap persistence.

## Features

- **Full DNS Resolution**: Support for all standard DNS record types (A, AAAA, CNAME, MX, TXT, NS, SRV, etc.)
- **ACME Challenge Support**:
  - DNS-01 challenge for wildcard certificates
  - HTTP-01 challenge for standard certificates
- **HTTP Management API**: RESTful API for DNS record management
- **Kubernetes Native**: ConfigMap-based configuration persistence
- **Cloud-Native**: Designed for Kubernetes deployment with health checks and graceful shutdown

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        jw238dns                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ DNS Server   │  │ HTTP API     │  │ ACME Handler │     │
│  │ (Port 53)    │  │ (Port 8080)  │  │              │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                  │                  │              │
│         └──────────────────┼──────────────────┘              │
│                            │                                 │
│                   ┌────────▼────────┐                       │
│                   │ Record Storage  │                       │
│                   └────────┬────────┘                       │
│                            │                                 │
└────────────────────────────┼─────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │ Kubernetes      │
                    │ ConfigMap       │
                    └─────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster (for deployment)
- kubectl configured

### Local Development

```bash
# Clone the repository
git clone <repository-url>
cd jw238dns

# Install dependencies
go mod download

# Run locally
go run cmd/jw238dns/main.go
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/kubernetes/

# Check deployment status
kubectl get pods -l app=jw238dns

# View logs
kubectl logs -l app=jw238dns -f
```

## Configuration

Configuration can be provided via environment variables or ConfigMap:

```yaml
dns_port: 53
http_port: 8080
log_level: info
log_format: json

# DNS records
records:
  - name: example.com.
    type: A
    ttl: 300
    value:
      - 192.168.1.1
  - name: www.example.com.
    type: CNAME
    ttl: 300
    value:
      - example.com.
```

### Environment Variables

- `DNS_PORT`: DNS server port (default: 53)
- `HTTP_PORT`: HTTP API port (default: 8080)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `LOG_FORMAT`: Log format (json, text)
- `CONFIGMAP_NAME`: ConfigMap name for persistence
- `NAMESPACE`: Kubernetes namespace

## API Documentation

### DNS Records Management

#### List all records
```bash
GET /api/v1/records
```

#### Get specific record
```bash
GET /api/v1/records/:name
```

#### Create record
```bash
POST /api/v1/records
Content-Type: application/json

{
  "name": "example.com.",
  "type": "A",
  "ttl": 300,
  "value": ["192.168.1.1"]
}
```

#### Update record
```bash
PUT /api/v1/records/:name
Content-Type: application/json

{
  "type": "A",
  "ttl": 600,
  "value": ["192.168.1.2"]
}
```

#### Delete record
```bash
DELETE /api/v1/records/:name
```

### Health Checks

```bash
# Health check
GET /health

# Readiness check
GET /ready
```

### ACME Challenges

HTTP-01 challenges are automatically handled at:
```
GET /.well-known/acme-challenge/:token
```

DNS-01 challenges are handled via TXT records:
```
_acme-challenge.example.com. TXT "challenge-token"
```

## Development

### Project Structure

```
jw238dns/
├── cmd/
│   └── jw238dns/          # Main application
├── internal/
│   ├── dns/               # DNS server and handlers
│   ├── acme/              # ACME challenge handlers
│   ├── api/               # HTTP API
│   ├── config/            # Configuration management
│   └── storage/           # Storage backends
├── deployments/
│   └── kubernetes/        # Kubernetes manifests
├── .trellis/              # Development workflow
│   ├── spec/              # Development guidelines
│   └── workspace/         # Development journals
└── README.md
```

### Development Workflow

This project uses the Trellis development workflow. See `.trellis/workflow.md` for details.

```bash
# Start a development session
/trellis:start

# Before coding, read guidelines
cat .trellis/spec/backend/index.md

# Create a task
./.trellis/scripts/task.sh create "Add DNS caching"

# After completing work
/trellis:finish-work
```

## Testing

```bash
# Run unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...
```

## Contributing

1. Read development guidelines in `.trellis/spec/backend/`
2. Create a task using `.trellis/scripts/task.sh`
3. Follow the coding standards
4. Write tests for new features
5. Ensure all tests pass
6. Submit a pull request

## License

[License Type] - See LICENSE file for details

## Support

For issues and questions, please open an issue on GitHub.

---

**Project Status**: Initial Setup
**Last Updated**: 2026-02-12

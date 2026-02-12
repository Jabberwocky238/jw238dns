# Project Architecture

> System design and component relationships for Go DNS Server

---

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    DNS Server Pod                       │ │
│  │                                                          │ │
│  │  ┌──────────────┐         ┌──────────────┐            │ │
│  │  │  DNS Server  │◄────────┤Storage Layer │            │ │
│  │  │  (Port 53)   │         │  Interface   │            │ │
│  │  └──────┬───────┘         └──────┬───────┘            │ │
│  │         │                        │                     │ │
│  │         │                 ┌──────┴───────┐            │ │
│  │         │                 │              │             │ │
│  │         │          ┌──────▼─────┐ ┌─────▼──────┐     │ │
│  │         │          │ ConfigMap  │ │ JSON File  │     │ │
│  │         │          │  Storage   │ │  Storage   │     │ │
│  │         │          └────────────┘ └────────────┘     │ │
│  │         │                                             │ │
│  │  ┌──────▼───────┐         ┌──────────────┐          │ │
│  │  │ ACME Manager │◄────────┤  DNS-01      │          │ │
│  │  │              │         │  Provider    │          │ │
│  │  └──────┬───────┘         └──────────────┘          │ │
│  │         │                                             │ │
│  │         │ (Store certs)                               │ │
│  │         ▼                                             │ │
│  │  ┌──────────────┐                                    │ │
│  │  │ K8s Secrets  │                                    │ │
│  │  └──────────────┘                                    │ │
│  │                                                        │ │
│  │  ┌──────────────┐                                    │ │
│  │  │ HTTP Server  │ (Management API)                   │ │
│  │  │  (Port 8080) │                                    │ │
│  │  └──────────────┘                                    │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Storage Layer (pkg/storage)

**Purpose**: Abstract data persistence, support multiple backends

**Interface**:
```go
type Storage interface {
    Get(ctx context.Context, domain, recordType string) (*DNSRecord, error)
    List(ctx context.Context, filter Filter) ([]*DNSRecord, error)
    Create(ctx context.Context, record *DNSRecord) error
    Update(ctx context.Context, record *DNSRecord) error
    Delete(ctx context.Context, domain, recordType string) error
    Watch(ctx context.Context) (<-chan Event, error)
}
```

**Implementations**:
- **ConfigMapStorage**: Stores records in Kubernetes ConfigMap
- **JSONFileStorage**: Stores records in local JSON file

**Key Design Decisions**:
- Interface-based design for easy extension
- Support watch mechanism for real-time updates
- Handle concurrent access with proper locking

---

### 2. DNS Server (pkg/dns)

**Purpose**: Handle DNS queries, resolve records from storage

**Key Features**:
- UDP and TCP protocol support
- Support 7 record types: A, AAAA, CNAME, TXT, SRV, MX, NS
- Wildcard domain matching
- Query logging and metrics

**Flow**:
```
Client Query → DNS Server → Storage Layer → Response
```

**Libraries Used**:
- `github.com/miekg/dns` - DNS protocol implementation

---

### 3. ACME Manager (pkg/acme)

**Purpose**: Automate TLS certificate issuance via DNS-01 challenge

**Components**:
- **ACME Client**: Communicates with Let's Encrypt
- **DNS-01 Provider**: Creates/deletes TXT records for validation
- **Certificate Manager**: Handles auto-renewal

**Flow**:
```
Request Cert → ACME Challenge → Create TXT Record →
Validation → Issue Cert → Store to K8s Secret → Auto-Renew
```

**Libraries Used**:
- `github.com/go-acme/lego/v4` - ACME client

---

### 4. HTTP Management API (pkg/http)

**Purpose**: Provide HTTP interface for DNS record and certificate management

**Design Principles**:
- Non-RESTful: Use GET/POST only
- Action-based endpoints: `/dns/add`, `/dns/delete`, etc.
- Unified JSON response format
- Token-based authentication

**Endpoints**:
- DNS: `/dns/add`, `/dns/delete`, `/dns/update`, `/dns/list`, `/dns/get`
- Cert: `/cert/request`, `/cert/renew`, `/cert/list`, `/cert/status`
- System: `/health`, `/metrics`, `/status`

**Libraries Used**:
- `github.com/gin-gonic/gin` - HTTP framework

---

## Data Flow

### DNS Query Flow

```
1. Client sends DNS query (e.g., example.com A?)
2. DNS Server receives query
3. Server queries Storage Layer for matching record
4. Storage returns DNSRecord or error
5. Server constructs DNS response
6. Server sends response to client
```

### DNS Record Management Flow

```
1. Admin sends HTTP POST /dns/add
2. HTTP Handler validates request
3. Handler calls Storage.Create()
4. Storage persists record (ConfigMap or JSON)
5. HTTP Handler returns success response
6. DNS Server picks up change (via Watch or reload)
```

### Certificate Issuance Flow

```
1. Admin sends HTTP POST /cert/request
2. ACME Manager initiates challenge
3. DNS-01 Provider creates TXT record via Storage
4. Let's Encrypt validates TXT record
5. Certificate issued
6. Manager stores cert to K8s Secret
7. DNS-01 Provider cleans up TXT record
```

---

## Configuration

### Example config.yaml

```yaml
storage:
  type: "configmap"  # or "jsonfile"
  configmap:
    namespace: "default"
    name: "dns-records"
  jsonfile:
    path: "/data/dns-records.json"

dns:
  listen: "0.0.0.0:53"
  protocols: ["udp", "tcp"]
  cache:
    enabled: true
    ttl: 300

acme:
  server: "https://acme-v02.api.letsencrypt.org/directory"
  email: "admin@example.com"
  storage:
    namespace: "default"
  auto_renew:
    enabled: true
    check_interval: "24h"

http:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true
    token: "${HTTP_AUTH_TOKEN}"
```

---

## Deployment

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dns-server
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: dnsd
        image: dns-server:latest
        ports:
        - containerPort: 53
          protocol: UDP
        - containerPort: 53
          protocol: TCP
        - containerPort: 8080
          protocol: TCP
        volumeMounts:
        - name: config
          mountPath: /etc/dnsd
      volumes:
      - name: config
        configMap:
          name: dnsd-config
```

---

## Security Considerations

1. **HTTP API Authentication**: Use strong token, rotate regularly
2. **RBAC**: Limit K8s permissions (ConfigMap, Secret access)
3. **Input Validation**: Validate all DNS records before storage
4. **Rate Limiting**: Prevent DNS query flooding
5. **TLS**: Use HTTPS for management API (optional)

---

## Performance Considerations

1. **Caching**: Implement DNS response cache
2. **Concurrent Queries**: Use goroutines for parallel processing
3. **Storage Optimization**: Batch operations when possible
4. **Metrics**: Monitor query latency, error rates

---

## Extension Points

1. **New Storage Backend**: Implement Storage interface (e.g., etcd, Redis)
2. **New Record Types**: Add handler in DNS server
3. **Custom Authentication**: Replace token auth with OAuth/OIDC
4. **Multi-tenancy**: Add namespace/tenant isolation

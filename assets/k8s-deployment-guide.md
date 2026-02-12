# jw238dns Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying jw238dns DNS server.

## Quick Start

```bash
# Apply all manifests
kubectl apply -f k8s-deployment.yaml

# Check deployment status
kubectl -n jw238dns get pods
kubectl -n jw238dns get svc

# Get DNS service external IP
kubectl -n jw238dns get svc jw238dns-dns-udp -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                    │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │              jw238dns Namespace                     │ │
│  │                                                     │ │
│  │  ┌──────────────┐      ┌──────────────┐           │ │
│  │  │   Pod 1      │      │   Pod 2      │           │ │
│  │  │  jw238dns    │      │  jw238dns    │           │ │
│  │  │  :53 :8080   │      │  :53 :8080   │           │ │
│  │  └──────┬───────┘      └──────┬───────┘           │ │
│  │         │                     │                    │ │
│  │         └──────────┬──────────┘                    │ │
│  │                    │                               │ │
│  │         ┌──────────▼──────────┐                    │ │
│  │         │   Services          │                    │ │
│  │         │  - DNS UDP/TCP      │                    │ │
│  │         │  - HTTP API         │                    │ │
│  │         └──────────┬──────────┘                    │ │
│  │                    │                               │ │
│  │         ┌──────────▼──────────┐                    │ │
│  │         │   ConfigMap         │                    │ │
│  │         │  DNS Records        │                    │ │
│  │         └─────────────────────┘                    │ │
│  └─────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

## Components

### 1. Namespace
- **jw238dns**: Isolated namespace for all resources

### 2. ConfigMaps
- **jw238dns-config**: DNS records configuration
- **jw238dns-app-config**: Application settings

### 3. Secrets
- **jw238dns-auth**: Authentication tokens and credentials

### 4. RBAC
- **ServiceAccount**: jw238dns service account
- **Role**: Permissions for ConfigMap and Secret access
- **RoleBinding**: Binds role to service account

### 5. Deployment
- **Replicas**: 2 (default), auto-scales 2-10
- **Strategy**: RollingUpdate with zero downtime
- **Resources**: 100m-500m CPU, 128Mi-512Mi memory
- **Probes**: Liveness and readiness checks

### 6. Services
- **jw238dns-dns-udp**: DNS over UDP (LoadBalancer)
- **jw238dns-dns-tcp**: DNS over TCP (LoadBalancer)
- **jw238dns-http**: HTTP management API (ClusterIP)

### 7. Ingress (Optional)
- **jw238dns-http**: HTTPS access to management API

### 8. Autoscaling
- **HorizontalPodAutoscaler**: CPU/Memory based scaling
- **PodDisruptionBudget**: Ensures availability during updates

### 9. Monitoring
- **ServiceMonitor**: Prometheus metrics collection

## Configuration

### DNS Records

Edit the `jw238dns-config` ConfigMap to add/modify DNS records:

```yaml
data:
  config.yaml: |
    records:
      - name: example.com.
        type: A
        ttl: 300
        value:
          - 192.168.1.1
```

Apply changes:
```bash
kubectl apply -f k8s-deployment.yaml
# Records are automatically reloaded via ConfigMap watch
```

### Authentication

Update the authentication token:

```bash
# Generate a secure token
TOKEN=$(openssl rand -base64 32)

# Update secret
kubectl -n jw238dns create secret generic jw238dns-auth \
  --from-literal=http-token="$TOKEN" \
  --from-literal=acme-email="admin@example.com" \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart pods to pick up new token
kubectl -n jw238dns rollout restart deployment jw238dns
```

### GeoIP Database

The GeoIP database is included in the Docker image. To update:

1. Download new GeoLite2-City.mmdb
2. Rebuild Docker image with updated database
3. Update deployment image tag

### ACME Certificates

Enable ACME in the app config:

```yaml
acme:
  enabled: true
  server: "https://acme-v02.api.letsencrypt.org/directory"
  email: "admin@example.com"
  auto_renew: true
```

## Deployment Scenarios

### Development/Testing

```bash
# Use staging Let's Encrypt
kubectl -n jw238dns edit configmap jw238dns-app-config
# Change acme.server to staging URL

# Use NodePort instead of LoadBalancer
kubectl -n jw238dns patch svc jw238dns-dns-udp -p '{"spec":{"type":"NodePort"}}'
```

### Production

```bash
# Use production Let's Encrypt
# Ensure proper monitoring is configured
# Set resource limits appropriately
# Configure backup for certificates
```

### High Availability

```bash
# Increase replicas
kubectl -n jw238dns scale deployment jw238dns --replicas=5

# Use anti-affinity to spread across nodes
# Add to deployment spec:
spec:
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: jw238dns
                topologyKey: kubernetes.io/hostname
```

## Monitoring

### Prometheus Metrics

Metrics are exposed at `/metrics` on port 8080:

```bash
# Port-forward to access metrics
kubectl -n jw238dns port-forward svc/jw238dns-http 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

### Health Checks

```bash
# Check health endpoint
kubectl -n jw238dns port-forward svc/jw238dns-http 8080:8080
curl http://localhost:8080/health

# Check pod status
kubectl -n jw238dns get pods
kubectl -n jw238dns describe pod <pod-name>
```

### Logs

```bash
# View logs
kubectl -n jw238dns logs -f deployment/jw238dns

# View logs from specific pod
kubectl -n jw238dns logs -f <pod-name>

# View logs from all pods
kubectl -n jw238dns logs -f -l app=jw238dns
```

## Troubleshooting

### DNS Not Resolving

```bash
# Check service external IP
kubectl -n jw238dns get svc jw238dns-dns-udp

# Test DNS resolution
dig @<external-ip> example.com

# Check pod logs
kubectl -n jw238dns logs -l app=jw238dns
```

### ConfigMap Not Updating

```bash
# Verify ConfigMap
kubectl -n jw238dns get configmap jw238dns-config -o yaml

# Check RBAC permissions
kubectl -n jw238dns auth can-i get configmaps --as=system:serviceaccount:jw238dns:jw238dns

# Restart pods to force reload
kubectl -n jw238dns rollout restart deployment jw238dns
```

### Certificate Issues

```bash
# Check ACME logs
kubectl -n jw238dns logs -l app=jw238dns | grep -i acme

# List certificates (secrets)
kubectl -n jw238dns get secrets -l app=jw238dns

# Check certificate expiry
kubectl -n jw238dns get secret tls-example-com -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -dates
```

### Performance Issues

```bash
# Check resource usage
kubectl -n jw238dns top pods

# Check HPA status
kubectl -n jw238dns get hpa

# Scale manually if needed
kubectl -n jw238dns scale deployment jw238dns --replicas=5
```

## Security Considerations

1. **Network Policies**: Add NetworkPolicy to restrict traffic
2. **Pod Security**: Uses non-root user, read-only filesystem
3. **RBAC**: Minimal permissions (ConfigMap and Secret access only)
4. **Secrets**: Store sensitive data in Kubernetes Secrets
5. **TLS**: Use Ingress with TLS for HTTP API access

## Backup and Recovery

### Backup DNS Records

```bash
# Export ConfigMap
kubectl -n jw238dns get configmap jw238dns-config -o yaml > dns-records-backup.yaml

# Export certificates
kubectl -n jw238dns get secrets -l app=jw238dns -o yaml > certificates-backup.yaml
```

### Restore

```bash
# Restore ConfigMap
kubectl apply -f dns-records-backup.yaml

# Restore certificates
kubectl apply -f certificates-backup.yaml
```

## Upgrading

```bash
# Update image
kubectl -n jw238dns set image deployment/jw238dns jw238dns=jw238dns:v2.0.0

# Or edit deployment
kubectl -n jw238dns edit deployment jw238dns

# Watch rollout
kubectl -n jw238dns rollout status deployment jw238dns

# Rollback if needed
kubectl -n jw238dns rollout undo deployment jw238dns
```

## Cleanup

```bash
# Delete all resources
kubectl delete -f k8s-deployment.yaml

# Or delete namespace
kubectl delete namespace jw238dns
```

## References

- [Kubernetes DNS Service](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/)
- [ConfigMap Hot Reload](https://kubernetes.io/docs/concepts/configuration/configmap/)
- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)

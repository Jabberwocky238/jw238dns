# 支持DNS01证书申请

## Goal
实现DNS-01 ACME协议支持，允许通过DNS TXT记录验证域名所有权，自动申请和续期Let's Encrypt等CA的TLS证书。

## Requirements

### ACME客户端实现
- 使用成熟的Go ACME库（推荐go-acme/lego）
- 支持ACME v2协议
- 支持Let's Encrypt生产和测试环境
- 支持其他兼容ACME协议的CA

### DNS-01验证流程
- 实现DNS-01 Challenge Provider接口
- 自动创建验证所需的TXT记录（_acme-challenge.domain.com）
- 等待DNS记录传播
- 验证完成后自动清理TXT记录
- 支持通配符证书申请（*.example.com）

### 证书管理
- 自动申请新证书
- 自动续期即将过期的证书（提前30天）
- 证书存储到Kubernetes Secret
- 支持多域名证书（SAN）
- 记录证书申请和续期日志

### 配置和控制
- 支持配置ACME服务器URL
- 支持配置账户邮箱
- 支持配置证书存储位置
- 提供手动触发申请/续期的接口
- 支持证书状态查询

## Acceptance Criteria

- [ ] 使用lego库实现ACME客户端
- [ ] 实现DNS-01 Provider，与存储层集成
- [ ] 支持自动创建和清理TXT记录
- [ ] 实现证书自动续期机制
- [ ] 证书存储到Kubernetes Secret
- [ ] 支持通配符证书申请
- [ ] 添加单元测试和集成测试
- [ ] 提供配置文件示例
- [ ] 代码通过golangci-lint检查

## Technical Notes

### 推荐使用的库
- `github.com/go-acme/lego/v4` - ACME客户端库
- `k8s.io/client-go` - Kubernetes客户端（存储证书到Secret）
- 标准库 `crypto/x509` - 证书解析

### DNS-01 Provider实现
```go
type DNS01Provider struct {
    storage storage.Storage
}

func (p *DNS01Provider) Present(domain, token, keyAuth string) error {
    // 创建TXT记录
    fqdn := "_acme-challenge." + domain
    value := // 计算challenge值
    return p.storage.Create(ctx, &storage.DNSRecord{
        Domain: fqdn,
        Type:   "TXT",
        Value:  value,
        TTL:    60,
    })
}

func (p *DNS01Provider) CleanUp(domain, token, keyAuth string) error {
    // 删除TXT记录
}
```

### 证书续期逻辑
```go
type CertManager struct {
    acmeClient *lego.Client
    storage    storage.Storage
    k8sClient  kubernetes.Interface
}

func (m *CertManager) AutoRenew(ctx context.Context) {
    // 定期检查证书过期时间
    // 提前30天自动续期
}
```

### 配置示例
```yaml
acme:
  server: "https://acme-v02.api.letsencrypt.org/directory"
  email: "admin@example.com"
  storage:
    type: "kubernetes-secret"
    namespace: "default"
  auto_renew:
    enabled: true
    check_interval: "24h"
    renew_before: "720h" # 30天
```

### 目录结构建议
```
pkg/acme/
├── client.go         # ACME客户端封装
├── dns01.go          # DNS-01 Provider实现
├── manager.go        # 证书管理器
├── storage.go        # 证书存储（K8s Secret）
└── config.go         # 配置结构
```

## Dependencies
- 依赖：02-12-storage-layer（需要存储层创建TXT记录）
- 依赖：02-12-dns-core（DNS服务器需要能解析TXT记录）

# 实现DNS核心功能

## Goal
实现DNS服务器核心功能，使用现成的DNS库处理DNS查询请求，支持常见的DNS记录类型，并与存储层集成。

## Requirements

### DNS服务器实现
- 使用成熟的Go DNS库（推荐miekg/dns）
- 支持UDP和TCP协议
- 监听标准DNS端口（53）或自定义端口
- 支持并发处理多个DNS查询

### 支持的记录类型（必须兼容）
- A记录（IPv4地址）
- AAAA记录（IPv6地址）
- CNAME记录（别名）
- TXT记录（文本记录，用于DNS01验证）
- SRV记录（服务记录，格式：priority weight port target）
- MX记录（邮件交换，包含优先级）
- NS记录（域名服务器）

### 查询处理
- 从存储层读取DNS记录
- 支持通配符域名（*.example.com）
- 实现DNS缓存机制（可选，提高性能）
- 支持递归查询和迭代查询
- 处理不存在的域名（NXDOMAIN）

### 性能和可靠性
- 支持优雅关闭
- 实现健康检查接口
- 记录查询日志（可配置级别）
- 支持metrics导出（Prometheus格式）

## Acceptance Criteria

- [ ] 使用miekg/dns库实现DNS服务器
- [ ] 支持UDP和TCP协议监听
- [ ] 实现所有要求的DNS记录类型查询
- [ ] 与存储层接口集成
- [ ] 实现通配符域名匹配
- [ ] 添加单元测试和集成测试
- [ ] 实现优雅关闭机制
- [ ] 提供配置文件示例
- [ ] 代码通过golangci-lint检查

## Technical Notes

### 推荐使用的库
- `github.com/miekg/dns` - DNS协议实现（最流行的Go DNS库）
- `github.com/prometheus/client_golang` - Prometheus metrics
- 标准库 `context` - 上下文管理

### 核心结构设计
```go
type DNSServer struct {
    storage  storage.Storage
    udpServer *dns.Server
    tcpServer *dns.Server
    cache    Cache // 可选
}

func (s *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    // 处理DNS查询
}
```

### 配置示例
```yaml
dns:
  listen: "0.0.0.0:53"
  protocols: ["udp", "tcp"]
  cache:
    enabled: true
    ttl: 300
  logging:
    level: info
    queries: true
```

### 目录结构建议
```
pkg/dns/
├── server.go         # DNS服务器主逻辑
├── handler.go        # 查询处理器
├── cache.go          # 缓存实现（可选）
├── metrics.go        # Metrics收集
└── config.go         # 配置结构
```

## Dependencies
- 依赖：02-12-storage-layer（存储层必须先完成）

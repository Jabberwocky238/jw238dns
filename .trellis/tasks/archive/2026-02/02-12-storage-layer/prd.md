# 抽象存储层接口

## Goal
设计并实现通用的存储层抽象接口，支持多种存储后端（Kubernetes ConfigMap和JSON文件），为DNS记录提供统一的数据访问层。

## Requirements

### 核心接口设计
- 定义通用的存储接口（Storage Interface）
- 支持DNS记录的CRUD操作
- 接口需要支持事务性操作（原子性更新）
- 支持批量操作以提高性能

### 数据结构
- 定义DNS记录的通用结构体（DNSRecord）
- 支持常见DNS记录类型：A、AAAA、CNAME、TXT、MX、NS
- 记录需包含：域名、类型、值、TTL、优先级（可选）
- 支持记录的元数据（创建时间、更新时间等）

### 存储实现
- **ConfigMap存储**：实现基于Kubernetes ConfigMap的存储
  - 使用client-go与K8s API交互
  - 支持watch机制监听配置变化
  - 处理并发更新冲突（使用ResourceVersion）
- **JSON文件存储**：实现基于本地JSON文件的存储
  - 支持文件锁防止并发写入冲突
  - 支持热加载（文件变化时自动重载）
  - 提供备份机制

### 错误处理
- 定义统一的错误类型
- 区分临时错误和永久错误
- 支持重试机制

## Acceptance Criteria

- [ ] 定义清晰的Storage接口，包含Get、List、Create、Update、Delete方法
- [ ] 实现DNSRecord结构体，支持JSON和YAML序列化
- [ ] 完成ConfigMapStorage实现，通过单元测试
- [ ] 完成JSONFileStorage实现，通过单元测试
- [ ] 实现存储层的集成测试
- [ ] 提供存储层使用示例和文档
- [ ] 代码通过golangci-lint检查

## Technical Notes

### 推荐使用的库
- `k8s.io/client-go` - Kubernetes客户端
- `k8s.io/apimachinery` - Kubernetes API类型
- `github.com/fsnotify/fsnotify` - 文件监听（可选）
- 标准库 `encoding/json` - JSON处理

### 接口设计参考
```go
type Storage interface {
    Get(ctx context.Context, domain string, recordType string) (*DNSRecord, error)
    List(ctx context.Context, filter Filter) ([]*DNSRecord, error)
    Create(ctx context.Context, record *DNSRecord) error
    Update(ctx context.Context, record *DNSRecord) error
    Delete(ctx context.Context, domain string, recordType string) error
    Watch(ctx context.Context) (<-chan Event, error)
}
```

### 目录结构建议
```
pkg/storage/
├── interface.go      # 接口定义
├── types.go          # 数据结构
├── configmap.go      # ConfigMap实现
├── jsonfile.go       # JSON文件实现
└── errors.go         # 错误定义
```

## Dependencies
- 无依赖，这是基础层

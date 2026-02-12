# HTTP动态端点管理

## Goal
使用Gin框架实现HTTP管理接口，支持动态修改DNS记录，提供基于GET和POST的简单接口，不使用RESTful风格。

## Requirements

### 接口设计原则
- 禁止使用RESTful API风格（不使用PUT/DELETE/PATCH）
- 仅使用GET和POST方法
- 使用明确的动作名称（如/add、/delete、/update、/list）
- 所有接口返回统一的JSON格式

### DNS记录管理接口
- **POST /dns/add** - 添加DNS记录
- **POST /dns/delete** - 删除DNS记录
- **POST /dns/update** - 更新DNS记录
- **GET /dns/list** - 列出DNS记录（支持过滤）
- **GET /dns/get** - 获取单条DNS记录

### 证书管理接口
- **POST /cert/request** - 请求新证书
- **POST /cert/renew** - 手动续期证书
- **GET /cert/list** - 列出所有证书
- **GET /cert/status** - 查询证书状态

### 系统接口
- **GET /health** - 健康检查
- **GET /metrics** - Prometheus metrics
- **GET /status** - 系统状态（DNS服务器状态、存储状态等）

### 安全性
- 支持Token认证（Bearer Token或自定义Header）
- 支持IP白名单
- 请求日志记录
- 参数验证和错误处理

### 响应格式
```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

## Acceptance Criteria

- [ ] 使用Gin框架实现HTTP服务器
- [ ] 实现所有DNS记录管理接口
- [ ] 实现证书管理接口
- [ ] 实现系统监控接口
- [ ] 添加Token认证中间件
- [ ] 添加请求日志中间件
- [ ] 实现统一的错误处理
- [ ] 添加接口测试
- [ ] 提供API文档和使用示例
- [ ] 代码通过golangci-lint检查

## Technical Notes

### 推荐使用的库
- `github.com/gin-gonic/gin` - Web框架
- `github.com/prometheus/client_golang` - Metrics导出
- 标准库 `net/http` - HTTP服务器

### 接口实现示例
```go
type DNSHandler struct {
    storage storage.Storage
    dns     *dns.Server
}

// POST /dns/add
func (h *DNSHandler) AddRecord(c *gin.Context) {
    var req struct {
        Domain string `json:"domain" binding:"required"`
        Type   string `json:"type" binding:"required"`
        Value  string `json:"value" binding:"required"`
        TTL    int    `json:"ttl"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, Response{Code: 400, Message: err.Error()})
        return
    }

    // 调用存储层添加记录
    // ...

    c.JSON(200, Response{Code: 0, Message: "success"})
}

// GET /dns/list
func (h *DNSHandler) ListRecords(c *gin.Context) {
    domain := c.Query("domain")
    recordType := c.Query("type")

    // 调用存储层查询
    // ...

    c.JSON(200, Response{
        Code: 0,
        Message: "success",
        Data: records,
    })
}
```

### 认证中间件
```go
func AuthMiddleware(token string) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader != "Bearer "+token {
            c.JSON(401, Response{Code: 401, Message: "unauthorized"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### 配置示例
```yaml
http:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true
    token: "your-secret-token"
  ip_whitelist:
    - "10.0.0.0/8"
    - "127.0.0.1"
  logging:
    enabled: true
    format: "json"
```

### 目录结构建议
```
pkg/http/
├── server.go         # HTTP服务器
├── handler_dns.go    # DNS记录处理器
├── handler_cert.go   # 证书处理器
├── handler_system.go # 系统接口处理器
├── middleware.go     # 中间件（认证、日志等）
├── response.go       # 统一响应结构
└── config.go         # 配置结构
```

### API文档示例
```
POST /dns/add
添加DNS记录

请求：
{
  "domain": "example.com",
  "type": "A",
  "value": "1.2.3.4",
  "ttl": 300
}

响应：
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "record-123"
  }
}
```

## Dependencies
- 依赖：02-12-storage-layer（需要存储层接口）
- 依赖：02-12-dns-core（需要DNS服务器状态）
- 依赖：02-12-dns01-acme（需要证书管理功能）

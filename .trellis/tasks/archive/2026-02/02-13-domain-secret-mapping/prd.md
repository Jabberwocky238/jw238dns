# Implement Strict Domain-to-Secret Mapping System

## Goal

实现一套严密的域名到 Kubernetes Secret 名称的双向映射系统，确保不同域名类型（普通域名、通配符域名）映射到唯一的 Secret，并且可以从 Secret 名称反向解析出原始域名。

## Problem Statement

当前的 `sanitizeDomain()` 函数存在严重问题：
1. 所有子域名都被归到根域名，导致不同域名覆盖同一个 Secret
2. 通配符域名 `*.example.com` 和普通域名 `example.com` 映射到同一个 Secret
3. 无法从 Secret 名称反向解析出原始域名
4. 缺乏严格的测试覆盖

**示例问题：**
```
*.mesh-worker.cloud      → tls--mesh-worker-cloud
mesh-worker.cloud        → tls--mesh-worker-cloud  (冲突！)
api.mesh-worker.cloud    → tls--mesh-worker-cloud  (冲突！)
*.api.mesh-worker.cloud  → tls--mesh-worker-cloud  (冲突！)
```

## Requirements

### 1. 域名分类和命名规则

#### 1.1 普通域名（Normal Domain）
- **格式**: `api.example.com`, `example.com`, `www.example.com`
- **Secret 前缀**: `tls-normal--`
- **转换规则**:
  - 点号 `.` → 下划线 `_`
  - 横杠 `-` → 保持不变
  - 示例：
    - `api.example.com` → `tls-normal--api_example_com`
    - `example.com` → `tls-normal--example_com`
    - `my-api.example.com` → `tls-normal--my-api_example_com`

#### 1.2 通配符域名（Wildcard Domain）
- **格式**: `*.example.com`, `*.api.example.com`
- **Secret 前缀**: `tls-wildcard--`
- **转换规则**:
  - 星号 `*.` → 双下划线 `__`
  - 点号 `.` → 下划线 `_`
  - 横杠 `-` → 保持不变
  - 示例：
    - `*.example.com` → `tls-wildcard--__example_com`
    - `*.api.example.com` → `tls-wildcard--__api_example_com`
    - `*.my-api.example.com` → `tls-wildcard--__my-api_example_com`

### 2. 核心数据结构

```go
// DomainType represents the type of domain
type DomainType string

const (
    DomainTypeNormal   DomainType = "normal"
    DomainTypeWildcard DomainType = "wildcard"
)

// DomainSecretMapping represents the bidirectional mapping between domain and secret
type DomainSecretMapping struct {
    // OriginalDomain is the original domain name (e.g., "*.example.com", "api.example.com")
    OriginalDomain string

    // DomainType indicates whether it's a normal or wildcard domain
    DomainType DomainType

    // SecretName is the Kubernetes Secret name (e.g., "tls-wildcard--__example_com")
    SecretName string

    // NormalizedDomain is the domain with special characters replaced
    // For normal: "api.example.com" → "api_example_com"
    // For wildcard: "*.example.com" → "__example_com"
    NormalizedDomain string
}
```

### 3. 核心函数

#### 3.1 DomainToSecret (域名 → Secret)

```go
// DomainToSecret converts a domain name to a Kubernetes Secret name
// Examples:
//   "api.example.com"      → "tls-normal--api_example_com"
//   "*.example.com"        → "tls-wildcard--__example_com"
//   "*.api.example.com"    → "tls-wildcard--__api_example_com"
func DomainToSecret(domain string) string
```

#### 3.2 SecretToDomain (Secret → 域名)

```go
// SecretToDomain converts a Kubernetes Secret name back to the original domain
// Examples:
//   "tls-normal--api_example_com"      → "api.example.com"
//   "tls-wildcard--__example_com"      → "*.example.com"
//   "tls-wildcard--__api_example_com"  → "*.api.example.com"
// Returns error if the secret name format is invalid
func SecretToDomain(secretName string) (string, error)
```

#### 3.3 ParseDomain (解析域名)

```go
// ParseDomain parses a domain and returns its mapping information
func ParseDomain(domain string) (*DomainSecretMapping, error)
```

#### 3.4 ValidateSecretName (验证 Secret 名称)

```go
// ValidateSecretName checks if a secret name is valid according to our naming convention
func ValidateSecretName(secretName string) error
```

### 4. 边界情况处理

#### 4.1 非法字符
- 域名中不应包含除 `a-z`, `0-9`, `.`, `-`, `*` 之外的字符
- 如果包含，返回错误

#### 4.2 空字符串
- 空域名返回错误
- 空 Secret 名称返回错误

#### 4.3 格式错误
- `**example.com` (多个星号) → 错误
- `*.*.example.com` (多级通配符) → 错误
- `*example.com` (缺少点号) → 错误
- `.example.com` (以点号开头) → 错误
- `example.com.` (以点号结尾) → 错误

#### 4.4 Kubernetes 命名限制
- Secret 名称必须符合 DNS-1123 subdomain 规范
- 最大长度 253 字符
- 只能包含小写字母、数字、`-` 和 `.`
- 必须以字母或数字开头和结尾

### 5. 测试要求

#### 5.1 单元测试覆盖

**测试文件**: `acme/domain_mapping_test.go`

**测试用例分类**:

1. **正常域名测试** (至少 10 个用例)
   - 单级域名: `example.com`
   - 二级域名: `api.example.com`
   - 三级域名: `v1.api.example.com`
   - 包含横杠: `my-api.example.com`
   - 多个横杠: `my-cool-api.example.com`

2. **通配符域名测试** (至少 10 个用例)
   - 根域名通配符: `*.example.com`
   - 子域名通配符: `*.api.example.com`
   - 多级子域名通配符: `*.v1.api.example.com`
   - 包含横杠: `*.my-api.example.com`

3. **双向转换测试** (至少 20 个用例)
   - 对每个域名，测试: `domain → secret → domain` 循环
   - 确保往返转换后域名完全一致

4. **边界情况测试** (至少 15 个用例)
   - 空字符串
   - 非法字符: `example@com`, `example com`, `example/com`
   - 格式错误: `**example.com`, `*.*.example.com`, `*example.com`
   - 超长域名 (>253 字符)
   - 以点号开头/结尾
   - 连续点号: `example..com`

5. **Kubernetes 兼容性测试** (至少 5 个用例)
   - 验证生成的 Secret 名称符合 K8s 规范
   - 测试最大长度限制
   - 测试特殊字符处理

#### 5.2 表格驱动测试

使用表格驱动测试（Table-Driven Tests）确保测试覆盖全面：

```go
func TestDomainToSecret(t *testing.T) {
    tests := []struct {
        name       string
        domain     string
        wantSecret string
        wantErr    bool
    }{
        {
            name:       "normal domain - single level",
            domain:     "example.com",
            wantSecret: "tls-normal--example_com",
            wantErr:    false,
        },
        {
            name:       "wildcard domain - root",
            domain:     "*.example.com",
            wantSecret: "tls-wildcard--__example_com",
            wantErr:    false,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := DomainToSecret(tt.domain)
            if got != tt.wantSecret {
                t.Errorf("DomainToSecret(%q) = %q, want %q", tt.domain, got, tt.wantSecret)
            }
        })
    }
}
```

#### 5.3 属性测试（Property-Based Testing）

可选：使用 fuzzing 测试确保双向转换的对称性：

```go
func FuzzDomainSecretRoundTrip(f *testing.F) {
    // Add seed corpus
    f.Add("example.com")
    f.Add("*.example.com")
    f.Add("api.example.com")

    f.Fuzz(func(t *testing.T, domain string) {
        // Skip invalid domains
        if !isValidDomain(domain) {
            t.Skip()
        }

        // Test round trip
        secret := DomainToSecret(domain)
        recovered, err := SecretToDomain(secret)
        if err != nil {
            t.Errorf("SecretToDomain failed: %v", err)
        }
        if recovered != domain {
            t.Errorf("Round trip failed: %q → %q → %q", domain, secret, recovered)
        }
    })
}
```

### 6. 实现文件

#### 6.1 新建文件

- `acme/domain_mapping.go` - 核心实现
- `acme/domain_mapping_test.go` - 单元测试

#### 6.2 修改文件

- `acme/storage.go` - 替换 `sanitizeDomain()` 和 `domainToK8sSecret()` 为新函数
- `acme/storage_test.go` - 更新相关测试

### 7. 向后兼容性

**不需要向后兼容**：
- 这是新功能，旧的命名方式有严重缺陷
- 部署新版本后，旧的 Secret 可以手动迁移或重新申请证书

### 8. 文档要求

在 `acme/domain_mapping.go` 文件顶部添加详细的包文档：

```go
// Package acme provides domain-to-secret mapping utilities.
//
// Domain Naming Convention:
//
// Normal domains (api.example.com):
//   - Prefix: tls-normal--
//   - Transformation: dots → underscores
//   - Example: api.example.com → tls-normal--api_example_com
//
// Wildcard domains (*.example.com):
//   - Prefix: tls-wildcard--
//   - Transformation: *. → __, dots → underscores
//   - Example: *.example.com → tls-wildcard--__example_com
//
// The mapping is bidirectional and symmetric:
//   domain → DomainToSecret() → secret
//   secret → SecretToDomain() → domain
```

## Acceptance Criteria

- [ ] 实现 `DomainSecretMapping` 结构体
- [ ] 实现 `DomainToSecret()` 函数
- [ ] 实现 `SecretToDomain()` 函数
- [ ] 实现 `ParseDomain()` 函数
- [ ] 实现 `ValidateSecretName()` 函数
- [ ] 所有单元测试通过（至少 60 个测试用例）
- [ ] 测试覆盖率 > 95%
- [ ] 双向转换测试 100% 通过
- [ ] 边界情况全部处理
- [ ] 更新 `acme/storage.go` 使用新函数
- [ ] 所有现有测试通过
- [ ] 代码通过 `go vet` 和 `golint` 检查
- [ ] 添加完整的包文档和函数文档

## Technical Notes

### Kubernetes Secret 命名规范

根据 [Kubernetes 文档](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)：

- 最多 253 字符
- 只能包含小写字母、数字、`-` 和 `.`
- 必须以字母或数字开头
- 必须以字母或数字结尾

### 为什么使用下划线而不是横杠

- 横杠 `-` 在域名中是合法字符（如 `my-api.example.com`）
- 如果用横杠替换点号，会导致歧义：
  - `my-api.example.com` → `my-api-example-com` (无法区分原始横杠和替换的横杠)
- 使用下划线 `_` 避免歧义：
  - `my-api.example.com` → `my-api_example_com` (清晰区分)

### 为什么使用双下划线表示通配符

- 单下划线 `_` 用于替换点号
- 双下划线 `__` 用于替换 `*.`
- 这样可以明确区分：
  - `*.example.com` → `__example_com` (通配符)
  - `_.example.com` → `__example_com` (如果域名真的叫 `_`，虽然不合法)

## Examples

### 完整示例

```go
// Example 1: Normal domain
domain := "api.example.com"
secret := DomainToSecret(domain)
// secret = "tls-normal--api_example_com"

recovered, _ := SecretToDomain(secret)
// recovered = "api.example.com"

// Example 2: Wildcard domain
domain := "*.api.example.com"
secret := DomainToSecret(domain)
// secret = "tls-wildcard--__api_example_com"

recovered, _ := SecretToDomain(secret)
// recovered = "*.api.example.com"

// Example 3: Parse domain
mapping, _ := ParseDomain("*.example.com")
// mapping.OriginalDomain = "*.example.com"
// mapping.DomainType = DomainTypeWildcard
// mapping.SecretName = "tls-wildcard--__example_com"
// mapping.NormalizedDomain = "__example_com"
```

## Success Metrics

- 零冲突：不同域名映射到不同 Secret
- 100% 可逆：所有 Secret 名称都能准确还原为原始域名
- 高性能：转换函数执行时间 < 1μs
- 高可靠：所有边界情况都有明确的错误处理

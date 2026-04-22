# Architecture

## 整体分层设计

系统采用经典四层架构，自上而下依次为：

```
┌─────────────────────────────────────────────┐
│              路由层 (Router)                  │  cmd/api/main.go
├─────────────────────────────────────────────┤
│           中间件层 (Middleware)                │  internal/middleware/
├─────────────────────────────────────────────┤
│            业务层 (Service)                   │  internal/auth/ user/ rbac/ audit/
├─────────────────────────────────────────────┤
│            数据层 (Store)                     │  internal/store/ + Repository
└─────────────────────────────────────────────┘
```

### 路由层

`cmd/api/main.go` 是唯一入口，职责：

1. 初始化基础设施（PostgreSQL、Redis）
2. 执行数据库迁移和 RBAC 种子数据
3. 实例化所有 Service 和 Handler
4. 定义路由分组并挂载中间件

路由分为三级：

| 分组 | 中间件 | 示例路径 |
|------|--------|----------|
| 公开 | 审计日志 | `/api/v1/auth/login` |
| 鉴权 | 审计日志 → JWT 验证 → 用户限流 | `/api/v1/users` |
| 管理 | 审计日志 → JWT 验证 → 用户限流 → RBAC 权限 | `/api/v1/roles` |

### 中间件层

中间件按顺序叠加，每个只做一件事：

| 中间件 | 文件 | 执行时机 | 职责 |
|--------|------|----------|------|
| SecurityHeaders | `security.go` | 请求进入 | 注入 4 个安全响应头 |
| Cors | `cors.go` | 请求进入 | 处理跨域预检和响应头 |
| RequestID | `request_id.go` | 请求进入 | 生成/透传 `X-Request-ID` |
| AuditLog | `audit.go` | 响应完成后 | 异步记录审计日志（含风险等级） |
| RateLimitByIP | `rate_limit.go` | 请求进入 | Redis 滑动窗口限流（按 IP） |
| JWTAuth | `auth.go` | 请求进入 | 验签 + 过期检查 + 黑名单校验 |
| RateLimitByUser | `rate_limit.go` | 请求进入 | Redis 滑动窗口限流（按用户） |
| RequirePermission | `rbac.go` | 请求进入 | RBAC 权限检查 |

### 业务层

每个领域模块包含 Handler（HTTP 接入）、Service（业务逻辑）、DTO（请求/响应结构）：

| 模块 | 包 | 职责 |
|------|------|------|
| 认证 | `internal/auth` | 注册、登录、登出、Token 刷新 |
| 用户 | `internal/user` | 用户 CRUD，定义 Repository 接口 |
| 权限 | `internal/rbac` | 角色/权限管理，CheckPermission，种子数据 |
| 审计 | `internal/audit` | 审计日志记录与多条件查询 |
| 安全 | `internal/security` | JWT 签发/验证，bcrypt 密码哈希 |
| 校验 | `internal/validator` | 用户名/邮箱/密码格式校验 |

### 数据层

| 组件 | 文件 | 职责 |
|------|------|------|
| PostgreSQL | `store/db.go` | GORM 连接初始化 |
| Redis | `store/redis.go` | go-redis 客户端初始化 |
| Token 黑名单 | `store/blacklist.go` | `Add(jti, ttl)` / `IsBlacklisted(jti)` |
| User Repository | `user/repository.go` | 接口 + GORM 实现，全部参数化查询 |

---

## 模块职责详解

### internal/auth — 认证模块

**Service** 持有三个依赖：`user.Repository`（数据库）、`JWTManager`（Token）、`TokenBlacklist`（Redis）。

- **Register**: 校验 → 检查 email/username 唯一 → bcrypt 哈希 → 入库
- **Login**: 查用户 → 比对密码 → 签发 access token (15min) + refresh token (7d)，两个都带 `jti`
- **Logout**: 解析 Bearer token → 提取 `jti` 和剩余 TTL → 写入 Redis 黑名单
- **RefreshToken**: 验证 refresh token → 旧 `jti` 加黑名单 → 签发全新 token 对

**Handler** 负责 JSON 绑定、输入清洗（`Sanitize`）、格式校验（`Validate`）、调用 Service、返回统一响应。

### internal/rbac — 权限模块

数据模型：`User ←多对多→ UserRole ←多对多→ Role ←多对多→ Permission`

- **CheckPermission(userID, permCode)**: Redis 缓存优先 → 缓存未命中时执行三表 JOIN 查询 → 回填缓存（5min TTL）
- **AssignRoleToUser**: 写入 `user_roles` → 主动失效该用户的权限缓存
- **Seed**: 幂等初始化 4 个角色 + 8 个权限码 + 对应关系

### internal/audit — 审计模块

- **AuditMiddleware** 在 `c.Next()` 之后执行，通过 goroutine 异步写入数据库，不阻塞响应
- 风险等级判定：`403` 或 `401 + POST /auth/login` → HIGH，其余 → LOW
- 只记录元数据（method、path、IP、status），不记录请求体，密码无法泄露

### internal/security — 安全工具

- **JWTManager**: HS256 签名，每个 token 携带唯一 `jti`（UUID），用于精确吊销
- **password.go**: 封装 bcrypt `GenerateFromPassword` / `CompareHashAndPassword`

### pkg/response — 统一响应

所有接口返回 `{ code, message, data }` 格式。`Error()` 函数在 `code >= 500` 时自动将 message 替换为 `"internal server error"`，防止泄露堆栈和内部信息。

---

## 请求处理完整链路：以登录为例

```
客户端 POST /api/v1/auth/login
  │
  ├─ SecurityHeaders      → 注入安全响应头
  ├─ Cors                 → 处理跨域
  ├─ RequestID            → 生成 X-Request-ID
  ├─ AuditLog (defer)     → 注册后置钩子
  ├─ RateLimitByIP        → Redis ZSET 滑动窗口检查
  │   ├─ ZREMRANGEBYSCORE 清除 60s 前的条目
  │   ├─ ZCARD 统计窗口内请求数
  │   ├─ >= 5 → 返回 429，中止
  │   └─ < 5  → ZADD 记录本次请求，放行
  │
  ├─ auth.Handler.Login
  │   ├─ ShouldBindJSON → 解析 JSON
  │   ├─ Sanitize()     → trim 空格
  │   ├─ Validate()     → 校验 email 格式
  │   └─ auth.Service.Login
  │       ├─ userRepo.FindByEmail   → SELECT * FROM users WHERE email = $1
  │       ├─ bcrypt.CompareHashAndPassword
  │       ├─ jwtManager.GenerateAccessToken  → HS256 签名，jti=uuid
  │       ├─ jwtManager.GenerateRefreshToken → HS256 签名，jti=uuid
  │       └─ 返回 { access_token, refresh_token, token_type }
  │
  ├─ response.OK → { code: 200, message: "success", data: { ... } }
  │
  └─ AuditLog (执行)
      └─ goroutine: INSERT INTO audit_logs (action, ip, status, risk_level, ...)
         action = "POST /api/v1/auth/login"
         risk_level = status==401 ? "HIGH" : "LOW"
```

后续带 Token 请求的鉴权链路：

```
客户端 GET /api/v1/admin-only
  │
  ├─ ... (SecurityHeaders / Cors / RequestID / AuditLog)
  ├─ JWTAuth
  │   ├─ 提取 Authorization: Bearer <token>
  │   ├─ jwt.ParseWithClaims → 验签 + 过期检查
  │   ├─ blacklist.IsBlacklisted(jti) → Redis EXISTS token:blacklist:<jti>
  │   │   ├─ 存在 → 401 "token has been revoked"
  │   │   └─ 不存在 → 注入 user_id/email/role 到 context
  │   └─ 放行
  ├─ RateLimitByUser → 同 IP 限流逻辑，key = ratelimit:user:<userID>
  ├─ RequirePermission("user:create")
  │   ├─ 从 context 取 user_id
  │   ├─ rbac.CheckPermission(userID, "user:create")
  │   │   ├─ Redis GET rbac:perms:user:<userID>
  │   │   │   ├─ 命中 → 逗号分割查找
  │   │   │   └─ 未命中 → DB 三表 JOIN → 写回 Redis (TTL 5min)
  │   │   └─ 返回 bool
  │   ├─ false → 403 "access denied"
  │   └─ true  → 放行
  └─ Handler → 业务逻辑
```

---

## 关键设计决策

### 为什么用 JWT 而不是 Session？

1. **无状态**: JWT 自包含用户信息（userID、email、role），服务端不需要维护 session 存储，水平扩展时无需 session 同步
2. **双 Token 机制**: Access Token 短有效期（15min）限制泄露窗口，Refresh Token 长有效期（7d）减少重新登录频率
3. **jti 标识**: 每个 Token 携带唯一 ID，支持精确吊销单个 Token，而非一刀切失效所有 Token

### 为什么需要 Token 黑名单？

JWT 的无状态特性意味着签发后无法主动失效。黑名单解决了三个场景：

1. **登出**: 用户主动登出后，旧 Token 不应继续可用
2. **Token 刷新**: 旧 Refresh Token 换取新 Token 后，旧 Token 应立即失效，防止重放攻击
3. **TTL 对齐**: 黑名单条目的过期时间等于 Token 的剩余有效期，Token 自然过期后黑名单自动清理，无需定时任务

选择 Redis 而非数据库：黑名单是高频读操作（每个请求都查），Redis 的 O(1) 查询远优于数据库索引查询。

### 为什么用 RBAC 而不是硬编码角色？

1. **灵活性**: 权限码（如 `user:create`）与角色解耦，新增权限只需数据库操作，无需改代码
2. **最小权限原则**: 每个角色只分配必要的权限码，auditor 只能读日志，developer 只能读用户
3. **多角色支持**: `user_roles` 多对多关系支持一个用户同时拥有多个角色，权限取并集
4. **缓存优化**: 权限查询结果缓存在 Redis（5min TTL），角色变更时主动失效，兼顾性能和一致性

### 为什么用 Redis 滑动窗口做限流？

1. **精确性**: 相比固定窗口，滑动窗口不存在"窗口边界突发"问题（两个窗口交界处可能允许 2 倍流量）
2. **实现**: 利用 Redis Sorted Set，score 为时间戳，ZREMRANGEBYSCORE 清除过期条目，ZCARD 统计当前窗口
3. **分布式**: Redis 天然支持多实例共享计数器，服务水平扩展后限流仍然准确

### 为什么审计日志异步写入？

1. **不阻塞响应**: 审计记录在 goroutine 中执行，用户感知的延迟不包含数据库写入时间
2. **容错**: 审计写入失败不影响业务请求的正常响应（best-effort）
3. **安全性**: 只记录元数据（HTTP method、path、IP、status），不记录请求体，从架构层面杜绝密码泄露

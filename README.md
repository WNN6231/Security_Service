# Security Service

基于 Go + Gin + GORM + PostgreSQL + Redis 的企业级安全服务，提供用户认证、RBAC 权限管理、审计日志和安全防护。

## 快速启动

```bash
# 1. 克隆项目
git clone <repo-url> && cd security-service

# 2. 复制环境变量（生产环境务必修改 JWT_SECRET）
cp .env.example .env

# 3. 一键启动
docker-compose up -d

# 4. 健康检查
curl http://localhost:8080/health
```

服务默认运行在 `http://localhost:8080`。

## 接口列表

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录（IP 限流 5次/分） |
| POST | `/api/v1/auth/refresh` | 刷新 Token |
| POST | `/api/v1/auth/logout` | 用户登出 |

### 需要登录（Bearer Token）

| 方法 | 路径 | 说明 | 所需权限 |
|------|------|------|----------|
| GET | `/api/v1/users` | 用户列表 | - |
| GET | `/api/v1/users/:id` | 用户详情 | - |
| PUT | `/api/v1/users/:id` | 更新用户 | - |
| DELETE | `/api/v1/users/:id` | 删除用户 | - |
| POST | `/api/v1/roles` | 创建角色 | `role:manage` |
| GET | `/api/v1/roles` | 角色列表 | `role:manage` |
| POST | `/api/v1/users/:id/roles` | 分配角色 | `role:manage` |
| GET | `/api/v1/audit-logs` | 审计日志查询 | `log:read` |

**审计日志查询参数**：`user_id`、`risk_level`（HIGH/LOW）、`start_time`、`end_time`（RFC 3339 格式）

## 安全设计

### 认证

- 密码使用 **bcrypt** 哈希存储，明文密码不入库、不入日志
- JWT 双 Token 机制：Access Token（15 分钟）+ Refresh Token（7 天），均携带 `jti`
- 登出 / 刷新时将旧 Token 的 `jti` 写入 **Redis 黑名单**，TTL 等于 Token 剩余有效期
- 认证中间件：验签 → 检查过期 → 查 Redis 黑名单

### RBAC 权限

- 查询链：`user → user_roles → role_permissions → permissions`
- 权限结果缓存至 Redis（5 分钟 TTL），角色变更时主动失效
- 内置角色：admin（全部权限）、auditor（log:read）、developer（user:read）、guest（无权限）

### 输入校验

- username：3-32 位，仅允许字母、数字、下划线
- password：8-72 位，必须包含大小写字母和数字
- email：标准格式校验
- 所有字符串字段 trim 空格，拒绝纯空白输入
- 全部数据库操作使用 GORM 参数化查询，防止 SQL 注入

### 限流

- 登录接口：同 IP 每分钟 5 次（Redis 滑动窗口）
- 鉴权接口：同用户 每分钟 30 次
- 超限返回 `429 Too Many Requests`

### 审计日志

- 记录所有 API 请求：用户 ID、HTTP Method + Path、客户端 IP、响应状态码、风险等级
- 风险等级：登录失败（401 + POST /auth/login）或权限拒绝（403）标记为 **HIGH**，其余为 **LOW**
- 请求体不入审计日志，确保密码等敏感字段不被记录

### 响应安全

- 统一 JSON 响应格式：`{ code, message, data }`
- 5xx 错误自动屏蔽内部详情，统一返回 `"internal server error"`
- 安全响应头：`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Content-Security-Policy: default-src 'self'`、`Referrer-Policy: strict-origin-when-cross-origin`

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务端口 |
| `DB_HOST` | `localhost` | PostgreSQL 主机 |
| `DB_PORT` | `5432` | PostgreSQL 端口 |
| `DB_USER` | `postgres` | PostgreSQL 用户名 |
| `DB_PASSWORD` | `postgres` | PostgreSQL 密码 |
| `DB_NAME` | `security_service` | 数据库名 |
| `DATABASE_URL` | - | 完整连接串（优先级高于上方分字段配置） |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `REDIS_PASSWORD` | - | Redis 密码 |
| `JWT_SECRET` | `change-me-in-production` | **生产环境必须修改** |

## 项目结构

```
cmd/api/              程序入口
internal/
  auth/               认证（注册/登录/登出/刷新）
  user/               用户 CRUD
  rbac/               角色权限管理 + 数据种子
  audit/              审计日志记录与查询
  middleware/          JWT 鉴权 / RBAC / 审计 / 限流 / CORS / 安全头
  security/           JWT 签发验证 / bcrypt 密码哈希
  store/              PostgreSQL + Redis 连接 / Token 黑名单
  validator/          输入校验工具函数
pkg/response/         统一 API 响应
```

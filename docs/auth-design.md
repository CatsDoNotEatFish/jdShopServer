# 鉴权与权限设计

## JWT Token 设计

### Access Token

- 类型：JWT（RS256 或 HS256）
- 有效期：2 小时
- 位置：HTTP Header `Authorization: Bearer <token>`

Payload 结构：

```json
{
  "sub": "1",
  "username": "zhangsan",
  "nickname": "张三",
  "roles": ["user"],
  "iat": 1720000000,
  "exp": 1720007200,
  "jti": "unique-token-id"
}
```

说明：
- `sub`：用户 ID
- `roles`：角色数组，用于 RBAC 中间件快速判断
- `jti`：Token 唯一 ID，可用于黑名单（服务端吊销时）

### Refresh Token

- 类型：不透明 Token（随机 256-bit 字符串，base64 编码）
- 有效期：30 天
- 存储：`refresh_tokens` 表，保存 SHA256(token) 而非原文
- 轮转策略：每次使用后吊销旧 Token、签发新 Token

轮转的好处：
- 如果 Refresh Token 被窃取，攻击者和合法用户会竞争使用
- 竞争后旧 Token 被吊销，下次合法用户刷新失败 → 触发重新登录 → 攻击者新 Token 也被吊销
- 不算完美防护（第一次攻击者可能先到），但大幅缩小攻击窗口

### Token 签发流程

```text
1. 用户登录 → 验证密码 (bcrypt)
2. 查询用户角色
3. 签发 Access Token (2h) + Refresh Token (30d)
4. Refresh Token SHA256 存库
5. 返回 Token 对
```

### Token 刷新流程

```text
1. 客户端携带 Refresh Token 请求 /auth/refresh
2. 服务端 SHA256(token) 查库
3. 验证未过期、未吊销
4. 吊销旧 Refresh Token
5. 签发新 Access Token + 新 Refresh Token
6. 返回新 Token 对
```

### Token 吊销

以下情况需要吊销 Refresh Token：
- 用户修改密码 → 吊销该用户所有 Refresh Token
- 用户登出 → 吊销当前 Refresh Token
- 管理员禁用用户 → 吊销该用户所有 Refresh Token

Access Token 不主动吊销（无状态设计），依赖短有效期自然过期。如需立即生效，可使用 Token 黑名单（内存 LRU 缓存）。

## 密码策略

### 密码哈希

使用 bcrypt，cost=10：

```go
passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
```

cost=10 在 1C1G 服务器上每次 hash 约 200-300ms，登录时延迟可接受。

### 密码规则

- 最小长度：6 字符
- 最大长度：64 字符
- 不强制大小写/数字/特殊字符组合（减少用户体验摩擦，后续可扩展）

### 登录保护

- 同一 IP 连续 5 次登录失败 → 15 分钟内拒绝该 IP 登录请求
- 同一用户名连续 5 次登录失败 → 15 分钟内拒绝该用户名登录
- 使用 `login_logs` 表记录，失败计数基于 `created_at > now - 15min`

被锁定时返回 `{"code": 10003, "message": "登录尝试过于频繁，请15分钟后再试"}`。

## RBAC 权限模型

### 模型

```text
User ──┬── Role ──┬── Permission
       │          │
       └── Role ──┘
```

- 用户可以有多个角色
- 角色可以有多个权限
- 用户最终权限 = 所有角色的权限的并集
- 权限在 JWT 的 `roles` 字段中编码，中间件不查库

### 中间件链

路由定义时指定所需权限：

```go
// router.go
r.Group(func(r chi.Router) {
    r.Use(middleware.Auth(jwtSecret))
    r.Use(middleware.RequireRole("admin"))

    r.Get("/api/v1/admin/users", adminHandler.ListUsers)
    r.Post("/api/v1/admin/announcements", announcementHandler.Create)
})
```

更细粒度的权限控制：

```go
r.Group(func(r chi.Router) {
    r.Use(middleware.Auth(jwtSecret))
    r.Use(middleware.RequirePermission("announcement:create"))

    r.Post("/api/v1/admin/announcements", announcementHandler.Create)
})
```

### 权限判断流程

```text
1. Auth 中间件解析 JWT → 提取 user_id、roles 到 context
2. RequireRole/RequirePermission 中间件从 context 读取
3. 权限不够 → 返回 403
4. 权限足够 → 进入 Handler
```

角色到权限的映射在 Token 签发时固化到 JWT 中。如果角色权限发生变更，需要用户重新登录（或等 Access Token 过期）才能生效。对于大多数场景，2 小时内生效是合理的。

## 预置角色和权限

### admin（管理员）

拥有所有权限。不可删除此角色。

### user（普通用户）

注册时默认分配，拥有以下权限：
- 访问个人资料
- 修改个人信息
- 心跳上报
- 公告读取
- 版本检查

## 安全配置

在 `config.yaml` 中：

```yaml
auth:
  jwt_secret: "至少32字节的随机字符串"   # 生产环境通过环境变量注入
  access_token_ttl: 7200                  # 秒，2小时
  refresh_token_ttl: 2592000              # 秒，30天
  bcrypt_cost: 10
  login_max_attempts: 5
  login_lock_minutes: 15
```

JWT Secret 必须通过环境变量 `JWT_SECRET` 注入，config.yaml 上的默认值仅用于本地开发。

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

Access Token 携带账号 `auth_version`。每个受保护接口都会同时校验账号状态和数据库中的当前版本；管理员禁用账号或用户修改密码时版本递增，因此此前签发的全部 Access Token 永久失效，即使账号随后重新启用也不能恢复。管理员禁用时还会吊销该账号全部 Refresh Token，重新启用后必须重新登录。

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
- 同一手机号连续 5 次登录失败 → 15 分钟内拒绝该手机号登录
- 使用 `login_logs` 表记录，失败计数基于 `created_at > now - 15min`

被锁定时返回 `{"code": 10003, "message": "登录尝试过于频繁，请15分钟后再试"}`。

### 手机号、图形验证码与短信验证

- 新注册普通账号使用中国大陆11位手机号作为登录账号，注册前必须验证短信验证码；主管理员仍使用内部账号 `admin`。
- 短信发送前必须提交服务端生成的一次性图形验证码。验证码为5位字母数字混合，使用PNG栅格图返回，包含逐字符旋转/错位、波形扭曲、干扰曲线和随机噪点；不使用会泄露明文答案的SVG `<text>`。答案只保存在进程内存中，5分钟过期，任何校验结果都会立即销毁。
- 同一手机号跨注册/改密用途统一限制为60秒1条、北京时间自然日6条；同一来源IP每小时最多20条。
- 6位短信验证码5分钟有效、最多试错5次、验证成功即消费。数据库只保存以 JWT Secret 为 pepper 的 HMAC-SHA256，不保存验证码明文。
- 连续短信会检查上一次验证码摘要，确保同一手机号不会连续收到相同验证码内容。
- 修改已绑定手机号账号的密码必须再次完成图形验证码与 `password_reset` 短信验证；成功后撤销全部 Refresh Token并递增 `auth_version`。
- 迁移前没有手机号的旧账号暂时保留用户名登录和旧密码改密，客户端明确标注为过渡兼容路径。

### 传输安全

公网密码、短信验证码、JWT 和短信平台凭据只通过 HTTPS/TLS 传输。Windows 客户端会拒绝非 localhost 的 HTTP 鉴权地址；Nginx HTTPS响应启用 HSTS，Go服务只监听 `127.0.0.1`。短信宝调用固定使用安全接口，HTTP地址会被服务端拒绝。浏览器开发者工具显示的是TLS加密前/解密后的本机数据，不代表公网抓包可读取明文。

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
    r.Use(middleware.RequireSuperAdmin(superAdminUsername))

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

### admin（主管理员专用角色）

拥有所有权限。该角色不可修改、不可删除，也不能分配给普通账号。管理接口同时校验 `SUPER_ADMIN_USERNAME` 对应的用户名和 `admin` 角色，仅持有角色但用户名不匹配的账号仍返回 403。

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
  super_admin_username: "admin"          # 唯一允许进入服务端管理台的账号
  access_token_ttl: 7200                  # 秒，2小时
  refresh_token_ttl: 2592000              # 秒，30天
  bcrypt_cost: 10
  login_max_attempts: 5
  login_lock_minutes: 15
```

JWT Secret 必须通过环境变量 `JWT_SECRET` 注入，config.yaml 上的默认值仅用于本地开发。主管理员用户名可通过 `SUPER_ADMIN_USERNAME` 覆盖，默认使用内置 `admin` 账号。

主管理员账号是服务端控制账号，不属于可远程禁用的客户端账号：后台禁止修改它的启停状态和角色，并在所有登录、资料及心跳响应中固定返回长期有效、三个产品板块全部开放。其他注册账号不能进入 `/admin` 或调用 `/api/v1/admin/*`。

## 账号使用期与产品板块权限

`user_access_control` 独立保存账号授权，不把产品板块开关混入 RBAC 角色。每次登录、Refresh Token 和心跳都会重新计算账号状态、到期时间以及三个模块开关。

- 初始注册策略为赠送 30 天且仅开放竞品监控；主管理员可在控制台“注册默认”中设置 1-3650 天和三个板块开关。
- 注册策略保存在 `registration_defaults`，注册成功时复制到新账号的 `user_access_control`。修改策略只影响之后注册的账号，不追溯修改已有账号。
- 管理员可通过 `/api/v1/admin/users/:id/access` 远程修改使用期和模块开关。
- 心跳返回授权租约；账号到期时客户端锁定本地业务。账号禁用时 Access Token 校验立即返回 401，客户端清除全部 Token、终止未完成任务并显示不可点击的“账户被禁用”。
- 网络短暂不可用时客户端保留有限租约；租约耗尽后回到登录页。
- 客户端登录后同时连接 `/api/v1/control/stream`。管理员变更账号状态、使用期或模块开关时，服务端按用户推送通知，客户端立即强制心跳确认并更新界面。
- SSE 仅负责降低生效延迟，不能替代心跳和租约；断线时自动重连，每 1 分钟心跳继续兜底。
## 敏感鉴权请求的应用层加密

登录、注册、发送短信、刷新/注销令牌和修改密码支持 `application/jdshop-encrypted+json` 请求体。Windows客户端登录页和 `www` 主管理员控制台登录页均必须先通过 `GET /api/v1/auth/encryption-key` 获取服务端 RSA-OAEP 公钥，再使用 AES-256-GCM 加密 JSON；AES 密钥由 RSA-OAEP-SHA256 包裹。信封包含服务端路径、毫秒时间戳和随机 `request_id`，这些字段同时参与 GCM AAD；服务端拒绝路径不匹配、超过2分钟、重复 `request_id` 或认证标签错误的请求。私钥只保存在服务端数据目录，文件权限为0600。

`auth.require_encrypted_requests=true` 时，上述接口拒绝普通JSON。应用层加密用于隐藏浏览器Network面板和本地代理中的业务字段、提高脚本复用成本，不能替代HTTPS，也不能阻止控制客户端机器的攻击者在输入框或JavaScript运行时读取明文；图形验证码、短信频控、登录限流和服务端风控仍是防滥用边界。

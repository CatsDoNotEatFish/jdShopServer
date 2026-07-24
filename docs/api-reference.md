# API 接口参考

生产基础路径: `https://api.jdshop.bbroot.com/api/v1`

API域名只开放 `/api/*`，访问根路径或 `/admin` 返回404。接口文档仅保存在仓库中，不再作为可交互测试页面部署到公网。主管理员控制台位于 `https://www.jdshop.bbroot.com/admin`，页面通过CORS调用API域名。

## 通用说明

### 响应格式

所有接口统一返回:

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

分页接口 data 包装:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 错误码

| code | HTTP 状态 | 含义 |
|------|-----------|------|
| 0 | 200 | 成功 |
| 10001 | 400 | 参数校验失败 |
| 10002 | 401 | 未认证（Token 缺失、过期、无效） |
| 10003 | 403 | 无权限 / 账号禁用 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（如手机号已注册） |
| 10006 | 429 | API请求频率或短信发送次数超限 |
| 10500 | 500 | 服务内部错误 |
| 10503 | 503 | 短信服务未配置或供应商暂不可用 |

### 鉴权方式

需鉴权的接口在 Header 中携带:

```text
Authorization: Bearer <access_token>
```

Access Token 有效期 2 小时，过期后使用 Refresh Token 换取新 Token。

生产环境只允许通过 `https://api.jdshop.bbroot.com` 传输密码、短信验证码和 Token。客户端会拒绝连接非 localhost 的 HTTP 鉴权地址；Nginx 同时启用 TLS 与 HSTS。`http://127.0.0.1` 只用于同机开发测试。

### 敏感请求应用层加密

`POST /auth/sms/send`、`POST /auth/register`、`POST /auth/login` 和 `PUT /user/password` 在生产配置下必须使用 `application/jdshop-encrypted+json`。客户端先读取 `GET /auth/encryption-key`，再用 RSA-OAEP-256 包装一次性 AES-256-GCM 密钥；信封同时绑定完整接口路径、毫秒时间戳和一次性 `request_id`。直接提交 `application/json` 会返回 `code=10001`。

本页为便于说明而展示的“请求体”均是**加密前逻辑字段**，不能直接照抄为生产 curl 请求。可执行实现以 Windows 客户端 `jd_monitor/license.py` 和管理台 `static/admin.html` 为准。加密信封结构如下：

```json
{
  "v": 1,
  "alg": "RSA-OAEP-256+A256GCM",
  "kid": "服务端公钥ID",
  "key": "Base64(RSA加密后AES密钥)",
  "nonce": "Base64(12字节随机数)",
  "ciphertext": "Base64(AES-GCM密文及认证标签)",
  "ts": 1784860000000,
  "request_id": "一次性随机ID",
  "path": "/api/v1/auth/login"
}
```

---

## 1. 健康检查

### GET /api/v1/health

无需鉴权。服务存活检查。

**curl 示例:**

```bash
curl http://127.0.0.1:8080/api/v1/health
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "healthy",
    "time": "2026-07-04T09:05:21Z",
    "version": "1.0.0"
  }
}
```

---

## 2. 手机号注册与验证码

### GET /api/v1/auth/captcha

获取一次性图形验证码。返回的 `image` 是可直接赋给 `<img src>` 的PNG Data URL。验证码由5位大写英文字母和数字混合组成，排除 `0/O/1/I/L` 等易混字符，并在服务端完成字符旋转、错位、波形扭曲、干扰曲线和噪点栅格化。响应图片不包含SVG文本或可直接提取的验证码元数据。验证码5分钟过期，校验一次后立即作废，无论成功失败都必须刷新。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "captcha_id": "随机ID",
    "image": "data:image/png;base64,...",
    "expires_at": "2026-07-23T13:05:00Z"
  }
}
```

### POST /api/v1/auth/sms/send

图形验证码通过后发送短信验证码。`purpose` 支持 `register` 和 `password_reset`。

```json
{
  "phone": "13800138000",
  "purpose": "register",
  "captcha_id": "上一步返回的ID",
  "captcha_code": "1234"
}
```

服务端强制执行：同一手机号60秒内最多1条（跨用途统一计算）、中国标准时间自然日最多6条、同一IP每小时最多20条；连续短信不会复用相同验证码。验证码使用 HMAC-SHA256 保存，5分钟有效、最多尝试5次。短信平台只调用 HTTPS 安全接口。

### POST /api/v1/auth/register

无需鉴权，但必须先完成图形验证码和注册短信验证。

**请求体:**

```json
{
  "phone": "13800138000",
  "password": "test123",
  "sms_code": "654321",
  "email": "test@example.com",
  "nickname": "Test User"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| phone | 是 | 中国大陆11位手机号 |
| password | 是 | 6-64 字符 |
| sms_code | 是 | 6位注册短信验证码 |
| email | 否 | 邮箱 |
| nickname | 否 | 显示昵称 |

生产请求必须按“敏感请求应用层加密”生成信封，不能直接发送上述逻辑字段。

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "username": "13800138000",
    "phone": "13800138000",
    "nickname": "Test User"
  }
}
```

**失败返回:**

```json
// 手机号已注册 (409)
{"code": 10005, "message": "该手机号已注册", "data": null}

// 参数校验失败 (400)
{"code": 10001, "message": "短信验证码错误", "data": null}
{"code": 10001, "message": "密码长度须为6-64字符", "data": null}
```

---

## 3. 登录

### POST /api/v1/auth/login

无需鉴权。成功返回 JWT Token 对。

**请求体:**

```json
{
  "phone": "13800138000",
  "password": "test123"
}
```

生产请求必须按“敏感请求应用层加密”生成信封，不能直接发送上述逻辑字段。

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "C33IPlK98wiuA9Ax5xdOg...",
    "expires_in": 7200,
    "user": {
      "id": 2,
      "username": "13800138000",
      "phone": "13800138000",
      "nickname": "Test User",
      "roles": ["user"]
    }
  }
}
```

| 字段 | 说明 |
|------|------|
| access_token | JWT，有效期 2 小时，后续请求携带 |
| refresh_token | 刷新 Token，有效期 30 天，用于续期 |
| expires_in | Access Token 有效期（秒） |
| user.roles | 角色数组，用于权限判断 |

**失败返回:**

```json
// 手机号或密码错误 (400)
{"code": 10001, "message": "手机号或密码错误", "data": null}

// 账号被禁用 (403)
{"code": 10003, "message": "账号已被禁用", "data": null}

// 登录频率限制 (403)
{"code": 10003, "message": "登录尝试过于频繁，请15分钟后再试", "data": null}
```

**登录保护规则:**
- 同 IP 或同手机号 5 分钟内连续失败 5 次 → 锁定 15 分钟
- 所有登录尝试记录到 `login_logs` 表

---

## 4. 刷新 Token

### POST /api/v1/auth/refresh

无需鉴权。使用 Refresh Token 换取新的 Access Token。

**请求体:**

```json
{
  "refresh_token": "C33IPlK98wiuA9Ax5xdOg..."
}
```

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"C33IPlK98wiuA9Ax5xdOg..."}'
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "newRefreshTokenHere...",
    "expires_in": 7200
  }
}
```

**重要:** Refresh Token 采用轮转策略——每次刷新后旧 Token 立即吊销，返回新 Refresh Token。如果 Refresh Token 被窃取，合法用户和攻击者竞争使用，竞争失败的一方下次发现 Token 已吊销会自动触发重新登录。

**失败返回:**

```json
// Token 已失效 (401)
{"code": 10002, "message": "Token已失效，请重新登录", "data": null}

// Token 已过期 (401)
{"code": 10002, "message": "Token已过期，请重新登录", "data": null}

// 账号被禁用 (403)
{"code": 10003, "message": "账号已被禁用", "data": null}
```

---

## 4.1 退出登录

### POST /api/v1/auth/logout

无需 Access Token。提交当前 Refresh Token 后立即吊销该登录会话；重复调用同样返回成功。

```json
{
  "refresh_token": "C33IPlK98wiuA9Ax5xdOg..."
}
```

---

## 5. 个人信息

### GET /api/v1/user/profile

需要鉴权。

**curl 示例:**

```bash
curl http://127.0.0.1:8080/api/v1/user/profile \
  -H "Authorization: Bearer $TOKEN"
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "username": "testuser",
    "email": "test@example.com",
    "nickname": "Test User",
    "avatar_url": null,
    "status": 1,
    "last_login_at": "2026-07-04T09:06:20Z",
    "created_at": "2026-07-04 09:06:16",
    "updated_at": "2026-07-04 09:06:16",
    "roles": ["user"]
  }
}
```

### PUT /api/v1/user/profile

需要鉴权。修改个人信息（只更新非空字段）。

**请求体:**

```json
{
  "nickname": "New Name",
  "email": "newemail@example.com",
  "avatar_url": ""
}
```

**curl 示例:**

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/user/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"nickname":"New Name","email":"new@example.com"}'
```

**成功返回 (200):** 返回更新后的完整用户信息（结构同 GET）。

### PUT /api/v1/user/password

需要鉴权。已绑定手机号的账号必须先以 `purpose=password_reset` 完成图形验证码和短信发送，再提交短信验证码。修改后所有 Refresh Token 吊销、`auth_version` 递增，需重新登录。迁移前尚未绑定手机号的旧账号临时保留旧密码校验方式。

**请求体:**

```json
{
  "new_password": "newPassword456",
  "sms_code": "654321"
}
```

生产请求必须携带 Access Token，并按“敏感请求应用层加密”生成信封，不能直接发送上述逻辑字段。

**成功返回 (200):**

```json
{"code": 0, "message": "密码修改成功", "data": null}
```

**失败返回 (400):**

```json
{"code": 10001, "message": "短信验证码错误", "data": null}
```

---

## 6. 心跳上报

### POST /api/v1/heartbeat

需要鉴权。建议客户端每 1 分钟上报一次。

**请求体:**

```json
{
  "device_id": "device-uuid-xxx",
  "platform": "windows",
  "app_version": "0.2.2",
  "app_version_code": 2026072202
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| device_id | 是 | 设备唯一标识 |
| platform | 否 | 平台: windows/mac/linux |
| app_version | 否 | 客户端版本号 |
| app_version_code | 否 | 客户端数字版本码，用于可靠比较更新 |

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/heartbeat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"dev-001","platform":"windows","app_version":"0.2.2","app_version_code":2026072202}'
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "has_new_version": true,
    "latest_version_code": 2026072401,
    "latest_version_name": "0.2.4",
    "latest_download_url": "https://jdshop-client-releases-hk.oss-cn-hongkong.aliyuncs.com/JDMonitor-0.2.4-win-x64.zip",
    "latest_file_hash": "sha256:bfdf42bde2095c5a1abb209117e65eefe883d902850d269843ccc4a85616e056",
    "latest_file_size": 134822980,
    "is_force_update": false,
    "access": {
      "allowed": true,
      "reason": "active",
      "expires_at": "2026-12-31T23:59:59Z",
      "remaining_seconds": 86400,
      "lease_seconds": 600,
      "modules": {
        "competitor_monitor": true,
        "merchant_backend": false,
        "analysis_center": false
      }
    }
  }
}
```

如果 `has_new_version` 为 true，客户端应提示用户更新。`is_force_update` 为 true 表示强制更新。服务端同时返回最新版本码、下载地址、文件大小和 SHA-256，客户端完成完整包校验后再替换程序。

客户端每 1 分钟上报一次心跳。账号到期时心跳返回 HTTP 200 且 `access.allowed=false`。管理员禁用账号时，账号授权版本立即递增，当前及此前签发的 Access Token 请求返回 HTTP 401；全部 Refresh Token 同时吊销。客户端必须清除本地 Token、终止未完成任务并回到登录页。账号重新启用后，旧 Token 仍然无效，必须重新登录。

### GET /api/v1/control/stream

需要 JWT。该接口建立按当前登录用户隔离的 Server-Sent Events（SSE）长连接。管理员修改账号启停状态、使用期或三个产品板块权限后，服务端立即发送通知；通知只用于唤醒客户端，客户端必须随后调用心跳接口获取权威授权结果。

```bash
curl -N --http1.1 http://127.0.0.1:8080/api/v1/control/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream"
```

事件示例为 `event: control`，JSON 数据包含 `type` 和 `issued_at`。`type` 可能为 `connected`、`access_changed` 或 `account_status_changed`。服务端每 15 秒发送 SSE 注释保活；客户端断线后应自动重连。此接口不得经过 30 秒通用请求超时，Nginx 也必须关闭 `proxy_buffering` 并配置长读取超时。

---

## 7. 公告（公开）

### GET /api/v1/announcements

无需鉴权。获取已发布公告列表。

**参数:**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| page | 1 | 页码 |
| page_size | 20 | 每页条数 |
| level | (空) | 筛选级别: info/warning/critical |

**curl 示例:**

```bash
# 获取所有已发布公告
curl "http://127.0.0.1:8080/api/v1/announcements"

# 只看 warning 级别的
curl "http://127.0.0.1:8080/api/v1/announcements?level=warning"

# 分页
curl "http://127.0.0.1:8080/api/v1/announcements?page=1&page_size=10"
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 1,
        "title": "System Maintenance",
        "content": "Maintenance on July 10th",
        "level": "warning",
        "is_published": 1,
        "published_at": "2026-07-04T09:06:34Z",
        "created_by": 1,
        "created_at": "2026-07-04 09:06:34",
        "updated_at": "2026-07-04T09:06:34Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

公开接口只返回投放范围为“全部账号”且当前时间有效的公告，用于兼容旧客户端。新版客户端登录后使用以下鉴权接口。

### GET /api/v1/user/announcements

需要 JWT。按当前用户、平台、客户端版本和生效时间返回可见公告，并记录送达回执。

查询参数：

| 参数 | 说明 |
|------|------|
| platform | 客户端平台，当前使用 windows |
| version_code | 数字客户端版本码 |

返回 `items`、`total`、`unread_count` 和 `requires_ack_count`。每条公告包含 `revision`、`is_read`、`is_acknowledged` 和展示策略。

### POST /api/v1/user/announcements/:id/read

需要 JWT。把当前公告修订记录为已读。

### POST /api/v1/user/announcements/:id/acknowledge

需要 JWT。把当前公告修订记录为已读且已确认。非目标用户提交时返回公告不存在。

## 8. 版本检查

### GET /api/v1/version/latest

无需鉴权。检查指定平台是否有新版本。

**参数:**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| platform | "windows" | 平台: windows/mac/linux/android/ios |
| current_version_code | 0 | 客户端当前版本号 |

**curl 示例:**

```bash
# 检查 Windows 平台是否有新版本
curl "http://127.0.0.1:8080/api/v1/version/latest?platform=windows&current_version_code=2026070100"
```

**有更新时返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "has_update": true,
    "is_force": false,
    "version": {
      "id": 1,
      "platform": "windows",
      "version_code": 2026070401,
      "version_name": "v1.0.1",
      "title": "v1.0.1 First Release",
      "description": "Bug fixes",
      "download_url": null,
      "file_size": null,
      "file_hash": null,
      "is_force": 0,
      "is_latest": 1,
      "created_at": "2026-07-04 09:06:34"
    }
  }
}
```

**无更新时返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "has_update": false,
    "is_force": false
  }
}
```

判断逻辑: `latest.version_code > current_version_code` 时有更新。

---

## 9. 管理员接口

以下接口仅允许 `SUPER_ADMIN_USERNAME` 指定的唯一主管理员（默认 `admin`）访问，并且该账号必须持有内置 `admin` 角色。普通注册账号即使误获 `admin` 角色也会返回 403。主管理员应通过管理台的加密登录流程获取 Token；下文 `$TOKEN` 仅表示已经安全取得的 Access Token，不提供会被生产环境拒绝的明文密码 curl 示例。

### 9.1 用户管理

#### GET /api/v1/admin/users

用户列表（分页、搜索、筛选）。

**参数:**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| page | 1 | 页码 |
| page_size | 20 | 每页条数 |
| keyword | (空) | 搜索用户名/昵称/邮箱 |
| status | (空) | 筛选: 0=禁用, 1=正常 |

**curl 示例:**

```bash
curl "http://127.0.0.1:8080/api/v1/admin/users?keyword=admin&status=1" \
  -H "Authorization: Bearer $TOKEN"
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 1,
        "username": "admin",
        "email": null,
        "nickname": "管理员",
        "avatar_url": null,
        "status": 1,
        "last_login_at": "2026-07-04T09:06:33Z",
        "last_heartbeat_at": "2026-07-22T09:30:00Z",
        "heartbeat_device_id": "2dbf6c95-...",
        "heartbeat_platform": "windows",
        "heartbeat_app_version": "0.2.4",
        "created_at": "2026-07-04 09:06:16",
        "updated_at": "2026-07-04 09:06:16",
        "RoleNames": "admin"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

注意: `RoleNames` 字段是逗号分隔的角色名（来自 SQL GROUP_CONCAT）。列表同时返回该账号最近一次心跳的时间、设备、平台和客户端版本，管理控制台据此以 10 分钟为窗口显示在线状态。

#### PUT /api/v1/admin/users/:id/status

启用/禁用普通用户。禁用时递增账号授权版本并吊销全部 Refresh Token；重新启用不会恢复旧 Token，用户必须重新登录。主管理员账号受保护，不能调用此接口禁用；其账号授权固定为长期有效并开放全部三个产品板块。

**请求体:**

```json
{"status": 0}
```

`status`: 0=禁用, 1=启用。

**curl 示例:**

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/admin/users/2/status \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": 0}'
```

**成功返回 (200):**

```json
{"code": 0, "message": "状态修改成功", "data": null}
```

#### POST /api/v1/admin/users/:id/roles

分配普通用户角色（替换模式，全量替换非追加）。内置 `admin` 角色为主管理员专用，不能分配给普通账号；主管理员自身的角色也不能修改。

**请求体:**

```json
{"role_ids": [1, 2]}
```

传空数组清空所有角色。

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/users/2/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role_ids": [2]}'
```

#### PUT /api/v1/admin/users/:id/access

修改账号使用期和三大产品板块开关。`expires_at` 传 `null` 表示长期有效。

```json
{
  "competitor_monitor": true,
  "merchant_backend": true,
  "analysis_center": false,
  "expires_at": "2026-12-31T23:59:59Z"
}
```

### 9.2 新用户注册默认策略

这里配置的是注册模板：新账号注册成功时复制一次，已有账号不会随模板变化。初始值为赠送 30 天、仅开放竞品监控。

#### GET /api/v1/admin/registration-defaults

读取当前注册默认策略。

```bash
curl http://127.0.0.1:8080/api/v1/admin/registration-defaults \
  -H "Authorization: Bearer $TOKEN"
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "usage_days": 30,
    "competitor_monitor": true,
    "merchant_backend": false,
    "analysis_center": false,
    "updated_at": "2026-07-23T12:00:00Z"
  }
}
```

#### PUT /api/v1/admin/registration-defaults

保存新用户默认赠送天数和三个产品板块。`usage_days` 必须是 1-3650 的整数，三个板块可以任意组合，也允许全部关闭。

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/admin/registration-defaults \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "usage_days": 7,
    "competitor_monitor": true,
    "merchant_backend": false,
    "analysis_center": false
  }'
```

成功响应的 `data` 与 GET 相同，并带有本次保存的 `updated_at`。天数不在允许范围时返回：

```json
{"code":10001,"message":"默认赠送天数须为1-3650天","data":null}
```

### 9.3 角色管理

#### GET /api/v1/admin/roles

角色列表（含权限）。

```bash
curl http://127.0.0.1:8080/api/v1/admin/roles \
  -H "Authorization: Bearer $TOKEN"
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "admin",
      "description": "系统管理员",
      "created_at": "2026-07-04 09:06:16",
      "permissions": [
        {"id": 1, "code": "user:list", "name": "查看用户列表", "description": "查看所有注册用户"},
        {"id": 2, "code": "user:update", "name": "修改用户状态", "description": "启用或禁用用户"}
      ]
    },
    {
      "id": 2,
      "name": "user",
      "description": "普通用户",
      "created_at": "2026-07-04 09:06:16",
      "permissions": [
        {"id": 7, "code": "announcement:list", "name": "查看公告列表", "description": "查看公告（含草稿）"},
        {"id": 13, "code": "version:list", "name": "查看版本列表", "description": "查看所有版本"}
      ]
    }
  ]
}
```

#### POST /api/v1/admin/roles

创建角色。

**请求体:**

```json
{
  "name": "editor",
  "description": "内容编辑",
  "permission_ids": [7, 8, 9, 10, 11]
}
```

#### PUT /api/v1/admin/roles/:id

更新角色（替换模式）。

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/admin/roles/3 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"senior-editor","permission_ids":[7,8,9,10,11,12]}'
```

#### DELETE /api/v1/admin/roles/:id

删除角色（内置 admin 角色不能修改或删除）。

#### GET /api/v1/admin/permissions

获取所有权限列表。

```bash
curl http://127.0.0.1:8080/api/v1/admin/permissions \
  -H "Authorization: Bearer $TOKEN"
```

### 9.4 公告管理

#### GET /api/v1/admin/announcements

管理端公告列表（含草稿）。

参数同公开接口，级别筛选同样适用。

```bash
curl "http://127.0.0.1:8080/api/v1/admin/announcements?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN"
```

#### POST /api/v1/admin/announcements

创建公告（默认草稿状态）。

**请求体:**

```json
{
  "title": "系统维护通知",
  "content": "服务器将于 7月10日 凌晨 2:00-4:00 进行维护升级，届时服务不可用。",
  "level": "warning",
  "display_mode": "banner",
  "show_policy": "once",
  "starts_at": "2026-07-25T02:00:00Z",
  "ends_at": "2026-07-26T02:00:00Z",
  "target_type": "all",
  "target_platform": "windows",
  "min_version_code": 2026072401,
  "max_version_code": 0,
  "target_user_ids": [],
  "action_text": "查看详情",
  "action_url": "https://www.jdshop.bbroot.com/notice/maintenance"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| title | 是 | 公告标题 |
| content | 是 | 公告内容 |
| level | 否 | info / warning / critical，默认 info |
| display_mode | 否 | center / banner / modal，默认 center |
| show_policy | 否 | once / every_start / require_ack，默认 once |
| starts_at / ends_at | 否 | RFC3339 生效/失效时间 |
| target_type | 否 | all / users；users 时 target_user_ids 至少一个 |
| target_platform | 否 | all / windows |
| min_version_code / max_version_code | 否 | 0或空值表示不限 |
| action_text / action_url | 否 | 操作按钮和 HTTPS 链接 |
| target_user_ids | 否 | 指定投放账号ID数组 |

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/announcements \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"System Maintenance","content":"...","level":"warning"}'
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "title": "System Maintenance",
    "content": "...",
    "level": "warning",
    "is_published": 0,
    "created_by": 1,
    "created_at": "2026-07-04 09:06:34",
    "updated_at": "2026-07-04 09:06:34"
  }
}
```

#### PUT /api/v1/admin/announcements/:id

编辑公告。所有字段可选；管理控制台提交完整配置。修改已发布公告会递增 `revision`、实时通知目标客户端，并使新修订重新进入未读状态。

**请求体:**

```json
{
  "title": "新标题",
  "content": "新内容",
  "level": "critical"
}
```

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/admin/announcements/2 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated Title"}'
```

返回更新后的公告对象。

#### DELETE /api/v1/admin/announcements/:id

删除公告（硬删除，不可恢复）。

```bash
curl -X DELETE http://127.0.0.1:8080/api/v1/admin/announcements/2 \
  -H "Authorization: Bearer $TOKEN"
```

#### POST /api/v1/admin/announcements/:id/publish

发布公告。设置 `is_published=1`、记录 `published_at`、递增发布修订号，并通过 SSE 实时通知在线目标客户端。

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/announcements/2/publish \
  -H "Authorization: Bearer $TOKEN"
```

返回: `{"code": 0, "message": "发布成功", "data": null}`

#### POST /api/v1/admin/announcements/:id/unpublish

下架公告。设置 `is_published=0`。

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/announcements/2/unpublish \
  -H "Authorization: Bearer $TOKEN"
```

返回: `{"code": 0, "message": "已下架", "data": null}`

### 9.5 版本管理

#### GET /api/v1/admin/versions

版本列表（分页、按平台筛选）。

**参数:**

| 参数 | 默认值 | 说明 |
|------|--------|------|
| page | 1 | 页码 |
| page_size | 20 | 每页条数 |
| platform | (空) | 筛选平台 |

```bash
curl "http://127.0.0.1:8080/api/v1/admin/versions?platform=windows" \
  -H "Authorization: Bearer $TOKEN"
```

#### POST /api/v1/admin/versions

发布新版本。自动将同平台其他版本的 `is_latest` 设为 0。

**请求体:**

```json
{
  "platform": "windows",
  "version_code": 2026070401,
  "version_name": "v1.0.1",
  "title": "v1.0.1 正式版",
  "description": "- 修复天猫采集崩溃\n- 优化利润核算性能",
  "download_url": "https://download.example.com/jdShop-v1.0.1.exe",
  "file_size": 52428800,
  "file_hash": "sha256:abc123def456...",
  "is_force": false
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| platform | 是 | windows / mac / linux / android / ios |
| version_code | 是 | 数字版本号（用于比较大小） |
| version_name | 是 | 显示版本号 |
| title | 是 | 更新标题 |
| description | 否 | 更新说明 |
| download_url | 否 | 下载地址 |
| file_size | 否 | 文件大小（字节） |
| file_hash | 否 | SHA256 校验值 |
| is_force | 否 | 是否强制更新，默认 false |

```bash
curl -X POST http://127.0.0.1:8080/api/v1/admin/versions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"windows","version_code":2026070401,"version_name":"v1.0.1","title":"v1.0.1 Release"}'
```

#### PUT /api/v1/admin/versions/:id

编辑版本信息。所有字段可选（不可修改 platform 和 version_code）。

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/admin/versions/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_force":true,"description":"Critical security update"}'
```

#### DELETE /api/v1/admin/versions/:id

删除版本。

```bash
curl -X DELETE http://127.0.0.1:8080/api/v1/admin/versions/1 \
  -H "Authorization: Bearer $TOKEN"
```

---

## 10. 未鉴权访问示例

如果 Token 缺失或过期，会返回 401:

```json
{"code": 10002, "message": "未提供认证凭证", "data": null}
```

如果用户名不是配置的主管理员，或没有内置 admin 角色，访问管理接口会返回 403：

```json
{"code": 10003, "message": "无操作权限", "data": null}
```

---

## 接口速查表

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | `/api/v1/health` | 无 | 健康检查 |
| POST | `/api/v1/auth/register` | 无 | 注册 |
| POST | `/api/v1/auth/login` | 无 | 登录 |
| GET | `/api/v1/auth/captcha` | 无 | 获取一次性图形验证码 |
| POST | `/api/v1/auth/sms/send` | 无 | 图形验证后发送短信验证码 |
| POST | `/api/v1/auth/refresh` | 无 | 刷新 Token |
| POST | `/api/v1/auth/logout` | 无 | 吊销当前 Refresh Token |
| GET | `/api/v1/user/profile` | JWT | 获取个人信息 |
| PUT | `/api/v1/user/profile` | JWT | 修改个人信息 |
| PUT | `/api/v1/user/password` | JWT | 修改密码 |
| POST | `/api/v1/heartbeat` | JWT | 心跳上报 |
| GET | `/api/v1/control/stream` | JWT | 账号控制实时事件流（SSE） |
| GET | `/api/v1/announcements` | 无 | 公开公告列表 |
| GET | `/api/v1/user/announcements` | JWT | 按账号、平台、版本获取公告并记录送达 |
| POST | `/api/v1/user/announcements/:id/read` | JWT | 标记公告修订已读 |
| POST | `/api/v1/user/announcements/:id/acknowledge` | JWT | 确认公告修订 |
| GET | `/api/v1/version/latest` | 无 | 检查最新版本 |
| GET | `/api/v1/admin/users` | admin | 用户列表 |
| PUT | `/api/v1/admin/users/:id/status` | admin | 启用/禁用用户 |
| POST | `/api/v1/admin/users/:id/roles` | admin | 分配角色 |
| PUT | `/api/v1/admin/users/:id/access` | admin | 设置账号使用期和产品板块 |
| GET | `/api/v1/admin/registration-defaults` | admin | 读取新用户注册默认策略 |
| PUT | `/api/v1/admin/registration-defaults` | admin | 设置默认赠送天数和产品板块 |
| GET | `/api/v1/admin/roles` | admin | 角色列表 |
| POST | `/api/v1/admin/roles` | admin | 创建角色 |
| PUT | `/api/v1/admin/roles/:id` | admin | 更新角色 |
| DELETE | `/api/v1/admin/roles/:id` | admin | 删除角色 |
| GET | `/api/v1/admin/permissions` | admin | 权限列表 |
| GET | `/api/v1/admin/announcements` | admin | 管理端公告列表 |
| POST | `/api/v1/admin/announcements` | admin | 创建公告 |
| PUT | `/api/v1/admin/announcements/:id` | admin | 编辑公告 |
| DELETE | `/api/v1/admin/announcements/:id` | admin | 删除公告 |
| POST | `/api/v1/admin/announcements/:id/publish` | admin | 发布公告 |
| POST | `/api/v1/admin/announcements/:id/unpublish` | admin | 下架公告 |
| GET | `/api/v1/admin/versions` | admin | 版本列表 |
| POST | `/api/v1/admin/versions` | admin | 发布版本 |
| PUT | `/api/v1/admin/versions/:id` | admin | 编辑版本 |
| DELETE | `/api/v1/admin/versions/:id` | admin | 删除版本 |
## 鉴权加密公钥

```http
GET /api/v1/auth/encryption-key
```

返回当前RSA公钥、算法标识和 `kid`。登录、注册、短信发送、令牌刷新/注销和修改密码使用 `Content-Type: application/jdshop-encrypted+json` 提交混合加密信封；`path` 必须绑定对应的远程 `/api/v1/...` 路径，时间戳与服务器时间差不得超过2分钟，`request_id` 只能使用一次。

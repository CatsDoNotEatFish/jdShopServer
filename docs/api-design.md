# API 接口设计

生产基础路径：`https://api.jdshop.bbroot.com/api/v1`

API域名只提供接口，不公开根路径测试台和管理页面。主管理员控制台单独部署在 `https://www.jdshop.bbroot.com/admin`，并跨域调用API域名。

## 响应格式

所有接口统一返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

列表接口 data 中增加分页字段：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

## 错误码

| code | HTTP 状态码 | 含义 |
|------|-----------|------|
| 0 | 200 | 成功 |
| 10001 | 400 | 参数校验失败 |
| 10002 | 401 | 未认证（Token 缺失、过期、无效） |
| 10003 | 403 | 无权限 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突 |
| 10006 | 429 | 短信发送频率或次数超限 |
| 10500 | 500 | 服务内部错误 |
| 10503 | 503 | 短信服务不可用 |

---

## 公开接口（无需鉴权）

### GET /api/v1/auth/captcha

返回5分钟有效、校验一次即销毁的图形验证码及 `captcha_id`。

### POST /api/v1/auth/sms/send

提交手机号、用途（`register` / `password_reset`）、`captcha_id` 和图形验证码后发送6位短信验证码。同手机号每分钟1条、北京时间每日6条，同IP每小时20条。

### POST /api/v1/auth/register

注册新用户。

请求：

```json
{
  "phone": "13800138000",
  "password": "Abc12345",
  "sms_code": "654321",
  "email": "zhangsan@example.com",
  "nickname": "张三"
}
```

参数校验：
- phone：中国大陆11位手机号
- password：6-64 字符
- sms_code：6位注册短信验证码
- email：可选，格式校验

成功返回：

```json
{
  "code": 0,
  "message": "注册成功",
  "data": {
    "id": 1,
    "username": "13800138000",
    "phone": "13800138000",
    "nickname": "张三"
  }
}
```

失败情况：手机号已注册（10005）、短信验证码错误或参数校验失败（10001）

### POST /api/v1/auth/login

登录，返回 JWT Token 对。

请求：

```json
{
  "phone": "13800138000",
  "password": "Abc12345"
}
```

成功返回：

```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "dGhpcyBpcyBh...",
    "expires_in": 7200,
    "user": {
      "id": 1,
      "username": "13800138000",
      "phone": "13800138000",
      "nickname": "张三",
      "roles": ["user"]
    }
  }
}
```

- access_token 有效期 2 小时
- refresh_token 有效期 30 天
- expires_in 单位为秒

失败情况：手机号或密码错误（10001）、账号已禁用（10003）、连续失败 5 次临时锁定 15 分钟（10003）

### POST /api/v1/auth/refresh

使用 Refresh Token 换取新的 Access Token。

请求：

```json
{
  "refresh_token": "dGhpcyBpcyBh..."
}
```

成功返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "新的refresh_token",
    "expires_in": 7200
  }
}
```

Refresh Token 轮转：每次使用后吊销旧 Token，签发新 Token，防止重放攻击。

### POST /api/v1/auth/logout

提交当前 `refresh_token` 并吊销该会话。接口设计为幂等操作，客户端即使已丢失或使用过该 Token，也可以安全地重复退出。

### GET /api/v1/health

服务健康检查。

返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "healthy",
    "time": "2026-07-04T12:00:00Z",
    "version": "1.0.0"
  }
}
```

### GET /api/v1/announcements

获取已发布公告列表（客户端拉取）。

参数：

```text
?page=1&page_size=20&level=warning
```

筛选条件：level 可选，不传时返回所有级别。

返回：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1,
        "title": "系统维护通知",
        "content": "将于 2026-07-10 进行系统维护...",
        "level": "warning",
        "published_at": "2026-07-04T10:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

### GET /api/v1/version/latest

获取指定平台的最新版本信息。

参数：

```text
?platform=windows&current_version_code=2026070301
```

- platform：windows, mac, linux, android, ios
- current_version_code：可选，客户端当前版本号，服务端对比后判断是否需要更新

返回：

```json
{
  "code": 0,
  "data": {
    "has_update": true,
    "is_force": false,
    "version": {
      "platform": "windows",
      "version_code": 2026070401,
      "version_name": "v1.2.4",
      "title": "v1.2.4 更新",
      "description": "- 修复天猫采集偶发崩溃\n- 优化利润核算性能",
      "download_url": "https://example.com/download/v1.2.4.exe",
      "file_size": 52428800,
      "file_hash": "sha256:abc123..."
    }
  }
}
```

---

## 需鉴权接口（JWT Bearer Token）

所有接口需在 Header 中携带：

```text
Authorization: Bearer <access_token>
```

### GET /api/v1/user/profile

获取当前用户信息。

返回：

```json
{
  "code": 0,
  "data": {
    "id": 1,
    "username": "zhangsan",
    "email": "zhangsan@example.com",
    "nickname": "张三",
    "avatar_url": null,
    "roles": ["user"],
    "last_login_at": "2026-07-04T10:00:00Z",
    "created_at": "2026-07-01T08:00:00Z"
  }
}
```

### PUT /api/v1/user/profile

修改个人信息（昵称、邮箱、头像）。

请求：

```json
{
  "nickname": "张三三",
  "email": "newemail@example.com"
}
```

成功返回更新后的用户信息。只更新传入的非空字段。

### PUT /api/v1/user/password

修改密码。

请求：

```json
{
  "new_password": "NewPass678",
  "sms_code": "654321"
}
```

成功返回 `{"code": 0, "message": "密码修改成功"}`。

已绑定手机号的账号必须先通过图形验证码发送 `password_reset` 短信，再提交短信验证码。修改后旧 Refresh Token 全部吊销且旧 Access Token 永久失效，需重新登录。尚未绑定手机号的迁移前旧账号暂时使用旧密码校验。

### POST /api/v1/heartbeat

客户端心跳上报。

请求：

```json
{
  "device_id": "device-uuid-xxx",
  "platform": "windows",
  "app_version": "v1.2.3"
}
```

返回：

```json
{
  "code": 0,
  "data": {
    "has_new_version": true,
    "latest_version_name": "v1.2.4",
    "is_force_update": false
  }
}
```

心跳间隔建议 1 分钟。服务端记录到 `heartbeat_logs`，同时在响应中告知是否有新版本。

响应同时包含 `access` 授权租约，包括账号是否可用、到期时间、剩余秒数以及竞品监控、商家后台、分析中心三个模块开关。

### GET /api/v1/control/stream

JWT 鉴权的 SSE 长连接，按用户 ID 订阅控制事件。管理员更新账号状态或账号授权后发送 `access_changed` / `account_status_changed`；客户端收到事件后立即强制执行一次心跳，不直接信任事件内容。服务端每 15 秒发送注释保活，客户端断线指数退避重连，一分钟周期心跳继续作为兜底。

---

## 管理员接口（仅限唯一主管理员）

路由前缀：`/api/v1/admin`

接口同时要求登录用户名等于 `SUPER_ADMIN_USERNAME`（默认 `admin`）且持有内置 `admin` 角色。普通注册账号即使被数据库误授 `admin` 角色也不能访问。主管理员账号及其角色受保护，不能通过管理接口禁用、修改授权、改角色或删除角色。

### 用户管理

#### GET /api/v1/admin/users

用户列表（分页、搜索）。

参数：

```text
?page=1&page_size=20&keyword=zhang&status=1
```

#### PUT /api/v1/admin/users/:id/status

启用或禁用用户。

请求：

```json
{
  "status": 0
}
```

#### POST /api/v1/admin/users/:id/roles

给用户分配角色。

请求：

```json
{
  "role_ids": [1, 2]
}
```

替换模式（全量替换，非追加），传空数组清空所有角色。

#### PUT /api/v1/admin/users/:id/access

设置指定账号到期时间和三个产品板块开关。

### 新用户注册默认策略

#### GET /api/v1/admin/registration-defaults

读取新账号注册时采用的默认赠送天数和三个产品板块开关。

#### PUT /api/v1/admin/registration-defaults

更新注册默认策略。`usage_days` 必须是 1-3650 的整数；更新只影响之后注册的账号，不修改已有账号。

```json
{
  "usage_days": 30,
  "competitor_monitor": true,
  "merchant_backend": false,
  "analysis_center": false
}
```

### 角色管理

#### GET /api/v1/admin/roles

角色列表。

返回每个角色的 id、name、description、permissions。

#### POST /api/v1/admin/roles

创建角色。

请求：

```json
{
  "name": "editor",
  "description": "内容编辑",
  "permission_ids": [5, 6, 7, 8]
}
```

#### PUT /api/v1/admin/roles/:id

更新角色。

请求：

```json
{
  "name": "editor_v2",
  "permission_ids": [5, 6, 7]
}
```

同样为替换模式。

#### DELETE /api/v1/admin/roles/:id

删除角色（自动解除所有用户的该角色关联）。

#### GET /api/v1/admin/permissions

获取所有权限列表（供角色管理页面的权限选择器使用）。

### 公告管理

#### POST /api/v1/admin/announcements

创建公告（默认草稿）。

请求：

```json
{
  "title": "系统维护通知",
  "content": "将于 2026-07-10 进行系统维护...",
  "level": "warning"
}
```

#### PUT /api/v1/admin/announcements/:id

编辑公告。

请求参数同创建，所有字段可选。

#### DELETE /api/v1/admin/announcements/:id

删除公告。

#### POST /api/v1/admin/announcements/:id/publish

发布公告。

设置 `is_published=1`，`published_at` 设为当前时间。

#### POST /api/v1/admin/announcements/:id/unpublish

下架公告。

设置 `is_published=0`。

#### GET /api/v1/admin/announcements

管理端公告列表（含草稿）。

参数与公开接口相同，但返回 `is_published`、`created_by` 等管理字段。

### 版本管理

#### POST /api/v1/admin/versions

发布新版本。

请求：

```json
{
  "platform": "windows",
  "version_code": 2026070401,
  "version_name": "v1.2.4",
  "title": "v1.2.4 更新",
  "description": "- 修复天猫采集偶发崩溃",
  "download_url": "https://example.com/download/v1.2.4.exe",
  "file_size": 52428800,
  "file_hash": "sha256:abc123...",
  "is_force": false
}
```

自动将同平台其他版本的 `is_latest` 设为 0。

#### PUT /api/v1/admin/versions/:id

编辑版本信息。

请求参数同创建，所有字段可选。不可修改 `platform` 和 `version_code`。

#### DELETE /api/v1/admin/versions/:id

删除版本。

#### GET /api/v1/admin/versions

版本列表（分页）。

参数：

```text
?page=1&page_size=20&platform=windows
```

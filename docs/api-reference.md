# API 接口参考

基础路径: `/api/v1`

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
| 10003 | 403 | 无权限 / 账号禁用 / 频率限制 |
| 10004 | 404 | 资源不存在 |
| 10005 | 409 | 资源冲突（如用户名已存在） |
| 10500 | 500 | 服务内部错误 |

### 鉴权方式

需鉴权的接口在 Header 中携带:

```text
Authorization: Bearer <access_token>
```

Access Token 有效期 2 小时，过期后使用 Refresh Token 换取新 Token。

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

## 2. 注册

### POST /api/v1/auth/register

无需鉴权。

**请求体:**

```json
{
  "username": "testuser",
  "password": "test123",
  "email": "test@example.com",
  "nickname": "Test User"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| username | 是 | 3-32 字符，字母数字下划线 |
| password | 是 | 6-64 字符 |
| email | 否 | 邮箱 |
| nickname | 否 | 显示昵称 |

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"test123","nickname":"Test User"}'
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "username": "testuser",
    "nickname": "Test User"
  }
}
```

**失败返回:**

```json
// 用户名已被占用 (409)
{"code": 10005, "message": "用户名已被占用", "data": null}

// 参数校验失败 (400)
{"code": 10001, "message": "用户名长度须为3-32字符", "data": null}
{"code": 10001, "message": "密码长度须为6-64字符", "data": null}
```

---

## 3. 登录

### POST /api/v1/auth/login

无需鉴权。成功返回 JWT Token 对。

**请求体:**

```json
{
  "username": "testuser",
  "password": "test123"
}
```

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"test123"}'
```

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
      "username": "testuser",
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
// 用户名或密码错误 (400)
{"code": 10001, "message": "用户名或密码错误", "data": null}

// 账号被禁用 (403)
{"code": 10003, "message": "账号已被禁用", "data": null}

// 登录频率限制 (403)
{"code": 10003, "message": "登录尝试过于频繁，请15分钟后再试", "data": null}
```

**登录保护规则:**
- 同 IP 或同用户名 5 分钟内连续失败 5 次 → 锁定 15 分钟
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

需要鉴权。修改密码（修改后所有 Refresh Token 吊销，需重新登录）。

**请求体:**

```json
{
  "old_password": "test123",
  "new_password": "newPassword456"
}
```

**curl 示例:**

```bash
curl -X PUT http://127.0.0.1:8080/api/v1/user/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"test123","new_password":"newPassword456"}'
```

**成功返回 (200):**

```json
{"code": 0, "message": "密码修改成功", "data": null}
```

**失败返回 (400):**

```json
{"code": 10001, "message": "旧密码错误", "data": null}
```

---

## 6. 心跳上报

### POST /api/v1/heartbeat

需要鉴权。建议客户端每 5 分钟上报一次。

**请求体:**

```json
{
  "device_id": "device-uuid-xxx",
  "platform": "windows",
  "app_version": "v1.0.0"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| device_id | 是 | 设备唯一标识 |
| platform | 否 | 平台: windows/mac/linux |
| app_version | 否 | 客户端版本号 |

**curl 示例:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/heartbeat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"dev-001","platform":"windows","app_version":"v1.0.0"}'
```

**成功返回 (200):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "has_new_version": true,
    "latest_version_name": "v1.0.1",
    "is_force_update": false
  }
}
```

如果 `has_new_version` 为 true，客户端应提示用户更新。`is_force_update` 为 true 表示强制更新。

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

以下接口需要 admin 角色。先登录获取 admin Token:

```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | grep -o '"access_token":"[^"]*"' | sed 's/"access_token":"//;s/"//')
```

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

注意: `RoleNames` 字段是逗号分隔的角色名（来自 SQL GROUP_CONCAT）。

#### PUT /api/v1/admin/users/:id/status

启用/禁用用户。

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

分配用户角色（替换模式，全量替换非追加）。

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

### 9.2 角色管理

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

删除角色（不能删除 admin 角色）。

#### GET /api/v1/admin/permissions

获取所有权限列表。

```bash
curl http://127.0.0.1:8080/api/v1/admin/permissions \
  -H "Authorization: Bearer $TOKEN"
```

### 9.3 公告管理

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
  "level": "warning"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| title | 是 | 公告标题 |
| content | 是 | 公告内容 |
| level | 否 | info / warning / critical，默认 info |

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

编辑公告。所有字段可选，只更新传入的非空字段。

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

发布公告。设置 `is_published=1` 并记录 `published_at` 时间。

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

### 9.4 版本管理

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

如果非 admin 用户访问管理接口，会返回 403:

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
| POST | `/api/v1/auth/refresh` | 无 | 刷新 Token |
| GET | `/api/v1/user/profile` | JWT | 获取个人信息 |
| PUT | `/api/v1/user/profile` | JWT | 修改个人信息 |
| PUT | `/api/v1/user/password` | JWT | 修改密码 |
| POST | `/api/v1/heartbeat` | JWT | 心跳上报 |
| GET | `/api/v1/announcements` | 无 | 公开公告列表 |
| GET | `/api/v1/version/latest` | 无 | 检查最新版本 |
| GET | `/api/v1/admin/users` | admin | 用户列表 |
| PUT | `/api/v1/admin/users/:id/status` | admin | 启用/禁用用户 |
| POST | `/api/v1/admin/users/:id/roles` | admin | 分配角色 |
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

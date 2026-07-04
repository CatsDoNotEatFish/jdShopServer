# 数据库表设计

数据库：SQLite，文件路径 `data/app.db`，WAL 模式。

## 核心表

### users

用户表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| username | TEXT NOT NULL UNIQUE | 用户名 |
| email | TEXT | 邮箱 |
| password_hash | TEXT NOT NULL | bcrypt 加密后的密码 |
| nickname | TEXT | 显示昵称 |
| avatar_url | TEXT | 头像 URL |
| status | INTEGER NOT NULL DEFAULT 1 | 1=正常, 0=禁用 |
| last_login_at | TEXT | 最近登录时间 (ISO 8601) |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |
| updated_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_users_username(username)`、`idx_users_status(status)`

### refresh_tokens

刷新令牌表，用于 JWT Refresh Token 管理。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER NOT NULL REFERENCES users(id) | |
| token_hash | TEXT NOT NULL UNIQUE | SHA256(RefreshToken) |
| expires_at | TEXT NOT NULL | 过期时间 |
| revoked | INTEGER NOT NULL DEFAULT 0 | 1=已吊销 |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_refresh_tokens_user(user_id)`、`idx_refresh_tokens_hash(token_hash)`

### roles

角色表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT NOT NULL UNIQUE | admin, editor, user |
| description | TEXT | 角色说明 |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

### permissions

权限表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| code | TEXT NOT NULL UNIQUE | announcement:create, user:delete 等 |
| name | TEXT NOT NULL | 权限名称 |
| description | TEXT | 权限说明 |

预置权限码：

```text
user:list          # 查看用户列表
user:update        # 修改用户状态
user:role:assign   # 分配角色
role:list          # 查看角色列表
role:create        # 创建角色
role:update        # 修改角色
role:delete        # 删除角色
announcement:list  # 查看公告列表
announcement:create # 创建公告
announcement:update # 修改公告
announcement:delete # 删除公告
announcement:publish # 发布/下架公告
version:list       # 查看版本列表
version:create     # 发布新版本
version:update     # 修改版本信息
version:delete     # 删除版本
```

### role_permissions

角色-权限关联表。

| 字段 | 类型 | 说明 |
|------|------|------|
| role_id | INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE | |
| permission_id | INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE | |

主键：`PRIMARY KEY (role_id, permission_id)`

### user_roles

用户-角色关联表。

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE | |
| role_id | INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE | |

主键：`PRIMARY KEY (user_id, role_id)`

### announcements

公告表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| title | TEXT NOT NULL | 公告标题 |
| content | TEXT NOT NULL | 公告内容 |
| level | TEXT NOT NULL DEFAULT 'info' | info, warning, critical |
| is_published | INTEGER NOT NULL DEFAULT 0 | 0=草稿, 1=已发布 |
| published_at | TEXT | 发布时间 |
| created_by | INTEGER NOT NULL REFERENCES users(id) | 创建人 |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |
| updated_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_announcements_published(is_published, published_at DESC)`

### app_versions

版本管理表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| platform | TEXT NOT NULL | windows, mac, linux, android, ios |
| version_code | INTEGER NOT NULL | 数字版本号，如 2026070401 |
| version_name | TEXT NOT NULL | 显示版本号，如 v1.2.3 |
| title | TEXT NOT NULL | 更新标题 |
| description | TEXT | 更新说明 |
| download_url | TEXT | 下载地址 |
| file_size | INTEGER | 文件大小（字节） |
| file_hash | TEXT | SHA256 校验值 |
| is_force | INTEGER NOT NULL DEFAULT 0 | 0=可选, 1=强制更新 |
| is_latest | INTEGER NOT NULL DEFAULT 1 | 同一平台只有一个最新 |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

唯一约束：`UNIQUE(platform, version_code)`
索引：`idx_app_versions_latest(platform, is_latest)`

### heartbeat_logs

客户端心跳记录表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER NOT NULL REFERENCES users(id) | |
| device_id | TEXT NOT NULL | 设备唯一标识 |
| platform | TEXT | 客户端平台 |
| app_version | TEXT | 客户端版本号 |
| ip_address | TEXT | 上报 IP |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_heartbeat_user(user_id, created_at DESC)`、`idx_heartbeat_time(created_at)`

### login_logs

登录审计日志表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER REFERENCES users(id) | 登录用户（失败时可能为空） |
| username | TEXT NOT NULL | 尝试登录的用户名 |
| ip_address | TEXT | 登录 IP |
| user_agent | TEXT | User-Agent |
| result | TEXT NOT NULL | success, failed, locked |
| fail_reason | TEXT | 失败原因 |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_login_logs_user(user_id, created_at DESC)`、`idx_login_logs_ip(ip_address, created_at)`

## 初始化数据

迁移脚本中预置：

- 角色：admin（管理员）、user（普通用户）
- 权限：全部预置权限码
- admin 角色拥有所有权限
- user 角色拥有基础权限（公告列表、版本检查）

## SQLite 优化配置

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-8000;       -- 8MB 页面缓存
PRAGMA busy_timeout=5000;      -- 5 秒忙等待
PRAGMA foreign_keys=ON;
PRAGMA temp_store=MEMORY;
```

在应用启动时执行，每次连接复用同一配置。

## 备份策略

- SQLite 备份使用 `VACUUM INTO` 或 `.backup` 命令
- 建议每天凌晨自动备份到 `backups/app-YYYY-MM-DD.db`
- 保留最近 7 天备份
- 备份文件可通过 `sqlite3` 命令行直接读取

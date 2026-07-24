# 数据库表设计

数据库：SQLite，文件路径 `data/app.db`，WAL 模式。

## 核心表

### users

用户表。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| username | TEXT NOT NULL UNIQUE | 用户名 |
| phone | TEXT UNIQUE | 绑定手机号；新注册普通账号必填，迁移前旧账号和主管理员可为空 |
| email | TEXT | 邮箱 |
| password_hash | TEXT NOT NULL | bcrypt 加密后的密码 |
| nickname | TEXT | 显示昵称 |
| avatar_url | TEXT | 头像 URL |
| status | INTEGER NOT NULL DEFAULT 1 | 1=正常, 0=禁用 |
| last_login_at | TEXT | 最近登录时间 (ISO 8601) |
| created_at | TEXT NOT NULL DEFAULT (datetime('now')) | |
| updated_at | TEXT NOT NULL DEFAULT (datetime('now')) | |

索引：`idx_users_username(username)`、`idx_users_phone(phone) WHERE phone IS NOT NULL`、`idx_users_status(status)`

新注册账号把手机号同时写入 `phone` 和内部兼容字段 `username`；登录优先按 `phone` 查找。`username` 暂时保留，用于主管理员 `admin` 和迁移前旧账号兼容，不再作为新用户注册字段。

### user_access_control

账号使用期限与三个产品板块的开关。权限按账号保存，不跟随 `user` 角色自动扩大。

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | INTEGER PK/FK | 对应用户 |
| competitor_monitor | INTEGER NOT NULL DEFAULT 1 | 竞品监控是否可用 |
| merchant_backend | INTEGER NOT NULL DEFAULT 0 | 商家后台是否可用 |
| analysis_center | INTEGER NOT NULL DEFAULT 0 | 分析中心是否可用 |
| expires_at | TEXT | 到期时间，NULL 表示长期有效 |
| updated_at | TEXT NOT NULL | 最近修改时间 |

新账号注册时从 `registration_defaults` 复制赠送天数和三个板块开关。之后修改注册默认策略不会追溯改变已写入本表的账号授权。

### registration_defaults

新用户注册默认策略。该表是固定 `id=1` 的单例配置，只能由唯一主管理员通过管理接口修改。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK, CHECK(id=1) | 单例主键，固定为 1 |
| usage_days | INTEGER NOT NULL | 默认赠送天数，范围 1-3650 |
| competitor_monitor | INTEGER NOT NULL DEFAULT 1 | 新账号是否默认开放竞品监控 |
| merchant_backend | INTEGER NOT NULL DEFAULT 0 | 新账号是否默认开放商家后台 |
| analysis_center | INTEGER NOT NULL DEFAULT 0 | 新账号是否默认开放分析中心 |
| updated_at | TEXT NOT NULL | 最近保存时间 |

迁移时初始化为 30 天、仅开放竞品监控。注册成功时根据 `usage_days` 从当前 UTC 时间计算 `expires_at`，并把三个开关复制到 `user_access_control`；因此它是注册模板，不是所有账号共享的实时权限。

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

### user_auth_versions

账号 Access Token 版本表，用于立即并永久撤销禁用前签发的 JWT。

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | INTEGER PK, FK → users.id | 账号 ID，删除账号时级联删除 |
| version | INTEGER NOT NULL DEFAULT 1 | 当前授权版本；禁用账号或修改密码时递增 |
| updated_at | TEXT NOT NULL | 最近更新时间 |

旧 Token 未携带版本时按版本 1 兼容；账号首次禁用后版本至少为 2，因此旧 Token 在重新启用后仍无法使用。

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

### sms_verifications

短信验证码发送和校验记录。验证码正文不落库，只保存使用 JWT 密钥作为 pepper 的 HMAC-SHA256；平台 API Key 也不进入数据库。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| phone | TEXT NOT NULL | 接收手机号 |
| purpose | TEXT NOT NULL | `register` 或 `password_reset` |
| code_hash | TEXT NOT NULL | 验证码 HMAC-SHA256，不保存明文 |
| sent_at | TEXT NOT NULL | 短信平台成功接受请求的时间 |
| expires_at | TEXT NOT NULL | 5分钟到期时间 |
| attempts | INTEGER NOT NULL DEFAULT 0 | 错误验证次数，最多5次 |
| consumed | INTEGER NOT NULL DEFAULT 0 | 是否已使用/作废 |
| request_ip | TEXT | 发送请求来源IP，用于小时限流 |
| created_at | TEXT NOT NULL | 记录创建时间 |

索引：`idx_sms_verifications_phone_purpose(phone, purpose, id DESC)`、`idx_sms_verifications_sent_at(sent_at)`。

### schema_migrations

迁移版本记录表，防止包含 `ALTER TABLE` 的迁移在服务重启时重复执行。

升级早期版本数据库时，迁移器会根据现存表、字段和索引补登记001-005的执行状态。若旧程序已经添加 `users.phone`、但尚未完成或记录005迁移，新迁移器会跳过重复的 `ALTER TABLE`，继续补齐短信表和索引，不需要删除或重建原数据库。

| 字段 | 类型 | 说明 |
|------|------|------|
| version | TEXT PK | 已完成的SQL迁移文件名 |
| applied_at | TEXT NOT NULL | 应用时间 |

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
- 建议每天凌晨自动备份到 `backups/app-YYYY-MM-DD-HHMMSS.db`；时间精确到秒，避免同一天的定时备份和手工备份重名
- 保留最近 7 天备份
- 备份文件可通过 `sqlite3` 命令行直接读取

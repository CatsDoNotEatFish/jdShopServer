# 架构设计

## 部署架构

```text
jdShop桌面端 ──> api.jdshop.bbroot.com/api/* ──> Go API
主管理员浏览器 ──> www.jdshop.bbroot.com/admin ──> Nginx静态管理台
                                                         │
                                                         └─跨域调用API域名
Go API ──> SQLite文件（WAL模式，data/app.db）
```

## 进程职责

### Nginx

- HTTPS 终止（Let's Encrypt/Certbot 或 ZeroSSL/acme.sh 自动续期；当前生产使用 ZeroSSL）
- `limit_req` 对 API 限流（30 req/min），登录接口额外限制（5 req/min）
- gzip 压缩 JSON 响应，降低带宽
- API域名只把 `/api/*` 反向代理到Go，根路径和 `/admin` 固定404
- 管理台域名只静态提供 `/admin`，不代理任何 `/api/*`
- 管理台页面通过CORS访问API域名
- `/api/v1/control/stream` 使用独立长连接配置，关闭缓冲并设置 1 小时读写超时

### Go 应用

- 所有 API 业务逻辑
- 不提供根路径API测试台、`/admin` 或其他静态文件路由
- JWT (HS256) 鉴权与 RBAC 权限校验
- SQLite 数据库读写（WAL 模式、纯 Go 驱动、无 CGO）
- 启动时自动执行数据库迁移

## 分层架构

```text
┌─────────────────────────────────────────┐
│  Handler (internal/handler/)            │  ← HTTP 请求解析、参数校验、响应序列化
│  - auth.go      register/login/refresh  │
│  - user.go      profile/password        │
│  - announcement.go  CRUD + publish      │
│  - version.go   CRUD + check latest     │
│  - heartbeat.go heartbeat report        │
│  - control.go   per-user SSE stream      │
│  - admin.go     users/roles management  │
│  - health.go    health check            │
├─────────────────────────────────────────┤
│  Middleware (internal/middleware/)       │  ← 请求预处理
│  - auth.go      JWT 解析, context 注入  │
│  - rbac.go      RequireRole 中间件      │
│  - logger.go    method/path/status/dur  │
│  - ratelimit.go 令牌桶 IP 限流          │
├─────────────────────────────────────────┤
│  Service (internal/service/)            │  ← 业务逻辑
│  - auth.go      注册/登录/Token刷新     │
│  - user.go      个人资料/密码修改       │
│  - announcement.go  公告业务            │
│  - version.go   版本管理/更新检查       │
│  - heartbeat.go 心跳+版本提醒           │
│  - control.go   按用户分发控制通知       │
│  - admin.go     用户/角色管理           │
│  - captcha.go   PNG验证码栅格化和扭曲    │
├─────────────────────────────────────────┤
│  Repository (internal/repository/)      │  ← SQL 数据访问
│  - db.go        数据库打开+迁移执行     │
│  - user.go      用户 CRUD + 角色查询    │
│  - role.go      角色 CRUD + 权限        │
│  - token.go     Refresh Token 管理      │
│  - announcement.go  公告 CRUD           │
│  - version.go   版本 CRUD + 最新查询    │
│  - heartbeat.go  心跳写入               │
│  - login_log.go  登录日志+失败计数      │
├─────────────────────────────────────────┤
│  SQLite (WAL, modernc.org/sqlite)       │
└─────────────────────────────────────────┘
```

调用方向: Handler → Service → Repository → SQLite。不允许跨层调用。

账号授权由 `user_access_control` 保存，作为登录、心跳和客户端三大板块展示的共同来源；`registration_defaults` 是新注册账号使用的授权模板，注册时复制一次，之后互不联动；RBAC 仍然只负责云端权限。服务端管理接口额外使用唯一主管理员边界：JWT 用户名必须匹配 `SUPER_ADMIN_USERNAME` 且持有内置 `admin` 角色，单独拥有角色不能进入后台。

实时控制采用“推送唤醒 + 心跳确认”：内存中的 `ControlHub` 只向目标用户的在线 SSE 连接发送变更信号，不承载最终权限数据；客户端收到信号后请求心跳重新计算数据库中的账号状态。进程重启或网络断线不会丢失授权事实，因为数据库和一分钟心跳仍是权威来源。

## 项目文件结构

```text
jdShopServer/
├── main.go                     # 入口: migrate / serve / version 三个子命令
├── config.yaml                 # 默认配置文件
├── config/
│   └── config.go               # 配置结构体 + YAML 加载 + 环境变量覆盖
├── internal/
│   ├── handler/                # HTTP 处理层
│   │   ├── response.go         # 统一响应函数（respondOK/Error/Paginated）
│   │   ├── auth.go
│   │   ├── user.go
│   │   ├── announcement.go
│   │   ├── version.go
│   │   ├── heartbeat.go
│   │   ├── control.go
│   │   ├── admin.go
│   │   └── health.go
│   ├── service/                # 业务逻辑层
│   │   ├── auth.go
│   │   ├── user.go
│   │   ├── announcement.go
│   │   ├── version.go
│   │   ├── heartbeat.go
│   │   ├── control.go
│   │   ├── access.go
│   │   ├── sms.go
│   │   ├── captcha.go
│   │   └── admin.go
│   ├── repository/             # 数据访问层
│   │   ├── db.go
│   │   ├── user.go
│   │   ├── role.go
│   │   ├── token.go
│   │   ├── announcement.go
│   │   ├── version.go
│   │   ├── heartbeat.go
│   │   ├── access.go
│   │   ├── registration_defaults.go
│   │   ├── sms_verification.go
│   │   └── login_log.go
│   ├── middleware/             # 中间件（4 个文件）
│   │   ├── auth.go             # JWT 解析
│   │   ├── rbac.go             # 角色检查
│   │   ├── logger.go           # 请求日志
│   │   └── ratelimit.go        # IP 限流
│   ├── model/
│   │   └── models.go           # 所有数据结构、请求/响应类型、校验方法
│   └── router/
│       └── router.go           # 路由注册 + 依赖注入
├── migrations/
│   ├── 001_init.sql            # 初始数据库迁移
│   ├── 002_user_access_control.sql # 使用期和三个客户端板块
│   ├── 003_user_auth_versions.sql  # Access Token永久失效版本
│   ├── 004_registration_defaults.sql # 新用户注册默认策略
│   └── 005_phone_sms_auth.sql      # 手机号与短信验证
├── deploy/
│   ├── nginx.conf              # 证书签发前HTTP引导配置
│   ├── nginx-https.conf        # ZeroSSL/acme.sh生产HTTPS配置
│   ├── jdshop.service
│   ├── deploy.sh
│   └── backup.sh
├── static/
│   └── admin.html              # 仅由www域名的Nginx静态提供
├── docs/                       # 项目文档
│   ├── README.md               # 导航
│   ├── architecture.md         # 本文件
│   ├── database-schema.md      # 表设计
│   ├── api-reference.md        # API 接口参考
│   ├── auth-design.md          # 鉴权设计
│   ├── deployment.md           # 部署指南
│   ├── development.md          # 开发指南
│   └── operations/             # 操作日志
├── data/                       # 运行时数据（.gitignore）
│   └── app.db
└── .gitignore
```

## 关键设计决策

### 为什么不用 ORM

手写 SQL 对于 SQLite 这种简单场景更透明。SQL 语句在 Repository 层集中管理，性能可预期，排查问题直接看 SQL。

### 为什么所有列表接口返回空数组而非 null

`items: []` 而非 `items: null`。Repository 层在查询无结果时返回空切片，前端无需做 null check。

### 为什么 Refresh Token 使用轮转策略

每次使用后吊销旧 Token、签发新 Token。如果 Refresh Token 被窃取，攻击者和合法用户竞争使用，竞争者的失败会暴露窃取行为，触发全局吊销。

### 为什么登录频率限制同时检查 IP 和用户名

防御两种攻击: 同 IP 暴力破解同一用户，同 IP 遍历用户字典。任一维度触发阈值即锁定。

## 1C1G 资源规划

| 进程 | 实际内存 | 说明 |
|------|---------|------|
| Go 应用 | 30-80MB | 纯 Go SQLite 驱动，无 CGO 开销 |
| Nginx | 10-20MB | 1 worker_processes |
| SQLite 缓存 | 8MB | PRAGMA cache_size=-8000 |
| 系统 | ~300MB | Debian minimal |
| 合计 | ~350-400MB | 留有 600MB+ 余量 |

### 内存优化措施

- `GOMEMLIMIT=200MiB` 环境变量硬限制
- SQLite: `synchronous=NORMAL`, `temp_store=MEMORY`
- Nginx: `worker_processes 1`, `gzip_min_length 256`
- Go 日志输出到 stdout（systemd journal 收集），不写文件

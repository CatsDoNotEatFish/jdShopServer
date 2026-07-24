# 操作记录

生产服务器安装、更新和排障执行 [生产服务器部署与运维手册](production-deployment-runbook.md)。发布 Windows 客户端版本时，执行 [Windows 客户端版本发布](client-update-release.md) 中的构建、上传、后台登记和验证流程。

## 2026-07-04 项目初始化与完整实现

背景:
- jdShop 需要云端后端提供用户系统、鉴权、公告、版本管理等服务
- 服务器资源受限（1C1G），需要极轻量方案

本次完成:

### 技术选型
- Go + Chi Router v5 + SQLite (WAL, 纯 Go 驱动 modernc.org/sqlite, 无 CGO)
- JWT HS256 双 Token 设计 + Refresh Token 轮转 + bcrypt 密码哈希
- RBAC 权限模型 (用户-角色-权限)
- Nginx 反向代理 + systemd 进程守护

### 初始化时代码实现 (17 个 Go 源文件, ~2000 行)
- `config/`: YAML 配置加载 + 环境变量覆盖
- `internal/model/`: 所有数据结构、请求/响应类型、校验方法
- `internal/repository/`: 8 个 Repository, 每个封装一类数据访问
- `internal/service/`: 6 个 Service, 封装核心业务逻辑
- `internal/middleware/`: JWT 鉴权、RBAC 角色验证、请求日志、IP 限流
- `internal/handler/`: 8 个 Handler, HTTP 请求处理
- `internal/router/`: 路由注册 + 依赖注入
- `main.go`: 入口 (serve / migrate / version 三个子命令)

### 数据库
- 15 张核心/迁移表: users, refresh_tokens, user_auth_versions, user_access_control, registration_defaults,
  sms_verifications, schema_migrations, roles, permissions, role_permissions, user_roles, announcements, app_versions, heartbeat_logs, login_logs
- 预置 admin 管理员 (admin/admin123) 和 user 普通用户角色
- 16 个预置权限码
- SQLite WAL 模式 + 必要索引 + 外键约束

### 初始化时 API 接口 (28 个端点，后续授权/SSE接口见下方变更记录)
- 公开 6 个: health, register, login, refresh, announcements, version/latest
- 鉴权 4 个: profile GET/PUT, password, heartbeat
- 管理 18 个: users CRUD, roles CRUD, permissions list, announcements CRUD+publish/unpublish, versions CRUD

### 安全特性
- 密码 bcrypt (cost=10) 哈希存储
- JWT 2 小时有效期 + Refresh Token 30 天轮转
- 登录失败限制 (5 次/15 分钟, IP + 用户名双维度)
- 密码修改或账号禁用后全部 Refresh Token 吊销，并递增 `auth_version` 使全部旧 Access Token 永久失效
- 登录审计日志 (login_logs)
- API 全局限流 (60 req/min, 登录接口 5 req/min)
- Nginx 安全头 + systemd 沙箱

### 部署就绪
- Nginx 配置文件 (限流 + gzip + HTTPS 就绪)
- systemd unit (GOMEMLIMIT=200MiB + 安全加固)
- 一键部署脚本 deploy.sh
- 数据库自动备份脚本 backup.sh
- 二进制大小: 17MB (开发编译), ~12MB (-ldflags="-s -w")

### API 验证结果
全部 28 个端点通过 curl 测试:
- 注册 (含重复检测) ✓
- 登录 (admin + 普通用户 + 错误密码) ✓
- Token 刷新 ✓
- 个人信息获取/修改/改密 ✓
- 心跳上报 + 版本提醒 ✓
- 公告 CRUD + 发布/下架 ✓
- 版本 CRUD + 最新版本检查 ✓
- 用户管理 (列表/禁启用/角色分配) ✓
- 角色管理 (CRUD + 权限设置) ✓
- 权限列表 ✓
- 未鉴权拦截 (401) ✓
- 非 admin 拦截 (403) ✓

### 初始化时文档 (8 个文档文件)
- README.md: 项目导航 + 快速开始
- architecture.md: 部署架构、分层设计、文件结构、设计决策
- database-schema.md: 10 张表的完整定义、索引、初始数据
- api-reference.md: 28 个接口的详细说明 + curl 示例 + 真实响应
- auth-design.md: JWT + Refresh Token 轮转 + RBAC + 登录保护
- deployment.md: Nginx + systemd + HTTPS + 备份 + 一键部署
- development.md: 开发环境、分层规范、编译、调试
- operations/README.md: 本文件

当前状态与后续待办:
- [x] 部署到香港生产服务器并配置 HTTPS（ZeroSSL/acme.sh）
- [x] 添加账号授权、Token失效、SSE控制和版本更新集成测试
- [x] 客户端集成登录、一分钟心跳、实时控制和版本检查
- [ ] CI/CD (GitHub Actions 自动编译部署)
- [ ] 更新清单离线签名和 Windows Authenticode 代码签名

## 2026-07-22 账号授权与三板块控制

- 新增 `user_access_control`，保存账号到期时间和竞品监控、商家后台、分析中心三个开关。
- 当时的新注册账号初始策略为仅开放竞品监控、使用期 30 天；该策略现已支持在管理控制台配置，见后文“新用户注册默认策略”。
- 新增管理员账号授权接口 `PUT /api/v1/admin/users/:id/access`。
- 心跳响应携带授权租约；客户端收到禁用/到期状态后锁定本地业务并返回登录页。
- 管理控制台用户管理新增使用期和板块权限编辑。
- 在线监控读取真实客户端心跳，并展示最近设备、平台和客户端版本。
- 客户端心跳周期调整为 1 分钟；修复单连接 SQLite 下角色列表嵌套查询导致管理控制台后续操作无响应的问题。
- 新增按用户隔离的 SSE 控制流：禁用/启用账号、调整使用期或板块权限后实时通知客户端，客户端强制心跳确认；断线时仍由一分钟心跳兜底。
- 香港服务器 Nginx 增加 SSE 专用反代规则（关闭缓冲、1 小时读写超时、15 秒应用层保活）。
- 2026-07-22 本机端到端验证：管理员禁用测试账号后，客户端约 0.016 秒收到控制事件并通过心跳确认禁用；测试账号及关联数据随后已清理。
- 账号禁用链路加强：客户端清除全部 Token，禁用前 Access Token 在重新启用后仍无效；未完成采集任务标记为 `stopped` 并立即取消浏览器操作，登录页显示不可点击的“账户被禁用”。
- 服务端管理台改为唯一主管理员模式：默认仅 `admin` 可登录；普通账号即使误获 `admin` 角色也无法访问管理接口。
- 主管理员账号和内置 `admin` 角色受到服务端保护，不能被禁用或改角色；主管理员固定长期有效并拥有三个产品板块的全部权限，普通账号不能被分配 `admin` 角色。
- 管理台隐藏主管理员的禁用、权限和角色按钮，并通过实际管理接口探测确认登录者身份。

## 2026-07-23 生产部署与客户端 0.2.3

- 香港生产服务器完成 Ubuntu 22.04、systemd、Nginx、SQLite 和 HTTPS 部署；`api.jdshop.bbroot.com` 仅提供API，`www.jdshop.bbroot.com` 仅提供主管理员控制台。
- Let's Encrypt 因共享注册域签发频率限制暂时不可用，改用 ZeroSSL/acme.sh；证书安装到 `/etc/nginx/ssl/jdshop/`，由 cron 自动续期。
- systemd 模板改用 `/etc/jdshop/jdshop.env`，JWT 密钥不再内联到 unit 或部署包。
- 数据库备份文件名改为 `app-YYYY-MM-DD-HHMMSS.db`，允许同一天多次手工和定时备份。
- 新增 [生产服务器部署与运维手册](production-deployment-runbook.md)，记录 ZIP 反斜杠警告、交互 Shell `set -e` 导致 SSH 断开、Nginx 404、HEAD 405 和证书限流排障。
- Windows 客户端发布为 `0.2.3`（版本码 `2026072302`），独立更新器展示下载/校验/解压/备份/安装/健康检查/回滚进度。
- 强制更新窗口在支持新启动器的客户端中不可取消、不可关闭；旧启动器第一次跨版本升级仍受引导版本边界限制。
- 客户端更新包使用阿里云 OSS 永久 HTTPS 对象地址，禁止把带 `Expires` 的一次性签名URL登记为版本下载地址。

## 2026-07-23 API域名与管理台域名隔离

- 删除Go服务根路径的可交互API测试台以及Go侧 `/admin`、`/static/*` 路由，API应用对这些路径统一返回404。
- 删除发布目录中的 `static/index.html`；升级脚本会同时清理服务器遗留的测试页面。
- `api.jdshop.bbroot.com` 只允许 `/api/*`，根路径和 `/admin` 由Nginx固定返回404。
- `www.jdshop.bbroot.com` 只静态提供 `/admin`，根路径跳转到管理台，`/api/*` 和其他路径返回404。
- 管理台使用 `https://api.jdshop.bbroot.com` 作为独立API来源，服务端CORS加入 `https://www.jdshop.bbroot.com`。
- 新增 `CORS_ALLOWED_ORIGINS` 环境变量覆盖，便于不改已有 `config.yaml` 即可更新生产允许来源。
- HTTPS证书需要同时覆盖 `api.jdshop.bbroot.com` 和 `www.jdshop.bbroot.com`。

## 2026-07-23 新用户注册默认策略

- 新增单例配置表 `registration_defaults`，持久化新用户默认赠送天数和竞品监控、商家后台、分析中心三个板块开关。
- 新增主管理员接口 `GET/PUT /api/v1/admin/registration-defaults`；赠送天数允许 1-3650 天。
- 管理控制台新增“注册默认”页面，可直接读取和保存策略，并显示最近保存时间。
- 注册成功时复制当时的策略到该账号的 `user_access_control`；后续修改默认策略只影响未来注册，不追溯修改已有账号。
- 初始策略保持为赠送 30 天、仅开放竞品监控，兼容原有注册行为。

## 2026-07-23 手机号注册、短信验证与传输安全

- 新注册普通账号改为手机号绑定；登录使用手机号加密码。内部 `username` 字段暂留用于主管理员和迁移前旧账号兼容。
- 新增 `GET /api/v1/auth/captcha` 和 `POST /api/v1/auth/sms/send`；注册和手机号账号改密均先图形验证、再短信验证。
- 图形验证码升级为5位字母数字混合PNG，服务端栅格化旋转、波形扭曲、干扰曲线和噪点，响应中不再包含可直接提取答案的SVG文本。
- 接入短信宝 HTTPS 安全接口，平台用户名、API Key、产品ID和正式报备模板只从 `/etc/jdshop/jdshop.env` 读取，仓库不保存真实凭据。
- 同手机号跨用途60秒1条、北京时间每日6条；同IP每小时20条。连续短信保证验证码不同，短信码5分钟有效且最多试错5次。
- 新增 `sms_verifications`，验证码仅以带服务端 pepper 的 HMAC-SHA256 保存；新增 `schema_migrations` 避免字段迁移重复执行。
- Windows客户端拒绝向非localhost的HTTP鉴权地址发送密码或Token；生产Nginx启用HSTS，短信服务拒绝HTTP供应商端点。
- 自动化测试使用本地假短信供应商和正式格式内容，不向真实手机号发送、不产生短信费用。
- 敏感鉴权请求使用 RSA-OAEP-256 + AES-256-GCM 应用层加密，信封绑定接口路径、时间戳和一次性 `request_id`；本地客户端和云端均拒绝登录、注册、短信发送、改密接口的普通明文 JSON。
- `www.jdshop.bbroot.com/admin` 的主管理员登录同样使用加密信封；浏览器Network面板中不应再出现包含 `username/password` 的明文JSON。普通 `Content-Type: application/json` 登录请求返回 `code=10001` 属于预期拒绝。
- 服务端私钥首次启动自动生成到数据目录 `auth_encryption_private.pem`，仅由服务进程读取；所有客户端升级完成后保持 `auth.require_encrypted_requests=true`。

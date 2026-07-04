# 操作记录

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

### 代码实现 (17 个 Go 源文件, ~2000 行)
- `config/`: YAML 配置加载 + 环境变量覆盖
- `internal/model/`: 所有数据结构、请求/响应类型、校验方法
- `internal/repository/`: 8 个 Repository, 每个封装一类数据访问
- `internal/service/`: 6 个 Service, 封装核心业务逻辑
- `internal/middleware/`: JWT 鉴权、RBAC 角色验证、请求日志、IP 限流
- `internal/handler/`: 8 个 Handler, HTTP 请求处理
- `internal/router/`: 路由注册 + 依赖注入
- `main.go`: 入口 (serve / migrate / version 三个子命令)

### 数据库
- 10 张核心表: users, refresh_tokens, roles, permissions, role_permissions,
  user_roles, announcements, app_versions, heartbeat_logs, login_logs
- 预置 admin 管理员 (admin/admin123) 和 user 普通用户角色
- 16 个预置权限码
- SQLite WAL 模式 + 必要索引 + 外键约束

### API 接口 (28 个端点)
- 公开 6 个: health, register, login, refresh, announcements, version/latest
- 鉴权 4 个: profile GET/PUT, password, heartbeat
- 管理 18 个: users CRUD, roles CRUD, permissions list, announcements CRUD+publish/unpublish, versions CRUD

### 安全特性
- 密码 bcrypt (cost=10) 哈希存储
- JWT 2 小时有效期 + Refresh Token 30 天轮转
- 登录失败限制 (5 次/15 分钟, IP + 用户名双维度)
- 密码修改后全部 Refresh Token 吊销
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

### 文档 (8 个文档文件)
- README.md: 项目导航 + 快速开始
- architecture.md: 部署架构、分层设计、文件结构、设计决策
- database-schema.md: 10 张表的完整定义、索引、初始数据
- api-reference.md: 28 个接口的详细说明 + curl 示例 + 真实响应
- auth-design.md: JWT + Refresh Token 轮转 + RBAC + 登录保护
- deployment.md: Nginx + systemd + HTTPS + 备份 + 一键部署
- development.md: 开发环境、分层规范、编译、调试
- operations/README.md: 本文件

后续待办:
- [ ] 部署到生产服务器并申请 HTTPS 证书
- [ ] 添加单元测试
- [ ] CI/CD (GitHub Actions 自动编译部署)
- [ ] 客户端集成 (jdShop 接入登录/心跳/版本检查)

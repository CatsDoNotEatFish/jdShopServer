# jdShopServer 后端文档导航

## 项目定位

jdShopServer 是 jdShop 项目的云端后端服务，部署在 1C1G 服务器上，为客户端提供用户系统、鉴权、公告、版本管理等云服务。

## 快速开始

```bash
# 编译
go build -ldflags="-s -w" -o jdshop-server .

# 初始化数据库
./jdshop-server migrate

# 启动服务
./jdshop-server serve
# 可选: 指定配置文件
CONFIG_PATH=/opt/jdshop/config.yaml JWT_SECRET=your-secret ./jdshop-server serve
```

健康检查: `curl http://127.0.0.1:8080/api/v1/health`
默认主管理员: `admin` / `admin123`（首次部署后立即改密；生产管理台为 `https://www.jdshop.bbroot.com/admin`）

## 技术栈

| 层级 | 选择 | 原因 |
|------|------|------|
| 语言 | Go 1.22+ | 编译型、低内存（30-80MB）、单二进制部署 |
| Web 框架 | Chi Router v5 | 轻量、标准库兼容、中间件生态 |
| 数据库 | SQLite (WAL, 纯 Go 驱动) | 零运维、无 CGO、1C1G 下最优 |
| 鉴权 | JWT (HS256) + bcrypt | 无状态、客户端友好 |
| 权限 | RBAC (用户-角色-权限) | 灵活可扩展 |
| 部署 | Nginx + systemd | 标准化反代 + 进程守护 |

## 文档索引

| 文档 | 内容 |
|------|------|
| [架构设计](architecture.md) | 部署架构、分层设计、模块划分、1C1G 资源规划 |
| [数据库表设计](database-schema.md) | 15 张核心/迁移表完整定义、索引、初始数据 |
| [API 接口参考](api-reference.md) | **所有接口的详细说明、请求示例、返回示例、curl 命令** |
| [鉴权与权限](auth-design.md) | JWT 双 Token 机制、Refresh Token 轮转、RBAC 模型 |
| [部署指南](deployment.md) | Ubuntu、Nginx、systemd、Let's Encrypt/ZeroSSL、备份与回滚 |
| [开发指南](development.md) | 环境要求、项目结构、分层规范、测试策略、编译配置 |
| [操作日志](operations/README.md) | 变更记录、待办事项 |
| [生产服务器部署与运维手册](operations/production-deployment-runbook.md) | 当前香港服务器逐步部署、证书、发布、恢复和故障处理 |
| [Windows 客户端版本发布](operations/client-update-release.md) | 客户端构建、OSS上传、版本登记、强制更新和回退 |

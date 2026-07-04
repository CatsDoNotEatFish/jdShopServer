# 部署指南

目标服务器：1 核 1G 内存，Linux（CentOS 7+ 或 Debian 11+）

## 前置准备

### 服务器初始化

```bash
# 更新系统
apt update && apt upgrade -y   # Debian/Ubuntu
# 或
yum update -y                   # CentOS

# 安装必备工具
apt install -y nginx certbot python3-certbot-nginx curl wget sqlite3
```

### 创建应用用户

```bash
useradd -r -s /bin/false jdshop
mkdir -p /opt/jdshop/{bin,data,backups,logs}
chown -R jdshop:jdshop /opt/jdshop
```

## Go 应用编译与部署

### 编译（在开发机上）

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.Version=1.0.0" \
  -o jdshop-server .

# 产物大小约 10-15MB
```

### 上传到服务器

```bash
scp jdshop-server root@your-server:/opt/jdshop/bin/
scp config.yaml root@your-server:/opt/jdshop/
scp migrations/ root@your-server:/opt/jdshop/migrations/ -r
```

### 配置文件

`/opt/jdshop/config.yaml`：

```yaml
server:
  port: 8080
  host: "127.0.0.1"       # 仅本地监听，Nginx 反向代理
  read_timeout: 10s
  write_timeout: 10s

database:
  path: "/opt/jdshop/data/app.db"
  max_open_conns: 1        # SQLite 单 writer
  wal_mode: true

auth:
  bcrypt_cost: 10
  access_token_ttl: 7200
  refresh_token_ttl: 2592000
  login_max_attempts: 5
  login_lock_minutes: 15

log:
  level: "info"            # debug, info, warn, error
  file: "/opt/jdshop/logs/app.log"

cors:
  allowed_origins:
    - "http://localhost:8787"
    - "http://127.0.0.1:8787"
```

敏感配置通过环境变量覆盖：

```bash
export JWT_SECRET="至少32字节的随机字符串"
export ADMIN_DEFAULT_PASSWORD="初始管理员密码"
```

## 数据库初始化

应用启动时自动执行迁移脚本 `migrations/` 目录，按文件名顺序执行尚未执行的 SQL 文件。

首次部署时：

```bash
cd /opt/jdshop
./bin/jdshop-server migrate   # 仅执行迁移，不启动服务
```

或直接启动服务（自动迁移）：

```bash
./bin/jdshop-server serve
```

## systemd 配置

`/etc/systemd/system/jdshop.service`：

```ini
[Unit]
Description=jdShop Cloud Backend
After=network.target

[Service]
Type=simple
User=jdshop
Group=jdshop
WorkingDirectory=/opt/jdshop
Environment="GOMEMLIMIT=200MiB"
Environment="JWT_SECRET=<填入真实密钥>"
ExecStart=/opt/jdshop/bin/jdshop-server serve
Restart=on-failure
RestartSec=5

# 安全加固
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/jdshop/data /opt/jdshop/logs /opt/jdshop/backups
ReadOnlyPaths=/opt/jdshop/bin /opt/jdshop/migrations

[Install]
WantedBy=multi-user.target
```

启动：

```bash
systemctl daemon-reload
systemctl enable jdshop
systemctl start jdshop
systemctl status jdshop
```

## Nginx 配置

`/etc/nginx/sites-available/jdshop`：

```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=30r/m;
limit_req_zone $binary_remote_addr zone=login_limit:10m rate=5r/m;

server {
    listen 80;
    server_name api.yourdomain.com;

    # 静态资源（如果有）
    root /opt/jdshop/static;

    # API 代理
    location /api/ {
        limit_req zone=api_limit burst=10 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    # 登录接口更严格的限流
    location /api/v1/auth/login {
        limit_req zone=login_limit burst=3 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 健康检查不限流
    location /api/v1/health {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
    }
}
```

启用并申请 HTTPS：

```bash
ln -s /etc/nginx/sites-available/jdshop /etc/nginx/sites-enabled/
rm /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx

# 申请证书
certbot --nginx -d api.yourdomain.com
```

Let's Encrypt 证书自动续期（certbot 自动添加 cron/systemd timer）。

## 备份

### 数据库备份脚本

`/opt/jdshop/deploy/backup.sh`：

```bash
#!/bin/bash
BACKUP_DIR="/opt/jdshop/backups"
DB_PATH="/opt/jdshop/data/app.db"
KEEP_DAYS=7

mkdir -p "$BACKUP_DIR"
sqlite3 "$DB_PATH" "VACUUM INTO '$BACKUP_DIR/app-$(date +%Y-%m-%d).db'"

# 清理旧备份
find "$BACKUP_DIR" -name "app-*.db" -mtime +$KEEP_DAYS -delete
```

cron 定时任务：

```bash
# 每天凌晨 3 点备份
0 3 * * * /bin/bash /opt/jdshop/deploy/backup.sh >> /opt/jdshop/logs/backup.log 2>&1
```

## 一键部署脚本

`deploy/deploy.sh`（在服务器上执行）：

```bash
#!/bin/bash
set -e

APP_DIR="/opt/jdshop"
SERVICE_NAME="jdshop"

echo "=== 停止旧服务 ==="
systemctl stop $SERVICE_NAME || true

echo "=== 复制新二进制 ==="
cp jdshop-server "$APP_DIR/bin/"
chmod +x "$APP_DIR/bin/jdshop-server"
chown jdshop:jdshop "$APP_DIR/bin/jdshop-server"

echo "=== 复制迁移脚本 ==="
cp migrations/*.sql "$APP_DIR/migrations/"
chown -R jdshop:jdshop "$APP_DIR/migrations/"

echo "=== 启动服务 ==="
systemctl start $SERVICE_NAME

echo "=== 检查状态 ==="
sleep 2
systemctl status $SERVICE_NAME --no-pager

echo "=== 健康检查 ==="
sleep 1
curl -s http://127.0.0.1:8080/api/v1/health | python3 -m json.tool
```

## 日志查看

```bash
# 应用日志
tail -f /opt/jdshop/logs/app.log

# systemd 日志
journalctl -u jdshop -f

# Nginx 访问日志
tail -f /var/log/nginx/access.log

# Nginx 错误日志
tail -f /var/log/nginx/error.log
```

## 更新流程

1. 在开发机上编译新版本
2. `scp jdshop-server root@server:/opt/jdshop/deploy/`
3. SSH 到服务器，执行 `/opt/jdshop/deploy/deploy.sh`

或使用 CI/CD 自动化（GitHub Actions 编译 + rsync 部署）。

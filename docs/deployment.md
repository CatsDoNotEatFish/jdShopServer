# 部署指南

本文是 jdShopServer 的标准生产部署说明。当前推荐环境是 Ubuntu 22.04 LTS x86_64、Nginx、systemd、SQLite和HTTPS。当前实际生产域名为 `api.jdshop.bbroot.com`；完整逐步操作和排障记录见 [生产服务器部署与运维手册](operations/production-deployment-runbook.md)。

## 1. 部署架构

```text
Windows 客户端
    │ HTTPS 443
    ▼
Nginx
    ├─ /api/v1/control/stream  → SSE专用反向代理
    ├─ /api/*                  → 127.0.0.1:8080
    └─ /admin                  → Go服务内置管理控制台
                                 │
                                 ├─ /opt/jdshop/data/app.db
                                 ├─ /opt/jdshop/logs/
                                 └─ /opt/jdshop/backups/
```

Go服务只监听 `127.0.0.1:8080`，公网只开放Nginx的80/443。不要在云安全组或主机防火墙中开放8080。

## 2. 支持环境和资源

- 推荐：Ubuntu 22.04 LTS x86_64；
- Go二进制使用 `linux/amd64`、`CGO_ENABLED=0` 构建；
- 最低建议：1核1GB内存、20GB磁盘；
- 当前systemd设置 `GOMEMLIMIT=200MiB`；
- CentOS 7已结束生命周期，不作为新服务器部署目标；
- 服务器需要Nginx、curl、unzip、sqlite3和证书工具。

Ubuntu安装依赖：

```bash
apt update
apt install -y nginx curl unzip sqlite3 ca-certificates socat cron
systemctl enable --now nginx
```

若使用Let's Encrypt/Certbot：

```bash
apt install -y certbot python3-certbot-nginx
```

## 3. DNS、安全组和端口

部署前把API域名的A记录指向服务器公网IP，并确认：

| 端口 | 来源 | 用途 |
|------|------|------|
| 22/TCP | 仅管理员固定IP或可信范围 | SSH |
| 80/TCP | 公网 | HTTP验证和跳转HTTPS |
| 443/TCP | 公网 | 客户端API、SSE和管理台 |
| 8080/TCP | 不开放公网 | Go服务本机监听 |

检查DNS：

```bash
getent hosts api.jdshop.bbroot.com
```

## 4. 目录和运行账号

```bash
useradd --system --home-dir /opt/jdshop --shell /usr/sbin/nologin jdshop

install -d -o root -g root -m 0755 \
  /opt/jdshop \
  /opt/jdshop/bin \
  /opt/jdshop/migrations \
  /opt/jdshop/static \
  /opt/jdshop/deploy

install -d -o jdshop -g jdshop -m 0750 \
  /opt/jdshop/data \
  /opt/jdshop/logs \
  /opt/jdshop/backups
```

推荐目录：

```text
/opt/jdshop/
├─ bin/jdshop-server
├─ config.yaml
├─ migrations/
├─ static/
├─ deploy/
├─ data/app.db
├─ logs/
└─ backups/
```

二进制、配置、迁移和静态文件由root拥有；只有data、logs和backups允许 `jdshop` 写入。

## 5. 构建和上传

开发机交叉编译：

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.Version=1.0.0" \
  -o jdshop-server .
```

上传后必须核对大小和SHA-256：

```bash
ls -lh /root/jdshop-server-*.zip
sha256sum /root/jdshop-server-*.zip
unzip -t /root/jdshop-server-*.zip
```

Windows PowerShell生成的ZIP可能带反斜杠路径分隔符，Linux `unzip`会警告并返回退出码1，但文件仍可能完整解出。自动化脚本不能仅因该警告直接终止，应同时检查目标文件是否存在。后续构建应优先生成标准正斜杠ZIP。

## 6. 应用配置

`/opt/jdshop/config.yaml` 保存非敏感配置：

```yaml
server:
  port: 8080
  host: "127.0.0.1"
  read_timeout: 10s
  write_timeout: 10s

database:
  path: "/opt/jdshop/data/app.db"
  max_open_conns: 1
  wal_mode: true

auth:
  super_admin_username: "admin"
  bcrypt_cost: 10
  access_token_ttl: 7200
  refresh_token_ttl: 2592000
  login_max_attempts: 5
  login_lock_minutes: 15

log:
  level: "info"
  file: "/opt/jdshop/logs/app.log"

cors:
  allowed_origins:
    - "http://localhost:8787"
    - "http://127.0.0.1:8787"
```

敏感值统一放在 `/etc/jdshop/jdshop.env`，不要写入Git、部署ZIP、systemd unit或操作日志：

```bash
install -d -o root -g root -m 0700 /etc/jdshop
openssl rand -base64 48
```

把生成值填入：

```text
JWT_SECRET=替换为至少32字节的随机密钥
SUPER_ADMIN_USERNAME=admin
```

然后设置权限：

```bash
chown root:root /etc/jdshop/jdshop.env
chmod 0600 /etc/jdshop/jdshop.env
```

`SUPER_ADMIN_USERNAME` 是唯一可以进入 `/admin` 和 `/api/v1/admin/*` 的账号。首次部署必须立即修改内置 `admin/admin123` 默认密码。不要把 `admin` 角色当作后台登录授权；服务端会额外核对唯一主管理员用户名，并拒绝普通账号获得内置管理员角色。

## 7. systemd服务

仓库模板 `deploy/jdshop.service` 使用环境变量文件：

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
EnvironmentFile=/etc/jdshop/jdshop.env
ExecStart=/opt/jdshop/bin/jdshop-server serve
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/jdshop/data /opt/jdshop/logs /opt/jdshop/backups
ReadOnlyPaths=/opt/jdshop/bin /opt/jdshop/migrations /opt/jdshop/static
StandardOutput=journal
StandardError=journal
SyslogIdentifier=jdshop

[Install]
WantedBy=multi-user.target
```

安装和启动：

```bash
install -o root -g root -m 0644 deploy/jdshop.service /etc/systemd/system/jdshop.service
systemctl daemon-reload
systemctl enable --now jdshop
systemctl status jdshop --no-pager --full
curl -fsS http://127.0.0.1:8080/api/v1/health
```

应用启动时会自动执行尚未应用的SQL迁移。也可在服务停止时手工运行：

```bash
cd /opt/jdshop
sudo -u jdshop /opt/jdshop/bin/jdshop-server migrate
```

## 8. Nginx和SSE

首次签发证书前安装仓库的 `deploy/nginx.conf` HTTP引导模板，把 `api.yourdomain.com` 替换为真实域名。证书安装完成后改用 `deploy/nginx-https.conf` 生产模板。两份模板都必须保留SSE精确匹配并放在通用 `/api/` 规则之前：

```nginx
location = /api/v1/control/stream {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
    proxy_send_timeout 1h;
    gzip off;
}
```

安装：

```bash
install -o root -g root -m 0644 /opt/jdshop/deploy/nginx.conf /etc/nginx/sites-available/jdshop
sed -i 's/api\.yourdomain\.com/api.jdshop.bbroot.com/g' /etc/nginx/sites-available/jdshop
ln -sfn /etc/nginx/sites-available/jdshop /etc/nginx/sites-enabled/jdshop
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx
```

健康接口只支持GET。`curl -I`发送HEAD时返回405不代表服务异常，应使用：

```bash
curl -i http://api.jdshop.bbroot.com/api/v1/health
```

## 9. HTTPS证书

### 9.1 方案A：Let's Encrypt / Certbot

```bash
certbot --nginx -d api.jdshop.bbroot.com --redirect
certbot renew --dry-run
```

Let's Encrypt对同一注册域名存在签发频率限制。如果共享父域 `bbroot.com` 在7天内达到上限，申请会返回 `too many certificates`。这不影响HTTP服务，但在证书签发前443不可用。

### 9.2 方案B：ZeroSSL / acme.sh

遇到Let's Encrypt限流时可用ZeroSSL。HTTP-01签发期间必须保持80端口公网可达。

```bash
curl https://get.acme.sh | sh -s email=你的邮箱
/root/.acme.sh/acme.sh --set-default-ca --server zerossl

/root/.acme.sh/acme.sh --issue \
  -d api.jdshop.bbroot.com \
  --webroot /var/www/html

install -d -o root -g root -m 0700 /etc/nginx/ssl/jdshop
/root/.acme.sh/acme.sh --install-cert -d api.jdshop.bbroot.com \
  --key-file /etc/nginx/ssl/jdshop/privkey.pem \
  --fullchain-file /etc/nginx/ssl/jdshop/fullchain.pem \
  --reloadcmd "systemctl reload nginx"
```

Nginx HTTPS server使用：

```nginx
ssl_certificate /etc/nginx/ssl/jdshop/fullchain.pem;
ssl_certificate_key /etc/nginx/ssl/jdshop/privkey.pem;
```

仓库 `deploy/nginx-https.conf` 已包含80端口ACME验证目录、其他HTTP请求跳转HTTPS、443证书路径、API限流、管理台反代和SSE长连接规则。安装并替换域名：

```bash
install -o root -g root -m 0644 \
  /opt/jdshop/deploy/nginx-https.conf \
  /etc/nginx/sites-available/jdshop
sed -i 's/api\.yourdomain\.com/api.jdshop.bbroot.com/g' /etc/nginx/sites-available/jdshop
nginx -t
systemctl reload nginx
```

acme.sh安装证书后会创建自动续期任务。检查：

```bash
/root/.acme.sh/acme.sh --list
crontab -l
openssl x509 -in /etc/nginx/ssl/jdshop/fullchain.pem -noout -issuer -subject -dates
```

## 10. 数据库备份

仓库 `deploy/backup.sh` 使用SQLite `VACUUM INTO` 在线生成一致备份，文件名精确到秒：

```text
/opt/jdshop/backups/app-YYYY-MM-DD-HHMMSS.db
```

同一天可以同时执行定时和手工备份而不发生文件已存在冲突。安装定时任务：

```bash
chmod 0755 /opt/jdshop/deploy/backup.sh
(crontab -l 2>/dev/null; echo '0 3 * * * /bin/bash /opt/jdshop/deploy/backup.sh >> /opt/jdshop/logs/backup.log 2>&1') | crontab -
```

手工验证：

```bash
/bin/bash /opt/jdshop/deploy/backup.sh
ls -lh /opt/jdshop/backups/
sqlite3 "$(ls -1t /opt/jdshop/backups/app-*.db | head -n1)" 'PRAGMA integrity_check;'
```

备份默认保留7天。生产建议另有异机或对象存储备份，服务器本机备份无法应对整盘损坏。

## 11. 更新和回滚

标准更新：

1. 先执行数据库备份；
2. 上传并校验新部署包；
3. 保留当前二进制的时间戳备份；
4. 停止服务；
5. 替换二进制、迁移、静态管理台和非敏感配置；
6. 启动服务并验证本机健康；
7. 验证HTTPS健康、管理台和SSE；
8. 观察日志至少数分钟。

可在解压后的发布目录执行仓库 `deploy/deploy.sh`。该脚本会在停止服务前检查 `/etc/jdshop/jdshop.env`，把旧二进制备份为带时间戳的文件，并同步 `/opt/jdshop/deploy/` 与systemd unit。脚本只更新Nginx模板，不自动覆盖 `/etc/nginx/sites-available/jdshop`，避免意外丢失生产域名和证书配置；Nginx规则变化需人工对比、测试后安装。

二进制回滚：

```bash
systemctl stop jdshop
cp /opt/jdshop/bin/jdshop-server.bak-YYYYMMDD-HHMMSS /opt/jdshop/bin/jdshop-server
chmod 0755 /opt/jdshop/bin/jdshop-server
systemctl start jdshop
curl -fsS http://127.0.0.1:8080/api/v1/health
```

数据库迁移通常只向前兼容。需要数据库回滚时必须先停止服务，并明确恢复到更新前备份；不要在服务运行时直接覆盖 `app.db`。

## 12. 验证命令

```bash
systemctl is-enabled jdshop
systemctl is-active jdshop
systemctl status jdshop --no-pager --full
journalctl -u jdshop -n 100 --no-pager

curl -fsS http://127.0.0.1:8080/api/v1/health
curl -fsS https://api.jdshop.bbroot.com/api/v1/health
curl -I https://api.jdshop.bbroot.com/admin

nginx -t
ss -lntp | grep -E ':(80|443|8080)'
```

使用有效普通账号JWT验证SSE：

```bash
curl -N --http1.1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" \
  https://api.jdshop.bbroot.com/api/v1/control/stream
```

应立即收到 `connected`，之后约每15秒看到注释保活。客户端正常情况下每1分钟心跳一次。

## 13. Shell执行安全

不要把包含 `set -e` 或 `set -Eeuo pipefail` 的多行安装脚本直接粘贴到当前交互式SSH Shell；任一预期内的非零退出码都可能结束当前Shell，让SSH窗口表现为闪退。应在子Shell中执行：

```bash
bash <<'JDSHOP_INSTALL'
set -Eeuo pipefail
# 安装命令
JDSHOP_INSTALL

echo "当前SSH仍然保持连接。"
```

脚本失败时保留输出，重新登录后先做只读检查，不要重复执行未知状态的整段安装脚本。

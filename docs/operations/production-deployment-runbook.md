# 生产服务器部署与运维手册

本文记录 jdShopServer 在全新香港服务器上的可重复部署流程和已遇到的故障。它是值班/重装/迁移时使用的操作手册；架构原则和通用配置见 [部署指南](../deployment.md)。

## 1. 当前生产基线

| 项目 | 当前约定 |
|------|----------|
| 系统 | Ubuntu 22.04 LTS x86_64 |
| 运行用户 | `jdshop`，无登录Shell |
| 应用目录 | `/opt/jdshop` |
| 敏感配置 | `/etc/jdshop/jdshop.env`，root:root，0600 |
| systemd | `jdshop.service` |
| 本地监听 | `127.0.0.1:8080` |
| API域名 | `api.jdshop.bbroot.com`，只允许 `/api/*` |
| 管理台域名 | `www.jdshop.bbroot.com`，只允许 `/admin` |
| 反向代理 | Nginx 1.18+ |
| HTTPS | ZeroSSL证书，由acme.sh安装和续期 |
| 数据库 | `/opt/jdshop/data/app.db`，SQLite WAL |
| 管理台 | `https://www.jdshop.bbroot.com/admin` |
| 健康检查 | `https://api.jdshop.bbroot.com/api/v1/health` |

不要把服务器IP、管理员密码、JWT密钥、证书账号邮箱或临时下载签名写入本文和Git。

## 2. 首次部署前检查

在服务器执行：

```bash
cat /etc/os-release
uname -m
whoami
timedatectl status
df -h /
free -h
getent hosts api.jdshop.bbroot.com
getent hosts www.jdshop.bbroot.com
```

应满足：Ubuntu 22.04、`x86_64`、当前用户root、域名解析到当前公网IP。云安全组仅放通管理来源的22，以及公网80/443。

安装依赖并确认Nginx：

```bash
apt update
apt install -y nginx curl unzip sqlite3 ca-certificates socat cron openssl
systemctl enable --now nginx
nginx -v
curl -I http://127.0.0.1/
```

## 3. 接收部署包

上传发布ZIP到 `/root`，然后记录：

```bash
ls -lh /root/jdshop-server-*.zip
sha256sum /root/jdshop-server-*.zip
unzip -t /root/jdshop-server-*.zip
```

哈希必须与开发机发布记录一致。不要在服务器端重新打包后继续沿用旧哈希。

### 3.1 Windows ZIP反斜杠警告

曾遇到：

```text
warning: ... appears to use backslashes as path separators
unzip退出码：1
```

这是Windows压缩工具生成路径格式导致的警告。处理方法：

```bash
STAGE_DIR="$(mktemp -d /root/jdshop-stage.XXXXXX)"
unzip -q /root/jdshop-server-*.zip -d "$STAGE_DIR" || UNZIP_CODE=$?
echo "unzip退出码：${UNZIP_CODE:-0}"
find "$STAGE_DIR" -maxdepth 2 -type f -printf '%p %s bytes\n' | sort
test -f "$STAGE_DIR/jdshop-server"
test -f "$STAGE_DIR/config.yaml"
test -f "$STAGE_DIR/deploy/jdshop.service"
test -f "$STAGE_DIR/static/admin.html"
```

只有核心文件均存在且原ZIP完整性测试通过时，才可忽略退出码1。禁止无条件写 `unzip ... && 后续安装`，否则警告会阻止后续步骤。

## 4. 安装应用

首次创建用户和目录：

```bash
id jdshop >/dev/null 2>&1 || useradd --system \
  --home-dir /opt/jdshop \
  --shell /usr/sbin/nologin \
  jdshop

install -d -o root -g root -m 0755 \
  /opt/jdshop /opt/jdshop/bin /opt/jdshop/migrations \
  /opt/jdshop/static /opt/jdshop/deploy

install -d -o jdshop -g jdshop -m 0750 \
  /opt/jdshop/data /opt/jdshop/logs /opt/jdshop/backups
```

从已确认的暂存目录安装：

```bash
install -o root -g root -m 0755 \
  "$STAGE_DIR/jdshop-server" \
  /opt/jdshop/bin/jdshop-server

install -o root -g root -m 0644 \
  "$STAGE_DIR"/migrations/*.sql \
  /opt/jdshop/migrations/

install -o root -g root -m 0644 \
  "$STAGE_DIR"/static/*.html \
  /opt/jdshop/static/

install -o root -g root -m 0644 \
  "$STAGE_DIR/config.yaml" \
  /opt/jdshop/config.yaml

install -o root -g root -m 0755 \
  "$STAGE_DIR/deploy/deploy.sh" \
  "$STAGE_DIR/deploy/backup.sh" \
  /opt/jdshop/deploy/

install -o root -g root -m 0644 \
  "$STAGE_DIR/deploy/nginx.conf" \
  "$STAGE_DIR/deploy/nginx-https.conf" \
  /opt/jdshop/deploy/
```

## 5. 创建敏感环境文件

生成随机密钥：

```bash
openssl rand -base64 48
```

创建 `/etc/jdshop/jdshop.env`：

```text
JWT_SECRET=实际随机密钥
SUPER_ADMIN_USERNAME=admin
CORS_ALLOWED_ORIGINS=https://www.jdshop.bbroot.com,http://localhost:8788,http://127.0.0.1:8788
SMSBAO_ENABLED=true
SMSBAO_USERNAME=短信宝后台用户名
SMSBAO_API_KEY=新建且未泄露的APIKey
SMSBAO_GOODSID=已报备产品ID
SMSBAO_CONTENT_TEMPLATE="【已报备短信签名】您的验证码是%s，5分钟内有效。"
AUTH_REQUIRE_ENCRYPTED_REQUESTS=true
```

确认安装后的 `/etc/systemd/system/jdshop.service` 包含 `UMask=0077`；数据库、日志和备份不得对其他系统用户开放读取权限。

然后：

```bash
chown root:root /etc/jdshop/jdshop.env
chmod 0600 /etc/jdshop/jdshop.env
stat -c '%U:%G %a %n' /etc/jdshop/jdshop.env
```

不要执行 `systemctl cat jdshop` 后把包含密钥的输出复制到聊天或工单。systemd unit只引用环境文件，不内联密钥。生产管理台跨域调用API，`CORS_ALLOWED_ORIGINS` 缺少 `https://www.jdshop.bbroot.com` 时会在浏览器中表现为登录请求被CORS拦截。短信签名、模板和产品ID必须先在短信宝后台审核报备；任何曾经粘贴到聊天或工单的API Key都必须先轮换再上线。

## 6. 安装和启动systemd

```bash
install -o root -g root -m 0644 \
  "$STAGE_DIR/deploy/jdshop.service" \
  /etc/systemd/system/jdshop.service

systemctl daemon-reload
systemctl enable --now jdshop
sleep 2
systemctl status jdshop --no-pager --full
journalctl -u jdshop -n 100 --no-pager
curl -fsS http://127.0.0.1:8080/api/v1/health
```

正常日志包含数据库迁移完成和监听 `127.0.0.1:8080`。若出现 `Connection refused`，依次检查服务状态、环境文件、配置路径、二进制架构和日志。

## 7. 配置Nginx

```bash
install -o root -g root -m 0644 \
  /opt/jdshop/deploy/nginx.conf \
  /etc/nginx/sites-available/jdshop

sed -i 's/api\.yourdomain\.com/api.jdshop.bbroot.com/g' \
  /etc/nginx/sites-available/jdshop
sed -i 's/www\.yourdomain\.com/www.jdshop.bbroot.com/g' \
  /etc/nginx/sites-available/jdshop

ln -sfn /etc/nginx/sites-available/jdshop /etc/nginx/sites-enabled/jdshop
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx
```

证书签发前的引导配置只开放ACME目录，根路径应返回404：

```bash
curl -i -H 'Host: api.jdshop.bbroot.com' http://127.0.0.1/
curl -i -H 'Host: www.jdshop.bbroot.com' http://127.0.0.1/
```

### 7.1 启用生产HTTPS模板后健康检查仍为404

曾出现Nginx配置测试成功，但Host请求 `/api/v1/health` 返回404。排查顺序：

1. `nginx -T | less` 确认实际加载的是哪个server块；
2. 检查模板域名是否已替换；
3. 检查 `sites-enabled/jdshop` 链接；
4. 检查 `location /api/` 的 `proxy_pass` 是否保留正确URI；
5. 用 `curl http://127.0.0.1:8080/api/v1/health` 排除Go服务问题；
6. 修正后 `nginx -t && systemctl reload nginx`。

HTTP引导模板返回404是预期行为。只有证书安装并切换到 `nginx-https.conf` 后，API域名的 `/api/v1/health` 才应返回200；此时若仍为404，再按以上顺序排查。

## 8. 安装ZeroSSL证书

Let's Encrypt曾因共享注册域 `bbroot.com` 的7天50张证书限制拒绝签发。当前生产改用ZeroSSL/acme.sh：

```bash
curl https://get.acme.sh | sh -s email=证书通知邮箱
/root/.acme.sh/acme.sh --set-default-ca --server zerossl

/root/.acme.sh/acme.sh --issue \
  -d api.jdshop.bbroot.com \
  -d www.jdshop.bbroot.com \
  --webroot /var/www/html

install -d -o root -g root -m 0700 /etc/nginx/ssl/jdshop

/root/.acme.sh/acme.sh --install-cert -d api.jdshop.bbroot.com \
  --key-file /etc/nginx/ssl/jdshop/privkey.pem \
  --fullchain-file /etc/nginx/ssl/jdshop/fullchain.pem \
  --reloadcmd 'systemctl reload nginx'
```

证书安装完成后使用仓库的生产模板配置443和80跳转：

```bash
install -o root -g root -m 0644 \
  /opt/jdshop/deploy/nginx-https.conf \
  /etc/nginx/sites-available/jdshop

sed -i 's/api\.yourdomain\.com/api.jdshop.bbroot.com/g' \
  /etc/nginx/sites-available/jdshop
sed -i 's/www\.yourdomain\.com/www.jdshop.bbroot.com/g' \
  /etc/nginx/sites-available/jdshop

nginx -t
systemctl reload nginx
```

该模板保留 `/.well-known/acme-challenge/` 的HTTP-01目录，其他80端口请求永久跳转到HTTPS。然后执行：

```bash
nginx -t
systemctl reload nginx
curl -i https://api.jdshop.bbroot.com/api/v1/health
curl -i http://api.jdshop.bbroot.com/api/v1/health
curl -I https://www.jdshop.bbroot.com/
curl -I https://www.jdshop.bbroot.com/admin
curl -o /dev/null -sS -w '%{http_code}\n' https://api.jdshop.bbroot.com/admin
curl -o /dev/null -sS -w '%{http_code}\n' https://www.jdshop.bbroot.com/api/v1/health
/root/.acme.sh/acme.sh --list
crontab -l
```

HTTP检查必须使用GET。`curl -I`是HEAD，而健康接口只允许GET，因此可能返回405；这与HTTP跳转或HTTPS证书是否有效不是同一问题。

## 9. 首次上线后的安全动作

1. 打开 `https://www.jdshop.bbroot.com/admin`，使用内置主管理员登录；
2. 立即修改默认密码；
3. 退出并用新密码重新登录；
4. 确认普通注册账号无法登录管理台；
5. 确认 `https://api.jdshop.bbroot.com/` 和该域名的 `/admin` 均返回404；
6. 确认 `www` 域名下的 `/api/*` 返回404；
7. 确认主管理员没有禁用、改权限、改角色按钮；
8. 打开“注册默认”，设置新用户默认赠送天数和三个板块权限并保存；
9. 确认注册页必须先完成图形验证码才能请求短信；使用已授权的测试手机号和正式报备内容发送一次，核对60秒倒计时与5分钟有效期；
10. 等待至少60秒后再测试改密短信，不连续向同一手机号发送相同内容；单手机号当天最多执行6次；
11. 注册一个临时普通账号，确认手机号加密码可以登录，并获得刚才设置的使用期和板块权限，同时已有账号权限不变；
12. 修改密码时确认短信验证通过后旧Access/Refresh Token全部失效，新密码可重新登录；
13. 从早期测试包升级时确认启动日志包含 `Database migrations completed`；迁移器可以识别未记录但已存在的 `phone` 字段和短信表，禁止通过删除生产数据库解决重复字段错误；
14. 不在聊天、截图或Shell历史中保存密码、JWT密钥或短信API Key。

## 10. 备份和恢复演练

安装cron：

```bash
chmod 0755 /opt/jdshop/deploy/backup.sh
(crontab -l 2>/dev/null; echo '0 3 * * * /bin/bash /opt/jdshop/deploy/backup.sh >> /opt/jdshop/logs/backup.log 2>&1') | crontab -
```

验证：

```bash
/bin/bash /opt/jdshop/deploy/backup.sh
LATEST_BACKUP="$(ls -1t /opt/jdshop/backups/app-*.db | head -n1)"
ls -lh "$LATEST_BACKUP"
sqlite3 "$LATEST_BACKUP" 'PRAGMA integrity_check;'
```

恢复步骤：

```bash
systemctl stop jdshop
cp /opt/jdshop/data/app.db /opt/jdshop/data/app.db.before-restore-$(date +%Y%m%d-%H%M%S)
cp /opt/jdshop/backups/app-YYYY-MM-DD-HHMMSS.db /opt/jdshop/data/app.db
rm -f /opt/jdshop/data/app.db-wal /opt/jdshop/data/app.db-shm
chown jdshop:jdshop /opt/jdshop/data/app.db
chmod 0640 /opt/jdshop/data/app.db
systemctl start jdshop
curl -fsS http://127.0.0.1:8080/api/v1/health
```

恢复前必须明确选择备份文件，不能把通配符直接交给 `cp`。

## 11. 日常发布

发布前：

```bash
/bin/bash /opt/jdshop/deploy/backup.sh
sha256sum /root/新部署包.zip
unzip -t /root/新部署包.zip
```

从新包暂存目录执行：

```bash
cd "$STAGE_DIR"
/bin/bash deploy/deploy.sh
```

脚本会更新应用文件、`/opt/jdshop/deploy/` 和systemd unit，但不会自动覆盖当前生效的Nginx站点。若发布包含Nginx变更，先比较 `deploy/nginx-https.conf` 与 `/etc/nginx/sites-available/jdshop`，保留真实域名和证书路径，执行 `nginx -t` 后再重新加载。

发布后：

```bash
systemctl status jdshop --no-pager --full
journalctl -u jdshop -n 100 --no-pager
curl -fsS http://127.0.0.1:8080/api/v1/health
curl -fsS https://api.jdshop.bbroot.com/api/v1/health
curl -I https://www.jdshop.bbroot.com/admin
nginx -t
```

再人工验证管理台登录、普通账号登录、心跳、禁用即时响应和版本检查。不要只以systemd显示active作为发布成功依据。

## 12. 常用排障

### SSH执行命令后断开或闪退

原因通常是把带 `set -e` 的脚本直接粘贴到当前交互Shell，遇到 `unzip`警告等非零退出码后Shell退出。重新连接后先只读检查：

```bash
whoami
uptime
id jdshop 2>&1 || true
ls -ld /opt/jdshop /opt/jdshop/bin /opt/jdshop/data 2>&1 || true
systemctl status jdshop --no-pager --full 2>&1 || true
journalctl -u jdshop -n 50 --no-pager 2>&1 || true
curl --max-time 5 -v http://127.0.0.1:8080/api/v1/health 2>&1 || true
```

以后使用子Shell：

```bash
bash <<'SCRIPT'
set -Eeuo pipefail
# 操作
SCRIPT
echo "当前SSH仍然保持连接。"
```

### systemd服务不存在

```bash
ls -l /etc/systemd/system/jdshop.service
systemctl daemon-reload
systemctl enable --now jdshop
```

如果用户和 `/opt/jdshop` 都不存在，说明安装在真正写入系统前已中止，应从创建运行账号步骤重新开始，不要假设已经部署了一半。

### 服务启动失败

```bash
systemctl status jdshop --no-pager --full
journalctl -u jdshop -n 200 --no-pager
sudo -u jdshop test -r /opt/jdshop/config.yaml
sudo -u jdshop test -x /opt/jdshop/bin/jdshop-server
sudo -u jdshop test -w /opt/jdshop/data
test -r /etc/jdshop/jdshop.env
file /opt/jdshop/bin/jdshop-server
```

### HTTPS连接被拒绝

```bash
ss -lntp | grep ':443'
nginx -t
systemctl status nginx --no-pager
ls -l /etc/nginx/ssl/jdshop/
tail -n 100 /var/log/nginx/error.log
```

若证书尚未签发，443连接被拒绝是预期状态；先保持HTTP可用，完成证书安装后再启用443。

### 客户端禁用不能立即生效

```bash
nginx -T | grep -A20 -B5 '/api/v1/control/stream'
journalctl -u jdshop -f
```

确认SSE规则关闭缓冲、超时为1小时，并使用有效用户Token执行：

```bash
curl -N --http1.1 \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Accept: text/event-stream' \
  https://api.jdshop.bbroot.com/api/v1/control/stream
```

即使SSE故障，一分钟心跳仍应生效；若一分钟后也无变化，应继续检查心跳接口、账号状态和客户端API地址。

## 13. 定期检查清单

每周：

- 检查 `systemctl is-active jdshop nginx`；
- 检查磁盘、内存和日志增长；
- 检查最近备份及 `PRAGMA integrity_check`；
- 检查异常登录和频繁限流日志。

每月：

- 检查证书到期日和acme.sh续期任务；
- 随机恢复一个备份到临时目录验证可用性；
- 清理确认无用的旧二进制备份和部署暂存目录；
- 检查Ubuntu安全更新并安排维护窗口；
- 用低权限账号复测管理接口边界。

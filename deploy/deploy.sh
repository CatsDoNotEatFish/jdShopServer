#!/bin/bash
# 一键部署脚本 (在服务器上执行)
set -Eeuo pipefail

APP_DIR="/opt/jdshop"
SERVICE_NAME="jdshop"
ENV_FILE="/etc/jdshop/jdshop.env"

echo "=== 检查生产环境变量 ==="
if [ ! -f "$ENV_FILE" ]; then
    echo "缺少 $ENV_FILE。首次部署请先创建该文件，并写入 JWT_SECRET 和 SUPER_ADMIN_USERNAME。" >&2
    exit 1
fi
chown root:root "$ENV_FILE"
chmod 0600 "$ENV_FILE"

echo "=== 停止旧服务 ==="
systemctl stop "$SERVICE_NAME" || true

echo "=== 备份当前二进制 ==="
if [ -f "$APP_DIR/bin/jdshop-server" ]; then
    cp "$APP_DIR/bin/jdshop-server" "$APP_DIR/bin/jdshop-server.bak-$(date +%Y%m%d-%H%M%S)"
fi

echo "=== 复制新二进制 ==="
cp jdshop-server "$APP_DIR/bin/"
chmod +x "$APP_DIR/bin/jdshop-server"
chown root:root "$APP_DIR/bin/jdshop-server"

echo "=== 复制迁移脚本 ==="
mkdir -p "$APP_DIR/migrations"
cp migrations/*.sql "$APP_DIR/migrations/"
chown -R root:root "$APP_DIR/migrations/"

echo "=== 复制管理控制台 ==="
mkdir -p "$APP_DIR/static"
rm -f "$APP_DIR/static/index.html"
cp static/*.html "$APP_DIR/static/"
chown -R root:root "$APP_DIR/static/"

echo "=== 更新部署模板和 systemd unit ==="
mkdir -p "$APP_DIR/deploy"
install -o root -g root -m 0755 deploy/deploy.sh deploy/backup.sh "$APP_DIR/deploy/"
install -o root -g root -m 0644 deploy/nginx.conf deploy/nginx-https.conf "$APP_DIR/deploy/"
install -o root -g root -m 0644 deploy/jdshop.service /etc/systemd/system/jdshop.service
systemctl daemon-reload

echo "=== 复制配置文件 ==="
if [ ! -f "$APP_DIR/config.yaml" ]; then
    cp config.yaml "$APP_DIR/"
    chown root:root "$APP_DIR/config.yaml"
    echo "  已安装默认配置：$APP_DIR/config.yaml"
fi

echo "=== 启动服务 ==="
systemctl start "$SERVICE_NAME"

echo "=== 等待启动 ==="
sleep 2

echo "=== 检查状态 ==="
systemctl status "$SERVICE_NAME" --no-pager

echo ""
echo "=== 健康检查 ==="
sleep 1
curl -fsS http://127.0.0.1:8080/api/v1/health
echo ""

echo ""
echo "部署完成。如在服务器首次部署，还需执行："
echo "  systemctl enable jdshop"
echo "  配置 Nginx，并使用 Certbot/Let's Encrypt 或 acme.sh/ZeroSSL 安装 HTTPS 证书"

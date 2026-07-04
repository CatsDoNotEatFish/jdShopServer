#!/bin/bash
# 一键部署脚本 (在服务器上执行)
set -e

APP_DIR="/opt/jdshop"
SERVICE_NAME="jdshop"

echo "=== 停止旧服务 ==="
systemctl stop $SERVICE_NAME || true

echo "=== 备份当前二进制 ==="
if [ -f "$APP_DIR/bin/jdshop-server" ]; then
    cp "$APP_DIR/bin/jdshop-server" "$APP_DIR/bin/jdshop-server.bak"
fi

echo "=== 复制新二进制 ==="
cp jdshop-server "$APP_DIR/bin/"
chmod +x "$APP_DIR/bin/jdshop-server"
chown jdshop:jdshop "$APP_DIR/bin/jdshop-server"

echo "=== 复制迁移脚本 ==="
mkdir -p "$APP_DIR/migrations"
cp migrations/*.sql "$APP_DIR/migrations/"
chown -R jdshop:jdshop "$APP_DIR/migrations/"

echo "=== 复制配置文件 ==="
if [ ! -f "$APP_DIR/config.yaml" ]; then
    cp config.yaml "$APP_DIR/"
    chown jdshop:jdshop "$APP_DIR/config.yaml"
    echo "  请编辑 $APP_DIR/config.yaml 并设置 JWT_SECRET"
fi

echo "=== 启动服务 ==="
systemctl start $SERVICE_NAME

echo "=== 等待启动 ==="
sleep 2

echo "=== 检查状态 ==="
systemctl status $SERVICE_NAME --no-pager

echo ""
echo "=== 健康检查 ==="
sleep 1
curl -s http://127.0.0.1:8080/api/v1/health
echo ""

echo ""
echo "部署完成。如在服务器首次部署，还需执行："
echo "  systemctl enable jdshop"
echo "  certbot --nginx -d api.yourdomain.com"

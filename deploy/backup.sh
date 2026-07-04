#!/bin/bash
# 数据库备份脚本
# 建议 cron: 0 3 * * * /bin/bash /opt/jdshop/deploy/backup.sh

BACKUP_DIR="/opt/jdshop/backups"
DB_PATH="/opt/jdshop/data/app.db"
KEEP_DAYS=7

mkdir -p "$BACKUP_DIR"

# SQLite 在线备份
sqlite3 "$DB_PATH" "VACUUM INTO '$BACKUP_DIR/app-$(date +%Y-%m-%d).db'"

echo "[$(date)] Backup completed: $BACKUP_DIR/app-$(date +%Y-%m-%d).db"

# 清理过期备份
find "$BACKUP_DIR" -name "app-*.db" -mtime +$KEEP_DAYS -delete

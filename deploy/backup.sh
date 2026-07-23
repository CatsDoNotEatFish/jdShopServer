#!/bin/bash
# 数据库备份脚本
# 建议 cron: 0 3 * * * /bin/bash /opt/jdshop/deploy/backup.sh
set -Eeuo pipefail

BACKUP_DIR="/opt/jdshop/backups"
DB_PATH="/opt/jdshop/data/app.db"
KEEP_DAYS=7
BACKUP_FILE="$BACKUP_DIR/app-$(date +%Y-%m-%d-%H%M%S).db"

mkdir -p "$BACKUP_DIR"

# SQLite 在线备份
sqlite3 "$DB_PATH" "VACUUM INTO '$BACKUP_FILE'"

echo "[$(date --iso-8601=seconds)] Backup completed: $BACKUP_FILE"

# 清理过期备份
find "$BACKUP_DIR" -name "app-*.db" -mtime +$KEEP_DAYS -delete

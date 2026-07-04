package repository

import (
	"database/sql"
	"fmt"

	"jdShopServer/internal/model"
)

type HeartbeatRepo struct {
	db *sql.DB
}

func NewHeartbeatRepo(db *sql.DB) *HeartbeatRepo {
	return &HeartbeatRepo{db: db}
}

func (r *HeartbeatRepo) Create(h *model.HeartbeatLog) error {
	_, err := r.db.Exec(
		`INSERT INTO heartbeat_logs (user_id, device_id, platform, app_version, ip_address)
		 VALUES (?, ?, ?, ?, ?)`,
		h.UserID, h.DeviceID, h.Platform, h.AppVersion, h.IPAddress,
	)
	return err
}

func (r *HeartbeatRepo) CleanupOlderThan(days int) error {
	_, err := r.db.Exec(
		`DELETE FROM heartbeat_logs WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	return err
}

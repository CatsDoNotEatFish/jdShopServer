package repository

import (
	"database/sql"
	"fmt"
	"time"

	"jdShopServer/internal/model"
)

type LoginLogRepo struct {
	db *sql.DB
}

func NewLoginLogRepo(db *sql.DB) *LoginLogRepo {
	return &LoginLogRepo{db: db}
}

func (r *LoginLogRepo) Create(log *model.LoginLog) error {
	_, err := r.db.Exec(
		`INSERT INTO login_logs (user_id, username, ip_address, user_agent, result, fail_reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		log.UserID, log.Username, log.IPAddress, log.UserAgent, log.Result, log.FailReason,
	)
	return err
}

func (r *LoginLogRepo) CountFailures(username, ip string, minutes int) (int, error) {
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339)
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM login_logs
		 WHERE result = 'failed' AND created_at > ? AND (username = ? OR ip_address = ?)`,
		since, username, ip,
	).Scan(&count)
	return count, err
}

func (r *LoginLogRepo) CleanupOlderThan(days int) error {
	_, err := r.db.Exec(
		`DELETE FROM login_logs WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	return err
}

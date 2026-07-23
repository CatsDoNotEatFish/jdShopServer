package repository

import (
	"database/sql"
	"time"

	"jdShopServer/internal/model"
)

type AccessRepo struct {
	db *sql.DB
}

func NewAccessRepo(db *sql.DB) *AccessRepo {
	return &AccessRepo{db: db}
}

func (r *AccessRepo) CreateDefault(userID int64, usageDays int) error {
	if usageDays <= 0 {
		usageDays = 30
	}
	expiresAt := time.Now().UTC().AddDate(0, 0, usageDays).Format(time.RFC3339)
	_, err := r.db.Exec(
		`INSERT OR IGNORE INTO user_access_control (
		 user_id, competitor_monitor, merchant_backend, analysis_center, expires_at, updated_at
		) VALUES (?, 1, 0, 0, ?, ?)`,
		userID, expiresAt, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (r *AccessRepo) FindByUserID(userID int64) (*model.AccessPolicy, error) {
	policy := &model.AccessPolicy{}
	err := r.db.QueryRow(
		`SELECT user_id, competitor_monitor, merchant_backend, analysis_center, expires_at, updated_at
		 FROM user_access_control WHERE user_id = ?`,
		userID,
	).Scan(
		&policy.UserID,
		&policy.CompetitorMonitor,
		&policy.MerchantBackend,
		&policy.AnalysisCenter,
		&policy.ExpiresAt,
		&policy.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (r *AccessRepo) Update(userID int64, req model.UpdateUserAccessRequest) error {
	_, err := r.db.Exec(
		`INSERT INTO user_access_control (
		 user_id, competitor_monitor, merchant_backend, analysis_center, expires_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		 competitor_monitor = excluded.competitor_monitor,
		 merchant_backend = excluded.merchant_backend,
		 analysis_center = excluded.analysis_center,
		 expires_at = excluded.expires_at,
		 updated_at = excluded.updated_at`,
		userID,
		boolInt(req.CompetitorMonitor),
		boolInt(req.MerchantBackend),
		boolInt(req.AnalysisCenter),
		req.ExpiresAt,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

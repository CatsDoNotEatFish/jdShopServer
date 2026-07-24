package repository

import (
	"database/sql"
	"time"

	"jdShopServer/internal/model"
)

type RegistrationDefaultsRepo struct {
	db *sql.DB
}

func NewRegistrationDefaultsRepo(db *sql.DB) *RegistrationDefaultsRepo {
	return &RegistrationDefaultsRepo{db: db}
}

func (r *RegistrationDefaultsRepo) Get() (model.RegistrationDefaults, error) {
	var defaults model.RegistrationDefaults
	var competitorMonitor, merchantBackend, analysisCenter int
	err := r.db.QueryRow(
		`SELECT usage_days, competitor_monitor, merchant_backend, analysis_center, updated_at
		 FROM registration_defaults WHERE id = 1`,
	).Scan(
		&defaults.UsageDays,
		&competitorMonitor,
		&merchantBackend,
		&analysisCenter,
		&defaults.UpdatedAt,
	)
	if err != nil {
		return model.RegistrationDefaults{}, err
	}
	defaults.CompetitorMonitor = competitorMonitor == 1
	defaults.MerchantBackend = merchantBackend == 1
	defaults.AnalysisCenter = analysisCenter == 1
	return defaults, nil
}

func (r *RegistrationDefaultsRepo) Update(req model.UpdateRegistrationDefaultsRequest) error {
	_, err := r.db.Exec(
		`UPDATE registration_defaults SET
		 usage_days = ?, competitor_monitor = ?, merchant_backend = ?, analysis_center = ?, updated_at = ?
		 WHERE id = 1`,
		req.UsageDays,
		boolInt(req.CompetitorMonitor),
		boolInt(req.MerchantBackend),
		boolInt(req.AnalysisCenter),
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

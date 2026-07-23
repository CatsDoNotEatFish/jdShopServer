package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"jdShopServer/internal/model"
)

type VersionRepo struct {
	db *sql.DB
}

func NewVersionRepo(db *sql.DB) *VersionRepo {
	return &VersionRepo{db: db}
}

func (r *VersionRepo) Create(v *model.AppVersion) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(
		`INSERT INTO app_versions (platform, version_code, version_name, title, description,
		 download_url, file_size, file_hash, is_force, is_latest)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, 0), 1)`,
		v.Platform, v.VersionCode, v.VersionName, v.Title, v.Description,
		v.DownloadURL, v.FileSize, v.FileHash, v.IsForce,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	v.ID = id

	// Set all other versions of same platform as not latest
	if _, err := tx.Exec(`UPDATE app_versions SET is_latest = 0 WHERE platform = ? AND id != ?`, v.Platform, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *VersionRepo) FindByID(id int64) (*model.AppVersion, error) {
	v := &model.AppVersion{}
	err := r.db.QueryRow(
		`SELECT id, platform, version_code, version_name, title, description, download_url,
		        file_size, file_hash, is_force, is_latest, created_at
		 FROM app_versions WHERE id = ?`, id,
	).Scan(&v.ID, &v.Platform, &v.VersionCode, &v.VersionName, &v.Title,
		&v.Description, &v.DownloadURL, &v.FileSize, &v.FileHash,
		&v.IsForce, &v.IsLatest, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *VersionRepo) FindLatest(platform string) (*model.AppVersion, error) {
	v := &model.AppVersion{}
	err := r.db.QueryRow(
		`SELECT id, platform, version_code, version_name, title, description, download_url,
		        file_size, file_hash, is_force, is_latest, created_at
		 FROM app_versions WHERE platform = ? AND is_latest = 1 ORDER BY version_code DESC LIMIT 1`, platform,
	).Scan(&v.ID, &v.Platform, &v.VersionCode, &v.VersionName, &v.Title,
		&v.Description, &v.DownloadURL, &v.FileSize, &v.FileHash,
		&v.IsForce, &v.IsLatest, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *VersionRepo) List(page, pageSize int, platform string) ([]model.AppVersion, int64, error) {
	var conditions []string
	var args []any

	if platform != "" {
		conditions = append(conditions, "platform = ?")
		args = append(args, platform)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM app_versions %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(
		`SELECT id, platform, version_code, version_name, title, description, download_url,
		        file_size, file_hash, is_force, is_latest, created_at
		 FROM app_versions %s ORDER BY version_code DESC LIMIT ? OFFSET ?`, where,
	)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.AppVersion
	for rows.Next() {
		var v model.AppVersion
		if err := rows.Scan(&v.ID, &v.Platform, &v.VersionCode, &v.VersionName, &v.Title,
			&v.Description, &v.DownloadURL, &v.FileSize, &v.FileHash,
			&v.IsForce, &v.IsLatest, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, nil
}

func (r *VersionRepo) Update(id int64, title, description, downloadURL, fileHash *string,
	fileSize *int64, isForce, isLatest *int) error {
	var setParts []string
	var args []any

	if title != nil {
		setParts = append(setParts, "title = ?")
		args = append(args, *title)
	}
	if description != nil {
		setParts = append(setParts, "description = ?")
		args = append(args, *description)
	}
	if downloadURL != nil {
		setParts = append(setParts, "download_url = ?")
		args = append(args, *downloadURL)
	}
	if fileHash != nil {
		setParts = append(setParts, "file_hash = ?")
		args = append(args, *fileHash)
	}
	if fileSize != nil {
		setParts = append(setParts, "file_size = ?")
		args = append(args, *fileSize)
	}
	if isForce != nil {
		setParts = append(setParts, "is_force = ?")
		args = append(args, *isForce)
	}
	if isLatest != nil {
		setParts = append(setParts, "is_latest = ?")
		args = append(args, *isLatest)
	}

	if len(setParts) == 0 {
		return nil
	}

	if isLatest != nil && *isLatest == 1 {
		var platform string
		if err := r.db.QueryRow(`SELECT platform FROM app_versions WHERE id = ?`, id).Scan(&platform); err != nil {
			return err
		}
		tx, err := r.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`UPDATE app_versions SET is_latest = 0 WHERE platform = ?`, platform); err != nil {
			return err
		}
		args = append(args, id)
		query := fmt.Sprintf("UPDATE app_versions SET %s WHERE id = ?", strings.Join(setParts, ", "))
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
		return tx.Commit()
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE app_versions SET %s WHERE id = ?", strings.Join(setParts, ", "))
	_, err := r.db.Exec(query, args...)
	return err
}

func (r *VersionRepo) Delete(id int64) error {
	var platform string
	var wasLatest int
	if err := r.db.QueryRow(`SELECT platform, is_latest FROM app_versions WHERE id = ?`, id).Scan(&platform, &wasLatest); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM app_versions WHERE id = ?`, id); err != nil {
		return err
	}
	if wasLatest == 1 {
		if _, err := tx.Exec(`UPDATE app_versions SET is_latest = 1 WHERE id = (
			SELECT id FROM app_versions WHERE platform = ? ORDER BY version_code DESC LIMIT 1
		)`, platform); err != nil {
			return err
		}
	}
	return tx.Commit()
}

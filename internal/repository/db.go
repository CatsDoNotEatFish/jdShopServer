package repository

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string, maxOpenConns int) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_fk=ON")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(1)

	return db, nil
}

func RunMigrations(db *sql.DB, migrationsDir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return err
	}
	if err := reconcileExistingMigrations(db); err != nil {
		return err
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		var applied int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, f).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return err
		}
		if f == "005_phone_sms_auth.sql" {
			hasPhone, err := columnExists(db, "users", "phone")
			if err != nil {
				return err
			}
			if hasPhone {
				data = []byte(strings.Replace(string(data), "ALTER TABLE users ADD COLUMN phone TEXT;", "", 1))
			}
		}
		if f == "001_init.sql" {
			if _, err := db.Exec(string(data)); err != nil {
				return err
			}
			if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, f); err != nil {
				return err
			}
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, f); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func reconcileExistingMigrations(db *sql.DB) error {
	type migrationArtifact struct {
		version string
		kind    string
		name    string
	}
	artifacts := []migrationArtifact{
		{version: "001_init.sql", kind: "table", name: "users"},
		{version: "002_user_access_control.sql", kind: "table", name: "user_access_control"},
		{version: "003_user_auth_versions.sql", kind: "table", name: "user_auth_versions"},
		{version: "004_registration_defaults.sql", kind: "table", name: "registration_defaults"},
	}
	for _, artifact := range artifacts {
		exists, err := schemaObjectExists(db, artifact.kind, artifact.name)
		if err != nil {
			return err
		}
		if exists {
			if _, err := db.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (?)`, artifact.version); err != nil {
				return err
			}
		}
	}

	hasPhone, err := columnExists(db, "users", "phone")
	if err != nil {
		return err
	}
	hasSMSVerifications, err := schemaObjectExists(db, "table", "sms_verifications")
	if err != nil {
		return err
	}
	hasPhoneIndex, err := schemaObjectExists(db, "index", "idx_users_phone")
	if err != nil {
		return err
	}
	hasPurposeIndex, err := schemaObjectExists(db, "index", "idx_sms_verifications_phone_purpose")
	if err != nil {
		return err
	}
	hasSentIndex, err := schemaObjectExists(db, "index", "idx_sms_verifications_sent_at")
	if err != nil {
		return err
	}
	if hasPhone && hasSMSVerifications && hasPhoneIndex && hasPurposeIndex && hasSentIndex {
		_, err = db.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES ('005_phone_sms_auth.sql')`)
		return err
	}
	return nil
}

func schemaObjectExists(db *sql.DB, kind, name string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, kind, name).Scan(&count)
	return count > 0, err
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

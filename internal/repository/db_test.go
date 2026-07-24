package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunMigrationsUpgradesLegacyDatabaseAndIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"), 1)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	for _, name := range []string{"001_init.sql", "002_user_access_control.sql", "003_user_auth_versions.sql"} {
		data, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			t.Fatalf("read legacy migration %s: %v", name, readErr)
		}
		if _, execErr := db.Exec(string(data)); execErr != nil {
			t.Fatalf("apply legacy migration %s: %v", name, execErr)
		}
	}

	if err := RunMigrations(db, migrationsDir); err != nil {
		t.Fatalf("upgrade legacy database: %v", err)
	}
	if err := RunMigrations(db, migrationsDir); err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}

	var phoneColumns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'phone'`).Scan(&phoneColumns); err != nil {
		t.Fatalf("inspect users schema: %v", err)
	}
	if phoneColumns != 1 {
		t.Fatalf("phone column count=%d, want 1", phoneColumns)
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 6 {
		t.Fatalf("applied migration count=%d, want 6", applied)
	}
}

func TestRunMigrationsRecognizesFullyAppliedUntrackedPhoneMigration(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "untracked.db"), 1)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	for _, name := range []string{"001_init.sql", "002_user_access_control.sql", "003_user_auth_versions.sql", "004_registration_defaults.sql", "005_phone_sms_auth.sql"} {
		data, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			t.Fatalf("read migration %s: %v", name, readErr)
		}
		if _, execErr := db.Exec(string(data)); execErr != nil {
			t.Fatalf("apply untracked migration %s: %v", name, execErr)
		}
	}

	if err := RunMigrations(db, migrationsDir); err != nil {
		t.Fatalf("reconcile untracked migration: %v", err)
	}
	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 6 {
		t.Fatalf("applied migration count=%d, want 6", applied)
	}
}

func TestRunMigrationsRepairsPartiallyAppliedPhoneMigration(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "partial.db"), 1)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	for _, name := range []string{"001_init.sql", "002_user_access_control.sql", "003_user_auth_versions.sql", "004_registration_defaults.sql"} {
		data, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			t.Fatalf("read migration %s: %v", name, readErr)
		}
		if _, execErr := db.Exec(string(data)); execErr != nil {
			t.Fatalf("apply migration %s: %v", name, execErr)
		}
	}
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN phone TEXT`); err != nil {
		t.Fatalf("seed partial phone migration: %v", err)
	}

	if err := RunMigrations(db, migrationsDir); err != nil {
		t.Fatalf("repair partial migration: %v", err)
	}
	var smsTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'sms_verifications'`).Scan(&smsTables); err != nil {
		t.Fatalf("inspect sms table: %v", err)
	}
	if smsTables != 1 {
		t.Fatalf("sms_verifications table count=%d, want 1", smsTables)
	}
}

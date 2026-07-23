package repository

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRoleListDoesNotDeadlockWithSingleDatabaseConnection(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "app.db"), 1)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	if err := RunMigrations(db, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, listErr := NewRoleRepo(db).List()
		done <- listErr
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("list roles: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("role list deadlocked while loading permissions with max_open_conns=1")
	}
}

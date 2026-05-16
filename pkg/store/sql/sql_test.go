package sql

import (
	stdsql "database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDropsLegacyApprovalsTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "keel.db")

	db, err := stdsql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %s", err)
	}
	if _, err := db.Exec("CREATE TABLE approvals (id text)"); err != nil {
		t.Fatalf("failed to create legacy approvals table: %s", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close sqlite db: %s", err)
	}

	store, err := New(Opts{DatabaseType: "sqlite3", URI: dbPath})
	if err != nil {
		t.Fatalf("failed to initialize store: %s", err)
	}
	defer store.Close()

	db, err = stdsql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to reopen sqlite db: %s", err)
	}
	defer db.Close()

	var table string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='approvals'").Scan(&table)
	if err != stdsql.ErrNoRows {
		t.Fatalf("expected approvals table to be dropped, got table=%q err=%v", table, err)
	}

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&table)
	if err != nil {
		t.Fatalf("expected audit_logs table to exist: %s", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite db file to remain: %s", err)
	}
}

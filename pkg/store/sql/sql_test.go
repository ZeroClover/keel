package sql

import (
	stdsql "database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/keel-hq/keel/types"
)

func TestNewInitializesFreshSQLiteStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "keel.db")

	store, err := New(Opts{DatabaseType: "sqlite3", URI: dbPath})
	if err != nil {
		t.Fatalf("failed to initialize store: %s", err)
	}
	defer store.Close()

	db, err := stdsql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %s", err)
	}
	defer db.Close()

	var table string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&table)
	if err != nil {
		t.Fatalf("expected audit_logs table to exist: %s", err)
	}
}

func TestNewDropsLegacyApprovalsTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "keel.db")

	db, err := stdsql.Open("sqlite", dbPath)
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

	db, err = stdsql.Open("sqlite", dbPath)
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

func TestNewReadsExistingAuditLogs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "keel.db")

	store, err := New(Opts{DatabaseType: "sqlite3", URI: dbPath})
	if err != nil {
		t.Fatalf("failed to initialize store: %s", err)
	}

	_, err = store.CreateAuditLog(&types.AuditLog{
		Username:     "user-1",
		Action:       types.AuditActionCreated,
		ResourceKind: types.AuditResourceKindWebhook,
		Identifier:   "webhook-1",
		Message:      "created webhook",
		Metadata:     types.JSONB{"source": "test"},
	})
	if err != nil {
		t.Fatalf("failed to create audit log: %s", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close sqlite store: %s", err)
	}

	store, err = New(Opts{DatabaseType: "sqlite3", URI: dbPath})
	if err != nil {
		t.Fatalf("failed to reopen store: %s", err)
	}
	defer store.Close()

	logs, err := store.GetAuditLogs(&types.AuditLogQuery{ResourceKindFilter: []string{"*"}})
	if err != nil {
		t.Fatalf("failed to read audit logs: %s", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}
	if logs[0].Identifier != "webhook-1" {
		t.Fatalf("unexpected audit log identifier: %q", logs[0].Identifier)
	}
}

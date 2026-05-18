## ADDED Requirements

### Requirement: SQLite Store Runs Without CGO
The SQLite store SHALL work when the Keel binary is compiled with `CGO_ENABLED=0`. `SQLStore.New(Opts{DatabaseType: "sqlite3", URI: <path>})` MUST continue to create new Keel SQLite databases and open, migrate, read, and write existing `keel.db` files without requiring manual export/import migration.

#### Scenario: Fresh SQLite store initializes with CGO disabled
- **WHEN** tests run with `CGO_ENABLED=0` and initialize `SQLStore.New(Opts{DatabaseType: "sqlite3", URI: <empty-db-path>})`
- **THEN** store initialization MUST succeed
- **AND** `SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'` MUST find the `audit_logs` table

#### Scenario: Legacy approvals table cleanup works with CGO disabled
- **GIVEN** an existing SQLite database file contains an `approvals` table
- **WHEN** Keel starts with a `CGO_ENABLED=0` binary
- **THEN** `SQLStore.New` MUST drop the `approvals` table before returning
- **AND** the database file MUST remain readable through the SQLite store

#### Scenario: Existing audit logs remain readable after driver replacement
- **GIVEN** an existing Keel SQLite database contains rows in `audit_logs`
- **WHEN** Keel starts with the pure Go SQLite driver
- **THEN** the existing audit log rows MUST remain queryable through the store

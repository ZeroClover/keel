# persistence Specification

## Purpose
TBD - created by archiving change remove-approvals-system. Update Purpose after archive.
## Requirements
### Requirement: SQLite Store Schema

Keel SHALL persist state in a SQLite database whose schema after migration contains exactly the `audit_logs` table (plus internal indexes). The `approvals` table MUST NOT be present.

#### Scenario: Fresh installation creates only audit_logs

- **WHEN** Keel starts against an empty SQLite database for the first time
- **THEN** `SELECT name FROM sqlite_master WHERE type='table'` MUST include `audit_logs`
- **AND** the result MUST NOT include `approvals`

### Requirement: Legacy `approvals` Table Is Dropped On Startup

`SQLStore.New(opts Opts)` SHALL execute `DROP TABLE IF EXISTS approvals` before running `AutoMigrate`. The operation MUST be idempotent (subsequent restarts are no-ops).

#### Scenario: Upgrading from previous Keel version with existing approvals data

- **GIVEN** a SQLite database containing a populated `approvals` table from a prior Keel release
- **WHEN** Keel starts using the new version
- **THEN** the `approvals` table MUST be dropped before `audit_logs` migration runs
- **AND** all rows previously present in `approvals` MUST be unrecoverable from within Keel
- **AND** the database file size MAY be reclaimed by SQLite's auto-vacuum or remain unchanged

#### Scenario: Drop failure aborts store initialization

- **WHEN** `db.Exec("DROP TABLE IF EXISTS approvals")` returns any error
- **THEN** `SQLStore.New` MUST return a non-nil error containing the underlying cause
- **AND** Keel MUST NOT continue with an initialized store while the legacy `approvals` table remains present

### Requirement: Store Interface Surface

The `store.Store` interface SHALL NOT contain any approval-related methods. The interface methods MUST be limited to: `CreateAuditLog`, `GetAuditLogs`, `AuditLogsCount`, `AuditStatistics`, `OK`, `Close`.

#### Scenario: Removed methods absent from interface

- **WHEN** a developer inspects the `store.Store` interface definition
- **THEN** the methods `CreateApproval`, `UpdateApproval`, `GetApproval`, `ListApprovals`, `DeleteApproval` MUST NOT be declared

### Requirement: Audit Log Action Vocabulary

`types/audit.go` SHALL NOT export any constant whose name begins with `AuditActionApproval` or whose value relates to approval lifecycle. The `AuditResourceKindApproval` constant MUST NOT be defined. The `AuditLogStats` struct MUST NOT contain `Approved` or `Rejected` int fields.

#### Scenario: Historical approval rows in audit_logs are tolerated

- **GIVEN** the `audit_logs` table contains historical rows with `resource_kind='approval'` from a prior version
- **WHEN** Keel queries audit logs via `/v1/audit`
- **THEN** the rows MUST be returned as-is (raw resource_kind value preserved)
- **AND** no aggregated `Approved`/`Rejected` count MUST appear in the stats response

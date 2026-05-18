## Context

Keel's runtime database is SQLite by default. `pkg/store/sql/sql.go` currently imports `github.com/jinzhu/gorm/dialects/sqlite`, whose only role is to blank-import `github.com/mattn/go-sqlite3`. With `CGO_ENABLED=0`, the current SQLite tests fail because `go-sqlite3` compiles a stub that cannot open the database.

The repository already has build paths that assume a static Linux binary, but the primary Dockerfile now installs Alpine C build dependencies and sets `CGO_ENABLED=1` to support SQLite. The Makefile also has ARM release logic that forces `CGO_ENABLED=1`, and its global linker flags assume an external linker.

GORM v1 separates the ORM dialect from the `database/sql` driver when `gorm.Open` receives two string arguments after the dialect. This lets Keel keep GORM's built-in `sqlite3` dialect while opening the pure Go driver's registered `sqlite` driver name.

## Goals / Non-Goals

**Goals:**
- Make the Keel binary build and test with `CGO_ENABLED=0`.
- Preserve the existing SQLite database file path, schema, and migration behavior.
- Remove `github.com/mattn/go-sqlite3` from Keel's active build/import graph.
- Remove C compiler and libc development package requirements from release/container builds.

**Non-Goals:**
- Do not migrate away from SQLite or GORM v1 as part of this change.
- Do not change the persisted schema beyond existing migrations.
- Do not add a new user-facing database configuration surface.
- Do not optimize SQLite performance beyond preserving current behavior.

## Decisions

1. Use `modernc.org/sqlite v1.36.3` behind the existing GORM v1 SQLite dialect.

   GORM v1 already registers its `sqlite3` dialect inside the `gorm` package. The CGO dependency comes from the optional wrapper import that registers the `sqlite3` database/sql driver. The implementation should remove that wrapper import, blank-import `modernc.org/sqlite`, and open SQLite with:

   ```go
   gorm.Open("sqlite3", "sqlite", uri)
   ```

   The first string keeps GORM's SQLite dialect. The second string selects the `database/sql` driver name registered by `modernc.org/sqlite`. Do not alias-register the modernc driver as `sqlite3` and do not add a custom GORM dialect for this change.

   This combination removes CGO at the driver boundary while preserving the existing GORM v1 models, migrations, and store API.

2. Keep `DatabaseType: "sqlite3"` as the store's SQLite dialect selector.

   Existing code and tests pass `sqlite3` into `sql.New`, and `sqlite3` is the GORM v1 dialect name. Renaming it to `sqlite` would not be a direct simplification because GORM v1 would no longer select its SQLite-specific dialect. The store should branch internally only for SQLite: use dialect `sqlite3` with driver `sqlite`, while non-SQLite values keep the existing `gorm.Open(opts.DatabaseType, opts.URI)` behavior.

3. Pin `modernc.org/sqlite v1.36.3`.

   The repository declares `go 1.23`. Current `modernc.org/sqlite` latest versions require newer Go releases, while `v1.36.3` declares `go 1.23.0`. This change should not upgrade the repository Go version.

4. Make CGO-disabled builds the default release path.

   Dockerfile and release-oriented Makefile targets should set `CGO_ENABLED=0` and stop requiring build-base, musl headers, cross C compilers, or external linker mode for SQLite. Release linker flags should keep version metadata and size trimming, but remove `-linkmode external` and `-extldflags -static` so CGO-disabled builds use Go's internal linker. Because `LDFLAGS` is shared, `make install` and `make install-debug` will also stop attempting external static linking.

5. Verify the Keel build/import graph, not dependency modules' own tests.

   `go mod why` includes tests for reachable dependency modules by default, so GORM v1's own tests can still explain a path to `github.com/mattn/go-sqlite3` through `github.com/jinzhu/gorm/dialects/sqlite` even after Keel stops importing that wrapper. This change should verify that Keel source, the main binary build graph, and the targeted package test graphs do not import or compile the CGO SQLite driver. Do not fork or vendor GORM solely to remove its upstream test-only `go-sqlite3` reference.

## Risks / Trade-offs

- Pure Go SQLite behavior can differ from `go-sqlite3` -> verify the existing SQLite store tests under `CGO_ENABLED=0`, including fresh database creation, legacy `approvals` cleanup, and existing audit-log reads.
- Pure Go SQLite adds a larger Go dependency set and can increase binary size -> accept the dependency growth as the trade-off for removing the C toolchain and CGO runtime requirement.

## Migration Plan

Implementation order is tracked in `tasks.md`. This change has no user-facing data migration step: existing `keel.db` files must continue to open through the store, and the existing startup migrations remain responsible for schema updates.

## 1. Dependency and Store Changes

- [x] 1.1 Add `modernc.org/sqlite v1.36.3` and remove `github.com/mattn/go-sqlite3` from Keel's build/import graph.
- [x] 1.2 Replace the `github.com/jinzhu/gorm/dialects/sqlite` import in `pkg/store/sql` with a blank import of `modernc.org/sqlite`.
- [x] 1.3 In `pkg/store/sql/sql.go` `connect()`, open `DatabaseType == "sqlite3"` with GORM dialect `sqlite3` and database/sql driver `sqlite`; keep other database types on the existing `gorm.Open(opts.DatabaseType, opts.URI)` path.
- [x] 1.4 In `pkg/store/sql/sql_test.go`, change direct `database/sql` opens from driver `sqlite3` to driver `sqlite`; rely on the package-level `modernc.org/sqlite` import in `sql.go`.
- [x] 1.5 Remove the stale `github.com/jinzhu/gorm/dialects/sqlite` blank import from `pkg/http/native_webhook_trigger_test.go`; tests that call `sql.New` need no direct SQLite driver import.
- [x] 1.6 Run `go mod tidy` and confirm `go.mod` removes Keel's `github.com/mattn/go-sqlite3` requirement while adding the pure Go SQLite dependency set.

## 2. Build Configuration

- [x] 2.1 Update the primary Dockerfile Go build stage to build Keel with `CGO_ENABLED=0`.
- [x] 2.2 Remove Dockerfile CGO SQLite build inputs from the primary Go stage: `build-base`, `musl-dev`, `binutils-gold`, `-linkmode external`, and `-extldflags '-static'`.
- [x] 2.3 Remove `-linkmode external` and `-extldflags -static` from shared Makefile `LDFLAGS` used by `build`, `install`, `install-debug`, and `build-binaries`, while keeping version metadata flags.
- [x] 2.4 Update `build-arm` to remove `CC=arm-linux-gnueabihf-gcc`, set `CGO_ENABLED=0`, preserve `GOARCH=arm GOOS=linux`, and keep `ARMFLAGS` limited to version-metadata `-X` flags with no `-linkmode external`, `-extldflags`, or build flags inside `-ldflags`.
- [x] 2.5 Update `build-binaries` to remove `CC=arm-linux-gnueabi-gcc`, set `CGO_ENABLED=0`, and use pure-Go-compatible linker flags.

## 3. Persistence Compatibility

- [x] 3.1 Implement or update tests for the persistence scenarios: fresh SQLite initialization, legacy `approvals` cleanup, and existing audit-log reads under `CGO_ENABLED=0`.

## 4. Verification

- [x] 4.1 Run `CGO_ENABLED=0 go test ./pkg/store/sql ./pkg/http ./trigger/poll ./trigger/pubsub`.
- [x] 4.2 Run `CGO_ENABLED=0 GOOS=linux go build ./cmd/keel`.
- [x] 4.3 Run `go mod tidy`, verify Keel's build/test dependency lists do not include `github.com/mattn/go-sqlite3` or `github.com/jinzhu/gorm/dialects/sqlite`, and run `go mod why -m -vendor github.com/mattn/go-sqlite3`; confirm tidy leaves no unplanned module-file changes and the main module does not need to vendor `github.com/mattn/go-sqlite3`.
- [x] 4.4 Run `docker build -t keel-cgo-free-test .` and confirm the primary Dockerfile builds without `build-base`, `musl-dev`, or `binutils-gold`.
- [x] 4.5 Run `docker run --rm --entrypoint /bin/keel keel-cgo-free-test --help` and confirm the image's Keel binary starts successfully.
- [x] 4.6 Inspect `/bin/keel` in `keel-cgo-free-test` with `ldd` or an equivalent tool and confirm it does not require dynamic C library dependencies.
- [x] 4.7 Run `openspec validate remove-cgo-dependency --strict`.

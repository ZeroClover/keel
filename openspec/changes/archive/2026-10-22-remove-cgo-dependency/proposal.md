## Why

Keel currently requires CGO because the SQLite store imports GORM's SQLite wrapper, which blank-imports `github.com/mattn/go-sqlite3`. This forces container and ARM builds to install a C toolchain and set `CGO_ENABLED=1`, even though Keel's runtime should be buildable as a pure Go binary.

## What Changes

- Replace the CGO-backed SQLite driver with `modernc.org/sqlite v1.36.3` while keeping the existing SQLite database file and GORM v1 persistence semantics.
- Remove `github.com/mattn/go-sqlite3` from Keel's build/import graph and stop importing `github.com/jinzhu/gorm/dialects/sqlite`.
- Build the Keel binary with `CGO_ENABLED=0` in the Dockerfile and release-oriented Makefile targets.
- Remove Alpine C build dependencies that exist only for SQLite CGO compilation.
- Update SQLite-backed tests so they run under `CGO_ENABLED=0`.

## Capabilities

### New Capabilities
- `cgo-free-builds`: Keel builds and verifies the main binary without CGO, without requiring a C compiler or libc development headers.

### Modified Capabilities
- `persistence`: SQLite persistence must keep the existing schema and upgrade behavior while running with a pure Go SQLite driver under `CGO_ENABLED=0`.

## Impact

- Affected code: `pkg/store/sql`, SQLite-using tests, `Dockerfile`, and release build targets in `Makefile`.
- Affected dependencies: remove `github.com/mattn/go-sqlite3` from Keel's `go.mod` requirements and build/import graph; add `modernc.org/sqlite v1.36.3`, which declares compatibility with Go 1.23.
- Affected systems: container builds, Linux ARM builds, `build-binaries`, local install/debug Makefile workflows that inherit `LDFLAGS`, local test workflows, and runtime database initialization against `/data/keel.db`.

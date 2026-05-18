## ADDED Requirements

### Requirement: CGO-Free Go Build
Keel SHALL build its main binary with `CGO_ENABLED=0` for direct Linux `go build` commands and Makefile targets `build`, `build-arm`, and `build-binaries`. These build paths MUST NOT require a C compiler, libc development headers, or external linker mode for SQLite support.

#### Scenario: Main binary builds with CGO disabled
- **WHEN** a developer runs `CGO_ENABLED=0 GOOS=linux go build ./cmd/keel`
- **THEN** the build MUST succeed

#### Scenario: ARM release build does not force CGO
- **WHEN** a developer invokes `make build-arm`
- **THEN** the target MUST build with `CGO_ENABLED=0 GOOS=linux GOARCH=arm`
- **AND** the target MUST NOT set `CC=arm-linux-gnueabihf-gcc`

#### Scenario: Gox release build does not force CGO
- **WHEN** a developer invokes `make build-binaries`
- **THEN** the target MUST build with `CGO_ENABLED=0`
- **AND** the target MUST NOT set `CC=arm-linux-gnueabi-gcc`

### Requirement: CGO SQLite Dependency Removed From Keel Builds
Keel source and build graphs SHALL NOT import or compile `github.com/mattn/go-sqlite3`, and repository code SHALL NOT import `github.com/jinzhu/gorm/dialects/sqlite` or any package solely to register the CGO-backed SQLite driver.

#### Scenario: Keel build graph excludes go-sqlite3
- **WHEN** a developer lists dependencies for the main Keel binary and targeted SQLite-using package tests
- **THEN** the dependency list MUST NOT include `github.com/mattn/go-sqlite3`
- **AND** the dependency list MUST NOT include `github.com/jinzhu/gorm/dialects/sqlite`

#### Scenario: Vendor graph excludes go-sqlite3
- **WHEN** a developer runs `go mod why -m -vendor github.com/mattn/go-sqlite3`
- **THEN** the command MUST report that the main module does not need to vendor `github.com/mattn/go-sqlite3`

#### Scenario: Source tree has no CGO SQLite wrapper import
- **WHEN** a developer searches Go source files for `github.com/jinzhu/gorm/dialects/sqlite`
- **THEN** no runtime or test file imports that package

### Requirement: Container Build Avoids C Toolchain
The primary Dockerfile SHALL build the Keel binary with CGO disabled and SHALL NOT install C build packages only needed for `go-sqlite3` compilation.

#### Scenario: Dockerfile builds without SQLite C dependencies
- **WHEN** the primary Dockerfile builds the Go stage
- **THEN** the Go build command MUST set `CGO_ENABLED=0`
- **AND** the Go build stage MUST NOT install `build-base`, `musl-dev`, or `binutils-gold` for SQLite compilation

#### Scenario: Container binary starts without C runtime dependency
- **WHEN** the primary Dockerfile image is built
- **THEN** `/bin/keel --help` MUST execute successfully in the resulting image
- **AND** `/bin/keel` MUST NOT require dynamic C library dependencies

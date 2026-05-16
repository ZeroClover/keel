# registry-client Specification

## Purpose
TBD - created by archiving change refactor-watcher-and-force-policy. Update Purpose after archive.
## Requirements
### Requirement: Client Interface

The `registry.Client` interface SHALL declare exactly three methods:

```go
type Client interface {
    Get(opts Opts) (*Repository, error)
    Digest(opts Opts) (string, error)
    GetCreatedTime(opts Opts) (time.Time, error)
}
```

The implementation `DefaultClient` SHALL forward `GetCreatedTime` to the underlying `docker.Registry`.

#### Scenario: Interface contains GetCreatedTime

- **WHEN** a developer searches `registry.go` for method signatures on `Client`
- **THEN** `GetCreatedTime(opts Opts) (time.Time, error)` MUST be declared
- **AND** all three methods MUST exist in this order: `Get`, `Digest`, `GetCreatedTime`

### Requirement: GetCreatedTime Resolution Algorithm

`docker.Registry.GetCreatedTime(repository, tag string)` SHALL implement:

1. Issue `GET /v2/<repository>/manifests/<tag>` with `Accept` header including OCI image manifest, OCI image index, and Docker manifest v2 media types.
2. Inspect the response `Content-Type`:
   - If a manifest list / image index, take `manifests[0]` and recurse from step 1 using its digest.
   - If a single-image manifest, read `.config.digest`.
3. Issue `GET /v2/<repository>/blobs/<config-digest>`.
4. Parse the response body as JSON; read `.created` as RFC3339.
5. Return the parsed `time.Time` and `nil`.

If any of the steps fails (network, HTTP non-2xx, JSON parse, missing `.created`), the method SHALL return `time.Time{}` (zero value) and a non-nil error describing the cause.

#### Scenario: Successful fetch of OCI image manifest

- **GIVEN** a registry returns a v2 manifest with `config.digest=sha256:abc` and a config blob containing `"created":"2024-05-01T10:00:00Z"`
- **WHEN** `GetCreatedTime` is invoked for that tag
- **THEN** the method MUST return `time.Parse(time.RFC3339, "2024-05-01T10:00:00Z")` and `nil` error

#### Scenario: Image index recursion

- **GIVEN** a registry returns an OCI image index for `tag=latest`, whose `manifests[0].digest=sha256:xyz`
- **AND** the v2 manifest at `sha256:xyz` references `config.digest=sha256:def` with `.created=2025-01-01T00:00:00Z`
- **WHEN** `GetCreatedTime` is invoked
- **THEN** the method MUST follow the index → manifest → config chain
- **AND** return the time `2025-01-01T00:00:00Z`

#### Scenario: Missing `.created` field

- **GIVEN** a config blob containing valid JSON but no `created` key
- **WHEN** `GetCreatedTime` is invoked
- **THEN** the method MUST return `time.Time{}` and an error mentioning `created`

#### Scenario: Network failure

- **WHEN** the underlying HTTP request returns a connection error
- **THEN** the method MUST return `time.Time{}` and an error wrapping the network error

### Requirement: No Public Created-Time Cache API

This Change SHALL NOT add a public registry cache API, Helm value, environment variable, or Prometheus metric for created-time caching. Any memoization requirement belongs to the poll watcher implementation, not the registry client contract.

#### Scenario: Registry package exposes no cache controls

- **WHEN** a developer inspects `registry/` and Helm chart values after this Change
- **THEN** no exported registry cache type or function for created time MUST exist
- **AND** no Helm value, environment variable, or Prometheus metric for created-time cache tuning MUST be added

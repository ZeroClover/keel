## ADDED Requirements

### Requirement: Annotation Schema

Keel SHALL recognise the following annotation keys on Kubernetes resources and Helm chart values:

| Key | Required | Purpose |
|---|---|---|
| `keel.sh/policy` | yes (to enable updates) | Mutually-exclusive policy selector |
| `keel.sh/filterTags` | no | RE2 regular expression filtering candidate tags |
| `keel.sh/extract` | no | Replacement template using capture groups from `filterTags` |

`keel.sh/policy` MUST accept exactly one of the following values:

- `semver:<constraint>` where `<constraint>` is parsable by `github.com/Masterminds/semver/v3`.
- `alphabetical` or `alphabetical:asc` or `alphabetical:desc`.
- `numerical` or `numerical:asc` or `numerical:desc`.
- `force`.
- `never` (equivalent to no policy).

The default order for `alphabetical` and `numerical` MUST be `asc` when no suffix is provided, matching Flux `NewAlphabetical("")` and `NewNumerical("")` behaviour.

#### Scenario: Valid semver constraint with pre-release

- **WHEN** a Deployment has `keel.sh/policy: "semver:>=1.0.0-0"`
- **AND** tags `[1.0.0-rc.1, 1.0.0-rc.2, 0.9.0]` are present in the registry
- **THEN** `Policy.Latest(...)` MUST return `1.0.0-rc.2`

#### Scenario: Semver constraint excluding pre-release

- **WHEN** a Deployment has `keel.sh/policy: "semver:>=1.0.0"`
- **AND** tags `[1.0.0, 1.0.0-rc.2, 1.1.0-beta.1]` are present
- **THEN** `Policy.Latest(...)` MUST return `1.0.0`

#### Scenario: Alphabetical descending

- **WHEN** a Deployment has `keel.sh/policy: "alphabetical:desc"` and no `filterTags`
- **AND** tags `[build-001, build-002, build-009, build-010]` are present
- **THEN** `Policy.Latest(...)` MUST return `build-010`

#### Scenario: Numerical with filter and extract

- **WHEN** a Deployment has
  - `keel.sh/policy: "numerical:desc"`
  - `keel.sh/filterTags: "^main-[a-f0-9]+-(?P<ts>[0-9]+)$"`
  - `keel.sh/extract: "$ts"`
- **AND** tags `[main-abc-100, main-def-200, prod-aaa-500]` are present
- **THEN** `Filter.Apply` MUST keep `main-abc-100` and `main-def-200`
- **AND** `Filter.Items()` MUST contain `"100"` and `"200"`
- **AND** `Policy.Latest(...)` MUST return `"200"`
- **AND** `Filter.GetOriginalTag("200")` MUST return `main-def-200`

#### Scenario: Numerical fail-fast on non-numeric value

- **WHEN** policy is `"numerical:desc"` and a candidate list `["100", "abc", "200"]` is passed to `Latest`
- **THEN** `Latest` MUST return an error and MUST NOT silently fall back to string ordering

### Requirement: Policy Interfaces

The Go interfaces `internal/policy.Policy` and `types.Policy` SHALL declare exactly the following methods:

```go
type Policy interface {
    Name() string
    Type() types.PolicyType
    Latest(candidates []string) (string, error)
}
```

Neither interface MUST declare `ShouldUpdate`, `Filter`, or `KeepTag`. The duplicate `types.Policy` interface exists because `types.TrackedImage` lives in the `types` package and MUST NOT import `internal/policy`.

#### Scenario: Compile-time check

- **WHEN** a developer attempts to call `policy.ShouldUpdate(...)` from any package
- **THEN** the Go compiler MUST emit "undefined: ShouldUpdate" or equivalent

#### Scenario: TrackedImage policy interface is updated

- **WHEN** a developer inspects `types/tracked_images.go`
- **THEN** `type Policy interface` MUST declare `Latest(candidates []string) (string, error)`
- **AND** it MUST NOT declare `ShouldUpdate`, `Filter`, or `KeepTag`

### Requirement: Filter Interfaces

The Go interfaces `internal/policy.Filter` and `types.Filter` SHALL declare exactly:

```go
type Filter interface {
    Apply(tags []string)
    Items() []string
    GetOriginalTag(key string) string
}
```

A concrete implementation `RegexFilter` MUST exist that compiles `keel.sh/filterTags` as RE2. When `keel.sh/extract` is non-empty, it MUST use that value as the replacement template. When `extract` is empty, it MUST preserve the original matched tag as the key, matching Flux `RegexFilter.Apply` behaviour.

`types.TrackedImage` SHALL include `Filter types.Filter` so the poll watcher can carry the parsed filter without importing `internal/policy` from the `types` package.

#### Scenario: Filter is nil when no annotation present

- **WHEN** a Deployment has `keel.sh/policy: "semver:>=0.0.0"` but no `keel.sh/filterTags`
- **THEN** `GetPolicyFromLabelsOrAnnotations(...)` MUST return `(SemVerPolicy, nil, nil)`
- **AND** Watcher MUST pass the full tag list to `Policy.Latest` unmodified

#### Scenario: Filter excludes non-matching tags

- **WHEN** `filterTags="^v(?P<v>\\d+\\.\\d+\\.\\d+)$"` and `extract="$v"`
- **AND** the tag list is `[v1.0.0, latest, v1.1.0-rc.1, 1.0.0]`
- **THEN** `Filter.Apply` followed by `Filter.Items()` MUST return exactly `["1.0.0", "1.1.0-rc.1"]`
- **AND** `Items()` MUST follow the input match insertion order so tests are stable

### Requirement: Legacy Annotation Handling

Legacy annotations, legacy policy keys, and legacy policy values SHALL be ineffective at runtime and MUST fail policy parsing:

- Policy values `major`, `minor`, `patch`, `all`, `glob:*`, `regexp:*` MUST return `(nil, nil, errUnsupportedPolicy)` or an equivalent non-nil error.
- Legacy key `keel.observer/policy` MUST return `(nil, nil, errUnsupportedPolicy)` or an equivalent non-nil error.
- Annotations `keel.sh/matchTag`, `keel.sh/match-tag`, `keel.sh/matchPreRelease` MUST return `(nil, nil, errUnsupportedPolicy)` or an equivalent non-nil error.
- Watcher and provider callers MUST catch the error, log an `ERROR`-level migration message, and skip that resource; Keel MUST NOT panic or abort process startup.

#### Scenario: Legacy `minor` policy returns an error

- **WHEN** a Deployment has `keel.sh/policy: "minor"`
- **THEN** `GetPolicyFromLabelsOrAnnotations(...)` MUST return a nil Policy, nil Filter, and a non-nil error
- **AND** an `ERROR` log entry MUST reference the legacy policy value `minor`
- **AND** the Deployment MUST be skipped for this scan
- **AND** Keel MUST NOT crash or block process startup

#### Scenario: Legacy `keel.observer/policy` key returns an error

- **WHEN** a Deployment has `keel.observer/policy: "all"` and no `keel.sh/policy`
- **THEN** `GetPolicyFromLabelsOrAnnotations(...)` MUST return a nil Policy, nil Filter, and a non-nil error
- **AND** an `ERROR` log entry MUST reference the legacy key `keel.observer/policy`

#### Scenario: matchTag annotation returns an error

- **WHEN** a Deployment has `keel.sh/policy: "force"` and `keel.sh/matchTag: "true"`
- **THEN** `GetPolicyFromLabelsOrAnnotations(...)` MUST return a nil Policy, nil Filter, and a non-nil error
- **AND** an `ERROR` log MUST reference the legacy matchTag annotation

### Requirement: No Policy Default

When no `keel.sh/policy` annotation is present on a resource, or when `keel.sh/policy: "never"` is present, `GetPolicyFromLabelsOrAnnotations` SHALL return `(nil, nil, nil)`. The Watcher and Provider MUST treat a nil Policy as "skip updates for this image" without logging an error.

#### Scenario: Resource without policy annotation is not updated

- **WHEN** a Deployment has no `keel.sh/policy` annotation
- **AND** a new tag appears in the registry
- **THEN** Keel MUST NOT patch the deployment
- **AND** Keel MUST NOT log an error (silent skip is correct)

#### Scenario: Explicit never policy is not updated

- **WHEN** a Deployment has `keel.sh/policy: "never"`
- **THEN** `GetPolicyFromLabelsOrAnnotations(...)` MUST return a nil Policy, nil Filter, and nil error
- **AND** Keel MUST NOT patch the deployment

### Requirement: SemVer Implementation Uses v3

The Go module MUST depend on `github.com/Masterminds/semver/v3` (not v1). All internal usage of semver parsing and constraints MUST import the `/v3` path. The deprecated direct dependency `github.com/Masterminds/semver` (without `/v3`) MUST NOT appear in `go.mod`.

#### Scenario: go.mod uses v3 only

- **WHEN** a developer runs `grep "Masterminds/semver" go.mod`
- **THEN** only `github.com/Masterminds/semver/v3` MUST appear

### Requirement: Force Policy Trivial Selector

The `force` policy implementation SHALL select the first element of the candidates list as the latest. Sorting of the candidate list (e.g., by image created time) is the caller's responsibility; the Force policy MUST NOT itself fetch metadata.

#### Scenario: Empty candidate list

- **WHEN** `force.Latest([])` is called
- **THEN** the policy MUST return an error and an empty string

#### Scenario: Single candidate

- **WHEN** `force.Latest(["any-tag"])` is called
- **THEN** the policy MUST return `("any-tag", nil)`

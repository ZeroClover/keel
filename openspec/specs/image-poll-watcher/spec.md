# image-poll-watcher Specification

## Purpose
TBD - created by archiving change refactor-watcher-and-force-policy. Update Purpose after archive.
## Requirements
### Requirement: Single Watcher Path

Poll trigger SHALL run exactly one watcher job type per tracked image: `WatchRepositoryTagsJob`. The legacy `WatchTagJob` (digest-based single-tag watcher) MUST NOT exist in the codebase.

#### Scenario: No WatchTagJob in source tree

- **WHEN** a developer searches the repository
- **THEN** no file named `single_tag_watcher.go` MUST exist
- **AND** no Go type named `WatchTagJob` MUST be declared

### Requirement: ImageIdentifier Drops KeepTag

`trigger/poll.getImageIdentifier(ref *image.Reference) string` SHALL accept exactly one parameter (the image reference) and return `<registry>/<short-name>` without any tag suffix.

#### Scenario: Identifier shape

- **WHEN** `getImageIdentifier` is invoked with `ref` pointing to `docker.io/library/nginx:1.25`
- **THEN** the result MUST be exactly `docker.io/library/nginx`
- **AND** the result MUST NOT contain `:1.25`

### Requirement: computeEvents Uses Latest + Filter

`WatchRepositoryTagsJob.computeEvents(tags []string)` SHALL execute the following sequence for each tracked image that shares the watcher's image identifier:

1. If a `Filter` is configured: call `filter.Apply(tags)`; let `candidates = filter.Items()` for non-Force policies.
2. Otherwise: let `candidates = tags`.
3. If the tracked image's policy is `Force`: build the candidate set from original tag names (not extracted keys), then sort those original tags by image-created-time descending using `registry.GetCreatedTime`. Tags whose created time cannot be fetched MUST be sorted to the tail of the list and MUST NOT be cached as zero-time results.
4. Call `latestKey, err = policy.Latest(candidates)`. On error, skip the tracked image (log at debug level).
5. Let `latestTag = filter.GetOriginalTag(latestKey)` if a filter is configured on a non-Force policy, else `latestTag = latestKey`.
6. If `latestTag != trackedImage.Image.Tag()`, emit a `types.Event` with the new tag.

#### Scenario: Force policy with created-time sort

- **GIVEN** policy `force`, no filter, candidate tags `["tag-A", "tag-B", "tag-C"]`
- **AND** `GetCreatedTime` returns: `tag-A=2024-01-01`, `tag-B=2024-03-01`, `tag-C=2024-02-01`
- **WHEN** `computeEvents` runs
- **THEN** the sorted candidate list MUST be `["tag-B", "tag-C", "tag-A"]`
- **AND** `policy.Latest` MUST be called with `["tag-B", "tag-C", "tag-A"]`
- **AND** the emitted event MUST carry `tag-B`

#### Scenario: Force policy with missing created-time

- **GIVEN** policy `force`, candidates `["a", "b", "c"]`
- **AND** `GetCreatedTime` returns valid time for `a` (2024-01-01) and `b` (2024-02-01) but errors for `c`
- **WHEN** `computeEvents` runs
- **THEN** the sorted candidate list MUST be `["b", "a", "c"]` (c at tail)
- **AND** the failed result for `c` MUST NOT be stored in the created-time cache

#### Scenario: Force policy with equal created-time uses deterministic tie-breaker

- **GIVEN** policy `force`, candidates `["b", "a", "c"]`
- **AND** `GetCreatedTime` returns the same timestamp for `a` and `b`, and a newer timestamp for `c`
- **WHEN** `computeEvents` runs
- **THEN** the sorted candidate list MUST put `c` before `a` and `b`
- **AND** tags with equal created time MUST be ordered by a deterministic tie-breaker independent of registry tag-list order

#### Scenario: Force policy with filter and extract uses original tags

- **GIVEN** policy `force`, `filterTags="^release-(?P<n>[0-9]+)$"`, `extract="$n"`, and tags `["release-1", "release-2", "dev-3"]`
- **AND** `GetCreatedTime` returns `release-1=2024-01-01` and `release-2=2024-02-01`
- **WHEN** `computeEvents` runs
- **THEN** created-time lookup MUST be invoked for original tags `release-1` and `release-2`, not for extracted keys `"1"` or `"2"`
- **AND** the emitted event MUST carry `release-2`

#### Scenario: Non-force policy bypasses created-time fetch

- **GIVEN** policy `semver:>=0.0.0`, candidates `["1.0.0", "1.1.0"]`
- **WHEN** `computeEvents` runs
- **THEN** no call to `GetCreatedTime` MUST occur
- **AND** the resulting event tag MUST be `1.1.0`

### Requirement: Created-Time Memoization

Watcher code SHALL memoise successful `GetCreatedTime` results by manifest digest. The memoization MUST:

- Use `(registry, repository, manifestDigest)` as the key.
- Store only results where `GetCreatedTime` returned `nil` error and a non-zero `time.Time`.
- Treat network errors, HTTP errors, JSON parse errors, and missing `.created` as cache misses on future polls.
- Be private to the watcher implementation.

#### Scenario: Cache hit avoids created-time fetch

- **GIVEN** the cache contains `key="docker.io/library/nginx@sha256:abc" -> t1`
- **WHEN** the watcher checks `cache.Get(key)`
- **THEN** the watcher MUST receive `(t1, true)` without invoking `GetCreatedTime`

#### Scenario: Failed created-time fetch is not cached

- **WHEN** `GetCreatedTime` returns `time.Time{}` and a non-nil error for digest `sha256:bad`
- **THEN** the watcher MUST NOT store `sha256:bad` in the cache
- **AND** a later poll for the same digest MUST attempt `GetCreatedTime` again

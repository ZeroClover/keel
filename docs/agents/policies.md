# Policies, annotations and polling

Verified against commit `9070252f` (2026-10-22). Requirements: `openspec/specs/image-policy/spec.md`, `openspec/specs/image-poll-watcher/spec.md`.

## Policy syntax

Parsed by `policy.GetPolicyFromLabelsOrAnnotations` (`internal/policy/policy.go`). For `keel.sh/policy`, `keel.sh/filterTags` and `keel.sh/extract`, an annotation wins over a label with the same key. An empty value or `never` means "not managed" (nil policy, no error).

| Value | Implementation | Selects |
|---|---|---|
| `semver:<constraint>` | `semver.go`, Masterminds semver v3 | Highest tag that parses as semver and satisfies the constraint; pre-releases only when the constraint allows them (e.g. `>=1.0.0-0`). The `semver:` prefix is case-sensitive; an empty constraint is an error. |
| `force` | `force.go` | The first candidate in the order the registry returned (poll) or the event's tag (webhooks, Pub/Sub). |
| `force:created` | `force.go` | Poll only: candidates sorted by image config `created` time, newest first, before taking the first. Costs one manifest + config fetch per uncached tag. |
| `alphabetical`, `alphabetical:asc`, `alphabetical:desc` | `alphabetical.go` | `asc` (default) picks the lexicographically **lowest** tag, `desc` the highest. |
| `numerical`, `numerical:asc`, `numerical:desc` | `numerical.go` | Same direction rule on integers; any candidate that isn't an integer makes selection fail. |

Rejected with an "unsupported policy" error (`isLegacyPolicyValue`, `isLegacyKey`): the values `all`, `major`, `minor`, `patch`, `glob:*`, `regexp:*`, and the keys `keel.sh/matchTag`, `keel.sh/match-tag`, `keel.sh/matchPreRelease`, and `keel.observer/policy` when no `keel.sh/policy` is present. A resource with an invalid policy is skipped and logged at ERROR.

### Tag filter

`keel.sh/filterTags` is a Go (RE2) regular expression; only matching tags are candidates. `keel.sh/extract` is an `ExpandString` template over the filter's named groups (e.g. filter `^main-(?P<ts>[0-9]+)$`, extract `$ts`); the policy then sorts the extracted keys and the winning key is mapped back to its original tag. If two tags extract to the same key, the later one wins. For `force`, the filter only narrows candidates; extracted keys are not used.

## Annotation and label keys

Key constants live in `types/types.go`.

| Key | Where it is read | Meaning |
|---|---|---|
| `keel.sh/policy` | annotation, then label | Policy (above) |
| `keel.sh/filterTags` | annotation, then label | Candidate tag regex |
| `keel.sh/extract` | annotation, then label | Sort-key template for filtered tags |
| `keel.sh/trigger` | annotation, then label (`util/policies`) | `poll` enables polling; any other value means default (webhooks / Pub/Sub only) |
| `keel.sh/pollSchedule` | annotation only | Cron spec, e.g. `@every 5m`; invalid values fall back to the default schedule |
| `keel.sh/notify` | annotation only | Comma-separated channels; only the Slack sender uses it |
| `keel.sh/releaseNotes` | annotation only | URL appended to the success notification |
| `keel.sh/imagePullSecret` | label or annotation, case-insensitive key | Secret name used for registry credentials, in addition to the pod's `imagePullSecrets` |
| `keel.sh/monitorContainers` | label or annotation, case-insensitive key | Regex on container names to manage; default all |
| `keel.sh/initContainers` | see `docs/agents/known-issues.md` | `"true"` also updates init containers |
| `keel.sh/update-time` | written by Keel on the pod template | Timestamp of the last update; never read |
| `keel.sh/digest` | defined, never read | No effect |

## Helm 3 `keel:` values

The Helm 3 provider reads these from each release's merged values (`provider/helm3/helm3.go`, `KeelChartConfig`). `policy`, `filterTags` and `extract` go through the same parser as the annotations.

```yaml
keel:
  policy: semver:^1
  filterTags: "^main-(?P<ts>[0-9]+)$"
  extract: "$ts"
  trigger: poll              # "default" or "poll"; anything else makes the whole block fail to parse
  pollSchedule: "@every 5m"  # default @every 1m
  notificationChannels: []
  images:
    - repository: image.repository   # dotted paths into the chart values
      tag: image.tag
      digest: image.digest           # optional; set from Event.Repository.Digest (empty for poll)
      releaseNotes: ""
      imagePullSecret: ""
```

Legacy `keel.matchTag` and `keel.matchPreRelease` are mapped to the legacy annotation keys so they are rejected like the annotations.

## Poll watcher (`trigger/poll/`)

- **Discovery.** Every 3 s the manager asks the providers for tracked images and passes the ones with `keel.sh/trigger: poll` to the watcher (`manager.go`, `watcher.go`).
- **Jobs.** There is one cron job per repository (`registry/name`, without tag), shared by all resources that use it. When several resources set different schedules, the last one seen wins. A job is only created if a digest lookup for the current tag succeeds; it then runs once immediately. Images that are no longer tracked are unwatched.
- **Default schedule.** `@every 1m` (`types.KeelPollDefaultSchedule`), overridable with `POLL_DEFAULTSCHEDULE`.
- **Selection** (`WatchRepositoryTagsJob.computeEvents`, `multi_tags_watcher.go`): list all tags, then for each tracked image sharing the repository:
  1. `force`: candidates are the original tags that pass the filter; with `force:created` they are sorted by created time (cached per digest; unknown times sort last and are not cached).
  2. Other policies: apply the filter (if any) and use its keys as candidates.
  3. `policy.Latest(candidates)`; on error skip this image.
  4. Map the key back to the original tag, and emit a `poll` event if it differs from the current tag and wasn't already emitted.
- Poll events carry repository and tag only (no digest), and providers apply them without a policy check; see `docs/agents/architecture.md` → Trust model.

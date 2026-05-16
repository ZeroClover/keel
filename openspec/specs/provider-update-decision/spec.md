# provider-update-decision Specification

## Purpose
TBD - created by archiving change refactor-watcher-and-force-policy. Update Purpose after archive.
## Requirements
### Requirement: External Events Must Satisfy Resource Policy

When a `types.Event` arrives at `Provider.Submit` originating from a webhook trigger (`pkg/http/*_webhook_trigger.go`), pubsub, or any non-poll source, the provider SHALL treat the event's tag as a candidate only. The provider MUST NOT update a deployment or Helm release unless the candidate tag is admitted by the resource's configured Policy and Filter.

The baseline repository check MUST still be:

```
shouldUpdate := containerImageRef.Tag() != eventRepoRef.Tag() &&
                containerImageRef.Repository() == eventRepoRef.Repository()
```

After the baseline check passes, non-poll events MUST call a policy-aware helper equivalent to `policyAllowsExternalTag(policy, filter, currentTag, eventTag)`.

#### Scenario: Webhook event outside semver range is rejected

- **GIVEN** a Deployment runs `nginx:1.25.3` with `keel.sh/policy: "semver:^1.25"`
- **AND** a Docker Hub webhook posts `{repository: "library/nginx", tag: "1.26.0"}`
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST NOT be patched
- **AND** the SemVer constraint `^1.25` MUST reject the event tag

#### Scenario: Webhook event inside semver range updates deployment

- **GIVEN** a Deployment runs `nginx:1.25.3` with `keel.sh/policy: "semver:^1.25"`
- **AND** a Docker Hub webhook posts `{repository: "library/nginx", tag: "1.25.4"}`
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST be patched to `nginx:1.25.4`

#### Scenario: Webhook event that downgrades within range is rejected

- **GIVEN** a Deployment runs `nginx:1.25.3` with `keel.sh/policy: "semver:^1.25"`
- **AND** a Docker Hub webhook posts `{repository: "library/nginx", tag: "1.25.2"}`
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST NOT be patched
- **AND** the provider MUST NOT accept the tag solely because it satisfies the SemVer constraint

#### Scenario: Webhook event rejected by filterTags

- **GIVEN** a Deployment runs `app:main-100` with `keel.sh/policy: "numerical:desc"`
- **AND** the Deployment has `keel.sh/filterTags: "^main-(?P<n>[0-9]+)$"` and `keel.sh/extract: "$n"`
- **AND** a webhook posts `{repository: "library/app", tag: "prod-200"}`
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST NOT be patched
- **AND** the event tag MUST be rejected before `Policy.Latest` is evaluated

#### Scenario: Force webhook event still respects filterTags

- **GIVEN** a Deployment runs `app:release-1` with `keel.sh/policy: "force"` and `keel.sh/filterTags: "^release-[0-9]+$"`
- **AND** a webhook posts `{repository: "library/app", tag: "dev-2"}`
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST NOT be patched

#### Scenario: Webhook event with same tag is a no-op

- **GIVEN** a Deployment runs `nginx:1.25.3`
- **AND** a webhook posts `{repository: "library/nginx", tag: "1.25.3"}`
- **THEN** the provider MUST NOT patch the Deployment
- **AND** no notification MUST be emitted

#### Scenario: Webhook event for unrelated repository is ignored

- **GIVEN** a Deployment runs `nginx:1.25.3`
- **AND** a webhook posts `{repository: "library/redis", tag: "7.0"}`
- **THEN** the provider MUST NOT patch the Deployment

### Requirement: Poll Path Trusts Watcher-Selected Tag

When a `types.Event` originates from `WatchRepositoryTagsJob` (poll trigger), the event's tag has already been selected by the configured policy. The provider MUST NOT re-evaluate the policy.

#### Scenario: Poll event flows through without re-running policy

- **GIVEN** a Deployment with `keel.sh/policy: "semver:^1.25"` runs `nginx:1.25.3`
- **AND** the poll trigger emits `nginx:1.25.4` (already selected by policy.Latest)
- **WHEN** the provider processes the event
- **THEN** the Deployment MUST be patched to `nginx:1.25.4`
- **AND** no second call to `policy.Latest` MUST occur in the provider path

### Requirement: External Policy Admission Helper

The helper used for non-poll events SHALL enforce these rules:

- nil Policy means updates are disabled and MUST reject the event tag.
- Force policy SHALL allow a different tag from the same repository only after `filterTags` admits the event tag when a filter is configured. It MUST NOT fetch image created-time on the external-event path.
- Non-Force policies SHALL evaluate the configured Filter against `[currentTag, eventTag]`; if the event tag is filtered out, reject it. If the current tag is filtered out but the event tag remains, the helper MUST continue with the remaining candidate set.
- Non-Force policies SHALL call `Policy.Latest(candidates)` and allow the update only when the selected original tag equals the event tag.

The Force-policy asymmetry is intentional: poll events sort registry tags by image created-time before selecting, while webhook/pubsub events are explicit external candidates and only need filter admission.

#### Scenario: Latest-based admission blocks downgrade

- **GIVEN** policy `semver:>=1.0.0`
- **AND** current tag `1.2.0`
- **AND** event tag `1.1.9`
- **WHEN** the helper evaluates `[currentTag, eventTag]`
- **THEN** `Policy.Latest(...)` MUST select `1.2.0`
- **AND** the helper MUST reject the event tag

#### Scenario: Current tag filtered out does not block admitted event tag

- **GIVEN** policy `numerical:desc`
- **AND** filterTags `^main-(?P<n>[0-9]+)$` with extract `$n`
- **AND** current tag `legacy-100`
- **AND** event tag `main-200`
- **WHEN** the helper evaluates `[currentTag, eventTag]`
- **THEN** the filtered candidates MUST contain only extracted key `"200"`
- **AND** `Policy.Latest(...)` MUST select `"200"`
- **AND** the helper MUST allow the event tag

#### Scenario: Force external admission does not fetch created-time

- **GIVEN** policy `force`
- **AND** current tag `old`
- **AND** event tag `manual`
- **WHEN** the helper evaluates the external event
- **THEN** the helper MUST NOT call `GetCreatedTime`
- **AND** the helper MUST allow the event tag when no filter is configured

### Requirement: checkForUpdate Helper Signature

The Go helper that decides whether to apply an update SHALL accept the event origin and enough policy context to distinguish poll from external events and to validate external event tags. It MAY accept a `policy.Policy` and `policy.Filter` directly or via a provider-specific config struct.

#### Scenario: Compile-time enforcement

- **WHEN** a developer removes the Policy/Filter argument from the external-event path
- **THEN** tests for out-of-range webhook events MUST fail

### Requirement: Notification Tag

The `types.Event.TriggerName` field SHALL convey the origin so that providers, downstream notifications, and audit logs can distinguish `poll` from external event sources. Provider update behaviour MUST differ by origin as specified above: only `TriggerName == types.TriggerTypePoll.String()` may trust the watcher-selected tag; all other values, including empty and unknown strings, MUST be treated as external and must pass policy admission.

#### Scenario: Native webhook trigger name is external

- **GIVEN** a `types.Event{TriggerName: "native"}` reaches `Provider.Submit`
- **THEN** the provider MUST process the event through external policy admission
- **AND** the provider MUST NOT treat it as a poll event

#### Scenario: Pubsub trigger name is external

- **GIVEN** a GCR Pub/Sub message is converted into a `types.Event`
- **THEN** the event `TriggerName` MUST be exactly `"pubsub"`
- **AND** the event `TriggerName` MUST NOT equal `types.TriggerTypePoll.String()`
- **AND** the provider MUST process the event through external policy admission

#### Scenario: Empty trigger name is fail-safe external

- **GIVEN** a `types.Event{TriggerName: ""}` reaches `Provider.Submit`
- **THEN** the provider MUST process the event through external policy admission
- **AND** the provider MUST NOT treat it as a poll event

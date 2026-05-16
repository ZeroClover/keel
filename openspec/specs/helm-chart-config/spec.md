# helm-chart-config Specification

## Purpose
TBD - created by archiving change refactor-policy-flux-style. Update Purpose after archive.
## Requirements
### Requirement: KeelChartConfig Field Set

The Go struct `provider/helm3.KeelChartConfig` SHALL contain the following fields and no fields related to approvals, matchTag, or matchPreRelease:

```go
type KeelChartConfig struct {
    Policy               string         `json:"policy"`
    FilterTags           string         `json:"filterTags"`
    Extract              string         `json:"extract"`
    Trigger              types.TriggerType `json:"trigger"`
    PollSchedule         string         `json:"pollSchedule"`
    Images               []ImageDetails `json:"images"`
    NotificationChannels []string       `json:"notificationChannels"`
    Plc                  types.Policy   `json:"-"`
    Filter               types.Filter   `json:"-"`
}
```

#### Scenario: Removed fields produce compile errors

- **WHEN** a developer references `KeelChartConfig{}.MatchTag` or `KeelChartConfig{}.MatchPreRelease`
- **THEN** the Go compiler MUST reject the access with "undefined field"

#### Scenario: Approval-related fields are absent

- **WHEN** a developer inspects `KeelChartConfig`
- **THEN** no field named `Approvals` or `ApprovalDeadline` MUST be declared

### Requirement: Chart Values Translate to Policy + Filter

When Helm provider reads `KeelChartConfig` from a release's values, it SHALL invoke `policy.GetPolicyFromLabelsOrAnnotations(...)` equivalent with the keys mapped as:

| Helm values key | Annotation equivalent |
|---|---|
| `keel.policy` | `keel.sh/policy` |
| `keel.filterTags` | `keel.sh/filterTags` |
| `keel.extract` | `keel.sh/extract` |

The mapping MUST yield identical `(Policy, Filter)` results regardless of whether the source is Kubernetes annotations or Helm values.

#### Scenario: Identical inputs produce identical policies

- **GIVEN** `KeelChartConfig{Policy: "semver:^1.2", FilterTags: "^v(?P<v>.*)$", Extract: "$v"}`
- **AND** a Kubernetes Deployment with the equivalent three annotations
- **WHEN** both flow into the policy parser
- **THEN** the parser MUST return Policy and Filter instances that, given the same tag list, produce the same `Latest` result

### Requirement: Legacy Helm Values Handling

Helm chart values `keel.matchTag` and `keel.matchPreRelease` SHALL be treated like their annotation equivalents: policy parsing MUST return a non-nil error, the Helm provider MUST log at ERROR level with migration guidance, and the release MUST be skipped for that event. The Helm provider MUST NOT abort process-level reconciliation.

Helm chart values `keel.approvals` and `keel.approvalDeadline` SHALL have no behavioural effect after the approvals system is removed. Their presence MUST NOT cause Keel to crash, log at ERROR level, or skip the release.

#### Scenario: Legacy match value skips release

- **WHEN** a release's values contain `keel.matchTag: true` and `keel.approvals: 2` alongside the new `keel.policy: "semver:^1.0"`
- **THEN** the Helm provider MUST skip the release for that event
- **AND** MUST log at ERROR level about `matchTag`
- **AND** MUST NOT return an error from `Submit(event)`

#### Scenario: Legacy approvals value is silently ignored

- **WHEN** a release's values contain `keel.approvals: 2` alongside the new `keel.policy: "semver:^1.0"`
- **THEN** the Helm provider MUST process the release using `policy: semver:^1.0`
- **AND** MUST NOT log an ERROR-level message about `approvals`

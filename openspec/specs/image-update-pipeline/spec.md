# image-update-pipeline Specification

## Purpose
TBD - created by archiving change remove-approvals-system. Update Purpose after archive.
## Requirements
### Requirement: Pipeline Has No Approval Stage

Keel SHALL process every accepted image-update event end-to-end without any manual approval stage. The pipeline order MUST be: trigger → event → provider policy check → deployment update → notification. No waiting state for human approval MAY exist between policy check and deployment update.

#### Scenario: Webhook update flows directly to deployment

- **WHEN** a registry webhook posts a new image tag that satisfies the deployment's policy
- **THEN** Keel MUST patch the target Kubernetes deployment within one reconciliation cycle
- **AND** Keel MUST NOT create any database row representing pending approval
- **AND** Keel MUST NOT emit a notification of type "approval requested" or similar

#### Scenario: Poll trigger update flows directly to deployment

- **WHEN** the poll trigger discovers a newer tag in the registry that satisfies the deployment's policy
- **THEN** Keel MUST submit the event to the provider, which patches the resource without intermediate approval
- **AND** the audit log MUST record only one entry of action `deployment-update` (no `approved`/`rejected` entries)

### Requirement: Approval Annotations Are Silently Ignored

The annotations `keel.sh/approvals` and `keel.sh/approvalDeadline` SHALL have no runtime effect. Their presence on a Kubernetes resource MUST NOT cause Keel to crash, log at error level, or skip the update.

#### Scenario: Deployment with legacy approval annotation

- **WHEN** a Deployment carries `keel.sh/approvals: "3"` and `keel.sh/approvalDeadline: "12"`
- **AND** Keel receives a qualifying image-update event for it
- **THEN** Keel MUST update the deployment immediately as if the annotations were absent

### Requirement: Provider Constructor Signatures

Provider constructors SHALL NOT accept an `approvals.Manager` parameter. The constructor signatures MUST be:

- `kubernetes.NewProvider(impl Implementer, sender notification.Sender, grc *k8s.GenericResourceCache) (*Provider, error)`
- `helm3.NewProvider(impl Implementer, sender notification.Sender) *Provider`
- `provider.New(providers []Provider) *DefaultProviders`

#### Scenario: Compile-time check on constructor

- **WHEN** a developer attempts to compile code calling any of the three constructors with an `approvals.Manager` argument
- **THEN** the Go compiler MUST reject the call with an arity mismatch error

### Requirement: HTTP API Surface

The HTTP server SHALL NOT expose any `/v1/approvals*` route. Requests to the previously supported approval routes (`GET /v1/approvals`, `POST /v1/approvals`, `PUT /v1/approvals`) MUST return HTTP 404. The OpenAPI document (if generated) MUST NOT mention the removed paths.

#### Scenario: Removed approval routes return 404

- **WHEN** an HTTP client issues `GET /v1/approvals`, `POST /v1/approvals`, or `PUT /v1/approvals`
- **THEN** Keel MUST respond with status code `404 Not Found`

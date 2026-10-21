# Architecture

Verified against commit `9070252f` (2026-10-22). Line numbers drift; the file paths are the anchor.

## Overview

```
 triggers                          providers                      side effects
 ─────────                         ─────────                      ────────────
 poll watcher  ──┐                 kubernetes (always on)  ──┐    notification senders
 GCR Pub/Sub   ──┼─ types.Event ─► helm3 (HELM3_PROVIDER)  ──┼──► (slack, teams, …, auditor)
 HTTP webhooks ──┘  (Submit)                                 │    auditor → SQLite audit_logs
                                                             └──► update the Kubernetes
                                                                  resource / Helm release
```

A trigger produces a `types.Event` (`types/types.go`: `Repository{Host, Name, Tag, Digest}`, `CreatedAt`, `TriggerName`). `provider.DefaultProviders.Submit` (`provider/provider.go`) hands it to every enabled provider; each provider finds the resources that use that repository, decides whether to update them, applies the update, and sends notifications.

## Startup (`cmd/keel/main.go`)

In order:

1. Parse flags (`--incluster`, `--kubeconfig`, `--ui-dir`) and set debug logging when `DEBUG=true`.
2. Open the SQLite store at `$XDG_DATA_HOME/keel.db` (default `/data/keel.db`); exit on failure.
3. Register the auditor sender explicitly, then configure all senders with `NOTIFICATION_LEVEL` and 10 attempts. Senders whose env vars are missing unregister themselves.
4. Build the Kubernetes implementer (in-cluster by default; `--no-incluster` uses `KUBERNETES_CONFIG` / `KUBERNETES_MASTERURL` / `KUBERNETES_CONTEXT`).
5. Register informers for Deployments, StatefulSets, DaemonSets and CronJobs into a shared resource cache (`internal/k8s`). They start with the workgroup in step 9.
6. `setupProviders`: the Kubernetes provider always; the Helm 3 provider when `HELM3_PROVIDER` is `1` or `true`.
7. Decode `DOCKER_REGISTRY_CFG` (fatal if invalid) and register the `secrets` credentials helper explicitly.
8. `setupTriggers`: the HTTP server on `:9300` (webhooks, health, metrics, and the admin API + UI when basic auth is configured); GCR Pub/Sub when `PUBSUB` is non-empty (requires `PROJECT_ID`); the poll manager (always on, see `docs/agents/known-issues.md`).
9. `internal/workgroup` runs everything; when any member returns, all stop. SIGINT stops providers and the HTTP server with a 10 s timeout.

Blank imports in `main.go` pull in the discord, mail, mattermost, slack, teams and webhook senders and the aws and gcr credentials helpers; each registers itself in `init()`.

## Triggers

| `TriggerName` | Source | Notes |
|---|---|---|
| `poll` | `trigger/poll/` | Selects tags with the resource's policy; the only trusted trigger |
| `pubsub` | `trigger/pubsub/` | Subscribes to the `gcr` topic when a tracked image is on `gcr.io` / `pkg.dev`; handles `INSERT` messages with a tag |
| `native`, `dockerhub`, `jfrog`, `quay`, `azure`, `github`, `harbor` | `pkg/http/<name>_webhook_trigger.go` | One route each under `/v1/webhooks/` |
| `registry-notification` | `pkg/http/registry_notifications.go` | Docker distribution notifications (Docker, GitLab, Harbor); route `/v1/webhooks/registry` |

`types.TriggerType` (the `keel.sh/trigger` value) only has `default` and `poll`: it decides whether the poll watcher tracks an image, not which webhooks are accepted. Webhook and Pub/Sub events reach every resource regardless of its trigger setting.

Poll scheduling and tag selection are described in `docs/agents/policies.md` → Poll watcher.

## Trust model

Both providers make the same decision per container:

- Same repository and a different tag are required (`provider/kubernetes/updates.go`, `provider/helm3/updates.go`).
- `shouldUpdateFromEvent` / `shouldUpdateReleaseFromEvent` return true immediately when `TriggerName == "poll"`.
- Any other trigger goes through `policy.AllowsTag(policy, filter, currentTag, eventTag)` (`internal/policy/policy.go`): nil policy or empty tag → reject; `force` → allow unless the filter rejects the tag; other policies → apply the filter to `{current, event}` and allow only if the policy's `Latest` picks the event tag.

Requirements: `openspec/specs/provider-update-decision/spec.md`. A poll event carries only repository and tag, so it is trusted for every resource using that repository, not only the one whose policy chose it; see `docs/agents/known-issues.md`.

## Providers

```go
// provider/provider.go
type Provider interface {
    Submit(event types.Event) error
    TrackedImages() ([]*types.TrackedImage, error)
    GetName() string
    Stop()
}
```

`DefaultProviders.Submit` logs provider errors and always returns nil.

**Kubernetes** (`provider/kubernetes/`): `Submit` queues events (buffer 100). `createUpdatePlans` walks every cached resource that has a valid policy, `checkForUpdate` rewrites matching container images (init containers only with `keel.sh/initContainers: "true"`) and stamps `keel.sh/update-time` on the pod template, then the implementer does a typed `Update()` (not a patch, no conflict retry) after setting `kubernetes.io/change-cause`. `TrackedImages` lists images from resources with a valid policy, including pull secrets from `keel.sh/imagePullSecret` and the pod spec.

**Helm 3** (`provider/helm3/`): reads the `keel:` block from each release's merged values, converts `policy` / `filterTags` / `extract` into the same annotation form and parses them with the Kubernetes code path, and upgrades the release through `UpdateReleaseFromChart` with `--set`-style values. Keys are listed in `docs/agents/policies.md`.

## Notifications and audit

`extension/notification/` keeps a registry of `Sender`s. `notification.New(ctx).Send` drops events below `NOTIFICATION_LEVEL` and retries each sender with exponential backoff; when a sender exhausts its attempts the remaining senders are skipped. The auditor sender (`extension/notification/auditor/`) is the only writer to the store and records each event as an `audit_logs` row, which `/v1/audit` and `/v1/stats` read.

## Registry access

`registry/` wraps `registry/docker/` with three operations: list tags (`/v2/<name>/tags/list`, paginated), get a digest, and read an image's config `created` time (following the first entry of a manifest list). Clients are cached per registry and credentials. Credentials come from `extension/credentialshelper`: aws (ECR hostnames), gcr (`gcr.io` / `pkg.dev`), and secrets (`DOCKER_REGISTRY_CFG`, then the image's pull secrets). Requirements: `openspec/specs/registry-client/spec.md`.

## Directory map

| Path | Contents |
|---|---|
| `cmd/keel/` | Entry point and wiring |
| `types/` | Domain types: `Event`, `Repository`, `TrackedImage`, policy/trigger enums, annotation keys, notification levels |
| `internal/policy/` | Policy parsing, `semver` / `force` / `alphabetical` / `numerical`, regex filter, `AllowsTag` |
| `internal/k8s/` | Informer-backed resource cache and the generic resource wrapper |
| `internal/workgroup/` | Run-until-first-exit goroutine group |
| `provider/` | `kubernetes/` and `helm3/` providers |
| `trigger/poll/`, `trigger/pubsub/` | Poll watcher and GCR Pub/Sub trigger |
| `pkg/http/` | HTTP server: webhook triggers, admin API, UI static files |
| `pkg/auth/` | Basic auth and JWT tokens |
| `pkg/store/sql/` | SQLite store (audit logs) |
| `registry/` | Registry client |
| `secrets/` | Credentials from Kubernetes Secrets and `DOCKER_REGISTRY_CFG` |
| `extension/notification/` | Senders and the notification dispatcher |
| `extension/credentialshelper/` | aws, gcr and secrets helpers |
| `util/` | Image reference parsing, version helpers, codecs, templates |
| `constants/` | Most env var names |
| `chart/keel/` | Helm chart for installing Keel |
| `ui/` | Vue 2 dashboard |
| `tests/` | E2E acceptance tests against a live cluster |
| `openspec/` | Capability specs and change proposals |

# Configuration, HTTP API and persistence

Verified against commit `9070252f` (2026-10-22). Most env var names are constants in `constants/constants.go` or `cmd/keel/main.go`; a few are read inline where noted. Before documenting a new variable, grep for `os.Getenv` to confirm it.

## Flags (`cmd/keel/main.go`)

| Flag | Default | Notes |
|---|---|---|
| `--incluster` / `--no-incluster` | in-cluster | Out-of-cluster mode is needed for the `KUBERNETES_*` variables and `--kubeconfig` to matter |
| `--kubeconfig` | `$HOME/.kube/config` | Overridden by `KUBERNETES_CONFIG` |
| `--ui-dir` | `www` (env `UI_DIR`) | Relative path; the image places the UI at `/www` |

## Environment variables

Boolean switches are compared to exact strings; the accepted values are listed per variable.

**Core**

| Variable | Default | Effect |
|---|---|---|
| `DEBUG` | off | `true` enables debug logs, `/debug/vars` and `/debug/pprof/*`, and a Slack startup message |
| `XDG_DATA_HOME` | `/data` | Directory of `keel.db` |
| `NOTIFICATION_LEVEL` | `info` | Minimum level for all senders, including the auditor: `debug`, `info`, `success`, `warn`/`warning`, `error`, `fatal`. Invalid values log an error and keep `info` |
| `RESTRICTED_NAMESPACE` | all namespaces | Watch a single namespace (`internal/k8s/watcher.go`); `""` or `keel` means all |

**Kubernetes (out-of-cluster only)**: `KUBERNETES_CONFIG`, `KUBERNETES_MASTERURL`, `KUBERNETES_CONTEXT`.

**Triggers and providers**

| Variable | Default | Effect |
|---|---|---|
| `POLL` | on | Intended to disable polling with `0`/`false`; currently has no effect (`docs/agents/known-issues.md`) |
| `POLL_DEFAULTSCHEDULE` | `@every 1m` | Default poll schedule (`types/types.go`) |
| `PUBSUB` | off | Any non-empty value enables the GCR Pub/Sub trigger |
| `PROJECT_ID` | — | Required when `PUBSUB` is set (startup is fatal otherwise) |
| `CLUSTER_NAME` | GCE metadata | Part of the Pub/Sub subscription name |
| `HELM3_PROVIDER` | off | `1` or `true` enables the Helm 3 provider |
| `HELM_DRIVER`, `HELM_KUBECONTEXT`, `HELM_KUBETOKEN`, `HELM_KUBEAPISERVER` | — | Helm 3 action config (`provider/helm3/implementer.go`); `HELM_DRIVER_SQL_CONNECTION_STRING` is read by the Helm library when `HELM_DRIVER=sql` |

**HTTP and auth**

| Variable | Default | Effect |
|---|---|---|
| `BASIC_AUTH_USER`, `BASIC_AUTH_PASSWORD` | — | Both set → admin API and UI are registered |
| `TOKEN_SECRET` | random per start | JWT signing secret; without it tokens die on restart and don't work across replicas |
| `AUTHENTICATED_WEBHOOKS` | off | `true` puts webhook routes behind admin auth (except `/v1/webhooks/registry`) |
| `PRIVATE_REGISTRY` | — | Prefix added to JFrog webhook image names (`pkg/http/jfrog_webhook_trigger.go`) |

**Registry credentials**

| Variable | Effect |
|---|---|
| `DOCKER_REGISTRY_CFG` | dockerconfigjson (`{"auths": {...}}`) used before pull secrets; invalid JSON is fatal at startup |
| `INSECURE_REGISTRY` | `true` skips TLS verification and allows falling back to plain HTTP (`registry/registry.go`) |
| `GOOGLE_APPLICATION_CREDENTIALS` | Key file for `gcr.io` / `pkg.dev`; otherwise workload identity. Also used implicitly by the Pub/Sub client |
| AWS SDK chain (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, IRSA) | ECR credentials; the region is taken from the registry hostname |

**Notification senders** (a sender stays registered only if its required variables are set)

| Sender | Variables |
|---|---|
| slack | `SLACK_BOT_TOKEN` (required), `SLACK_BOT_NAME` (default `keel`), `SLACK_CHANNELS` (comma-separated, default `general`) |
| teams | `TEAMS_WEBHOOK_URL` |
| discord | `DISCORD_WEBHOOK_URL` |
| mattermost | `MATTERMOST_ENDPOINT` (required), `MATTERMOST_USERNAME` (default `keel`) |
| mail | `MAIL_SMTP_SERVER`, `MAIL_FROM`, `MAIL_TO` (required); `MAIL_SMTP_PORT` (default 25), `MAIL_SMTP_USER`, `MAIL_SMTP_PASS` |
| webhook | `WEBHOOK_ENDPOINT` |
| auditor | none; always on |

## Helm chart (`chart/keel`)

`chart/keel/templates/deployment.yaml` maps values to env vars; secret-backed values (`AWS_SECRET_ACCESS_KEY`, `SLACK_BOT_TOKEN`, `TEAMS_WEBHOOK_URL`, `DISCORD_WEBHOOK_URL`, `MAIL_SMTP_PASS`, `BASIC_AUTH_PASSWORD`, Google credentials) live in `chart/keel/templates/secret.yaml` and reach the pod through `envFrom`, so they are lost when `secret.create=false` and no `secret.name` is given. Points that are easy to get wrong:

- `HELM3_PROVIDER` is set only when `helmProvider.enabled` and `helmProvider.version == "v3"`; the default `version` is `"v2"`, so a default install has no Helm provider.
- `TOKEN_SECRET`, `AUTHENTICATED_WEBHOOKS`, `RESTRICTED_NAMESPACE`, `MATTERMOST_USERNAME`, `PRIVATE_REGISTRY`, `HELM_KUBE*` and `KUBERNETES_*` have no dedicated values; use `extraEnv`.
- Port 9300 is hard-coded in the deployment and service templates.
- `persistence.enabled` mounts a PVC at `/data`; without it the audit log is lost with the pod.
- The commented `keel:` block in `values.yaml` is Keel's own self-update config, read by the Helm 3 provider; it is not used by any template.
- Chart version and `appVersion` are placeholders that CI overwrites from the release tag (`docs/agents/development.md`).

Requirements: `openspec/specs/helm-chart-config/spec.md`.

## HTTP server (`pkg/http/`)

One listener on `:9300` (`types.KeelDefaultPort`), CORS `*`, every `OPTIONS` answered 200.

| Routes | Registered when | Auth |
|---|---|---|
| `GET /healthz`, `GET /version`, `/metrics` (Prometheus) | always | none |
| `POST /v1/webhooks/{native,dockerhub,jfrog,quay,azure,github,harbor}` | always | admin auth only with `AUTHENTICATED_WEBHOOKS=true` |
| `POST /v1/webhooks/registry` | always | never |
| `/debug/vars`, `/debug/pprof/*` | `DEBUG=true` | none |
| `POST /v1/auth/login` | basic auth configured | none |
| `/v1/auth/{info,user,logout,refresh}`, `GET /v1/resources`, `PUT /v1/policies`, `GET`/`PUT /v1/tracked`, `GET /v1/audit`, `GET /v1/stats` | basic auth configured | admin |
| UI: `/css/`, `/assets/`, `/js/`, `/img/`, `/loading/`, fallback `/` → `index.html` | basic auth configured and `--ui-dir` set | none |

Admin auth (`pkg/http/auth.go`, `pkg/auth/`) accepts HTTP Basic credentials or a Bearer JWT (HS512, 12 h, refreshed on each validated request). `PUT /v1/policies` writes `keel.sh/policy` without validating it, and `PUT /v1/tracked` writes `keel.sh/trigger` / `keel.sh/pollSchedule`.

## Persistence (`pkg/store/sql/`)

SQLite through gorm v1 (`github.com/jinzhu/gorm`) on the pure-Go `modernc.org/sqlite` driver, file `$XDG_DATA_HOME/keel.db`. On start the store retries the connection for up to 300 s, runs `DROP TABLE IF EXISTS approvals` (cleanup from the removed approvals feature), and auto-migrates `types.AuditLog` (`audit_logs`). Audit logs are the only stored data. Requirements: `openspec/specs/persistence/spec.md`.

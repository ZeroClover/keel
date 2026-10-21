# Keel agent guide

Keel is a Kubernetes operator that watches container registries and updates Deployments, StatefulSets, DaemonSets, CronJobs and Helm 3 releases when a new image tag satisfies the resource's policy. This repository is the ZeroClover/keel fork (Go module path is still `github.com/keel-hq/keel`): one Go binary (`cmd/keel`), a Vue 2 dashboard (`ui/`), the image `ghcr.io/zeroclover/keel`, and the Helm chart in `chart/keel`.

## Core constraints

- **Poll events skip the policy check.** Providers apply an event whose `TriggerName` is `"poll"` without calling `policy.AllowsTag`, because the poll watcher already selected the tag with the resource's policy. Every other trigger (webhooks, Pub/Sub, empty or unknown names) must pass the target resource's policy and tag filter. Any change to triggers, `types.Event`, or the provider update path has to keep this split; see `docs/agents/architecture.md` → Trust model.
- **Builds are CGO-free.** SQLite comes from the pure-Go `modernc.org/sqlite` driver and every build path sets `CGO_ENABLED=0`. Don't add dependencies that need cgo.
- **Behavior changes go through OpenSpec.** `openspec/specs/` holds the intended behavior of each capability. For a change to user-visible behavior, create a change with `/opsx:propose` (Codex: `$openspec-propose`), implement it with `/opsx:apply`, and archive it when done. Where code and spec currently disagree, `docs/agents/known-issues.md` records it; don't quietly change either side to match the other.
- **Credentials stay out of git and logs.** Registry passwords, tokens and webhook URLs reach Keel through env vars or Kubernetes Secrets. `ui/.env` is committed; only `ui/.env.local` and `ui/.env.*.local` are git-ignored.
- **Legacy upstream files are not maintained.** `docs/agents/development.md` → Legacy files lists them (Azure/PowerShell/compose builds, extra Dockerfiles, `deployment/`, the root `values.yaml`, …). Don't update them to follow a change, and don't read them as a description of current behavior.
- **Commits** use Conventional Commits in English: `feat`, `fix`, `refactor`, `docs`, `chore`, `ci`, with an optional scope such as `chart`, `policy`, `docker` or `release`.

## Working rules

- Keep changes to what was asked: no drive-by refactors, no speculative options or abstractions, no comments or docstrings on code you didn't change. Validate at system boundaries (webhook payloads, annotations, env vars) and trust internal code.
- Implement the general logic rather than special-casing test inputs. If a test or the task itself looks wrong, say so instead of working around it.
- Read the relevant code before making claims about how it behaves.
- Remove temporary files and scripts you created before finishing.

## Verification

CI runs `make test` (Go unit tests; some registry tests call Docker Hub and Quay and fail offline) and `cd ui && yarn && yarn run lint --no-fix`. Run the one that covers your change before finishing; `go test ./<pkg>/...` is fine while iterating. Build, run and E2E commands are in `docs/agents/development.md`.

## Where to read next

Open the file that matches the task. Each one cites its source files, so re-check against the code when a detail matters.

| File | Read when |
|---|---|
| `docs/agents/architecture.md` | Orienting; startup wiring; how an event flows from trigger to provider; directory map |
| `docs/agents/policies.md` | Policies, tag filters, `keel.sh/*` annotations, Helm `keel:` values, poll scheduling |
| `docs/agents/configuration.md` | Env vars and flags, Helm chart values, HTTP routes and auth, persistence |
| `docs/agents/extending.md` | Adding a notification sender, webhook trigger, provider or credentials helper |
| `docs/agents/development.md` | Build, tests, CI, release, chart publishing, OpenSpec workflow, legacy files |
| `docs/agents/ui.md` | Working in `ui/` |
| `docs/agents/known-issues.md` | Before changing behavior, or when code, specs and docs disagree |
| `openspec/specs/<capability>/spec.md` | Normative requirements for one capability |

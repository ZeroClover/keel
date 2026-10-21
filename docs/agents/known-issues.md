# Known issues and drift

Verified against commit `9070252f` (2026-10-22). Each entry is a place where the code, the specs, or the docs disagree, or where the code doesn't do what its own comments or configuration say. Fix behavior through an OpenSpec change, and remove the entry here in the same commit.

## Behavior

- **`POLL` cannot disable polling.** `cmd/keel/main.go` (`setupTriggers`) tests `os.Getenv("POLL") != "0" || os.Getenv("POLL") != "false"`, which is always true. The chart's `polling.enabled: false` sets `POLL=false` with no effect.
- **Poll events are trusted across resources.** A poll event carries only repository and tag; both providers apply it to every resource using that repository without a policy check (`provider/kubernetes/kubernetes.go` `createUpdatePlans`, `shouldUpdateFromEvent`; same in `provider/helm3`). A tag chosen by one resource's policy can update another resource with a different policy or filter, or one not set to `trigger: poll`. `openspec/specs/provider-update-decision/spec.md` assumes one resource per repository.
- **`force:created` ignores registry credentials.** `sortByCreatedTime` in `trigger/poll/multi_tags_watcher.go` builds `registry.Opts` without username/password, so on private registries every created-time lookup fails and the order falls back to the registry's.
- **Init containers: label opt-in is half-supported.** `TrackedImages` and the update loop accept `keel.sh/initContainers` as label or annotation, but `checkForUpdate` (`provider/kubernetes/updates.go`) only checks the annotation, so a label-only opt-in is polled but never updated.
- **`PUT /v1/tracked` can't set a working schedule.** It validates the schedule with `time.ParseDuration` but stores it as `keel.sh/pollSchedule`, which is parsed as a cron spec: `5m` is accepted then silently replaced by the default, `@every 5m` is rejected (`pkg/http/tracked_endpoint.go`).
- **`PUT /v1/policies` doesn't validate the policy** before writing the annotation (`pkg/http/policy_endpoint.go`).
- **GitHub webhook continues after a 400.** For non-`CONTAINER` packages the handler writes 400 but doesn't return (`pkg/http/github_webhook_trigger.go`).
- **Credentials helper order.** Helpers are tried in map order; the `secrets` helper returns empty, non-nil credentials when the image's secrets don't match the registry, which ends the search before aws/gcr are tried (`secrets/secrets.go`, `extension/credentialshelper/credentialshelper.go`).
- **One sender failure stops the rest.** `notification.Send` returns as soon as one sender exhausts its retries (`extension/notification/notification.go`).
- **Chart default installs no Helm provider.** `helmProvider.version` defaults to `"v2"` but the template only sets `HELM3_PROVIDER` for `"v3"`; `tillerNamespace` / `tillerAddress` are unused Helm 2 leftovers (`chart/keel/values.yaml`, `chart/keel/templates/deployment.yaml`).
- **Stale comments:** the poll manager comment says a 60 s scan (code: 3 s, `trigger/poll/manager.go`); `types/types.go` says the default schedule is `@every 10m` (code: `@every 1m`); `main.go` says `DEBUG` accepts "1 or anything else" (code: only `true`).

## Security

- `/v1/webhooks/registry` is never behind auth, even with `AUTHENTICATED_WEBHOOKS=true` (`pkg/http/http.go` `registerWebhookRoutes`).
- Failed Basic auth logs the attempted password (`pkg/http/auth.go`); a failed token check logs the token.
- Secret decode failures log raw secret data (`secrets/secrets.go`); the ECR token is logged at debug level (`extension/credentialshelper/aws/aws.go`).
- Without `TOKEN_SECRET` the JWT secret is random per start; the JWT `username` claim is always `admin`.

## UI

- The policy menu (`ui/src/views/dashboard/Analysis.vue`) offers `patch`, `minor`, `major`, `all`, `glob:` and `regexp:`, which the backend rejects, and has no way to choose `semver:`, `alphabetical`, `numerical` or `force:created`.
- It compares the policy to `'nil policy'`; the backend now returns `""` or `"invalid"` (`pkg/http/resources_endpoint.go`).
- `ui/README.md` describes approvals, legacy policies and an endpoint that no longer exists.

## User-facing docs

- `readme.md`: says the Helm provider is enabled by default; lists only four webhook registries (there are eight routes); development section mentions Go 1.12, dep, gcc/mingw and `build.ps1`; Terraform example uses a nonexistent chart version and the upstream image.
- `chart/keel/README.md`: Helm 2 commands, `gcr.pubsub.enabled` (the key is `gcr.pubSub.enabled`), no `discord.*`, `notificationLevel` or `helmProvider.version`, and polling described as digest-based.
- `deployment/README.md` refers to a generator script that doesn't exist.

## Dead code

`keel.sh/digest` constant (never read), `constants.DefaultNamespace`, `secrets.lookupSecrets` / `getPodImagePullSecrets`, `pkg/http` `authHeadersMiddleware` and `dailyStats`, `pkg/store/sql` `logsWeeklyStats` (counts the removed `approved` action), `provider/kubernetes` `getDesiredImage`, the unused digest in poll `watchDetails`, and `TrackedImage.Tags` (never filled, so the watcher's "lowest tag" lookup is always empty).

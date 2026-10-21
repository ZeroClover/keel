# Extension points

Verified against commit `9070252f` (2026-10-22). A new extension is user-visible behavior: propose it as an OpenSpec change first (`AGENTS.md` → Core constraints), and add its env vars to `docs/agents/configuration.md` and, if users should set them, to the Helm chart.

## Notification sender

Interface (`extension/notification/notification.go`):

```go
type Sender interface {
    Configure(*Config) (bool, error)          // false = not configured; the sender is unregistered
    Send(event types.EventNotification) error
}
```

1. Create `extension/notification/<name>/<name>.go`. Read the sender's env vars in `Configure` (add the names to `constants/constants.go`) and return `false` when the required ones are missing.
2. Register in `init()` with `notification.RegisterSender("<name>", &sender{})`; it panics on an empty name, a nil sender or a duplicate name.
3. Add a blank import in `cmd/keel/main.go` next to the other senders.
4. Return an error from `Send` on delivery failure so the dispatcher's retry applies; `Config.Level` filtering is done by the dispatcher, not the sender.

The auditor is the exception to steps 2–3: `main.go` constructs it with the store and registers it explicitly.

## Webhook trigger

1. Create `pkg/http/<registry>_webhook_trigger.go` with a `(s *TriggerServer) <registry>Handler` that decodes the payload, rejects bad input with 400 (and returns), builds a `types.Event` with `Repository.Name`, `Repository.Tag` (and `Digest` when the payload has one), `CreatedAt` and a new, unique `TriggerName`, then calls `s.trigger(event)`.
2. Never set `TriggerName` to `"poll"`, and never take it from the request body: the providers trust poll events without a policy check (`docs/agents/architecture.md` → Trust model).
3. Register the route in **both** branches of `registerWebhookRoutes` in `pkg/http/http.go`, wrapping it in `s.requireAdminAuthorization` in the authenticated branch.
4. Add `<registry>_webhook_trigger_test.go` with a captured payload, following the existing tests in `pkg/http/`.

## Provider

1. Create `provider/<name>/` implementing `provider.Provider` (`Submit`, `TrackedImages`, `GetName`, `Stop`).
2. Reuse `policy.GetPolicyFromLabelsOrAnnotations` and the poll-versus-external decision used by `provider/kubernetes/updates.go` (`shouldUpdateFromEvent`) so policies, filters and the trust model behave the same as in the existing providers.
3. Construct it in `setupProviders` in `cmd/keel/main.go`, behind an env var if it is optional (the Helm 3 provider uses `HELM3_PROVIDER`).
4. Report images through `TrackedImages` with `Trigger`, `PollSchedule`, `Policy`, `Filter` and pull secrets filled in; the poll watcher and the Pub/Sub manager both depend on it.

## Credentials helper

Interface (`extension/credentialshelper/credentialshelper.go`):

```go
type CredentialsHelper interface {
    GetCredentials(image *types.TrackedImage) (*types.Credentials, error)
    IsEnabled() bool
}
```

1. Create `extension/credentialshelper/<name>/`. Return `credentialshelper.ErrUnsupportedRegistry` for registries the helper doesn't handle.
2. Register in `init()` with `credentialshelper.RegisterCredentialsHelper("<name>", helper)` and add a blank import in `cmd/keel/main.go`.
3. Helpers are tried in random (map) order and the first non-nil result wins, so a helper must return nil credentials (or an error) when it has nothing for the image.

The `secrets` helper is registered explicitly in `main.go` because it needs the Kubernetes client and `DOCKER_REGISTRY_CFG`.

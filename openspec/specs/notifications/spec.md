# notifications Specification

## Purpose
TBD - created by archiving change remove-hipchat-and-chatbot. Update Purpose after archive.
## Requirements
### Requirement: Notification Sender Registry

Keel SHALL continue to provide a notification subsystem that lets external systems receive image-update events via single-direction (outbound) senders. This Change only constrains the sender set after HipChat and interactive chat bots are removed; it MUST NOT change the existing notification retry, level filtering, or extension mechanics except where HipChat/bot code is deleted.

#### Scenario: Built-in senders are registered at startup

- **WHEN** Keel starts with default build tags
- **THEN** the registered sender set MUST include at minimum: `slack`, `teams`, `discord`, `mattermost`, `mail`, `webhook`, `auditor`
- **AND** non-HipChat senders MAY keep their existing registration mechanism (blank import or runtime registration)

#### Scenario: HipChat sender is no longer available

- **WHEN** a user configures `HIPCHAT_TOKEN` (or any `HIPCHAT_*` environment variable)
- **THEN** the variable MUST be silently ignored (no HipChat sender is registered)
- **AND** the value MUST NOT cause Keel to crash or emit a startup error

#### Scenario: Slack notification sender keeps using bot-style token

- **WHEN** `SLACK_BOT_TOKEN`, `SLACK_BOT_NAME`, `SLACK_CHANNELS` are set
- **THEN** the Slack notification sender MUST initialise and deliver messages to the listed channels using the Slack Web API
- **AND** the message MUST be sent without requiring the deleted `SLACK_APP_TOKEN` / `SLACK_APPROVALS_CHANNEL` variables

### Requirement: Removal of Interactive ChatOps

Keel SHALL NOT provide any in-process chat bot or interactive ChatOps interface for receiving user commands. There MUST be no implementation of `keel get deployments`, `keel approve`, `keel reject` style commands via Slack, HipChat or any other chat platform.

#### Scenario: bot package is absent from the binary

- **WHEN** a developer inspects the compiled binary or repository tree
- **THEN** no Go package under `github.com/keel-hq/keel/bot/...` MUST exist
- **AND** no blank import of such a package MUST appear in `cmd/keel/main.go`
- **AND** no Go file MUST import `github.com/slack-go/slack/socketmode`

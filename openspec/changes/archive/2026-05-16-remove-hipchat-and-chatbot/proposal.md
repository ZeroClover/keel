## Why

HipChat 服务自 2019 年起已被 Atlassian 关闭,Keel 残留的 HipChat bot、通知 sender、配置变量与 Go 依赖项是死代码。整个 `bot/` 包(Slack + HipChat ChatOps for Approval)也已被新的 GitOps 工作流取代——审批流程将整体移除(见 [[remove-approvals-system]])。本 Change 通过删除 HipChat 与 chat bot 实现来减少代码体积、依赖、安全攻击面与配置混淆,但保留 Slack 作为**单向通知 sender**(仍受用户欢迎)。

## What Changes

- **BREAKING**: 删除 HipChat 通知 sender。任何配置了 `HIPCHAT_TOKEN` / `HIPCHAT_USER_NAME` / `HIPCHAT_PASSWORD_NAME` 等环境变量的部署将不再触发 HipChat 通知。
- **BREAKING**: 删除整个 `bot/` 包(`bot/bot.go`、`bot/slack/`、`bot/hipchat/`、`bot/formatter/`、`bot/approvals.go`、`bot/deployments.go`)。Slack/HipChat ChatOps 命令(`keel get deployments`、`keel approve`)将不再可用。
- **BREAKING**: 删除 chat bot 专用环境变量:`SLACK_APP_TOKEN`、`SLACK_APPROVALS_CHANNEL`。这两个仅供 `bot/slack` 使用。
- 保留环境变量:`SLACK_BOT_TOKEN`、`SLACK_BOT_NAME`、`SLACK_CHANNELS`。这些被 `extension/notification/slack/slack.go` 当作单向 webhook/bot token 复用,删除会破坏现有通知配置。
- 删除 Go 依赖:`github.com/daneharrigan/hipchat`、`github.com/tbruyelle/hipchat-go`。保留 `github.com/slack-go/slack`——`extension/notification/slack/slack.go:11` 仍依赖它发送通知。`go mod tidy` 后 `go.sum` 显著瘦身。
- 更新 Helm chart `chart/keel/values.yaml`、`chart/keel/templates/deployment.yaml`、`chart/keel/templates/secret.yaml`、`deployment/deployment-template.yaml`、`chart/keel/README.md`、根 `readme.md`、`ARCHITECTURE.md`,移除所有 HipChat 与 chat bot 引用。
- 保留 `extension/notification/slack/`、`extension/notification/teams/`、`extension/notification/discord/`、`extension/notification/mattermost/`、`extension/notification/mail/`、`extension/notification/webhook/`、`extension/notification/auditor/`。

## Capabilities

### New Capabilities
- `notifications`: 描述 Keel 在删除 HipChat 与 chat bot 后所支持的单向通知 sender 集合与扩展机制。

### Modified Capabilities
无。本批 OpenSpec changes 以空 `openspec/specs/` 为起点创建;归档顺序假设为:本 Change → [[remove-approvals-system]] → [[refactor-policy-flux-style]] → [[refactor-watcher-and-force-policy]]。

## Impact

- 代码:删除约 12 个目录/文件(`bot/` 整目录、`extension/notification/hipchat/`、相关测试文件)。
- 配置:`chart/keel/values.yaml` 删除 `hipchat:` 段与 bot 相关条目;`secret.yaml` 删除对应 Secret 引用。
- 依赖:`go.mod` 删除两个 hipchat 包;`go mod tidy` 收敛 `go.sum`。
- 文档:`readme.md`、`ARCHITECTURE.md`、`chart/keel/README.md` 章节重写,Bot/HipChat 提及全部移除。
- 升级路径:用户必须从其 Deployment Manifest 与 Helm values 中移除上述 BREAKING 环境变量;由依赖 chat bot 进行审批的用户将由 [[remove-approvals-system]] 提供 GitOps 替代方案。
- 风险:Slack 通知 sender 与 Slack bot 共用部分配置项(`SLACK_TOKEN`),需在删除时仔细区分。

## 1. 删除 HipChat 通知 sender

- [x] 1.1 删除整个目录 `extension/notification/hipchat/`(含 `hipchat.go` 与测试)。
- [x] 1.2 移除 `cmd/keel/main.go:44` 的 `_ "github.com/keel-hq/keel/extension/notification/hipchat"` blank import。
- [x] 1.3 删除 `constants/constants.go:20-28` 的 8 个 HipChat 常量(`EnvHipchatToken`、`EnvHipchatBotName`、`EnvHipchatChannels`、`EnvHipchatApprovalsChannel`、`EnvHipchatApprovalsUserName`、`EnvHipchatApprovalsBotName`、`EnvHipchatApprovalsPasswort`、`EnvHipchatConnectionAttempts`)。
- [x] 1.4 `grep -rn "Hipchat\|HIPCHAT\|hipchat" --include="*.go" .` 清理任何残留 import / 引用。

## 2. 删除整个 bot/ 包

- [x] 2.1 删除整个目录 `bot/`(含 `bot.go`、`approvals.go`、`deployments.go`、`slack/`、`hipchat/`、`formatter/`)及其全部测试文件。
- [x] 2.2 删除 `cmd/keel/main.go:18` 的 `"github.com/keel-hq/keel/bot"` import。
- [x] 2.3 删除 `cmd/keel/main.go:57-58` 的两行 bot blank import(`_ "github.com/keel-hq/keel/bot/hipchat"` 与 `_ "github.com/keel-hq/keel/bot/slack"`)。
- [x] 2.4 删除 `cmd/keel/main.go:265` 的 `bot.Run(implementer, approvalsManager)` 调用。
- [x] 2.5 删除 `cmd/keel/main.go:286` shutdown 路径中的 `bot.Stop()` 调用。
- [x] 2.6 删除 `constants/constants.go` 中仅 bot 使用的两个常量:`EnvSlackAppToken`、`EnvSlackApprovalsChannel`。保留 `EnvSlackBotToken`、`EnvSlackBotName`、`EnvSlackChannels`(`extension/notification/slack/slack.go:36-48` 仍使用)。
- [x] 2.7 `grep -rn "bot\.\|keel-hq/keel/bot" --include="*.go" .` 确认无任何残留引用。

## 3. 清理 Go module 依赖

- [x] 3.1 编辑 `go.mod`,删除 `github.com/daneharrigan/hipchat` 与 `github.com/tbruyelle/hipchat-go` require 行。
- [x] 3.2 运行 `go mod tidy` 让 `go.sum` 自动收敛。验证 `github.com/slack-go/slack` **仍存在**(被 `extension/notification/slack` 使用)。
- [x] 3.3 运行 `go build ./...` 通过。

## 4. 清理 Helm chart 与部署模板

- [x] 4.1 删除 `chart/keel/values.yaml` 中 `hipchat:` 段(约 line 110-118)与 `slack.appToken`、`slack.approvalsChannel` 两个 bot 专用字段。
- [x] 4.2 删除 `chart/keel/templates/deployment.yaml` 中 HipChat 环境变量段(约 line 159-169)与 `SLACK_APPROVALS_CHANNEL` 注入段(约 line 152-153)。
- [x] 4.3 删除 `chart/keel/templates/secret.yaml` 中 bot 专用 Secret key `SLACK_APP_TOKEN`(约 line 19),以及 HipChat Secret key `HIPCHAT_TOKEN`、`HIPCHAT_APPROVALS_PASSWORT`(约 line 24-27)。
- [x] 4.4 更新 `chart/keel/README.md`,删除 HipChat 与 chat bot 章节。
- [x] 4.5 删除 `deployment/deployment-template.yaml` 中 HipChat 段(约 line 198-210)与 bot 专用 env 行。

## 5. 文档与示例更新

- [x] 5.1 更新根 `readme.md`,删除 line 37 附近 HipChat 提及与 ChatOps 章节。在文档顶部 "Upgrading" 段落记录 Breaking Changes 清单(`HIPCHAT_*`、`SLACK_APP_TOKEN`、`SLACK_APPROVALS_CHANNEL` 已移除;`/v1/chat` 类 endpoint 不再可用)。
- [x] 5.2 更新 `ARCHITECTURE.md` line 62、144 附近的 HipChat 提及;在 "Directory Structure" 表中删除 `bot/` 行;在 "Notifications" 段落 sender 列表中删除 HipChat。
- [x] 5.3 删除 `ARCHITECTURE.md` 中独立的 "Approvals + Bot" 章节(若与 [[remove-approvals-system]] 重叠,以本 Change 删除 bot 部分,Approval 文字由后续 Change 处理)。

## 6. 测试与回归验证

- [x] 6.1 运行 `go test ./...`,确认无 hipchat / bot 包测试残留导致的失败。
- [x] 6.2 `docker compose up keel` 启动本地实例,设置 `SLACK_BOT_TOKEN=xoxb-test` `SLACK_CHANNELS=#test`,触发一次模拟 image-update 事件,确认 Slack notification sender 仍正常发送消息(单向)。
- [x] 6.3 启动 Keel,设置 `HIPCHAT_TOKEN=anything` `SLACK_APP_TOKEN=xapp-anything`,确认进程不 panic、未记录 startup error,这些变量被静默忽略。

## 7. 验证 OpenSpec 工件

- [x] 7.1 运行 `openspec validate remove-hipchat-and-chatbot --strict`,确认 4 个 artifact(proposal/design/specs/tasks)均通过。
- [x] 7.2 准备 PR 标题:"refactor: remove HipChat and chat bot package",PR 描述引用本 Change 的 proposal.md 与 Breaking 清单。

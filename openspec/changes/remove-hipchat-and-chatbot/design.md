## Context

Keel 当前实现两类 chat 集成:

1. **ChatOps bot**(`bot/`):双向交互,通过 Slack/HipChat 接收用户命令(`keel get deployments`、`keel approve`)。HipChat 自 2019 年 Atlassian 关闭服务后不可用;Slack bot 是与 Approval 工作流耦合的,在 [[remove-approvals-system]] 中整体被 GitOps 取代。
2. **Notification sender**(`extension/notification/{slack,hipchat,teams,...}`):单向 outgoing,把更新事件推送到外部系统。

`extension/notification/slack/slack.go:11,36-48` 复用了 `bot/slack` 所用的 `slack-go/slack` 库与三个环境变量(`SLACK_BOT_TOKEN`、`SLACK_BOT_NAME`、`SLACK_CHANNELS`)。若不审慎区分,删除 bot 会一同破坏单向通知。

## Goals / Non-Goals

**Goals:**
- 删除所有 HipChat 相关代码、依赖、配置、文档。
- 删除整个 `bot/` 包(Slack + HipChat 双向 ChatOps)。
- 保留 Slack / Teams / Discord / Mattermost / Mail / Webhook / Auditor 通知 sender。
- 保留 Slack notification sender 仍依赖的环境变量与 Go 模块。
- `go build ./... && go test ./...` 在本 Change 完成后通过。

**Non-Goals:**
- **不**修改剩余 sender 的内部逻辑或消息格式。
- **不**实现 GitOps Approval 替代——见 [[remove-approvals-system]]。
- **不**为废弃环境变量提供运行时检测/警告(部署者由 Release Notes 引导)。

## Decisions

### 1. 删除整个 `bot/` 包,而非渐进迁移

**选择**:一次性删除 `bot/bot.go`、`bot/slack/`、`bot/hipchat/`、`bot/formatter/`、`bot/approvals.go`、`bot/deployments.go` 全部 8 个 Go 文件 + 测试。

**理由**:bot 包功能(查询 deployments、approve/reject)与即将移除的 Approval 系统紧耦合;独立保留 Slack ChatOps 无意义,徒增维护成本。Slack notification sender 走的是独立代码路径(`extension/notification/slack`),不依赖 `bot/`。

**备选**:保留 `bot/slack` 仅作为查询能力。**否决**:用户已有 Kubernetes Dashboard / kubectl / Keel UI,重复能力。

### 2. 环境变量保留策略

| 变量 | 处理 | 原因 |
|---|---|---|
| `HIPCHAT_*`(8 个) | **删除** | sender 与 bot 都使用 |
| `SLACK_APP_TOKEN` | **删除** | 仅 bot 使用(Socket Mode) |
| `SLACK_APPROVALS_CHANNEL` | **删除** | 仅 bot 用于接收 approve/reject 命令 |
| `SLACK_BOT_TOKEN` | **保留** | sender `slack.go:36` 仍读取 |
| `SLACK_BOT_NAME` | **保留** | sender `slack.go:41` 仍读取 |
| `SLACK_CHANNELS` | **保留** | sender `slack.go:47` 仍读取 |

**理由**:`SLACK_BOT_TOKEN` 虽含 "BOT" 字样,但在通知 sender 中作为 Bot User OAuth Token(`xoxb-...`)使用,与 chat bot 概念不同。重命名属于二次破坏,留待未来 spec 演进。

### 3. 不在运行时检测废弃环境变量

用户若仍设置 `HIPCHAT_TOKEN`,Keel 静默忽略。`cmd/keel/main.go` 不打印 ERROR 或 WARN 日志。

**理由**:Release Notes 与 [[remove-approvals-system]] 启动检测合并为单一统一的"Breaking Migration"输出,避免每个废弃变量产生噪声。后续如需要,可在专门的 migration-warner Change 中加。

### 4. `notifications` capability spec 只描述本 Change 影响的目标后状态

不为"被删除的能力"写 spec(HipChat、ChatOps bot)。新建 `notifications` spec 只约束删除完成后的 sender 集合、HipChat 不再注册、Slack 单向通知不依赖 Slack Socket Mode。既有通知等级、重试与扩展机制不属于本 Change 的行为变更,不得在此 Change 中新增约束。

## Risks / Trade-offs

- **风险**:用户依赖 HipChat 通知 → **缓解**:HipChat 服务自 2019 年关闭,无人能正常使用此 sender。
- **风险**:用户依赖 Slack/HipChat bot 命令进行审批 → **缓解**:[[remove-approvals-system]] 同时提供 GitOps 工作流替代;审批本身被废除。
- **风险**:删除环境变量造成 Helm chart 升级失败 → **缓解**:`chart/keel/values.yaml` 默认值即关闭,真正使用者很少;Release Notes 列出 Breaking 清单。
- **风险**:误删 Slack notification sender 引用的 slack-go 依赖 → **缓解**:`go mod tidy` 由 Go toolchain 决定保留,本任务清单仅删 hipchat 包名。

## Migration Plan

1. PR 顺序由 4 个 OpenSpec Change 同步:本 Change(remove-hipchat-and-chatbot)→ [[remove-approvals-system]] → [[refactor-policy-flux-style]] → [[refactor-watcher-and-force-policy]]。
2. 每个 PR 独立通过 `go build ./...` 与 `go test ./...`。
3. 用户升级前需检查的 Breaking 清单写入 `readme.md` 顶部 "Upgrading" 段落。
4. 回滚:本 Change 仅删除代码与配置,无数据迁移;`git revert` 可完整恢复。

## Resolved Decisions

- `chart/keel/templates/deployment.yaml` 不保留对 `SLACK_APP_TOKEN` 的兼容读取,本 Change 直接删除 bot 专用变量。

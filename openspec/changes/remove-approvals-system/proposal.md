## Why

Keel 当前的 Approval 工作流(`keel.sh/approvals: "2"` 注解 → SQLite `approvals` 表 → HTTP `/v1/approvals` API → UI 视图 → chat bot 命令)是 2018 年遗留设计。它存在三类问题:

1. **重复 GitOps 工具链**:Argo CD、Flux、GitHub PR Review、Renovate 等已成为审批镜像更新的事实标准。Keel 内置审批与这些工具同时存在时,反而打乱"以 Git 为单一真理"的工作流。
2. **横跨 17 个文件**:`approvals/`、`extension/approval/`、`provider/{kubernetes,helm3}/approvals.go`、`pkg/http/approvals_endpoint*.go`、`pkg/store/sql/approvals.go`、`types/approvals.go`、`bot/approvals.go`、整个 UI 视图与 store module……是仓库中最大的横切关注点之一,使 provider/trigger/store 接口都背着 `approvals.Manager` 依赖。
3. **与 chat bot 紧耦合**:[[remove-hipchat-and-chatbot]] 删除 bot 后,审批的唯一交互入口只剩 Web UI 与 REST API,本就单薄;再加上 Slack/HipChat 部分已去除,继续保留半残的审批工作流意义不大。

本 Change 一次性删除审批工作流,并在启动时通过 `DROP TABLE IF EXISTS approvals` 清理历史数据库残留。

## What Changes

- **BREAKING**: 删除 `keel.sh/approvals`、`keel.sh/approvalDeadline` 注解(及对应常量 `KeelMinimumApprovalsLabel`、`KeelApprovalDeadlineLabel`、`KeelApprovalDeadlineDefault`)。当用户保留这些注解时,Keel 静默忽略而非报错(运行时无效)。
- **BREAKING**: 删除 HTTP 端点 `GET /v1/approvals`、`POST /v1/approvals`、`PUT /v1/approvals`。所有调用方收到 404。
- **BREAKING**: 删除 Web UI `/approvals` 路由与对应视图 `ui/src/views/approvals/Approvals.vue`。Dashboard 的 "Pending Approvals" 卡片、`Required Approvals` 列、`Policy & Approvals Control` 段一并删除。
- **BREAKING(数据)**: 启动时执行 `DROP TABLE IF EXISTS approvals`。所有历史 approval 记录**不可恢复**。
- 删除 17 个文件 / 目录(详见 design.md 与 tasks.md)。
- 删除 `provider.New(...)`、`kubernetes.NewProvider(...)`、`helm3.NewProvider(...)` 签名中的 `approvalsManager` 参数。
- 删除 `pkg/store.Store` 接口中 5 个 Approval 方法。
- 删除 `types.AuditLogStats.Approved/Rejected` 字段与 4 个 `AuditActionApproval*` 常量。
- 删除 Prometheus 指标 `pending_approvals`。

## Capabilities

### New Capabilities
- `image-update-pipeline`: 描述删除审批后的事件流——从 trigger 到 provider 到 deployment update 不再有"审批等待"阶段。
- `persistence`: 描述 SQLite 持久化层在删除 `approvals` 表后的 schema、迁移行为(含 DROP)、剩余的 `audit_logs` 表。
- `web-dashboard`: 描述删除审批视图后的 UI 路由集合与功能边界。

### Modified Capabilities
无。本批 OpenSpec changes 以空 `openspec/specs/` 为起点创建;归档顺序假设为:[[remove-hipchat-and-chatbot]] → 本 Change → [[refactor-policy-flux-style]] → [[refactor-watcher-and-force-policy]]。

## Impact

- **代码**:删除约 17 个 Go/Vue 文件 + 修改 8 个核心 Go 文件签名。
- **数据库**:启动时无条件 `DROP TABLE IF EXISTS approvals`;运维方需在升级前自行 dump(若依赖审计意义,导出到 `audit_logs` 表外部备份)。
- **API**:Breaking removal of 3 routes;OpenAPI/Swagger 同步删除。
- **CLI/Chart**:`chart/keel/values.yaml` 中 `approvals` 段落删除;Helm 升级 path 无 PV 迁移。
- **依赖**:无新增 / 删除;`provider.New(...)` 签名变化,所有调用方编译更新。
- **关联 Change**:[[remove-hipchat-and-chatbot]] 删除 chat bot 中的 approve/reject 命令;本 Change 删除剩余的核心引擎与数据/UI 表面。

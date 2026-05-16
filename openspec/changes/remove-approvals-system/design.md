## Context

Approval 工作流是 Keel 中最大的横切模块。当前实现:

- **核心包**:`approvals/` 内 `Manager` 接口提供 Create/Approve/Reject/SubscribeApproved/StartExpiryService。
- **Provider 集成**:Kubernetes 与 Helm3 provider 在更新前调用 `approvalManager.Create()`、订阅 `SubscribeApproved` channel 等待批准。`kubernetes.NewProvider(implementer, sender, approvalsManager, grc)` 与 `helm3.NewProvider(impl, sender, approvalsManager)` 直接吃 manager。
- **DefaultProviders**:`provider.New(providers, approvalsManager)` 启动 goroutine `subscribeToApproved` 处理批准事件。
- **HTTP API**:`/v1/approvals` 三个路由,`pkg/http/http.go:37,58,78` 注入 `ApprovalManager`。
- **存储**:GORM 表 `Approvals`,`pkg/store/sql/sql.go:34-37` 在 `AutoMigrate` 中创建;5 个 store 方法承载读写。
- **Audit**:`types/audit.go:13-16` 定义 4 个 `AuditActionApproval*` 常量;`AuditLogStats.Approved/Rejected` 字段汇总到 Dashboard。
- **UI**:Vue 路由 `/approvals`、专属 store module、Dashboard 视图三处使用。
- **bot/**:`bot/approvals.go` 实现 Slack/HipChat 命令 `keel approve <id>`、`keel reject <id>`、`keel list approvals`。已在 [[remove-hipchat-and-chatbot]] 删除。
- **Prometheus**:`cmd/keel/main.go:213-223` 注册 `pending_approvals` Gauge。

用户决策已明确:**彻底删除,不留 stub、不留迁移期、不留参数 deprecation**。历史审批数据通过 `DROP TABLE` 一次性丢弃。

## Goals / Non-Goals

**Goals:**
- 删除所有 `approvals` 命名空间内的 Go 代码、HTTP API、Vue 视图、Bot 命令(后者由 [[remove-hipchat-and-chatbot]] 处理)。
- 缩小 provider/store/http 接口表面,移除 `approvalsManager` 参数与字段。
- 启动时无条件 `DROP TABLE IF EXISTS approvals`,清理历史 DB 残留。
- `go build ./...` 与 `go test ./...` 通过(全部 approval 相关测试一并删除)。

**Non-Goals:**
- **不**保留废弃 annotation 的运行时警告(本 Change 已声明 Breaking)。后续若需要 migration-warner,作为独立 Change。
- **不**提供 GitOps 替代实现——Keel 仅作为 image-update engine,审批的位置交还给 PR/CD 工具。
- **不**做数据导出工具——`audit_logs` 表保留,可继续观察更新历史;但 `approvals` 表本身不导出。
- **不**保留 `/v1/approvals` 返回 410 Gone 的兼容层——一律 404。

## Decisions

### 1. DROP TABLE 在 New() 中无条件执行

```go
// pkg/store/sql/sql.go
func New(opts Opts) (*SQLStore, error) {
    // ... connect ...
    if err := db.Exec("DROP TABLE IF EXISTS approvals").Error; err != nil {
        return nil, err
    }
    err = db.AutoMigrate(&types.AuditLog{}).Error
    // ...
}
```

**理由**:幂等;升级到本版本第一次启动即清理,后续启动 no-op。`IF EXISTS` 保证全新部署不报错。

**备选**:数据库迁移工具(`golang-migrate`)管理 schema 版本。**否决**:Keel 当前不使用 migration framework,引入仅为单次 DROP 不值得。

**风险**:数据不可恢复 → 升级前在 Release Notes 强烈建议运维 `sqlite3 keel.db .dump approvals > backup.sql`。

DROP 失败 MUST 使 store 初始化失败。否则 Keel 会在"approval 已完全移除"的版本中继续带着 legacy `approvals` 表启动,与目标后状态冲突。

### 2. provider.New 与 NewProvider 签名直接破坏

不保留 `approvalsManager nil` 的兼容形式。变更:

```go
// before
func (k *kubernetes.NewProvider)(impl Implementer, sender notification.Sender, am approvals.Manager, grc *k8s.GRC) (*Provider, error)
// after
func (k *kubernetes.NewProvider)(impl Implementer, sender notification.Sender, grc *k8s.GRC) (*Provider, error)
```

```go
// before
func provider.New(providers []Provider, am approvals.Manager) *DefaultProviders
// after
func provider.New(providers []Provider) *DefaultProviders
```

**理由**:Approval 是接口的核心维度,nil 兼容会让调用方误以为可重新启用。一次性 churn 即清。

### 3. UI 路由整体删除

`router.config.js:32-36` 的 `/approvals` 路由删除;`router.config.js:84-88` 的注释残留同步删除;`store/index.js` 不再注册 approvals module;Dashboard `Analysis.vue` 删除 4 处引用:`Pending Approvals` 卡片、`approve/reject` 按钮列、`Required Approvals` 列、`fetchData` 中的 `dispatch('GetApprovals')`。

`store/getters.js` 删除 3 个 getters:`approvalsPending`、`approvalsApprovedCount`、`approvalsRejectedCount`。

### 4. AuditLog 保留;Approval 审计动作删除

`types/audit.go` 中:
- 删除 `AuditActionApprovalApproved/Rejected/Expired/Archived` 四个常量。
- 删除 `AuditResourceKindApproval` 常量。
- `AuditLogStats.Approved/Rejected int` 字段删除。
- `audit.AuditStatistics()` 的 approval action 分支删除(`pkg/store/sql/audit.go`)。
- `pkg/http/stats_endpoint.go` 的 `ApprovalsApproved/Rejected` 字段删除。

**保留**:`audit_logs` 表本身、其他 `AuditAction*` 常量、`/v1/audit` API。

### 5. types/types.go 与 jsonenums 清理

- 删除常量 `KeelMinimumApprovalsLabel`、`KeelApprovalDeadlineLabel`、`KeelApprovalDeadlineDefault`(types.go:63-73)。
- 删除枚举 `TriggerTypeApproval`(types.go:173);更新 `String()` switch 与 `types/triggertype_jsonenums.go` 自动生成文件。
- 删除 `NotificationUpdateApproved`、`NotificationUpdateRejected` 枚举;更新 `types/notification_jsonenums.go`。
- 删除 `types/approvals.go` 整文件(`Approval` struct、`GetApprovalQuery`)。

### 6. Prometheus 指标 pending_approvals 删除

`cmd/keel/main.go:213-223` 整段 `GaugeFunc` 注册删除。**不**替换为 stub。

## Risks / Trade-offs

- **风险**:存量用户依赖 Keel 审批 UI 进行运维 → **缓解**:Release Notes 顶部红字声明 Breaking;给出 GitHub PR Review / GitOps 替代示例。
- **风险**:`DROP TABLE` 误伤 `audit_logs` → **缓解**:DROP 与 AutoMigrate 顺序:**先** DROP 单表,**再** AutoMigrate 显式枚举剩余表。
- **风险**:外部脚本调用 `/v1/approvals` 静默失败 → **缓解**:404 是 HTTP 语义正确;Release Notes 列出所有移除的 endpoint。
- **风险**:`KeelChartConfig`(`provider/helm3/helm3.go`)的 `Approvals`、`ApprovalDeadline` 字段被用户的 Helm values 设置 → **缓解**:本 Change 删除字段;[[refactor-policy-flux-style]] 同时引入新 chart config schema;升级期间用户的 chart 旧字段被 Helm 静默忽略。

## Migration Plan

1. 合并顺序:本 Change 在 [[remove-hipchat-and-chatbot]] 之后(因为 chat bot 内有 `approvals` import,删除顺序保证编译干净)。
2. 数据库备份:Release Notes 提供命令 `sqlite3 /data/keel.db ".dump approvals" > approvals-backup-$(date +%F).sql`。
3. 回滚:`git revert` 可恢复代码;但 `approvals` 表已被 DROP,需运维方从备份恢复。

## Resolved Decisions

- `dashboard.Analysis.vue` 的 `Policy & Approvals Control` 列名改为 `Policy`。

## 1. 删除核心 approvals/ 包

- [x] 1.1 删除文件 `approvals/approvals.go` 与 `approvals/approvals_test.go`(整个目录)。
- [x] 1.2 删除文件 `extension/approval/approval_collector.go`(整个 `extension/approval/` 目录)。
- [x] 1.3 `grep -rn '"github.com/keel-hq/keel/approvals"' --include="*.go" .` 列出所有 import 站点,后续步骤逐一处理。

## 2. 删除 types 中的 Approval 类型

- [x] 2.1 删除文件 `types/approvals.go`(整文件,含 `Approval` struct、`GetApprovalQuery`、`VoteStatus` 枚举)。
- [x] 2.2 修改 `types/types.go:63-73`,删除常量 `KeelMinimumApprovalsLabel`、`KeelApprovalDeadlineLabel`、`KeelApprovalDeadlineDefault`。
- [x] 2.3 修改 `types/types.go:173,182-183`,删除枚举值 `TriggerTypeApproval` 与 `String()` 的对应 case;同步更新自动生成的 `types/triggertype_jsonenums.go`。
- [x] 2.4 删除 `types/types.go` 中的 `NotificationUpdateApproved`、`NotificationUpdateRejected` 枚举值;同步更新 `types/notification_jsonenums.go`。
- [x] 2.5 删除 `types/audit.go:13-16` 的 `AuditActionApprovalApproved/Rejected/Expired/Archived` 四个常量。
- [x] 2.6 删除 `types/audit.go:20` 的 `AuditResourceKindApproval` 常量。
- [x] 2.7 删除 `types/audit.go` 中 `AuditLogStats` struct 的 `Approved`、`Rejected` 字段。
- [x] 2.8 删除 `types/types_test.go` 中的 `TestExpired`、`TestNotExpired`。

## 3. 删除 store 中的 Approval 方法与表

- [x] 3.1 删除文件 `pkg/store/sql/approvals.go`(整文件,5 个方法实现)。
- [x] 3.2 修改 `pkg/store/store.go:15-19`,删除 5 个 Approval 方法声明:`CreateApproval`、`UpdateApproval`、`GetApproval`、`ListApprovals`、`DeleteApproval`。
- [x] 3.3 修改 `pkg/store/sql/sql.go:26-48`:
   - 在 `New()` 中 `connect` 成功后、`AutoMigrate` 之前,插入 `db.Exec("DROP TABLE IF EXISTS approvals")`;失败时返回 error,阻止 store 初始化继续。
   - 从 `AutoMigrate(&types.Approval{}, &types.AuditLog{})` 中移除 `&types.Approval{}`。
- [x] 3.4 修改 `pkg/store/sql/audit.go` 的 `AuditStatistics()` 方法,删除针对 `AuditActionApproval*` 与 `AuditResourceKindApproval` 的分支与计数。
- [x] 3.5 运行 `go test ./pkg/store/...`,确认全部通过(可能需补充 approval 相关测试用例的删除)。

## 4. 删除 HTTP API

- [x] 4.1 删除文件 `pkg/http/approvals_endpoint.go` 与 `pkg/http/approvals_endpoint_test.go`(整两个文件)。
- [x] 4.2 修改 `pkg/http/http.go:37`,删除 `ApprovalManager approvals.Manager` 字段。
- [x] 4.3 修改 `pkg/http/http.go:58,78`,删除 `approvalsManager approvals.Manager` 字段与构造时的 `approvalsManager: opts.ApprovalManager` 赋值。
- [x] 4.4 修改 `pkg/http/http.go` 路由注册段,删除 3 条 `/v1/approvals*` 路由(`GET`、`POST`、`PUT`)。
- [x] 4.5 修改 `pkg/http/stats_endpoint.go`,删除 `ApprovalsApproved`、`ApprovalsRejected` 字段与对应 JSON tag。
- [x] 4.6 修改 `pkg/http/native_webhook_trigger_test.go`,删除 `approver()` helper 与 `am := approvals.New(...)` 创建,调整 `provider.New(...)` 调用签名(不传 approvalsManager)。

## 5. 删除 Provider 中的 Approval 集成

- [x] 5.1 删除文件 `provider/kubernetes/approvals.go` 与 `provider/kubernetes/approvals_test.go`(整两个文件)。
- [x] 5.2 删除文件 `provider/helm3/approvals.go`(整文件)。
- [x] 5.3 修改 `provider/kubernetes/kubernetes.go`:
   - 删除 struct 字段 `approvalManager`(或类似命名)。
   - 修改 `NewProvider(impl, sender, approvalsManager, grc) → NewProvider(impl, sender, grc)`,删除参数。
   - 删除 `checkForApprovals`、`updateComplete` 等调用(若存在)。
- [x] 5.4 修改 `provider/helm3/helm3.go`:
   - 删除 struct 字段 `approvalManager`。
   - 修改 `NewProvider(impl, sender, approvalsManager) → NewProvider(impl, sender)`。
   - 删除 `KeelChartConfig` struct 中的 `Approvals`、`ApprovalDeadline` 字段(在 [[refactor-policy-flux-style]] 中会替换为新字段,本 Change 仅删除)。
- [x] 5.5 修改 `provider/provider.go`:
   - 删除 struct 字段 `approvalsManager`。
   - 修改 `New(providers, approvalsManager) → New(providers)`,删除参数。
   - 删除 `subscribeToApproved` goroutine 与对 `approvedCh` 的 select。
- [x] 5.6 修改 `provider/kubernetes/kubernetes_test.go` 与 `provider/helm3/helm3_test.go`,删除 `approver()` helper、`approvals.New(...)` 创建,调整构造函数调用。

## 6. 删除 trigger 中的 Approval 引用

- [x] 6.1 修改 `trigger/poll/manager_test.go`(line 10, 78, 159):删除 `"github.com/keel-hq/keel/approvals"` import 与三处 `am := approvals.New(...)`,调整 `provider.New(...)` 调用。
- [x] 6.2 修改 `trigger/poll/watcher_test.go`(import 约 line 9;`approvals.New(...)` 约 line 82, 144, 177, 236, 280, 350, 433, 480, 522, 569):同上。
- [x] 6.3 修改 `trigger/poll/multi_tags_watcher_test.go`(line 11, 35, 84, 250):同上。
- [x] 6.4 修改 `trigger/pubsub/manager_test.go`(line 12, 93):同上。

## 7. 删除 cmd/keel/main.go 中的 Approval 连接代码

- [x] 7.1 修改 `cmd/keel/main.go:17`,删除 `"github.com/keel-hq/keel/approvals"` import。
- [x] 7.2 修改 `cmd/keel/main.go:207-225`,删除 `approvalsManager := approvals.New(...)`、`pendindApprovalsCounter` Prometheus 注册、`go approvalsManager.StartExpiryService(ctx)`。
- [x] 7.3 修改 `cmd/keel/main.go:231,258,377`,从 `ProviderOpts`、`TriggerOpts`、`HttpOpts` 中删除 `approvalsManager` / `approvalManager` / `ApprovalManager` 字段与赋值。
- [x] 7.4 修改 `cmd/keel/main.go:299,353` 等位置,删除 `ProviderOpts.approvalsManager`、`TriggerOpts.approvalsManager` 字段声明。
- [x] 7.5 修改 `cmd/keel/main.go:312,331,346`,调整 `kubernetes.NewProvider`、`helm3.NewProvider`、`provider.New` 调用签名(去掉 approvalsManager 参数)。
- [x] 7.6 运行 `go vet ./...` 与 `go build ./...`,确认编译通过。

## 8. 删除 acceptance 测试中的 Approval 用例

- [x] 8.1 修改 `tests/acceptance_test.go`,删除 `TestApprovals`、`TestApprovalsWithAuthentication` 两个测试函数(及相关 helper)。

## 9. 前端 UI 清理

- [x] 9.1 删除整个目录 `ui/src/views/approvals/`(`Approvals.vue`)。
- [x] 9.2 删除文件 `ui/src/store/modules/approvals.js`。
- [x] 9.3 修改 `ui/src/config/router.config.js:32-36`,删除 `/approvals` 路由声明;同时删除 line 84-88 注释中的 stale 引用。
- [x] 9.4 修改 `ui/src/store/index.js`,删除 `import approvals from './modules/approvals'` 与 `approvals` module 注册。
- [x] 9.5 修改 `ui/src/store/getters.js`,删除 3 个 getters:`approvalsPending`、`approvalsApprovedCount`、`approvalsRejectedCount`。
- [x] 9.6 修改 `ui/src/store/modules/resources.js:15-18`,删除对 `_required_approvals` 字段的解析。
- [x] 9.7 修改 `ui/src/views/dashboard/Analysis.vue`:
   - 删除 line 38 的 `<chart-card title="Pending Approvals">` 整块。
   - 删除 line 167-168 的 Approve/Reject 按钮列。
   - 删除 line 235 的 `'Required Approvals'` 列定义。
   - 将 line 252 的 `'Policy & Approvals Control'` 改为 `'Policy'`。
   - 删除 line 381 的 `setApproval(resource, increase)` 方法。
   - 删除 line 404 的 `dispatch('SetApproval', payload)`。
   - 删除 line 431 的 `dispatch('GetApprovals')`。
- [x] 9.8 `cd ui && yarn install && yarn run build`,确认 build 通过且 bundle 不含 `GetApprovals`/`SetApproval` 字符串(`grep -r "GetApprovals" ui/dist/` 应为空)。

## 10. Helm chart 与文档

- [x] 10.1 修改 `chart/keel/values.yaml`,删除 `approvals:` 段(若存在 `enable: true/false`、`expiry` 等)。
- [x] 10.2 修改 `chart/keel/templates/deployment.yaml`,删除 Approval 相关环境变量注入(若有)。
- [x] 10.3 更新 `chart/keel/README.md`,删除 Approvals 章节。
- [x] 10.4 更新 `readme.md`,删除 "Manual Approvals" / `keel.sh/approvals` 注解示例段落;在升级说明中列出 Breaking:annotation 与 API endpoint 失效、DB 表 DROP。
- [x] 10.5 更新 `ARCHITECTURE.md`:
   - 删除 line 152-165 的 Approvals 段。
   - 删除 line 168-175 "Data Flow" 中的 step 4 (Approval check)。
   - 删除 line 183-184 表格中 `keel.sh/approvals`、`keel.sh/approvalDeadline` 行。
   - 删除 line 61 表格中 `approvals/`、`bot/` 行(bot/ 在 [[remove-hipchat-and-chatbot]] 已处理,确认不重复)。
   - 删除 line 273 "Common Tasks" 表中 `Modify approval workflow` 行。

## 11. 测试与验证

- [x] 11.1 运行 `go build ./...` 通过。
- [x] 11.2 运行 `go test ./...` 全部通过(无 `approvals` 包依赖残留)。
- [x] 11.3 `grep -rn "approvalsManager\|ApprovalManager\|approvalManager" --include="*.go" .` 应为空。
- [x] 11.4 `grep -rn '"github.com/keel-hq/keel/approvals"' --include="*.go" .` 应为空。
- [x] 11.5 启动 keel(`docker compose up keel` 或 `make run`):
   - 提交带 `keel.sh/approvals: "2"` 注解的 Deployment + 触发更新事件 → 应直接更新,不出现 approval 等待。
   - `curl -i http://localhost:9300/v1/approvals` → GET/POST/PUT 均应返回 404。
   - `sqlite3 /data/keel.db "SELECT name FROM sqlite_master WHERE type='table';"` → 不应包含 `approvals`。
- [x] 11.6 启动一次,然后退出,再次启动 → 日志中 DROP 应静默(IF EXISTS no-op)。

## 12. 验证 OpenSpec 工件

- [x] 12.1 运行 `openspec validate remove-approvals-system --strict`,确认 4 个 artifact 均通过。
- [x] 12.2 准备 PR 标题:"refactor: remove approvals workflow",PR 描述强调 Breaking、DB 备份建议、GitOps 替代示例。

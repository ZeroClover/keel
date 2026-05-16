## 1. 升级 Masterminds/semver v1 → v3

- [x] 1.1 编辑 `go.mod`,把直接依赖 `github.com/Masterminds/semver v1.5.0` 删除;把 `github.com/Masterminds/semver/v3 v3.3.1 // indirect` 改为直接 `require`(去掉 `// indirect` 标记)。
- [x] 1.2 修改 `util/version/version.go:8`,把 `"github.com/Masterminds/semver"` 改为 `"github.com/Masterminds/semver/v3"`。
- [x] 1.3 修改 `provider/kubernetes/kubernetes.go:9` 与任何其他 `Masterminds/semver` import,统一改为 `/v3`。
- [x] 1.4 全仓库 `grep -rn '"github.com/Masterminds/semver"' --include="*.go" .` 确认无残留(允许 `/v3` 形式)。
- [x] 1.5 运行 `go mod tidy`,确认 `go.mod` 中只剩 `/v3`。
- [x] 1.6 运行 `go build ./...`、`go test ./util/version/...` 通过。验证 v3 API 调用(`NewVersion`、`NewConstraint`、`Prerelease()`、`Major()`、`Minor()`、`Patch()`、`Original()`)行为兼容。

## 2. 新增 Filter 类型

- [x] 2.1 创建文件 `internal/policy/filter.go`,实现 `RegexFilter` struct:
   - 字段 `Regexp *regexp.Regexp`、`Replace string`、`filtered map[string]string`、`items []string`(或等价结构,用于保持输入匹配顺序)。
   - `NewRegexFilter(pattern, replace string) (*RegexFilter, error)`(编译正则失败返回 error)。
   - `Apply(list []string)`:遍历 tags,`FindStringSubmatchIndex` 拿 match;当 `Replace == ""` 时直接用原始 `item` 作为 key,否则用 `Regexp.ExpandString(nil, Replace, item, m)` 计算 key;存 `filtered[key] = item` 并按输入匹配顺序记录 key。
   - `Items() []string`:按输入匹配顺序返回 keys;不得依赖 map iteration。
   - `GetOriginalTag(key string) string`:`return f.filtered[key]`。
- [x] 2.2 创建 `internal/policy/filter_test.go`:覆盖 named capture、`Replace=""` 默认保留原 tag、`Replace="$0"` 显式等价、空 list、非法 regex。
- [x] 2.3 借鉴源:`/tmp/flux-image-reflector-controller/internal/policy/filter.go`。

## 3. 重写 Policy 接口

- [x] 3.1 修改 `internal/policy/policy.go`,把 `Policy` 接口改为:
   ```go
   type Policy interface {
       Name() string
       Type() types.PolicyType
       Latest(candidates []string) (string, error)
   }
   ```
- [x] 3.2 删除 `Options` struct(`MatchTag`、`MatchPreRelease`)。
- [x] 3.3 删除 `LegacyPolicyPopulate(ref *image.Reference) Policy` 函数(`policy.go:143-156`)。
- [x] 3.4 改写 `GetPolicyFromLabelsOrAnnotations(labels, annotations map[string]string) (Policy, Filter, error)`:
   - 读取 `keel.sh/policy`(优先 annotations,回退 labels);当前 legacy key 处理位于 `internal/policy/policy.go:115` 附近,改为若只存在 `keel.observer/policy` 则返回 `(nil, nil, errUnsupportedPolicy)`。
   - 旧值(`major`/`minor`/`patch`/`all`/`glob:*`/`regexp:*`)→ 返回 `(nil, nil, errUnsupportedPolicy)`。
   - 检测 `keel.sh/matchTag`、`keel.sh/match-tag`、`keel.sh/matchPreRelease` 任一存在 → 返回 `(nil, nil, errUnsupportedPolicy)`。
   - 新值解析为 SemVer / Alphabetical / Numerical / Force;空值/never → `(nil, nil, nil)`。
   - 若 `keel.sh/filterTags` 非空,创建 `RegexFilter`;`extract` 缺省为空字符串,由 `RegexFilter.Apply` 保留完整原始 tag。
   - 解析失败返回 error。
- [x] 3.5 删除 `policy.go` 中残留的 `GetPolicy(string, *Options) Policy`、`ParseSemverPolicy(string, bool)` 等旧函数;若有外部调用 grep 后改写。
- [x] 3.6 删除 `NilPolicy` dummy object。调用方必须用 `policy == nil` 表示"未启用/never,静默跳过";legacy/invalid policy 必须走 error 分支并记录 ERROR 后跳过资源。
- [x] 3.7 修改 `types/tracked_images.go`:
   - 把 `types.Policy` 接口同步改为 `(Name, Type, Latest)` 三方法,删除 `ShouldUpdate`、`Filter`、`KeepTag`。
   - 新增 `types.Filter` 接口,方法与 `internal/policy.Filter` 相同。
   - 给 `TrackedImage` 新增 `Filter types.Filter` 字段。不得从 `types` 包 import `internal/policy`,避免 import cycle。

## 4. 实现 SemVer Policy v2

- [x] 4.1 重写 `internal/policy/semver.go`:
   ```go
   type SemVer struct {
       Range      string
       constraint *semver.Constraints
   }
   func NewSemVer(r string) (*SemVer, error) { ... }
   func (p *SemVer) Name() string { return "semver" }
   func (p *SemVer) Type() types.PolicyType { return types.PolicyTypeSemver }
   func (p *SemVer) Latest(versions []string) (string, error) {
       var latest *semver.Version
       for _, tag := range versions {
           v, err := semver.NewVersion(tag); if err != nil { continue }
           if p.constraint.Check(v) && (latest == nil || v.GreaterThan(latest)) {
               latest = v
           }
       }
       if latest == nil { return "", errNoMatch }
       return latest.Original(), nil
   }
   ```
- [x] 4.2 删除文件 `internal/policy/semverpolicytype_jsonenums.go`(`SemverPolicyType` 枚举废弃)。
- [x] 4.3 重写 `internal/policy/semver_test.go`:覆盖 `>=1.0.0` vs `>=1.0.0-0`、`^1`、`~1.2`、`>=1.0.0, <2.0`、非法 constraint、空候选集。

## 5. 实现 Alphabetical Policy

- [x] 5.1 创建 `internal/policy/alphabetical.go`:
   ```go
   type Alphabetical struct { Order string /* "asc" | "desc" */ }
   func NewAlphabetical(order string) (*Alphabetical, error)  // 默认 "asc"(Flux 行为)
   func (p *Alphabetical) Latest(list []string) (string, error)
   func (p *Alphabetical) Name() string { return "alphabetical" }
   func (p *Alphabetical) Type() types.PolicyType { return types.PolicyTypeAlphabetical }
   ```
- [x] 5.2 实现稳定字典序;默认/`asc` 选择字典序最小值,`desc` 选择字典序最大值(对齐 Flux `NewAlphabetical("")` 默认 ASC)。
- [x] 5.3 创建 `internal/policy/alphabetical_test.go`:覆盖 asc/desc、字符串带 `-` / 数字混合、空候选。
- [x] 5.4 借鉴源:`/tmp/flux-image-reflector-controller/internal/policy/alphabetical.go`。

## 6. 实现 Numerical Policy

- [x] 6.1 创建 `internal/policy/numerical.go`:
   ```go
   type Numerical struct { Order string }
   func NewNumerical(order string) (*Numerical, error)  // 默认 "asc"(Flux 行为)
   func (p *Numerical) Latest(list []string) (string, error)  // strconv.ParseInt 失败立即报错
   ```
- [x] 6.2 fail-fast 行为:遇到无法 ParseInt 的元素返回 error,**不**降级为字符串排序。
- [x] 6.3 创建 `internal/policy/numerical_test.go`:覆盖 asc/desc、负数、空候选、非数字元素 fail-fast。
- [x] 6.4 借鉴源:`/tmp/flux-image-reflector-controller/internal/policy/numerical.go`。

## 7. 重写 Force Policy

- [x] 7.1 重写 `internal/policy/force.go`:
   ```go
   type Force struct{}
   func NewForce() *Force { return &Force{} }
   func (p *Force) Name() string { return "force" }
   func (p *Force) Type() types.PolicyType { return types.PolicyTypeForce }
   func (p *Force) Latest(candidates []string) (string, error) {
       if len(candidates) == 0 { return "", errEmpty }
       return candidates[0], nil
   }
   ```
- [x] 7.2 移除 `KeepTag()` / `matchTag` 字段(`force.go:5-13,33`)。
- [x] 7.3 重写 `internal/policy/force_test.go`:覆盖空候选、单元素、多元素返回首位。

## 8. 删除 Glob / Regexp 旧实现

- [x] 8.1 删除文件 `internal/policy/glob.go`、`internal/policy/glob_test.go`、`internal/policy/regexp.go`、`internal/policy/regexp_test.go`(若存在)。
- [x] 8.2 修改 `internal/policy/policy_test.go`:删除针对旧 Policy 的测试(`GlobPolicy`、`RegexpPolicy`);保留改写后的 `GetPolicyFromLabelsOrAnnotations` 测试,新增解析 `semver:>=1.0.0-0`、`alphabetical:asc`、`numerical:desc`、`force` 的用例,并覆盖 `keel.observer/policy` / 旧 policy 值 / 旧 match 注解返回 error。

## 9. types 与 constants 调整

- [x] 9.1 修改 `types/types.go:382-390`,`PolicyType` 枚举:
   - 删除 `PolicyTypeGlob`、`PolicyTypeRegexp`。
   - 新增 `PolicyTypeAlphabetical`、`PolicyTypeNumerical`。
   - 重排枚举值(避免破坏 jsonenums)。同步更新 `types/policytype_jsonenums.go`。
- [x] 9.2 修改 `types/types.go`,删除常量 `KeelForceTagMatchLegacyLabel`、`KeelForceTagMatchLabel`、`KeelMatchPreReleaseAnnotation`。
- [x] 9.3 修改 `types/types.go`,新增常量 `KeelFilterTagsAnnotation = "keel.sh/filterTags"`、`KeelExtractAnnotation = "keel.sh/extract"`。

## 10. KeelChartConfig 调整

- [x] 10.1 修改 `provider/helm3/helm3.go::KeelChartConfig`:
   - 删除字段 `MatchTag`、`MatchPreRelease`。
   - 保留现有字段 `Policy string \`json:"policy"\``(语义改为新 policy 语法)。
   - 新增字段 `FilterTags string \`json:"filterTags"\``、`Extract string \`json:"extract"\``。
   - 保留现有字段 `Trigger types.TriggerType \`json:"trigger"\``。
   - 保留现有字段 `NotificationChannels []string \`json:"notificationChannels"\``;不得改名为 `NotificationChan` 或改为 `json:"notify"`。
   - 保留/更新内部字段 `Plc types.Policy \`json:"-"\`` 并新增 `Filter types.Filter \`json:"-"\``。
   - (验证字段 `Approvals` / `ApprovalDeadline` 已由 [[remove-approvals-system]] 删除)。
- [x] 10.2 修改 `provider/helm3/helm3.go` 中读取 chart values 后构造 policy 的函数,改为调用 `GetPolicyFromLabelsOrAnnotations` 等价逻辑(通过把 `KeelChartConfig` 三字段塞入临时 annotation map)。
- [x] 10.3 `chart/keel/values.yaml` 与 `chart/keel/README.md`:删除 `matchTag`、`matchPreRelease` 说明;新增 `policy`、`filterTags`、`extract` 示例(三种 policy 各一)。

## 11. Watcher 与 Provider 调用方桥接

- [x] 11.1 修改 `trigger/poll/multi_tags_watcher.go::computeEvents`(line 85 附近),改写为:
   ```go
   // 1. 用 Filter.Apply(tags) 过滤(若 filter 为 nil,直接用 tags)
   // 2. 取 filter.Items() 或 tags
   // 3. latestKey, err := policy.Latest(candidates)
   // 4. 若 filter 非 nil,latest := filter.GetOriginalTag(latestKey),否则 latest := latestKey
   // 5. 与 current 比较,不同则发 event
   ```
   注意:Force policy 路径下,本 Change 暂以 registry 返回顺序作为候选;[[refactor-watcher-and-force-policy]] 将插入 created-time 排序。
- [x] 11.2 修改 `trigger/poll/watcher.go::Watch`,删除 `policy.KeepTag()` 调用(line 95-100、175、241、255);`getImageIdentifier(ref, keepTag)` 简化为只用 `registry+name`。
- [x] 11.3 修改 `trigger/poll/multi_tags_watcher.go::getRelatedTrackedImages`,删除对 `Policy.KeepTag()` 的调用,只按单参数 `getImageIdentifier(x.Image)` 聚合相关 tracked images。
- [x] 11.4 在 provider 公用位置或各 provider 包内实现最小 external admission helper,供 Kubernetes/Helm webhook/pubsub 路径替代旧 `ShouldUpdate`;行为按 [[refactor-watcher-and-force-policy]] 的 `provider-update-decision` spec:
   - nil policy 或空 event tag 拒绝。
   - Force policy 只检查 `filterTags` 是否放行 event tag,不得调用 `Latest([currentTag,eventTag])`。
   - Non-Force policy 对 `[currentTag,eventTag]` 应用 Filter;event tag 被过滤则拒绝;current tag 被过滤不应单独拒绝。
   - Non-Force policy 调用 `Latest(candidates)`,只有选中的 original tag 等于 event tag 才允许。
- [x] 11.5 修改 `provider/kubernetes/updates.go::checkForUpdate`(若调用 `policy.ShouldUpdate`),改为先做 repo/tag diff,再从资源 annotations/labels 解析 `(Policy, Filter, error)` 并调用 `policyAllowsExternalTag(...)`;parse error 时记录 ERROR 并拒绝该事件,nil Policy 时静默拒绝。不得用 `repo match && tag diff` 直接绕过 policy。本 Change 末态可对 poll/webhook/pubsub 统一执行 helper;[[refactor-watcher-and-force-policy]] 再加入 poll 快路径以避免二次 `Latest`。
- [x] 11.6 修改 `provider/helm3/updates.go` 同上调整:从 release values 解析 `(Policy, Filter, error)`,parse error 时记录 ERROR 并拒绝该事件,nil Policy 时静默拒绝。
- [x] 11.7 修改全部 `provider/{kubernetes,helm3}` 中遗留的 `policy.ShouldUpdate` 调用,逐一替换为 `policyAllowsExternalTag(...)` 或等价 policy-aware helper。
- [x] 11.8 修改 `provider/{kubernetes,helm3}/kubernetes.go,helm3.go::TrackedImages()`,在解析 annotation / Helm values 时接收 `(Policy, Filter, error)`;error 时记录 ERROR 并跳过,nil Policy 时静默跳过,否则把 Filter 写入 `TrackedImage.Filter`。
- [x] 11.9 运行 `go build ./...` 与 `go test ./...`,确认全部通过(可能需要更新现有测试 fixture)。

## 12. 文档与示例

- [x] 12.1 更新 `readme.md`:
   - "Policy" 段落重写,列出 4 种新值与示例;
   - "Upgrading from <previous>" 段落给出完整迁移映射表(major→`semver:^X`、minor→`semver:^X.Y`、patch→`semver:~X.Y.Z`、all→`semver:>=0.0.0-0`、glob→alphabetical+filterTags、regexp→numerical/alphabetical+filterTags+extract)。
- [x] 12.2 更新 `ARCHITECTURE.md::Policies` 段(line 113-134),用新 PolicyType 列表替换。
- [x] 12.3 更新 `chart/keel/README.md`,删除 matchTag/matchPreRelease;新增 policy/filterTags/extract 章节,给出 dev/rc/stable 三套场景示例。

## 13. OpenSpec 工件验证

- [x] 13.1 运行 `openspec validate refactor-policy-flux-style --strict`,确认 4 个 artifact 均通过。
- [x] 13.2 准备 PR 标题:"refactor: replace policy system with Flux-style schema",PR 描述附完整 Breaking Annotation 映射表。

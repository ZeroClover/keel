## Context

### 当前 Policy 系统

`internal/policy/policy.go:14-20` 定义 `Policy` 接口:

```go
type Policy interface {
    ShouldUpdate(current, new string) (bool, error)
    Name() string
    Type() types.PolicyType
    Filter(tags []string) []string
    KeepTag() bool
}
```

`types/tracked_images.go` 也定义了一个结构等价的 `types.Policy` 接口,并由 `types.TrackedImage.Policy` 使用。重写 policy 接口时必须同步修改这两个接口;否则 `internal/policy` 的实现删掉 `ShouldUpdate` / `Filter` / `KeepTag` 后,`TrackedImage.Policy` 仍要求旧方法,PR 末态会编译失败。

调用模型:Watcher 对每个候选 tag 调用 `ShouldUpdate(current, tag)`,返回 true 的取走。这有几个问题:

- `ShouldUpdate` 把"过滤"与"选择"耦合,且只在 Watcher 内嵌入排序;
- `Filter()` 与 `ShouldUpdate` 重复:Glob/Regexp `Filter()` 调字符串排序后返回排好序的列表,Watcher 取 `Filter()[0]` 作为 latest;
- `KeepTag()` 是 Force policy 的特殊语义("matchTag=true 时不切换 tag,仅 watch digest"),硬塞进基类接口;
- pre-release 匹配通过 `matchPreRelease` 注解 + ad-hoc 字符串等值。

### Flux 模型(借鉴目标)

`fluxcd/image-reflector-controller/internal/policy/`:

```go
type Policer interface {
    Name() string
    Latest(versions []string) (string, error)
}

type Filter interface {
    Apply(list []string)
    Items() []string
    GetOriginalTag(key string) string
}
```

四种 Policer:`SemVer`(基于 `Masterminds/semver/v3` 约束)、`Alphabetical`(asc/desc)、`Numerical`(asc/desc,fail-fast)。Filter 仅 1 种(`RegexFilter`,带 `Replace` 模板),通过 `keel.sh/filterTags` + `keel.sh/extract` 控制。

调用模型:Watcher 拿到所有候选 tags → 经 `Filter.Apply()` → 取 `Filter.Items()` 得到 extracted key 列表 → 传给 `Policy.Latest()` → 拿到 latest key → `Filter.GetOriginalTag(key)` 反查原始 tag → 与 current 比较。

### 与本仓库的整合点

- **Watcher 调用方**:`trigger/poll/multi_tags_watcher.go::computeEvents` 必须改写,但完整的 Force-by-created-time 改造在 [[refactor-watcher-and-force-policy]]。本 Change 内 watcher 改写为"用新接口选 latest,Force 路径维持现有 tag-list 顺序",作为 bridge。
- **Provider 调用方**:`provider/{kubernetes,helm3}/updates.go::checkForUpdate` 当前调 `policy.ShouldUpdate(current, eventTag)`。新接口删除 `ShouldUpdate`,但 webhook/pubsub 事件仍必须经过资源声明的 Policy/Filter 校验;完整的 event-origin 分流在 [[refactor-watcher-and-force-policy]]。
- **Helm Chart**:`KeelChartConfig` 三字段调整需要本 Change 完成。

## Goals / Non-Goals

**Goals:**
- 修复三类已知 bug(pre-release 等值、字典序排序、Force 不排序)。
- 用四个 Policy + 一个 Filter 描述全部用法。
- 升级 `Masterminds/semver` 到 v3,统一仓库内 semver 版本。
- 新 annotation schema 简单、可读、与 Flux `imagepolicies.image.toolkit.fluxcd.io` CRD 对齐。
- 旧 annotation / 旧 policy 值 fail-fast 返回 error;最外层调用者记录 ERROR 迁移日志并跳过该资源,**不**抛 panic。

**Non-Goals:**
- **不**实现 Force-by-created-time 的 Registry 调用——见 [[refactor-watcher-and-force-policy]]。
- **不**改 `trigger/pubsub/` 与 `pkg/http/*_webhook_trigger.go` 的事件 schema——webhook/pubsub 仍提供 tag,但 provider 必须按资源 Policy/Filter 校验该 tag 后才能更新。
- **不**为旧 annotation 提供自动迁移工具——文档列举映射表足够。
- **不**在 chart 模板内强制校验 `keel.sh/policy` 值合法性——运行时已记录 ERROR。

## Decisions

### 1. 新 annotation schema(对齐 Flux)

```yaml
metadata:
  annotations:
    keel.sh/policy: "semver:>=1.0.0-0"           # 互斥四选一
    keel.sh/filterTags: "^v(?P<v>\\d+\\.\\d+\\.\\d+)$"  # 可选
    keel.sh/extract: "$v"                          # 可选
```

`keel.sh/policy` 取值文法(EBNF):

```
policy   ::= "semver:" constraint
           | "alphabetical" ( ":" ( "asc" | "desc" ) )?
           | "numerical"    ( ":" ( "asc" | "desc" ) )?
           | "force"
           | "never"
```

`constraint` 是 `Masterminds/semver/v3` 接受的字符串(如 `>=1.0.0-0`、`^1.2`、`~1.2.3`、`>=1.0.0-0, <2.0`)。

**理由**:
- Flux 用户(GitOps 主流)零学习成本;
- `semver:^1.2` 一句话表达旧 `minor` 语义,`semver:~1.2.3` 表达 `patch` 语义,`semver:>=0.0.0` 表达 `all`,旧四种"档位"统一为 constraint;
- `keel.sh/filterTags` + `keel.sh/extract` 用单组命名捕获(`(?P<v>...)`)解决 commit-timestamp tag / dev-rc-stable 隔离等复杂场景。

**备选方案**:保留旧策略名,加 `keel.sh/preReleaseInclude: true` 修复 bug。**否决**:旧策略名隐藏了"档位是否含 pre-release"的复杂度,且无法表达 range constraint;一次性破坏不如多次。

### 2. SemVer v3 升级 + Constraint.Check 默认行为

`Masterminds/semver/v3` 在 `Constraint.Check(v)` 中默认**排除** pre-release:`>=1.0.0` 不匹配 `1.0.0-rc.1`。要包含 pre-release,需 `>=1.0.0-0`(即"高于或等于第一个 pre-release")。

**对外承诺**:文档中显式示例:

| 用户意图 | 推荐 annotation |
|---|---|
| 仅 stable 版本 | `keel.sh/policy: "semver:>=0.0.0"` |
| 包含 pre-release | `keel.sh/policy: "semver:>=0.0.0-0"` |
| 锁 1.x major | `keel.sh/policy: "semver:^1"` |
| 锁 1.2.x patch | `keel.sh/policy: "semver:~1.2"` |
| 任何主线 dev tag | `keel.sh/policy: "alphabetical:desc"` + `filterTags: ".*-dev$"` |

### 3. Filter 实现:命名捕获组 + ExpandString

```go
type RegexFilter struct {
    filtered map[string]string  // extracted → original
    Regexp   *regexp.Regexp
    Replace  string              // "$v" / "${ts}" 等模板
}

func (f *RegexFilter) Apply(list []string) {
    f.filtered = map[string]string{}
    for _, item := range list {
        m := f.Regexp.FindStringSubmatchIndex(item)
        if m == nil { continue }
        key := item
        if f.Replace != "" {
            // 把 Replace 模板中的命名组替换为实际值
            result := f.Regexp.ExpandString(nil, f.Replace, item, m)
            key = string(result)
        }
        f.filtered[key] = item
    }
}
func (f *RegexFilter) Items() []string { /* keys */ }
func (f *RegexFilter) GetOriginalTag(k string) string { return f.filtered[k] }
```

**理由**:`ExpandString` 是 Go 标准库,已被 Flux 验证;不引入新依赖。与 Flux 一致,`Replace=""` 时保留原始 tag 作为 key;仅当 `keel.sh/extract` 非空时才使用 `ExpandString` 生成 extracted key。Keel 的 `Items()` 必须按输入匹配顺序返回 keys,避免 map iteration 让测试和事件选择不稳定。

### 4. Force Policy 在本 Change 是"占位"

```go
type Force struct{}
func (p *Force) Latest(candidates []string) (string, error) {
    if len(candidates) == 0 { return "", errEmpty }
    return candidates[0], nil
}
```

调用方必须**预先排序**好(本 Change 下,maintain 现有 tag-list 顺序,即 registry 返回顺序——这意味着 Force policy 在本 Change 后仍**不确定**)。[[refactor-watcher-and-force-policy]] 才补上 created-time 排序,使 Force 真正可靠。

**理由**:依赖切片清晰——semver v3 升级与 policy 接口重写是一个独立 PR,registry/watcher 改造再叠加。把"修 bug"分两阶段。

### 5. 旧 annotation fail-fast error + 外层 ERROR 日志

Parser 顺序:

```go
func GetPolicyFromLabelsOrAnnotations(...) (Policy, Filter, error) {
    raw, fromLegacyKey := readPolicy("keel.sh/policy", "keel.observer/policy")
    if fromLegacyKey {
        return nil, nil, errUnsupportedPolicy
    }
    // 检测旧值并 ERROR
    if raw == "major" || raw == "minor" || raw == "patch" || raw == "all" ||
       strings.HasPrefix(raw, "glob:") || strings.HasPrefix(raw, "regexp:") {
        return nil, nil, errUnsupportedPolicy
    }
    // 检测旧 annotation 并 ERROR
    if hasAnnotation("keel.sh/matchTag") || hasAnnotation("keel.sh/match-tag") {
        return nil, nil, errUnsupportedPolicy
    }
    if hasAnnotation("keel.sh/matchPreRelease") {
        return nil, nil, errUnsupportedPolicy
    }
    if raw == "" || raw == "never" {
        return nil, nil, nil
    }
    // 新解析
    return parseNew(raw)
}
```

**理由**:`NilPolicy` 不能同时表达"用户没有启用 Keel"与"用户配置了已废弃/非法策略"。旧配置和解析失败是配置错误,应返回 error;Watcher / Provider / Helm provider 捕获后记录显眼 ERROR 并 `continue` 跳过该资源,这样既不会阻塞 Keel 进程,也不会把配置错误伪装成正常禁用更新。

### 6. Helm KeelChartConfig 字段更新

```go
// provider/helm3/helm3.go
type KeelChartConfig struct {
    Policy               string   `json:"policy"`            // "semver:>=1.0.0-0"
    FilterTags           string   `json:"filterTags"`
    Extract              string   `json:"extract"`
    Trigger              types.TriggerType `json:"trigger"`
    PollSchedule         string   `json:"pollSchedule"`
    Images               []ImageDetails `json:"images"`
    NotificationChannels []string `json:"notificationChannels"` // 保持现有字段名 / 类型 / JSON tag
    Plc                  types.Policy `json:"-"`
    Filter               types.Filter `json:"-"`
    // 删除:Approvals, ApprovalDeadline (Change 2 已删),
    //       MatchTag, MatchPreRelease (本 Change 删)
}
```

values.yaml 旧字段 `keel.matchTag: true` 等被静默忽略;[[refactor-policy-flux-style]] 不做 chart-side 校验。

## Risks / Trade-offs

- **风险**:用户旧 annotation 全失效 → **缓解**:`readme.md` 顶部 Upgrade 段提供完整映射表;调用方 ERROR 日志给出迁移指引;该资源被跳过但 Keel 进程继续运行。
- **风险**:`Masterminds/semver/v3` 默认排除 pre-release,用户首次升级后"该升的版本不升" → **缓解**:文档显式 `>=X.Y.Z-0` 表;PR 描述与 release notes 重点高亮。
- **风险**:Force policy 在本 Change 完成后仍不确定 → **缓解**:本 Change PR 描述写明"Force 的真正修复见 PR4";避免被误解为"已修"。
- **风险**:`internal/policy/semverpolicytype_jsonenums.go` 删除后外部消费方(若有)失效 → **缓解**:该枚举仅 internal package 使用,grep 全仓库确认无外部消费。
- **风险**:`util/version/version.go` 仍用 v1,会双版本共存 → **缓解**:作为同 Change 任务一并升级到 v3,验证 `NewAvailable` API 行为兼容。

## Migration Plan

1. PR 顺序:[[remove-hipchat-and-chatbot]] → [[remove-approvals-system]] → **本 Change** → [[refactor-watcher-and-force-policy]]。
2. 单元测试在本 Change 内全部跑通(新 policy/filter 全覆盖);集成测试在 Change 4 完成后跑。
3. 用户升级步骤:
   - 把 `keel.sh/policy: major` → `keel.sh/policy: "semver:>=0.0.0-0"`(若需 pre-release)或 `semver:>=0.0.0`(仅 stable)。
   - 把 `keel.sh/policy: minor` → `keel.sh/policy: "semver:^X.Y"`(锁主版本)。
   - 把 `keel.sh/policy: patch` → `keel.sh/policy: "semver:~X.Y.Z"`。
   - 把 `keel.sh/policy: glob:release-*` → `keel.sh/policy: "alphabetical:desc"` + `keel.sh/filterTags: "^release-.*$"`。
	   - 把 `keel.sh/policy: regexp:^build-([0-9]+)$` → `keel.sh/policy: "numerical:desc"` + `keel.sh/filterTags: "^build-([0-9]+)$"` + `keel.sh/extract: "$1"`。
	   - 删除所有 `keel.sh/matchTag` / `keel.sh/match-tag` / `keel.sh/matchPreRelease` 注解。
	   - 把 legacy key `keel.observer/policy` 改为 `keel.sh/policy` 并使用新语法。

## Resolved Decisions

- `keel.sh/policy: "alphabetical"` 与 `keel.sh/policy: "numerical"` 无 order suffix 时默认 `asc`,对齐 Flux `NewAlphabetical("")` / `NewNumerical("")` 行为。
- `semver_test.go` 必须包含 `>=1.0.0` vs `>=1.0.0-0` 对比测试,固定 semver v3 pre-release 行为。

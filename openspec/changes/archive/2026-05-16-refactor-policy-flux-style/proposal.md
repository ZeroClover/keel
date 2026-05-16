## Why

Keel 当前的 Policy 子系统(`internal/policy/`)存在 3 类**事实性 bug**:

1. **SemVer pre-release 比较错误**(`semver.go:94`):用整段 pre-release 字符串等值比较,导致 `1.0.0-dev.rc1 → 1.0.0-dev.rc2` 在 `minor`/`patch` 策略下被错误拒绝;`matchPreRelease=true` 的"全等"语义实际无法处理同通道内的版本递增。
2. **Glob/Regexp 用字典序排序**(`regexp.go:51-59`、`glob.go:32,45-48`):`dev-9` 被认为大于 `dev-10`,导致 CI 主线 tag 推进时跳号。
3. **Force policy 不排序**(`force.go:22-25`):`Filter()` 直接返回 tags 原顺序;Docker Registry V2 spec **不保证 tag 顺序**,使 `force` policy 选出的 latest tag 因 registry 实现差异(Docker Hub / GHCR / GCR / ECR)而不同,行为不确定。

此外,`Masterminds/semver` 依赖仍停留在 v1(2018 年),v3 已成为社区标准。`go.mod` 已间接引入 v3,主代码却仍用 v1,造成两个版本共存。

本 Change **彻底重构 Policy 体系,对齐 Flux ImagePolicy** (`fluxcd/image-reflector-controller`)。新引入 `Filter`(可选,基于命名捕获组的正则过滤+提取)与 4 种 Policy(`semver` / `alphabetical` / `numerical` / `force`)。废弃旧的 `glob` / `regexp` / `all|major|minor|patch` 策略名与 `matchPreRelease` / `matchTag` 注解。Force policy 的"按 created time 排序"语义所需的 Registry 扩展与 Watcher 改造由 [[refactor-watcher-and-force-policy]] 完成。

## What Changes

- **BREAKING(annotation)**:
  - 新增 `keel.sh/policy` 值语法:`semver:<constraint>` / `alphabetical:<asc|desc>` / `numerical:<asc|desc>` / `force` 四选一(互斥)。
  - 新增 `keel.sh/filterTags`(可选)与 `keel.sh/extract`(可选,配合 filterTags 命名捕获组)。
  - 删除注解:`keel.sh/matchPreRelease`、`keel.sh/matchTag`、`keel.sh/match-tag`。
  - 废弃策略名:`major`、`minor`、`patch`、`all`、`glob:*`、`regexp:*`,以及 legacy key `keel.observer/policy`。当用户保留这些值/key 时,policy 解析返回 `(nil, nil, errUnsupportedPolicy)`;最外层调用者记录 `ERROR` 级别迁移指引并跳过该资源。
- **BREAKING(Go API)**:
  - 重写 `internal/policy/policy.go` 与 `types/tracked_images.go` 中的 `Policy` 接口:由 `(ShouldUpdate, Name, Type, Filter, KeepTag)` 改为 `(Name, Type, Latest(candidates []string) (string, error))`。
  - 新增 `Filter` 接口:`Apply(tags []string)` / `Items() []string` / `GetOriginalTag(key string) string`;`types.TrackedImage` 新增 `Filter types.Filter` 字段。
  - 删除 `Options.MatchTag`、`Options.MatchPreRelease`、`LegacyPolicyPopulate()`。
  - `GetPolicyFromLabelsOrAnnotations(...)` 返回 `(Policy, Filter, error)`。
- **BREAKING(依赖)**:
  - `github.com/Masterminds/semver v1.5.0` 升级到 `github.com/Masterminds/semver/v3 v3.3.1`(已存在的间接依赖提升为直接依赖)。
  - v3 中 `Constraint.Check(v)` 默认排除 pre-release;需用 `>=X.Y.Z-0` 形式显式包含。
- **新文件**:
  - `internal/policy/filter.go`:借鉴 `image-reflector-controller/internal/policy/filter.go`,基于 `regexp.ExpandString` 实现命名捕获组替换。
  - `internal/policy/alphabetical.go`:支持 `asc`(默认,对齐 Flux)与 `desc`,稳定的字符串字典序。
  - `internal/policy/numerical.go`:支持 `asc`(默认,对齐 Flux)与 `desc`,fail-fast(非数字直接报错,不 fallback 字符串)。
  - 重写后 `internal/policy/semver.go`、`internal/policy/force.go`、`internal/policy/policy.go`。
- **删除文件**:`internal/policy/glob.go`、`internal/policy/regexp.go`、`internal/policy/glob_test.go`、`internal/policy/semverpolicytype_jsonenums.go`(`SemverPolicyType` 枚举与四个内部值一并废弃)。
- **types**:
  - `types/policy.go`(或 types.go 内的 PolicyType 枚举)调整:`PolicyTypeGlob`、`PolicyTypeRegexp` 删除;新增 `PolicyTypeAlphabetical`、`PolicyTypeNumerical`。
  - `types/types.go` 删除常量:`KeelForceTagMatchLegacyLabel`、`KeelForceTagMatchLabel`、`KeelMatchPreReleaseAnnotation`。
  - 新增常量:`KeelFilterTagsAnnotation = "keel.sh/filterTags"`、`KeelExtractAnnotation = "keel.sh/extract"`。
- **`provider/helm3/helm3.go::KeelChartConfig`**:保留现有 `Policy string` 字段但改为新语法解析;删除字段 `MatchTag`、`MatchPreRelease`;新增字段 `FilterTags`(string)、`Extract`(string)。

## Capabilities

### New Capabilities
- `image-policy`: 描述新的 Policy/Filter 接口、annotation schema 与四种 policy 的语义,作为后续 [[refactor-watcher-and-force-policy]] 调度的依据。
- `helm-chart-config`: 描述 Helm `KeelChartConfig` 字段集与从 chart values 解析出 Policy/Filter 的过程。

### Modified Capabilities
无。本批 OpenSpec changes 以空 `openspec/specs/` 为起点创建;归档顺序假设为:[[remove-hipchat-and-chatbot]] → [[remove-approvals-system]] → 本 Change → [[refactor-watcher-and-force-policy]]。后续 Change 消费本 Change 新建的 `image-policy` / `helm-chart-config` 契约。

## Impact

- **代码**:重写 ~3 个核心文件(policy.go/semver.go/force.go),新增 3 个文件(filter.go/alphabetical.go/numerical.go),删除 2 个旧策略实现(glob/regexp)。
- **依赖**:`go.mod` 升级 semver 到 v3;`go mod tidy` 后 v1 完全消失。
- **接口**:`Policy` 与 `Filter` 是后续 [[refactor-watcher-and-force-policy]] 的输入契约,本 Change 同步重写 watcher 调用以保证编译,但**完整的 Watcher 数据流改造在下一 Change**。本 Change 完成后,`trigger/poll/multi_tags_watcher.go` 必须使用新 `Latest+Filter` 接口编译通过、单元测试通过(集成新行为)。
- **向后兼容**:无。用户必须按 README 升级指南改写 annotation 与 Helm values;旧 annotation / 旧 policy 值解析为 error,调用方输出 ERROR 日志并跳过该资源。
- **关联 Change**:依赖 [[remove-approvals-system]] 已删除 `KeelChartConfig.Approvals`/`ApprovalDeadline` 字段,本 Change 才能干净地新增字段。

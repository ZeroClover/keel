## Context

### Force policy 的可预测性问题

Docker Registry HTTP API V2 `GET /v2/<name>/tags/list` 响应:

```json
{ "name": "library/nginx", "tags": ["latest", "1.25", "1.25.3", ...] }
```

但 [V2 spec](https://docs.docker.com/registry/spec/api/#tags) **没有规定**这个数组的顺序:

- Docker Hub:按 push 时间降序(实测)。
- GitHub Container Registry:按字典序升序。
- Google Container Registry:无固定顺序。
- AWS ECR:lexicographic。
- Harbor:按数据库 PK 升序。

[[refactor-policy-flux-style]] 中的 Force `Latest()` 取 `candidates[0]`,因此**同一镜像**在不同 registry 选出不同 latest。这是历史 bug `force.go:22-25` 的根因。

### 修复路径

每个 OCI/Docker image 的 config blob 内有 `created` 字段(RFC3339 时间戳)。fetch 顺序:

```
GET /v2/<repo>/manifests/<tag>
  → 解析 mediaType
  → 若是 application/vnd.docker.distribution.manifest.v2+json → 取 .config.digest
  → 若是 application/vnd.oci.image.index.v1+json → 选第一个 manifest,递归
  → 若是 application/vnd.docker.distribution.manifest.list.v2+json → 同上
GET /v2/<repo>/blobs/<config-digest>
  → JSON 解析 .created
```

每次 poll 增加 `2N` 次 HTTP 请求(N=候选 tag 数)。对默认 1 min poll 间隔与 typical N<50 可接受,但需缓存。

### Watcher 路径统一

当前 Watcher 曾有两条路径:

- `WatchTagJob`(`single_tag_watcher.go`):用于 force+matchTag=true,watch 单 tag 的 digest。
- `WatchRepositoryTagsJob`(`multi_tags_watcher.go`):用于其他 policy,fetch tag list 后 filter+select。

[[refactor-policy-flux-style]] 已删除 matchTag annotation 与 `KeepTag()` 调用入口,`WatchTagJob` 失去入口。本 Change 删除残留文件并确认所有 watcher 都走 `WatchRepositoryTagsJob`。

## Goals / Non-Goals

**Goals:**
- Force policy 在所有支持 OCI image config 的 registry 上行为可预测(按 created time 降序)。
- Watcher 只有一条数据流:`tags → Filter → candidates → policy.Latest → originalTag → event`。
- Provider 区分清楚 poll(Watcher 已用 policy 选)与 webhook/pubsub(外部候选 tag 必须再用资源 Policy/Filter 校验)两种语义。
- Created time 调用可有内部缓存,避免每次 poll 都重复 HTTP,但缓存不得改变正确性。
- `go build ./... && go test ./...` 通过,**E2E** 在本 Change 后跑通(覆盖 4 个 Phase 5 验证场景)。

**Non-Goals:**
- **不**支持 fetch created time 失败的 registry(零值丢末尾即可)。
- **不**实现 manifest list 的平台多 architecture 选择——一律取 `manifests[0]`(简化)。
- **不**改 webhook 路径的事件 schema 或 trigger 实现——webhook 端仍发送候选 tag,但 Provider 不信任发送方绕过 policy。
- **不**改 pubsub 载荷解析逻辑;只要求进入 Provider 前能区分非 poll 事件来源。
- **不**新增 created-time cache 的公开配置、Prometheus 指标或 TTL 机制;本 Change 只允许 watcher 内部使用简单 digest cache,且只缓存成功结果。

## Decisions

### 1. `GetCreatedTime` 返回 `(time.Time, error)` 还是 `(time.Time)`?

**选择**:返回 `(time.Time, error)`。Watcher 在 sort comparator 内 swallow error 并把失败 tag 排到末尾;但 API 仍保留 error 给单测与诊断脚本使用。

**理由**:与 `Get`、`Digest` 接口风格一致;隐藏 error 会让 cache 难以区分"未 fetched" vs "fetch 失败"。

### 2. Manifest list 多平台选择

**选择**:取 `manifests[0]`,记录 platform 到 debug 日志,但不暴露 API。

**理由**:Keel 跑在集群内,镜像架构通常单一(amd64 或 arm64)。多 arch image 的 created time 通常一致(同一 CI build)。复杂的 platform-aware 选择属于 Container runtime 关注点,Keel 不引入。

### 3. Created time cache 的 key 是什么?

**选择**:key = `<registry>+<repo>+<digest>`(digest 即 manifest digest)。

```go
type createdTimeCache struct {
    mu    sync.Mutex
    items map[string]time.Time
}
```

**理由**:`digest` 内容寻址,immutable 一旦 fetched 永远有效;tag 可变,不能作 cache key。Watcher 在 fetch GetCreatedTime 前先 fetch manifest digest(已有 `Digest` 方法),用 digest 查 cache。

**备选**:不 cache(每次 poll 都 fetch)。**否决**:对 50-tag image,每分钟 100 个 HTTP 请求,对 Docker Hub 很容易触发 rate-limit。

### 4. Cache 命中策略与 fetch 顺序

Force 路径按以下步骤处理候选 tag:

1. 先把 `filterTags` 匹配到的原始 tag 集合取出;`extract` 只用于非 Force policy 的 comparison key,不用于 registry 查询。
2. 对每个原始 tag 调 `Digest` 拿 manifest digest,用 `<registry>/<repo>@<digest>` 查 watcher 私有 cache。
3. cache miss 时调 `GetCreatedTime`;只有 `err == nil && !created.IsZero()` 的结果写入 cache。
4. 按 created time 降序排序,失败或零值 tag 排到末尾;相同时间用一个固定 tie-breaker,任务实现采用字典序升序。
5. 把排序后的原始 tag 列表交给 `Force.Latest`。

每个 tag 至少需 1 个 HTTP(`Digest` HEAD `/manifests/`),cache miss 再加 2 个(GET `/manifests/`、GET `/blobs/`)。冷启动后第一次 poll 慢,后续 poll 对成功解析过的 digest 仅需 HEAD。失败或零值结果不缓存,避免 transient registry error 被长期固定为"最旧"。

`forceOriginalTags(tags, filter)` 的规则:

- filter 为 nil:返回原始 `tags`。
- filter 非 nil:调用 `filter.Apply(tags)`,但返回匹配到的原始 tags,而不是 `filter.Items()` 的 extracted keys。
- 原因:Force 的排序依据是镜像 metadata,必须用真实 tag 调 registry API;`extract` 仅适用于 SemVer/Alphabetical/Numerical 的 comparison key。

**理由**:`Digest` 的 HEAD 调用极轻;HEAD 失败本身就意味着 tag 不可访问,跳过该 tag。

### 5. Force 以外的 policy 是否需要 created-time 排序?

**否**。SemVer / Alphabetical / Numerical 都对候选集合做 deterministic 的字符串/数字比较,无需依赖 registry 元数据。仅 Force 路径插入排序步骤。

### 6. Provider event-origin 决策:poll 信任 watcher,外部 event 校验 policy

`provider/kubernetes/updates.go::checkForUpdate` 当前:

```go
shouldUpdate, err := policy.ShouldUpdate(currentTag, eventTag)
```

新模型:

- **poll 路径**:Watcher 已经用 `Latest+Filter` 选出 eventTag,等于"已经过 policy"。Provider 只检查 `currentTag != eventTag && repo匹配`,不得重复调用 `Latest`。
- **webhook / pubsub 路径**:发送方(Docker Hub、GCR Pub/Sub、native webhook 等)只提供候选 tag。Provider 必须把 event tag 放回该资源的 Policy/Filter 语义中校验,不符合策略则拒绝更新。

```go
if containerImageRef.Repository() != eventRepoRef.Repository() ||
   containerImageRef.Tag() == eventRepoRef.Tag() {
    return false
}
if event.TriggerName == types.TriggerTypePoll.String() {
    return true
}
return policyAllowsExternalTag(plc, filter, containerImageRef.Tag(), eventRepoRef.Tag())
```

`policyAllowsExternalTag` 规则:

- 若 `plc == nil`:拒绝更新。
- 若 policy 是 `force`:event tag 通过 `filterTags`(如有)即可更新;force 本身不限制 tag 顺序,external event 路径不 fetch created-time。
- 若 policy 不是 `force`:对 `[currentTag, eventTag]` 应用 Filter(如有);event tag 未通过 Filter 时拒绝;调用 `plc.Latest(candidates)` 后,只有当选中的 original tag 等于 event tag 时才允许更新。
- 该规则不仅检查 SemVer constraint,还会拒绝仍满足 constraint 但低于 current 的降级 tag。

**理由**:外部 webhook 是触发信号,不是 policy 决策结果。把 event tag 与 current tag 一起交给 `Latest` 能复用新 policy 模型,同时保持 SemVer / Alphabetical / Numerical 的"只前进不后退"语义;Force 仍保留"用户显式要求强制更新"语义。

## Risks / Trade-offs

- **风险**:某些 registry 不返回正确的 config blob created time(老版 GCR 或 buildkit 早期版本) → **缓解**:零值 fallback 到列表末尾,文档说明"Force policy 行为对 created-time 元数据可用性敏感"。
- **风险**:Created-time cache 误缓存失败结果 → **缓解**:只缓存 non-zero time 且 error 为 nil 的结果。
- **风险**:OCI image index 多 architecture 选第一个可能不是用户期望的 → **缓解**:文档披露;同 CI build 的 created time 通常一致;未来若用户报告问题,引入 `keel.sh/platform` annotation。
- **风险**:`registry.Client` 接口扩展破坏第三方实现(如有 mock client) → **缓解**:已知调用方只有内部 mock(`registry_test.go`),一并更新。

## Migration Plan

1. PR 顺序:本 Change 紧随 [[refactor-policy-flux-style]],作为 PR4。
2. 合并前至少用本地 registry + httptest 覆盖 Force created-time 排序;多 registry staging soak 不作为本 Change 的完成条件。
3. 失败回退:`git revert` 即恢复 `WatchTagJob` 与旧 `multi_tags_watcher`;cache 与 GetCreatedTime 调用一并消失。

## Resolved Decisions

- 不引入 `FORCE_REQUIRE_CREATED_TIME`;零值/失败 tag 排末尾。
- `GetCreatedTime` 沿用全局 registry HTTP client timeout,不引入新参数。
- Force policy 的 poll 与 external event 路径有意不对称:poll 需要在 registry tag list 中按 created-time 选最新;webhook/pubsub 已给出显式候选 tag,只按 filterTags 做准入,不在 Provider 中 fetch created-time。
- 只有 `TriggerName == "poll"` 能走 poll 快路径;`pubsub`、webhook provider 名、空字符串和未知字符串都必须走 external policy admission。

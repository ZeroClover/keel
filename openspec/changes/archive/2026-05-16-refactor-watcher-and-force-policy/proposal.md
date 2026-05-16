## Why

[[refactor-policy-flux-style]] 重写 Policy 接口,但留下两个 bridge:

1. **Force policy 候选排序未知**:Force.Latest 只是返回 `candidates[0]`,而 Docker Registry V2 spec **不保证 tag 列表顺序**。同一 image push 后,Docker Hub、GHCR、GCR、ECR 返回的 `/v2/<repo>/tags/list` 顺序不一致,因此 force policy 仍**不可预测**。本 Change 通过 fetch 每个候选 tag 的 manifest → config blob → `created` 字段,按时间降序排序后再交给 Force,使行为确定。
2. **`WatchTagJob`(digest watch)残留**:`trigger/poll/single_tag_watcher.go` 处理旧的 "force + matchTag=true → watch digest 不变 tag" 用例。该用法已在 [[refactor-policy-flux-style]] 删除 annotation / `KeepTag()` 入口后失效;本 Change 删除残留文件,所有 watcher 只走 `WatchRepositoryTagsJob` 路径。

此外,Provider 路径(`provider/kubernetes/updates.go`、`provider/helm3/updates.go`)在 [[refactor-policy-flux-style]] 已删除 `policy.ShouldUpdate` 调用。本 Change 明确事件来源语义:poll event 已由 watcher 用 policy 选过,Provider 不重复执行 `Latest`;webhook/pubsub 等外部 event 只是候选 tag,仍必须经过资源声明的 Policy/Filter 校验后才能更新。

## What Changes

- **Registry Client 扩展**:
  - `registry.Client` 接口新增方法 `GetCreatedTime(opts Opts) (time.Time, error)`。
  - `registry.DefaultClient.GetCreatedTime`:wrapper 转给底层 `docker.Registry.GetCreatedTime`。
  - `registry/docker/manifest.go::GetCreatedTime(repository, tag string) (time.Time, error)`:三段式调用——
    1. `GET /v2/<repo>/manifests/<tag>` 拿到 manifest,从中读 config blob digest;
    2. `GET /v2/<repo>/blobs/<config-digest>` 拿 OCI image config JSON;
    3. 解析 `.created` 字段(RFC3339 时间戳)。
    遇到 manifest list / OCI image index → 取第一个平台兼容的 manifest 递归处理。
    失败返回零值 `time.Time{}` + non-nil error(由 Watcher 把失败 tag 排到末尾,但不缓存失败结果)。
- **Watcher 重构**:
  - 删除文件 `trigger/poll/single_tag_watcher.go`(整个 WatchTagJob)。
  - 确认 `policy.KeepTag()` 调用与 keepTag 分支已不存在;`getImageIdentifier(ref *image.Reference) string` 仅 `registry+name`。
  - 改写 `trigger/poll/multi_tags_watcher.go::computeEvents`,对 Force policy 路径在调用 `Latest` 前先 fetch created time 排序:
    ```go
    // 1. filter.Apply(tags)
    // 2. 若 policy 是 Force:用 filter 匹配到的 original tags 排序;extract 不参与 created-time 查询
    //    fetch GetCreatedTime,按 time 降序排序;失败/零值排到末尾且不写缓存
    // 3. policy.Latest(sortedCandidates) → latest key
    // 4. 非 Force 路径才用 filter.GetOriginalTag(latestKey) → 原始 tag(若 filter 为 nil,直接用 latestKey)
    // 5. 与 current 比较,不同则触发 event
    ```
- **Provider event-origin 决策**:
  - `provider/kubernetes/updates.go::checkForUpdate`:poll 路径只做 repo/tag diff 检查;webhook/pubsub 路径先确认 repo/tag diff,再用该资源的 Policy/Filter 校验 event tag。
  - `provider/helm3/updates.go` 同上调整。
  - 强化测试:webhook/pubsub 路径拒绝不满足 policy 的 tag、拒绝 policy 范围内的降级 tag;poll 路径用 watcher 选出的 latest 后才进入 update path。
- **创建时间缓存(内部优化)**:
  - Watcher 可以用私有 digest→createdTime 缓存减少重复 HTTP。
  - 仅缓存成功且非零的 created time;网络错误、解析错误、缺失 `.created` 的零值结果不得缓存。
  - 不新增公开 env、Prometheus 指标或可配置 LRU;需要性能调优时另开 Change。
- **删除 KeepTag 相关跟踪**:
  - `trigger/poll/manager.go` 等如有 `KeepTag` 引用,一并清理。

## Capabilities

### New Capabilities
- `registry-client`: 描述 Registry Client 接口三个方法(`Get`、`Digest`、`GetCreatedTime`)与 Docker Registry V2 路径行为。
- `image-poll-watcher`: 描述 Poll trigger watcher 的统一行为——单一 `WatchRepositoryTagsJob` 路径、Force 路径下 created-time 排序、created-time 缓存策略。
- `provider-update-decision`: 描述 Provider(kubernetes / helm3)在 webhook 与 poll 两种事件源下的 update 决策规则。

### Modified Capabilities
无。本批 OpenSpec changes 以空 `openspec/specs/` 为起点创建;归档顺序假设为:[[remove-hipchat-and-chatbot]] → [[remove-approvals-system]] → [[refactor-policy-flux-style]] → 本 Change。本 Change 消费前一 Change 的 `Policy.Latest` / `Filter` 契约,但不重新定义 `image-policy`。

## Impact

- **代码**:
  - 新增方法 `docker.Registry.GetCreatedTime`(约 80 行,含 OCI image index 解包)。
  - 可选新增 watcher 私有 created-time cache(简单 map/mutex;只缓存成功结果)。
  - 删除 `trigger/poll/single_tag_watcher.go` 整文件。
  - 简化 `trigger/poll/watcher.go::getImageIdentifier` 签名(去 keepTag 参数)。
  - 改写 `multi_tags_watcher.go::computeEvents`(约 60 行变更)。
  - 改写 `provider/{kubernetes,helm3}/updates.go::checkForUpdate` 为 event-origin aware policy 校验(约 40 行)。
- **HTTP 流量**:Force policy 每次 poll 触发 N 个 `GET /v2/.../manifests/*` 与 N 个 `GET /v2/.../blobs/*`,N=候选 tag 数。默认 poll 间隔 1m,Force policy 用户一般仅追主 tag 数量 < 50,可接受;Cache 命中后单次只查 N 个 manifest HEAD。
- **测试**:
  - `registry/docker/manifest_test.go` 新增 `TestGetCreatedTime`(httptest mock registry,返回 v2 manifest + config blob)。
  - `trigger/poll/multi_tags_watcher_test.go` 整体重写以匹配新 Latest+Filter 接口与 Force 路径排序。
  - `provider/kubernetes/updates_test.go` 与 `provider/helm3/updates_test.go` 重写覆盖 webhook vs poll 两路径。
- **关联 Change**:依赖 [[refactor-policy-flux-style]] 引入的 `Policy.Latest`、`Filter.GetOriginalTag` 接口与 `PolicyTypeForce` 枚举。
- **回滚风险**:本 Change 之后,旧的 "force + matchTag=true → watch digest" 用法**彻底不可恢复**。文档强烈建议改用 immutable tag(`v1.2.3`)+ semver policy。

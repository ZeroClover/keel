## 1. 扩展 Registry Client 接口

- [x] 1.1 修改 `registry/registry.go`,把 `Client` 接口扩展为:
   ```go
   type Client interface {
       Get(opts Opts) (*Repository, error)
       Digest(opts Opts) (string, error)
       GetCreatedTime(opts Opts) (time.Time, error)
   }
   ```
- [x] 1.2 在 `registry/registry.go` 实现 `DefaultClient.GetCreatedTime(opts Opts) (time.Time, error)`,与 `Get`/`Digest` 同样的 `getRegistryClient` 流程拿到 `*docker.Registry`,然后调用 `r.GetCreatedTime(opts.Name, opts.Tag)`。
- [x] 1.3 `grep -rln "registry.Client" --include="*.go" .` 找到所有实现接口的 mock(`registry_test.go` 等),逐个补充 `GetCreatedTime` 方法。

## 2. 实现 docker.Registry.GetCreatedTime

- [x] 2.1 在 `registry/docker/manifest.go` 新增方法 `GetCreatedTime(repository, tag string) (time.Time, error)`:
   - 第一步:`GET /v2/<repository>/manifests/<tag>`,设置 Accept 头包含 `application/vnd.docker.distribution.manifest.v2+json`、`application/vnd.oci.image.manifest.v1+json`、`application/vnd.oci.image.index.v1+json`、`application/vnd.docker.distribution.manifest.list.v2+json`。
   - 第二步:根据 `Content-Type` 解析:
     - manifest list / image index:取 `manifests[0]`,递归 step 1 用其 digest 作为 reference。
     - single manifest:JSON 解析拿 `.config.digest`。
   - 第三步:`GET /v2/<repository>/blobs/<config-digest>`,JSON 解析 `.created`(`time.Parse(time.RFC3339, str)`)。
   - 任意失败返回 `time.Time{}` + 描述性 error;`.created` 缺失返回 `time.Time{}` + 包含 `created` 关键字的 error。
   - 参考 `github.com/distribution/distribution/v3/manifest/schema2` 与 `github.com/opencontainers/image-spec/specs-go/v1` 中的类型定义。
- [x] 2.2 创建 `registry/docker/manifest_test.go::TestGetCreatedTime`:
   - 用 `httptest.Server` 模拟 V2 registry。
   - case 1:返回 v2 manifest + config blob `"created":"2024-05-01T10:00:00Z"` → 期望对应 time。
   - case 2:返回 OCI image index(`manifests[0].digest`)→ 递归 fetch → 期望底层 created time。
   - case 3:config blob 缺失 `created` 字段 → 期望 zero time + error。
   - case 4:HTTP 500 → 期望 zero time + error。
- [x] 2.3 运行 `go test ./registry/docker/...` 通过。

## 3. 控制 created-time 缓存范围

- [x] 3.1 不新增公开 `CREATEDTIME_CACHE_SIZE` env、Prometheus cache 指标或 registry 包公开 cache API。若需要缓存,仅在 watcher 内实现私有 `createdTimeCache`:
   ```go
   type createdTimeCache struct {
       mu    sync.Mutex
       items map[string]time.Time
   }
   func (c *createdTimeCache) Get(key string) (time.Time, bool)
   func (c *createdTimeCache) Set(key string, t time.Time)
   ```
   key 约定:`<registry>/<repository>@<digest>`(由调用方拼接)。
- [x] 3.2 cache 只写入 `err == nil && !created.IsZero()` 的结果;网络错误、HTTP 非 2xx、JSON 解析错误、缺失 `.created` 均不得写入 cache。
- [x] 3.3 创建对应 `*_test.go`,覆盖 Get/Set 与"失败/零值不缓存"。

## 4. 删除 single_tag_watcher 残留

- [x] 4.1 删除文件 `trigger/poll/single_tag_watcher.go`(整个 `WatchTagJob`)。
- [x] 4.2 确认 `trigger/poll/watcher.go::getImageIdentifier` 已是 `(ref *image.Reference) string`,实现仅返回 `ref.Registry() + "/" + ref.ShortName()`。
- [x] 4.3 `grep -rn "KeepTag\|WatchTagJob\|keepTag" --include="*.go" trigger/ provider/ pkg/` 应为空(除测试历史 fixture 已重写的部分)。

## 5. 重写 multi_tags_watcher 的 computeEvents

- [x] 5.1 修改 `trigger/poll/multi_tags_watcher.go::computeEvents(tags []string) ([]types.Event, error)`:
   ```go
   for _, ti := range allRelatedTrackedImages {
       // 1. Force 路径:使用 filter 匹配到的 original tags,再按 created time 排序
       var candidates []string
       filter := ti.Filter
       if ti.Policy.Type() == types.PolicyTypeForce {
           candidates = sortByCreatedTime(originalTagsForForce(tags, filter), ti.Image, j.registryClient, j.cache)
       } else if filter != nil {
           // 2. 非 Force 路径:filter.Apply 后使用 extracted keys
           filter.Apply(tags)
           candidates = filter.Items()
       } else {
           candidates = tags
       }
       // 3. 选 latest
       latestKey, err := ti.Policy.Latest(candidates)
       if err != nil { continue }
       // 4. 反查 original
       var latestTag string
       if ti.Policy.Type() != types.PolicyTypeForce && filter != nil { latestTag = filter.GetOriginalTag(latestKey) } else { latestTag = latestKey }
       if latestTag == "" || latestTag == ti.Image.Tag() { continue }
       // 5. 发 event
       events = append(events, types.Event{
           Repository: types.Repository{Name: ti.Image.Repository(), Tag: latestTag},
           TriggerName: types.TriggerTypePoll.String(),
       })
   }
   ```
- [x] 5.2 实现 helper `sortByCreatedTime(tags []string, img *image.Reference, client registry.Client, cache *createdTimeCache) []string`:
   - 对每个 tag:`Digest` 拿 manifest digest → cache 查 → miss 时 fetch GetCreatedTime;仅成功且非零时 cache set。
   - 失败 / 零值的 tag 排到末尾,且失败 / 零值不得写入 cache。
   - 时间相等的 tag 用字典序作 tie-breaker,满足 spec 的 deterministic 要求。
- [x] 5.3 实现 helper `originalTagsForForce(tags []string, filter types.Filter) []string`:filter 为 nil 时返回原始 tags;filter 非 nil 时在 helper 内调用一次 `filter.Apply(tags)`,遍历 `filter.Items()` 并用 `filter.GetOriginalTag(key)` 返回命中的原始 tags,不得返回 `extract` 后的 keys。
- [x] 5.4 `WatchRepositoryTagsJob` struct 增加字段 `cache *createdTimeCache`;`NewWatchRepositoryTagsJob` 构造函数同步更新。
- [x] 5.5 修改 `trigger/poll/watcher.go::addJob`,把 cache 注入 Job。

## 6. 调整 Watcher 测试

- [x] 6.1 确认 `types.TrackedImage.Filter` 已由 [[refactor-policy-flux-style]] 添加;本 Change 只消费该字段,不再新增。
- [x] 6.2 重写 `trigger/poll/multi_tags_watcher_test.go`,fixture 用新接口:`ti.Policy = policy.NewSemVer("...")`、`ti.Filter = policy.NewRegexFilter(...)`;覆盖 force-with-sort、force+filterTags+extract 使用 original tag 查询、semver、numerical+filter+extract。
- [x] 6.3 重写 `trigger/poll/watcher_test.go`,删除 `keepTag` 相关 fixture;补充 `getImageIdentifier` 单参数测试。

## 7. Provider event-origin 决策

- [x] 7.1 修改 `provider/kubernetes/updates.go::checkForUpdate` 与同包 helper:
   - 基础决策仍先检查 `containerImageRef.Repository() == eventRepoRef.Repository()` 且 tag 不同。
   - poll 路径(`event.TriggerName == types.TriggerTypePoll.String()`):直接允许 update,不得重复执行 `policy.Latest`。
   - webhook/pubsub/其他非 poll 路径:复用 [[refactor-policy-flux-style]] 已实现的 `policyAllowsExternalTag(policy, filter, currentTag, eventTag)`。
   - helper 分支必须保持为: nil policy → reject; Force → 只检查 filterTags,不调用 `Latest` 且不 fetch created-time; Non-Force → 对 `[currentTag,eventTag]` 应用 Filter 后调用 `Latest` 拒绝越界和降级 tag; event tag 未通过 Filter → reject; current tag 未通过 Filter 不得单独导致 reject。
   - 删除任何 `KeepTag` 检查与 digest 比较分支。
- [x] 7.2 修改 `provider/helm3/updates.go` 同上调整。
- [x] 7.3 修改 `trigger/pubsub/pubsub.go`,创建事件时设置 `TriggerName: "pubsub"`。未知或空 TriggerName 仍必须按 external event 处理,不得走 poll 快路径。
- [x] 7.4 重写 `provider/kubernetes/updates_test.go`:
   - webhook 路径:同 repo + tag 不同但不满足 deployment policy 时拒绝 update。
   - webhook 路径:同 repo + tag 不同且满足 policy 且不降级时触发 update。
   - webhook 路径:tag 满足 SemVer constraint 但低于 current 时拒绝 update。
   - force webhook 路径:通过 filterTags 即触发 update,且不得调用 `Latest([current,event])`。
   - pubsub 路径:`TriggerName="pubsub"` 走 external admission;`TriggerName=""`/未知字符串也走 external admission。
   - poll 路径:event 直接 update(注 watcher 已经过 policy),不得重复调用 Latest。
   - 同 tag:no-op。
   - 不同 repo:no-op。
- [x] 7.5 重写 `provider/helm3/updates_test.go` 同上。

## 8. 文档与配置

- [x] 8.1 更新 `readme.md::Force policy` 段落:
   - 说明新语义"按 image created time 降序"。
   - 移除"latest + matchTag=true → watch digest"用法。
   - 推荐 immutable tag + semver policy + GitOps webhook 替代。
- [x] 8.2 更新 `ARCHITECTURE.md::Triggers` 段落,删除 `WatchTagJob` 提及;`Common Tasks` 表中删除 KeepTag 相关行。
- [x] 8.3 不更新 Helm chart 添加 created-time cache 配置;本 Change 不引入公开 cache knob。

## 9. E2E 与回归

- [x] 9.1 启动 `docker compose up keel` + 本地 registry,执行计划文档 Phase 5 的 4 个场景:
   - 场景 1:SemVer `>=1.0.0-0` 包含 pre-release → 推送 `1.0.0-rc.2` 触发 update。
   - 场景 2:SemVer `>=1.0.0` 排除 pre-release → 推送 `1.1.0-rc.1` 忽略,推送 `1.1.0` 升级。
   - 场景 3:Numerical commit timestamp → 推送 `main-def-200` 触发 update。
   - 场景 4:Force 按 created time → 推送 `tag-1`,等 2 秒推送 `tag-2`,poll 后应选 `tag-2`。
   - Verified with Colima Kubernetes, `docker compose` Keel + `registry:2`, namespace `keel-e2e`, image prefix `keel-e2e-20260516-0853`:
     - Scenario 1: `semver:>=1.0.0-0` updated `semver-prerelease` from `1.0.0-rc.1` to `1.0.0-rc.2`.
     - Scenario 2: `semver:>=1.0.0` ignored `1.1.0-rc.1` for five 2-second polls, then updated `semver-stable` from `1.0.0` to `1.1.0`.
     - Scenario 3: `numerical:desc` with `filterTags`/`extract` updated `numerical` from `main-abc-100` to `main-def-200`.
     - Scenario 4: `force` updated `force-live` from `tag-1` to `tag-2`; local image config timestamps were `tag-1=2026-05-16T16:54:24+08:00`, `tag-2=2026-05-16T16:55:47+08:00`.
- [x] 9.2 使用 httptest 或本地 registry 覆盖至少一个 GetCreatedTime 失败 case,确认失败 tag 排末尾且不进入 cache。

## 10. 关闭旧 issue 与最终验证

- [x] 10.1 全仓库 `grep -rn "KeepTag\|matchTag\|MatchTag\|match-tag\|matchPreRelease\|MatchPreRelease" --include="*.go" .` 应为空(除 [[refactor-policy-flux-style]] 已加的 legacy-warner ERROR 日志字符串)。
- [x] 10.2 全仓库 `grep -rn "ShouldUpdate" --include="*.go" .` 应为空。
- [x] 10.3 `go test ./...` 全部通过。
- [x] 10.4 验证 4 个 OpenSpec Change 全部进入 archive 状态后,`openspec/specs/` 含 `notifications`、`image-update-pipeline`、`persistence`、`web-dashboard`、`image-policy`、`helm-chart-config`、`registry-client`、`image-poll-watcher`、`provider-update-decision` 共 9 个 capability。
   - Verified after archiving this Change: `openspec list --json` reports no active changes, and `openspec list --specs --json` reports all 9 required capabilities.

## 11. OpenSpec 工件验证

- [x] 11.1 运行 `openspec validate refactor-watcher-and-force-policy --strict`,确认 4 个 artifact 均通过。
- [x] 11.2 准备 PR 标题:"refactor: registry GetCreatedTime, deterministic Force policy, single watcher path",PR 描述附 4 个手动验证场景结果。
   - PR title: `refactor: registry GetCreatedTime, deterministic Force policy, single watcher path`
   - PR description verification section:
     - `go build ./...`
     - `go test ./...`
     - `openspec validate refactor-watcher-and-force-policy --strict`
     - Manual E2E with Colima Kubernetes, `docker compose` Keel + local `registry:2`:
       - SemVer `>=1.0.0-0`: pushed `1.0.0-rc.2`; deployment updated from `1.0.0-rc.1` to `1.0.0-rc.2`.
       - SemVer `>=1.0.0`: pushed `1.1.0-rc.1`; deployment remained at `1.0.0` for five 2-second polls; pushed `1.1.0`; deployment updated to `1.1.0`.
       - Numerical timestamp: pushed `main-def-200`; deployment updated from `main-abc-100` to `main-def-200`.
       - Force created time: pushed `tag-1`, later pushed `tag-2`; deployment updated from `tag-1` to `tag-2`.

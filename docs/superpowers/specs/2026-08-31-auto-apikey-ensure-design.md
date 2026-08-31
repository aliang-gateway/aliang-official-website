# 设计：注册即备 auto API key + 用户自建 key

日期：2026-08-31
状态：已获用户批准（设计对话中逐节确认）

## 1. 背景与问题

- 新注册用户在 keys 页面看不到任何 API key，也没有创建入口。
- 现有幂等建 key 逻辑 `EnsureUserKeyInGroup`（`backend/internal/sub2api/gateway.go:130`）只挂在套餐发放链路（`executePackagePurchaseFulfillment` 内调用，routes.go:7552），注册后永远不会触发。
- 前端 `/keys` 页面只有列表/筛选/启停/删除/复制，无创建表单；且 BFF `GET /groups/available`（`handleFilteredGroupsAvailablePassthrough`，routes.go:1695）对普通用户按「活跃订阅 tier 绑定的分组」过滤，新用户结果为空——即使有表单也选不了分组。

### 业务逻辑结论（已核实代码与上游文档）

上游 sub2api 是真正的计费网关（`docs/sub2api-apikey-api-reference.md`、`docs/sub2api-user-api-reference.md`）：

1. 计费 = 用户 USD `balance` 余额，调用按 `原始费用 × 分组倍率` 扣费。
2. 分组两类：`subscription_type` 为空 = **非订阅制（standard/balance）分组**，凭余额即可用；`subscription_type` = "subscription" = **订阅制分组**，须持有该组有效订阅。
3. `is_exclusive=true` 的专属分组需管理员授权；非专属分组对所有用户开放。
4. 上游 `GET /api/v1/groups/available`（用户 JWT）返回「当前用户可用」分组 = 非专属组 + 已授权专属组；新用户也会拿到非订阅制分组。

因此：新用户注册时即可在非订阅制标准分组上备好 auto key（余额为 0，充值后立即可用）；订阅制分组的 key 仍由套餐发放时补齐（现状保留）。

## 2. 目标 / 非目标

**目标**

- 一个幂等、可重复调用的「确保 auto key 存在」入口 `EnsureDefaultUserKeys`。
- 注册成功后自动 ensure 一次；keys 页面加载时兜底 ensure 一次。
- 用户可在 keys 页面自建 key：名字 + 分组（下拉）。
- 重复调用绝不覆盖已有 key、绝不产生重复 key、绝不影响已有主流程。

**非目标**

- 不改计费、不赠送余额/额度。
- 不改 fulfillment 现有发放逻辑（继续用 `EnsureUserKeyInGroup`）。
- 不动本地 legacy `als_api_keys` 表及其服务（非 proxy 模式除外，本设计不涉及）。

## 3. 核心设计

### 3.1 `EnsureDefaultUserKeys`（`backend/internal/sub2api/gateway.go` 新方法）

```
EnsureDefaultUserKeys(ctx, userID) (EnsureResult, error)
  1. GetBearerTokenByUserID(userID)              // 复用现有 sub2apiauth 服务
  2. GET /api/v1/groups/available（用户 token）    // 需在 proxy.Client 新增用户侧 ListAvailableGroups
  3. 过滤：status == "active" 且 subscription_type != "subscription"
  4. 按 platform 去重，每平台取第一个组 → 目标组集合 T
  5. 对每个 g ∈ T：调用现有 EnsureUserKeyInGroup(userID, g, idemKey)
```

- 第 5 步完全复用现有 `EnsureUserKeyInGroup`（查重 + 409 容忍），fulfillment 与本设计共用同一实现。
- 幂等键改为确定性键 `auto-ensure:u<userID>:g<groupID>`（同用户同组永远同一键；fulfillment 原有 parentIdempotencyKey 派生方式保持不变，为兼容两者，`EnsureUserKeyInGroup` 增加显式幂等键入参或新增带键变体，旧调用点行为不变）。
- 部分组失败不中断其余组：聚合已建/已存在结果与失败错误一并返回。

### 3.2 安全保证（硬性约束）

| 风险 | 防护 |
|---|---|
| 覆盖/修改已有 key | ensure 只创建、零更新：从不调用 PUT/DELETE；用户自建 key 与已有 key 一概不碰 |
| 重复调用产生重复 key | 每组先 `ListUserAPIKeys(groupID, "auto-key")` 查重；该组存在任何同名 key（含停用状态）即返回，不创建 |
| 并发触发（注册钩子 / 页面兜底 / fulfillment worker） | ① 确定性幂等键 `auto-ensure:u<userID>:g<groupID>`（注意：上游对 `Idempotency-Key` 的去重行为未在文档中承诺，**不作为主要保证**）；② 主要保证 = 创建前 `ListUserAPIKeys` 查重 + ③ 409 Conflict 视为成功 |
| auto-key 误删循环 | `auto-key` 受保护不可删（现状保留：`apikey.IsProtectedAPIKeyName` 位于 `backend/internal/apikey/service.go:54`，routes.go 处处引用，前端 `api-keys.ts:35`、`account/page.tsx:559`） |
| 上游故障 / 无上游 token | ensure 失败只记日志，不影响注册响应与 keys 列表返回 |

### 3.3 触发点（两处，均幂等可重入）

1. **注册成功后**：`handleAuthPassthrough`（routes.go:2377）register 分支，`captureSub2APITokens` 成功后启动 goroutine 异步调用 `EnsureDefaultUserKeys`（context 超时 10s；失败仅 `slog.Warn`，不阻塞注册响应）。
2. **keys 页面兜底**：新增 `POST /api-keys/ensure-auto`（authenticated），内部调 `EnsureDefaultUserKeys`，返回 `{ "ensured": n, "created": [groupID...], "failed": [...] }`。前端 keys 页面加载时（`loadApiKeys` 前）fire-and-forget 调用一次。

### 3.4 用户自建 key（前端表单）

- `POST /api-keys` 透传链路已存在（前端 `/api-keys` BFF → 后端 `handleAPIKeysCreatePassthrough` → 上游 `/api/v1/api-keys`），后端零改动。
- `/keys` 页面新增「创建 Key」表单：名字（必填）+ 分组下拉（数据源见 3.5）。提交 body：`{ "name": "...", "group_id": n }`。
- 创建成功后刷新列表。自建 key 名称不为 `auto-key`，可正常启停/删除。

### 3.5 分组下拉数据源（一处 BFF 调整）

`handleFilteredGroupsAvailablePassthrough` 的本地过滤从「仅本地授权组」改为：

```
上游 available ∩（非订阅制标准组 ∪ 本地授权组）
```

- 标准组对所有人可见可选（配合余额计费）；订阅制组仍只对已购套餐用户可见，避免选到用不了的组。
- ensure 与自建表单使用同一份列表，口径一致。

## 4. 错误处理

- `EnsureDefaultUserKeys`：单组失败（网络/5xx）记入结果 `failed` 并继续其余组；全部失败时返回错误，由调用方决定是否记日志。
- 注册钩子：goroutine panic 用 recover 包裹；任何错误只 `slog.Warn`。
- ensure 端点：上游不可用时返回 502 + 错误信息，前端静默忽略（兜底语义）。
- BFF 分组过滤（改后语义）：上游 groups/available 失败时，**普通用户一律回退为空列表（不再保留现状对有订阅用户返回 502 的路径）**，admin 走原透传。

## 5. 测试计划

- `gateway` 单测（httptest mock 上游）：查重跳过 / 新建成功 / 409 容忍 / 订阅组被过滤 / 平台去重 / 部分失败聚合 / 确定性幂等键格式。**防重的验证以「创建前查重 + 409 容忍」为准，不依赖上游幂等键去重行为。**
- `routes` 单测：ensure 端点鉴权与响应结构；注册分支 ensure 失败不影响注册 2xx 响应。
- 分组过滤单测：新用户可见标准组、不可见未购订阅组；已购用户可见其订阅组。
- 前端：表单校验（名字必填、分组必选）与提交错误展示（沿用现有 `extractApiError` 模式）。

## 6. 涉及文件

| 文件 | 改动 |
|---|---|
| `backend/internal/proxy/passthrough.go` | 新增用户侧 `ListAvailableGroups`（复用现有请求模式） |
| `backend/internal/sub2api/gateway.go` | 新增 `EnsureDefaultUserKeys`；`EnsureUserKeyInGroup` 支持显式幂等键（旧调用点不变） |
| `backend/internal/httpapi/routes.go` | 注册分支异步钩子；`POST /api-keys/ensure-auto` 端点；分组 BFF 过滤调整 |
| `frontend/app/api-keys/ensure-auto/route.ts` | 新增 Next BFF 转发路由（静态段优先于 `[id]`，否则兜底触发不可达） |
| `frontend/app/(app)/keys/page.tsx` | 创建 key 表单；加载时兜底 ensure 调用 |
| 对应 `*_test.go` / 前端测试 | 见第 5 节 |

## 7. 交付方式

实现阶段使用 git worktree 隔离开发（用户要求），主工作区不受影响。

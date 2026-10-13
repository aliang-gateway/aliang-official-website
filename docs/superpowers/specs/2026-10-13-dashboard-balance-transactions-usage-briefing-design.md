# Dashboard 余额流水卡 + Token 用量简报卡 设计

- 日期:2026-10-13
- 状态:已与需求方确认设计方向(流水列表 / 聚合简报卡 / 跟随现有范围选择器 / 方案 A 最小侵入)
- 范围:仅前端 `frontend/`,零后端改动

## 背景

用户登录后的 dashboard(`frontend/app/(app)/dashboard/page.tsx`)目前有:StatusCard(余额当前值 + 今日请求数/消费/tokens + 套餐额度条)、CacheRateCard(缓存命中率)、TokenTrendCard(token 消费曲线)、ModelShareCard(模型分布)。

两个缺口:

1. **余额只有当前值,没有变化**。后端已有 `GET /wallet/transactions?limit=N`(`backend/internal/httpapi/routes.go:3616`,返回 `user.WalletTransaction[]`:type / amount_micros / currency / balance_after_micros / description / created_at),但前端从未调用——BFF 只有 `/api/wallet/redeem`。
2. **token 用量视图分散**。今日 tokens 在 StatusCard、曲线在 TokenTrendCard、模型分布在 ModelShareCard、缓存率在 CacheRateCard,没有一眼看完的聚合「简报」。

关键数据事实:`/api/dashboard/trend` BFF 是纯透传(`frontend/app/api/dashboard/trend/route.ts`),上游返回的每日行已含 input_tokens / output_tokens / cache_read_tokens / cache_creation_tokens / requests / cost / actual_cost;`useDashboardData` 现有解析(`parseTokenTrendResponse`)只保留 total_tokens 点,拆分被丢弃。**简报卡所需数据已在现有请求的响应里,零新增请求。**

已知约束(需向用户如实呈现):`als_wallet_transactions` 表当前**只有 `recharge` 一种写入**(`backend/internal/user/service.go:842` 充值卡兑换)。API 消费走 sub2api 上游计费,不落本地流水。因此流水卡当前实际呈现「充值记录 + 每笔后余额」;消费侧由简报卡的消费金额回答。

## 需求决策(已确认)

| 决策点 | 结论 |
| --- | --- |
| 余额变化形态 | 流水列表(最近 N 笔,含类型/金额±/变动后余额/时间) |
| token 简报形态 | 聚合简报卡:范围总量 + input/output/cache 拆分 + top 模型 + 消费金额 |
| 简报时间范围 | 跟随 dashboard 顶部已有范围选择器,与曲线/模型分布口径一致 |
| 实现方案 | 方案 A:流水独立 BFF + 卡片级自取数(仿 CacheRateCard);简报卡复用已有 trend/models 响应;零后端改动 |

被否掉的备选:聚合进 dashboard home BFF(改聚合层、耦合重、单项失败拖累整体);后端新增 summary 接口(前端已有数据,重复计算)。

## 设计

### 1. Token 用量简报卡 UsageBriefingCard

新增 `frontend/components/dashboard/UsageBriefingCard.tsx`,导出加入 `frontend/components/dashboard/index.ts`。

数据流:`useDashboardData`(`frontend/lib/hooks/use-dashboard-data.ts`)已 fetch `/api/dashboard/trend?{queryString}`,其 payload 现只被 `parseTokenTrendResponse` 压缩使用。新增解析函数(放 `frontend/lib/dashboard-home.ts`,与现有 parse 函数同层):

```ts
export type TrendUsageSummary = {
  start_date: string;
  end_date: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  total_tokens: number;
  actual_cost: number;   // 优先 actual_cost,回退 cost
};
export function parseTrendUsageSummary(trendPayload: unknown): TrendUsageSummary;
```

实现上复用 `parseDashboardTrendEnvelope`(`frontend/lib/dashboard-analytics-adapter.ts:181`)取完整行,reduce 聚合。`useDashboardData` 在现有 `setTokenTrend(parseTokenTrendResponse(trendPayload))` 处用同一 payload 多调一次解析,`setUsageSummary(...)`;新增 state `usageSummary` 加入 `DashboardData`。零新增请求、口径与曲线一致。

展示内容:

- 标题「用量简报」+ 范围标签(复用 trend 范围的 label,含 start_date ~ end_date)
- 三主指标:总 tokens(formatMetricNumber)、请求次数、消费金额(actual_cost,formatMetricCurrency)
- 拆分行:输入 / 输出 / 缓存读 / 缓存写 四项数字
- 最常用模型:`modelShare.items[0]`(模型名 + 占比百分比);items 为空时该行隐藏

空态:范围内无请求(points 全 0 或缺失)时指标显示 `--` 并给一句空态文案;不整卡隐藏(简报是本需求主体)。

样式遵循现有 clay-panel / block-card 设计系统,与 StatusCard 同风格(MONO 小标签、tabular-nums)。

### 2. 余额流水卡 WalletTransactionsCard

新增 `frontend/components/dashboard/WalletTransactionsCard.tsx`,导出加入 `index.ts`。

BFF:新增 `frontend/app/api/wallet/transactions/route.ts`,仿 `frontend/app/api/wallet/redeem/route.ts` 写法——`getApiBaseUrl()`、透传 `authorization` 头、`fetch(`${apiBaseUrl}/wallet/transactions?limit=8`)`、`cache: "no-store"`、透传上游状态码。`limit` 固定 8(卡片概览定位;后端上限 100,详情页将来可加)。

取数:卡片内 `useEffect` 自取数(逐字仿 `frontend/components/dashboard/CacheRateCard.tsx` 的模式:sessionToken 空则跳过、AbortController、cancelled 标志、失败静默)。卡片挂了只影响自己,不拖累首页(与 CacheRateCard 同等地位)。

解析:卡内新增宽容解析(或抽到 `frontend/lib/dashboard-home.ts` 便于复用/测试):容忍 `{transactions:[...]}` / `{data:[...]}` / 裸数组三种形态,字段走 asNumber/asString 归一,金额/时间缺失显示 `--`。

展示(最近 8 条,按 id 倒序即后端默认):

```
┌─ 余额变动 · 最近 8 笔 ─────────────┐
│ [充值] +¥50.00   余额 ¥120.50      │
│        10-12 14:30                 │
│ [充值] +¥30.00   余额 ¥70.50      │
│        10-08 09:12                 │
│ …                                  │
└────────────────────────────────────┘
```

- 每行:类型徽章 + 金额(带符号,正数绿色/负数红色/异常中性)+ 变动后余额 + 时间
- 金额符号与颜色按 amount_micros 正负渲染,**不按 tx_type 硬编码**——当前只有 recharge(正数),后端将来新增消费类型时 UI 自动适配
- tx_type 未知值回退中性徽章(原样显示 type 文本)
- 空态:「暂无流水记录」
- 失败态:静默降级为空态文案(与 CacheRateCard 同策略),不显示红 error

### 3. 布局(page.tsx)

```
[DashboardHeader]                        ← 不动
[StatusCard]                             ← 不动
[UsageBriefingCard] [WalletTransactionsCard]  ← 新增一行两列 grid lg:grid-cols-2,窄屏堆叠
[CacheRateCard]                          ← 不动
[TokenTrendCard]                         ← 不动
[ModelShareCard]                         ← 不动
[keys / ticket / details tiles]          ← 不动
```

新增卡行插在 StatusCard 与 CacheRateCard 之间:用户关心的余额变化与用量概览在首屏可见;两卡等宽,空间不足时各自 min-w-0 截断。

### 4. i18n

`frontend/messages/zh.json` 与 `en.json` 的 `dashboard` namespace 新增两卡全部文案 key(标题、范围标签、指标 label、空态、流水类型徽章等)。中文为准,英文对照翻译。不新增 namespace。

### 5. 错误处理汇总

| 场景 | 行为 |
| --- | --- |
| 流水 BFF 上游 5xx / 网络失败 | 卡片静默降级空态,页面其余不受影响 |
| 流水列表为空 | 「暂无流水记录」 |
| trend 范围内无数据 | 简报指标 `--` + 空态文案,卡不隐藏 |
| modelShare 空 | top 模型行隐藏 |
| session 失效 | 既有页面级 401 跳转逻辑覆盖(卡片请求也会 401,静默) |

## 验证

- `pnpm lint` + `pnpm build`(frontend)
- 本地 `pnpm dev` 用真实账号目验:两卡渲染、范围切换联动简报、流水空态
- 前端若有测试基建(vitest 等)则给 `parseTrendUsageSummary` 与流水解析补单测(空数组/字段缺失/形态变体);plan 阶段确认基建后定
- 不部署;部署走既有流程(另行确认)

## 非目标(YAGNI)

- 不做余额趋势曲线(已确认流水列表即可)
- 不做流水分页/筛选/导出(卡片定位,详情页将来再说)
- 不做「今日/周/月」三列固定对比(已确认跟随选择器)
- 不改后端、不加消费侧本地流水记账(消费计费属上游职责)
- 不动现有四张卡片与入口 tiles

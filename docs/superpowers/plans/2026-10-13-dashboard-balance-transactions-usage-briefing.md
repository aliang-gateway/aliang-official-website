# Dashboard 余额流水卡 + Token 用量简报卡 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户 dashboard 新增两张卡——余额变动流水卡(充值记录 + 每笔后余额)与 token 用量聚合简报卡(范围内 tokens/请求/消费 + input/output/cache 拆分 + top 模型)。

**Architecture:** 零后端改动。流水卡走新建 BFF 路由 `/api/wallet/transactions` 透传后端现成 `GET /wallet/transactions`,卡片内自取数(仿 CacheRateCard,失败静默降级);简报卡复用 `useDashboardData` 已 fetch 的 trend 响应,新增解析保留 input/output/cache 拆分并在前端聚合,零新增请求。

**Tech Stack:** Next.js App Router(BFF route handlers)、next-intl(zh/en)、TypeScript、vitest。

**Spec:** `docs/superpowers/specs/2026-10-13-dashboard-balance-transactions-usage-briefing-design.md`

**工作目录:** 所有前端命令在 `frontend/` 下执行(pnpm)。

---

## 背景速览(给零上下文的实施者)

- Dashboard 页:`frontend/app/(app)/dashboard/page.tsx`,卡片顺序:DashboardHeader → StatusCard → CacheRateCard → TokenTrendCard → ModelShareCard → 三个入口 tile。两张新卡插在 StatusCard 之后、CacheRateCard 之前,一行两列。
- 数据 hook:`frontend/lib/hooks/use-dashboard-data.ts` 并行 fetch 6 个 BFF 接口,其中 `/api/dashboard/trend?{queryString}` 是纯透传(`frontend/app/api/dashboard/trend/route.ts`),响应的 `trend` 行含 `date/requests/input_tokens/output_tokens/cache_read_tokens/cache_creation_tokens/cost/actual_cost`。现有 `parseTokenTrendResponse` 只取 total_tokens 点,拆分被丢弃——本计划新增解析把它捡回来。
- 后端流水接口:`GET /wallet/transactions?limit=N`(已认证,`backend/internal/httpapi/routes.go:3616`),返回 `{transactions:[{id,type,amount_micros,currency,balance_after_micros,description,created_at}]}`,按 id 倒序,limit 上限 100。**当前只有 `recharge` 一种 type**(充值卡兑换);消费走 sub2api 上游计费不落本地流水。
- 前端钱包 BFF 现状:`frontend/app/api/wallet/` 下只有 `redeem/`。
- i18n:`frontend/messages/zh.json` / `en.json` 的 `dashboard` namespace(两文件 key 需同步增删)。
- 测试:vitest,用例放 `frontend/lib/__tests__/`(现有 `dashboard-analytics-adapter.test.ts` 可参考风格)。

## 文件结构(改动全景)

| 动作 | 路径 | 职责 |
| --- | --- | --- |
| Modify | `frontend/lib/dashboard-types.ts` | 新增 `TrendUsageSummary`、`WalletTransactionItem` 展示类型 |
| Modify | `frontend/lib/dashboard-home.ts` | 新增 `parseTrendUsageSummary`、`parseWalletTransactions` 纯解析函数 |
| Create | `frontend/lib/__tests__/dashboard-briefing.test.ts` | 两个解析函数的单测 |
| Modify | `frontend/lib/hooks/use-dashboard-data.ts` | 新增 `usageSummary` state 并接线 |
| Create | `frontend/app/api/wallet/transactions/route.ts` | BFF GET 透传 |
| Create | `frontend/components/dashboard/UsageBriefingCard.tsx` | 简报卡 |
| Create | `frontend/components/dashboard/WalletTransactionsCard.tsx` | 流水卡 |
| Modify | `frontend/components/dashboard/index.ts` | barrel 导出 |
| Modify | `frontend/app/(app)/dashboard/page.tsx` | 插入两卡(一行两列) |
| Modify | `frontend/messages/zh.json`、`en.json` | dashboard namespace 新 key |

---

### Task 1: 展示类型 + `parseTrendUsageSummary`(TDD)

**Files:**
- Modify: `frontend/lib/dashboard-types.ts`
- Modify: `frontend/lib/dashboard-home.ts`
- Test: `frontend/lib/__tests__/dashboard-briefing.test.ts`(新建)

- [ ] **Step 1: 在 `dashboard-types.ts` 末尾(`UnknownRecord` 之前)加类型**

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
  actual_cost: number;
};

export type WalletTransactionItem = {
  id: number;
  type: string;
  /** 主体单位(元/美元):amount_micros / 1e6,正=入账 负=扣减 */
  amount: number;
  currency: string;
  balance_after: number;
  description: string;
  created_at: string;
};
```

- [ ] **Step 2: 写失败测试 `frontend/lib/__tests__/dashboard-briefing.test.ts`**

```ts
import { describe, it, expect } from "vitest";

import { parseTrendUsageSummary, parseWalletTransactions } from "../dashboard-home";

const trendPayload = {
  data: {
    granularity: "day",
    start_date: "2026-10-07",
    end_date: "2026-10-13",
    trend: [
      {
        date: "2026-10-07",
        requests: 10,
        input_tokens: 100,
        output_tokens: 50,
        cache_read_tokens: 200,
        cache_creation_tokens: 10,
        cost: 0.1,
        actual_cost: 0.2,
      },
      {
        date: "2026-10-08",
        requests: 5,
        input_tokens: 40,
        output_tokens: 30,
        cache_read_tokens: 0,
        cache_creation_tokens: 0,
        cost: 0.05,
      },
    ],
  },
};

describe("parseTrendUsageSummary 按范围聚合 trend 行", () => {
  it("汇总 requests/input/output/cache/total/cost,total_tokens 无字段时回退 input+output", () => {
    const summary = parseTrendUsageSummary(trendPayload);
    expect(summary.requests).toBe(15);
    expect(summary.input_tokens).toBe(140);
    expect(summary.output_tokens).toBe(80);
    expect(summary.cache_read_tokens).toBe(200);
    expect(summary.cache_creation_tokens).toBe(10);
    // 第二行缺 actual_cost 回退 cost=0.05 → 0.2 + 0.05
    expect(summary.actual_cost).toBeCloseTo(0.25, 6);
    // 两行都无 total_tokens → (100+50)+(40+30)=220
    expect(summary.total_tokens).toBe(220);
    expect(summary.start_date).toBe("2026-10-07");
    expect(summary.end_date).toBe("2026-10-13");
  });

  it("空 trend 返回全 0 并保留日期", () => {
    const summary = parseTrendUsageSummary({
      data: { granularity: "day", start_date: "2026-10-07", end_date: "2026-10-13", trend: [] },
    });
    expect(summary.requests).toBe(0);
    expect(summary.total_tokens).toBe(0);
    expect(summary.actual_cost).toBe(0);
    expect(summary.start_date).toBe("2026-10-07");
  });

  it("null/畸形 payload 不抛错,返回全 0", () => {
    const summary = parseTrendUsageSummary(null);
    expect(summary.total_tokens).toBe(0);
    expect(summary.start_date).toBe("");
  });
});
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd frontend && pnpm vitest run lib/__tests__/dashboard-briefing.test.ts`
Expected: FAIL — `parseTrendUsageSummary` 未导出(import error)

- [ ] **Step 4: 在 `dashboard-home.ts` 实现**

在文件 import 的 `./dashboard-types` 类型导入列表里加入 `TrendUsageSummary`、`WalletTransactionItem`。在文件末尾(`export { parseTokenTrendResponse };` 之前)加:

```ts
export function parseTrendUsageSummary(trendPayload: unknown): TrendUsageSummary {
  const envelope = parseDashboardTrendEnvelope(trendPayload);
  const summary = envelope.trend.reduce(
    (acc, row) => ({
      requests: acc.requests + row.requests,
      input_tokens: acc.input_tokens + row.input_tokens,
      output_tokens: acc.output_tokens + row.output_tokens,
      cache_read_tokens: acc.cache_read_tokens + row.cache_read_tokens,
      cache_creation_tokens: acc.cache_creation_tokens + row.cache_creation_tokens,
      total_tokens: acc.total_tokens + row.total_tokens,
      actual_cost: acc.actual_cost + row.actual_cost,
    }),
    {
      requests: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_read_tokens: 0,
      cache_creation_tokens: 0,
      total_tokens: 0,
      actual_cost: 0,
    },
  );

  return {
    ...summary,
    start_date: envelope.start_date,
    end_date: envelope.end_date,
  };
}
```

(`parseDashboardTrendEnvelope` 已从 `./dashboard-analytics-adapter` 导入;其行级 `actual_cost` 已自带 `pickFirstFiniteNumber(actual_cost, cost, 0)` 回退,`total_tokens` 已自带 `input+output` 回退,无需在此重复处理。)

- [ ] **Step 5: 跑测试确认通过**

Run: `cd frontend && pnpm vitest run lib/__tests__/dashboard-briefing.test.ts`
Expected: PASS(3 tests)

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/dashboard-types.ts frontend/lib/dashboard-home.ts frontend/lib/__tests__/dashboard-briefing.test.ts
git commit -m "feat: parseTrendUsageSummary——按范围聚合 trend 行的 token 用量简报解析"
```

---

### Task 2: `parseWalletTransactions`(TDD)

**Files:**
- Modify: `frontend/lib/dashboard-home.ts`
- Test: `frontend/lib/__tests__/dashboard-briefing.test.ts`

- [ ] **Step 1: 追加失败测试**

```ts
describe("parseWalletTransactions 宽容解析三种形态", () => {
  const row = {
    id: 7,
    type: "recharge",
    amount_micros: 50_000_000,
    currency: "CNY",
    balance_after_micros: 120_500_000,
    description: "Recharge via card code",
    created_at: "2026-10-12T14:30:00Z",
  };

  it("标准形态 {transactions:[...]},金额 micros→主体单位", () => {
    const list = parseWalletTransactions({ transactions: [row] });
    expect(list).toHaveLength(1);
    expect(list[0]).toEqual({
      id: 7,
      type: "recharge",
      amount: 50,
      currency: "CNY",
      balance_after: 120.5,
      description: "Recharge via card code",
      created_at: "2026-10-12T14:30:00Z",
    });
  });

  it("data 包裹形态 {data:[...]} 与裸数组", () => {
    expect(parseWalletTransactions({ data: [row] })).toHaveLength(1);
    expect(parseWalletTransactions([row])).toHaveLength(1);
  });

  it("字段缺失回退:type→unknown,currency→CNY,金额→0", () => {
    const list = parseWalletTransactions({ transactions: [{ id: 1 }] });
    expect(list[0].type).toBe("unknown");
    expect(list[0].currency).toBe("CNY");
    expect(list[0].amount).toBe(0);
    expect(list[0].balance_after).toBe(0);
  });

  it("非列表 payload 返回空数组", () => {
    expect(parseWalletTransactions(null)).toEqual([]);
    expect(parseWalletTransactions({})).toEqual([]);
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd frontend && pnpm vitest run lib/__tests__/dashboard-briefing.test.ts`
Expected: FAIL — `parseWalletTransactions` 未导出

- [ ] **Step 3: 在 `dashboard-home.ts` 实现(放在 `parseTrendUsageSummary` 之后)**

```ts
export function parseWalletTransactions(payload: unknown): WalletTransactionItem[] {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  // 当前后端只返回 {transactions:[...]}(routes.go walletTransactionsResponse);
  // data 包裹与裸数组是防御性兼容,与库内 extractEnvelope 风格一致。
  const list = Array.isArray(payload)
    ? payload
    : Array.isArray(inner?.transactions)
      ? inner.transactions
      : Array.isArray(inner?.data)
        ? inner.data
        : [];

  return list
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map((item) => ({
      id: asNumber(item.id),
      type: asString(item.type) || "unknown",
      amount: asNumber(item.amount_micros) / 1_000_000,
      currency: asString(item.currency, "CNY"),
      balance_after: asNumber(item.balance_after_micros) / 1_000_000,
      description: asString(item.description),
      created_at: asString(item.created_at),
    }));
}
```

(若 `UnknownRecord` 不在 `./dashboard-types` 的既有 import 列表则补上;`asRecord`/`asString`/`asNumber` 均为该文件既有工具。)

- [ ] **Step 4: 跑测试确认通过**

Run: `cd frontend && pnpm vitest run lib/__tests__/dashboard-briefing.test.ts`
Expected: PASS(7 tests)

- [ ] **Step 5: Commit**

```bash
git add frontend/lib/dashboard-home.ts frontend/lib/__tests__/dashboard-briefing.test.ts
git commit -m "feat: parseWalletTransactions——钱包流水宽容解析(micros→主体单位)"
```

---

### Task 3: `use-dashboard-data` 接线 `usageSummary`

**Files:**
- Modify: `frontend/lib/hooks/use-dashboard-data.ts`

- [ ] **Step 1: import 区加 `parseTrendUsageSummary`(来自 `@/lib/dashboard-home`)与类型 `TrendUsageSummary`(来自 `@/lib/dashboard-types`)**

- [ ] **Step 2: `DashboardData` 类型加字段 `usageSummary: TrendUsageSummary | null;`**

- [ ] **Step 3: 加 state 并在四处同步维护**

- `const [usageSummary, setUsageSummary] = useState<TrendUsageSummary | null>(null);`
- `loadDashboard` 内无 sessionToken 早退分支、catch 块:与 `setTokenTrend(null)` 并排加 `setUsageSummary(null);`(共两处)
- 成功路径:在 `setTokenTrend(parseTokenTrendResponse(trendPayload));` 之后加 `setUsageSummary(parseTrendUsageSummary(trendPayload));`(同一 payload,零新增请求)
- 外层 effect 的 `!sessionToken` 分支:同样加 `setUsageSummary(null);`

- [ ] **Step 4: return 对象加 `usageSummary,`**

- [ ] **Step 5: 类型与构建检查**

Run: `cd frontend && pnpm lint && pnpm build`
Expected: 无新增错误(hook 尚无消费者,纯增量)

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/hooks/use-dashboard-data.ts
git commit -m "feat: useDashboardData 暴露 usageSummary(复用 trend 响应,零新增请求)"
```

---

### Task 4: BFF 路由 `/api/wallet/transactions`

**Files:**
- Create: `frontend/app/api/wallet/transactions/route.ts`

- [ ] **Step 1: 新建文件(仿同目录 `redeem/route.ts` 的解析-回传风格)**

```ts
import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// 透传后端 GET /wallet/transactions(用户钱包流水,按 id 倒序)。
// 与 redeem 同款 BFF 直通:上游状态码原样返回,limit 由查询串控制。
export async function GET(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const url = new URL(request.url);
  let upstream: Response;
  try {
    upstream = await fetch(`${apiBaseUrl}/wallet/transactions${url.search}`, {
      method: "GET",
      headers: {
        "content-type": request.headers.get("content-type") ?? "application/json",
        accept: request.headers.get("accept") ?? "application/json",
        Authorization: request.headers.get("Authorization") ?? "",
      },
      cache: "no-store",
    });
  } catch {
    return NextResponse.json({ error: "upstream request failed" }, { status: 502 });
  }

  try {
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json({ error: "invalid json response from upstream" }, { status: 502 });
  }
}
```

- [ ] **Step 2: 构建检查**

Run: `cd frontend && pnpm lint`
Expected: 通过

- [ ] **Step 3: Commit**

```bash
git add frontend/app/api/wallet/transactions/route.ts
git commit -m "feat: BFF 透传 GET /wallet/transactions(用户钱包流水)"
```

---

### Task 5: i18n 文案(zh + en)

**Files:**
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: 两文件 `dashboard` namespace 内(任意稳定位置,建议紧邻 `balance`/`todayRequests` 一组指标 key 之后)插入同序 key**

zh.json:

```json
"usageBriefing": "用量简报",
"usageBriefingDescription": "所选时间范围内的 Token 用量与消费一览。",
"briefingRequests": "请求次数",
"briefingSpend": "消费金额",
"briefingInputTokens": "输入",
"briefingOutputTokens": "输出",
"briefingCacheRead": "缓存读",
"briefingCacheCreation": "缓存写",
"topModel": "最常用模型",
"briefingEmpty": "所选范围内暂无请求数据。",
"walletTransactions": "余额变动",
"walletTransactionsDescription": "最近的钱包流水与每笔变动后的余额。",
"transactionRecharge": "充值",
"walletTransactionsEmpty": "暂无流水记录。"
```

en.json:

```json
"usageBriefing": "Usage Briefing",
"usageBriefingDescription": "Token usage and spend for the selected range at a glance.",
"briefingRequests": "Requests",
"briefingSpend": "Spend",
"briefingInputTokens": "Input",
"briefingOutputTokens": "Output",
"briefingCacheRead": "Cache read",
"briefingCacheCreation": "Cache write",
"topModel": "Top model",
"briefingEmpty": "No requests recorded in this range yet.",
"walletTransactions": "Balance Activity",
"walletTransactionsDescription": "Recent wallet transactions and the balance after each one.",
"transactionRecharge": "Recharge",
"walletTransactionsEmpty": "No transactions yet."
```

注意:未知 tx_type 徽章直接显示原始 type 文本,不走 i18n(spec 既定)。

- [ ] **Step 2: 校验两文件 JSON 合法且 key 集合一致**

Run: `cd frontend && node -e "const zh=require('./messages/zh.json'), en=require('./messages/en.json'); const a=Object.keys(zh.dashboard), b=Object.keys(en.dashboard); console.log(a.length, b.length, a.filter(k=>!b.includes(k)).length, b.filter(k=>!a.includes(k)).length)"`
Expected: 两侧 key 数相同、差集为 0(`0 0` 結尾)

- [ ] **Step 3: Commit**

```bash
git add frontend/messages/zh.json frontend/messages/en.json
git commit -m "feat: dashboard 简报卡/流水卡 zh+en 文案"
```

---

### Task 6: `UsageBriefingCard` 组件

**Files:**
- Create: `frontend/components/dashboard/UsageBriefingCard.tsx`

- [ ] **Step 1: 新建组件**

```tsx
"use client";

import { useTranslations } from "next-intl";

import { formatMetricCurrency, formatMetricNumber, formatShortDate } from "@/lib/dashboard-format";
import { TREND_RANGE_OPTIONS } from "@/lib/dashboard-format";
import type { ModelShareDatum, TrendRange, TrendUsageSummary } from "@/lib/dashboard-types";

type ModelShare = { start_date: string; end_date: string; items: ModelShareDatum[] };

type UsageBriefingCardProps = {
  usageSummary: TrendUsageSummary | null;
  modelShare: ModelShare | null;
  selectedRange: TrendRange;
};

const MONO = { fontFamily: "var(--font-editorial-mono)" } as const;

function compact(value: number) {
  return formatMetricNumber(value, { notation: "compact", maximumFractionDigits: 1 });
}

/** 首页用量简报卡:范围内 tokens/请求/消费 + input/output/cache 拆分 + top 模型。
 *  数据全部来自 useDashboardData 已拉取的 trend/models 响应,零新增请求。
 *  范围内无数据时指标显示 "--" 并给空态文案,不整卡隐藏(简报是固定信息位)。 */
export function UsageBriefingCard({ usageSummary, modelShare, selectedRange }: UsageBriefingCardProps) {
  const t = useTranslations("dashboard");
  const rangeLabel = TREND_RANGE_OPTIONS.find((option) => option.value === selectedRange)?.label ?? selectedRange;
  const hasData = (usageSummary?.requests ?? 0) > 0;

  const headline = [
    { label: t("totalTokens"), value: usageSummary ? compact(usageSummary.total_tokens) : "--" },
    { label: t("briefingRequests"), value: usageSummary ? formatMetricNumber(usageSummary.requests) : "--" },
    { label: t("briefingSpend"), value: usageSummary ? formatMetricCurrency(usageSummary.actual_cost) : "--" },
  ];

  const breakdown = [
    { label: t("briefingInputTokens"), value: usageSummary?.input_tokens ?? null },
    { label: t("briefingOutputTokens"), value: usageSummary?.output_tokens ?? null },
    { label: t("briefingCacheRead"), value: usageSummary?.cache_read_tokens ?? null },
    { label: t("briefingCacheCreation"), value: usageSummary?.cache_creation_tokens ?? null },
  ];

  const topModel = modelShare?.items[0] ?? null;

  return (
    <article className="block-card min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]" style={MONO}>
            {t("usageBriefing")}
          </p>
          <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("usageBriefing")}</h2>
          <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("usageBriefingDescription")}</p>
        </div>
        <div className="shrink-0 rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] px-3 py-1 text-xs font-semibold text-[var(--portal-muted)]">
          {rangeLabel}
          {usageSummary?.start_date ? ` · ${formatShortDate(usageSummary.start_date)} → ${formatShortDate(usageSummary.end_date)}` : ""}
        </div>
      </div>

      <ol className="mt-4 grid grid-cols-3 gap-3">
        {headline.map((cell) => (
          <li key={cell.label} className="min-w-0 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
            <p className="truncate text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--portal-muted)]" style={MONO}>
              {cell.label}
            </p>
            <p className="mt-1 truncate text-2xl font-extrabold tracking-tight text-[var(--portal-ink)]">{cell.value}</p>
          </li>
        ))}
      </ol>

      <ol className="mt-3 grid grid-cols-4 gap-3">
        {breakdown.map((cell) => (
          <li key={cell.label} className="min-w-0">
            <p className="truncate text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--portal-muted)]" style={MONO}>
              {cell.label}
            </p>
            <p className="mt-1 truncate font-semibold text-[var(--portal-ink)]">{cell.value === null ? "--" : compact(cell.value)}</p>
          </li>
        ))}
      </ol>

      {topModel ? (
        <p className="mt-3 truncate text-sm text-[var(--portal-muted)]">
          {t("topModel")}:{topModel.model} · {(topModel.share * 100).toFixed(1)}%
        </p>
      ) : null}

      {!hasData ? <p className="mt-3 text-sm text-[var(--portal-muted)]">{t("briefingEmpty")}</p> : null}
    </article>
  );
}
```

(样式变量与 StatusCard/ModelShareCard 同系:`block-card`、`--portal-*`、MONO 小标签;`share` 是 0–1 小数,展示 ×100。若 `--accent-wash` 等变量在两种主题下观感不对,以相邻卡实际用色为准微调,不改数据逻辑。)

- [ ] **Step 2: lint 检查**

Run: `cd frontend && pnpm lint`
Expected: 通过(组件尚无消费者,允许暂未导出)

- [ ] **Step 3: Commit**

```bash
git add frontend/components/dashboard/UsageBriefingCard.tsx
git commit -m "feat: UsageBriefingCard——token 用量聚合简报卡"
```

---

### Task 7: `WalletTransactionsCard` 组件

**Files:**
- Create: `frontend/components/dashboard/WalletTransactionsCard.tsx`

- [ ] **Step 1: 新建组件(自取数逐字仿 `CacheRateCard`:sessionToken 守卫、AbortController、cancelled、失败静默)**

```tsx
"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import { parseWalletTransactions } from "@/lib/dashboard-home";
import type { WalletTransactionItem } from "@/lib/dashboard-types";

const TRANSACTIONS_LIMIT = 8;

const MONO = { fontFamily: "var(--font-editorial-mono)" } as const;

/** 非法货币码会让 Intl.NumberFormat 抛 RangeError,先收敛到合法形态。 */
function formatAmount(value: number, currency: string) {
  const code = /^[A-Za-z]{3}$/.test(currency) ? currency.toUpperCase() : "CNY";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: code,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.abs(value));
}

function formatTransactionTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

/** 首页余额变动卡:最近 8 笔钱包流水 + 每笔后余额。卡片自取数,
 *  失败静默降级为空态,不影响页面其他部分(与 CacheRateCard 同策略)。
 *  金额符号/颜色按 amount 正负渲染,不按 tx_type 硬编码——后端将来
 *  新增消费类型时 UI 自动适配;未知 type 显示原始文本。 */
export function WalletTransactionsCard({ sessionToken }: { sessionToken: string }) {
  const t = useTranslations("dashboard");
  const [transactions, setTransactions] = useState<WalletTransactionItem[] | null>(null);

  useEffect(() => {
    if (!sessionToken) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();

    const load = async () => {
      try {
        const response = await fetch(`/api/wallet/transactions?limit=${TRANSACTIONS_LIMIT}`, {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as unknown;
        if (!cancelled) {
          setTransactions(parseWalletTransactions(payload));
        }
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }
        // 静默:流水取不到就维持空态,不打断首屏。
      }
    };

    void load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [sessionToken]);

  const list = transactions ?? [];

  return (
    <article className="block-card min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]" style={MONO}>
            {t("walletTransactions")}
          </p>
          <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("walletTransactions")}</h2>
          <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("walletTransactionsDescription")}</p>
        </div>
        <div className="shrink-0 rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] px-3 py-1 text-xs font-semibold text-[var(--portal-muted)]">
          {t("points", { count: list.length })}
        </div>
      </div>

      {list.length === 0 ? (
        <p className="mt-4 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 text-sm text-[var(--portal-muted)]">
          {t("walletTransactionsEmpty")}
        </p>
      ) : (
        <ol className="mt-4 divide-y divide-[var(--portal-line)] rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)]">
          {list.map((txn) => {
            const positive = txn.amount > 0;
            const negative = txn.amount < 0;
            return (
              <li key={txn.id} className="flex items-center gap-3 px-4 py-3">
                <span className="shrink-0 rounded-full border border-[var(--portal-line)] bg-[var(--bone)]/55 px-2.5 py-0.5 text-xs font-bold text-[var(--portal-muted)]">
                  {txn.type === "recharge" ? t("transactionRecharge") : txn.type}
                </span>
                <span
                  className={`shrink-0 font-semibold tabular-nums ${
                    positive ? "text-emerald-600 dark:text-emerald-300" : negative ? "text-red-600 dark:text-red-300" : "text-[var(--portal-muted)]"
                  }`}
                >
                  {positive ? "+" : negative ? "−" : ""}
                  {formatAmount(txn.amount, txn.currency)}
                </span>
                <span className="min-w-0 flex-1 truncate text-right text-xs text-[var(--portal-muted)]">
                  {t("balance")}: {formatAmount(txn.balance_after, txn.currency)} · {formatTransactionTime(txn.created_at)}
                </span>
              </li>
            );
          })}
        </ol>
      )}
    </article>
  );
}
```

(`--bone` 变量来自 ModelShareCard 同款用法;余额列复用既有 `balance` key。)

- [ ] **Step 2: lint 检查**

Run: `cd frontend && pnpm lint`
Expected: 通过

- [ ] **Step 3: Commit**

```bash
git add frontend/components/dashboard/WalletTransactionsCard.tsx
git commit -m "feat: WalletTransactionsCard——余额变动流水卡(自取数,失败静默)"
```

---

### Task 8: 页面集成 + barrel 导出

**Files:**
- Modify: `frontend/components/dashboard/index.ts`
- Modify: `frontend/app/(app)/dashboard/page.tsx`

- [ ] **Step 1: `index.ts` 追加两行**

```ts
export { UsageBriefingCard } from "./UsageBriefingCard";
export { WalletTransactionsCard } from "./WalletTransactionsCard";
```

- [ ] **Step 2: `page.tsx` 引入组件**

import 块的 `@/components/dashboard` 具名导入列表加入 `UsageBriefingCard, WalletTransactionsCard`。

- [ ] **Step 3: 在 `<StatusCard …/>` 与 `{/* 缓存命中率 */}` 注释之间插入**

```tsx
      {/* ★ 用量简报 + 余额变动(一行两列,窄屏堆叠) */}
      <div className="grid gap-4 lg:grid-cols-2">
        <UsageBriefingCard
          usageSummary={data.usageSummary}
          modelShare={data.modelShare}
          selectedRange={trend.selectedRange}
        />
        <WalletTransactionsCard sessionToken={data.sessionToken} />
      </div>
```

- [ ] **Step 4: 构建检查**

Run: `cd frontend && pnpm lint && pnpm build`
Expected: 通过

- [ ] **Step 5: Commit**

```bash
git add frontend/components/dashboard/index.ts "frontend/app/(app)/dashboard/page.tsx"
git commit -m "feat: dashboard 集成用量简报卡与余额变动卡(一行两列)"
```

---

### Task 9: 终验

- [ ] **Step 1: 全量测试**

Run: `cd frontend && pnpm test`
Expected: 全部 PASS(含新增 7 例,无既有用例回归)

- [ ] **Step 2: lint + build**

Run: `cd frontend && pnpm lint && pnpm build`
Expected: 通过

- [ ] **Step 3: 本地目验(需有效账号 session)**

Run: `cd frontend && pnpm dev` 后登录进 `/dashboard`,核对:
1. 简报卡:总 tokens/请求/消费与曲线同范围一致;切换范围(7d/30d/90d)简报数字与徽标日期联动
2. 简报卡拆分行四项数字非负;top 模型行显示且百分比 = share×100
3. 流水卡:有充值记录时显示行(金额 +¥xx.xx 绿色、变动后余额、时间);无记录时空态文案;拔掉后端(或 401)时页面其余部分不受影响
4. 窄屏(<lg)两卡堆叠不溢出

- [ ] **Step 4: 汇总报告**

向需求方报告验证结果;合并/部署走既有流程,不在本计划范围内擅自执行。

---

## 风险与回滚

- 全部改动在前端,逐 task 独立 commit,任一步出问题 `git revert` 对应提交即可
- `parseTrendUsageSummary` 对畸形 payload 全零兜底,不会让首页原有卡片失败(它只新增 state,不改既有解析)
- 流水卡与 CacheRateCard 同为自取数,请求失败互不影响

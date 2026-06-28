// Dashboard 首页数据聚合层:把多个上游 payload 组合成首页响应。
// 从 app/dashboard/page.tsx 提取,保持行为逐字一致。

import { asRecord, asString } from "@/lib/api-response";

import {
  parseDashboardModelsEnvelope,
  parseDashboardSimpleTrendPoints,
  parseDashboardTrendEnvelope,
} from "./dashboard-analytics-adapter";
import { isTrendGranularity } from "./dashboard-format";
import type {
  BalanceSummary,
  DashboardHomeResponse,
  DashboardMetricSummary,
  ModelShareDatum,
  PackageQuota,
  PackageSummary,
  PurchaseOptions,
  TokenTrendResponse,
  TrendSeries,
  UnknownRecord,
} from "./dashboard-types";

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function asOptionalNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function pickFirstFiniteNumberOrNull(...values: unknown[]) {
  for (const value of values) {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
  }
  return null;
}

function parseTrendSeries(value: unknown): TrendSeries {
  const points = parseDashboardSimpleTrendPoints(value);
  return {
    aggregation_owner: "dashboard_app",
    aggregation_reason: "upstream_raw_logs_incomplete",
    interval: "day",
    points,
  };
}

function parseTokenTrendResponse(payload: unknown): TokenTrendResponse {
  const envelope = parseDashboardTrendEnvelope(payload);
  const granularity = isTrendGranularity(envelope.granularity) ? envelope.granularity : "day";

  return {
    series: {
      aggregation_owner: "dashboard_app",
      aggregation_reason: "upstream_raw_logs_incomplete",
      interval: granularity,
      points: parseDashboardSimpleTrendPoints(payload, "total_tokens"),
    },
    start_date: envelope.start_date,
    end_date: envelope.end_date,
    granularity,
  };
}

function extractEnvelopeOrRoot(payload: unknown) {
  const root = asRecord(payload);
  return asRecord(root?.data) ?? root;
}

export function parseDashboardMetricSummary(homePayload: unknown, accountPayload: unknown): DashboardMetricSummary {
  const stats = extractEnvelopeOrRoot(homePayload);
  const profile = extractEnvelopeOrRoot(accountPayload);
  const profileBalance = asOptionalNumber(profile?.balance) ?? asOptionalNumber(asRecord(profile?.balance)?.amount);

  return {
    balance: profileBalance,
    today_requests: asOptionalNumber(stats?.today_requests),
    today_spend: pickFirstFiniteNumberOrNull(stats?.today_actual_cost, stats?.today_cost),
    today_token: asOptionalNumber(stats?.today_tokens),
    cumulative_token: asOptionalNumber(stats?.total_tokens),
  };
}

export function normalizeModelShareData(payload: unknown): { start_date: string; end_date: string; items: ModelShareDatum[] } {
  const envelope = parseDashboardModelsEnvelope(payload);
  const palette = ["#06b6d4", "#10b981", "#f59e0b", "#8b5cf6", "#ef4444", "#14b8a6", "#f97316", "#6366f1"];
  const ranked = envelope.models
    .map((item) => ({ model: item.model, value: item.total_tokens }))
    .filter((item) => item.model && item.value > 0)
    .sort((left, right) => {
      if (right.value !== left.value) {
        return right.value - left.value;
      }
      return left.model.localeCompare(right.model, "en", { sensitivity: "base" });
    });
  const total = ranked.reduce((sum, item) => sum + item.value, 0);

  return {
    start_date: envelope.start_date,
    end_date: envelope.end_date,
    items: total > 0
      ? ranked.map((item, index) => ({
          model: item.model,
          value: item.value,
          share: item.value / total,
          stroke: palette[index % palette.length],
        }))
      : [],
  };
}

function buildSinglePointTrendSeries(value: number): TrendSeries {
  return {
    aggregation_owner: "dashboard_app",
    aggregation_reason: "upstream_raw_logs_incomplete",
    interval: "day",
    points: [{ bucket_start: new Date().toISOString(), value }],
  };
}

function parsePackageQuotaListFromSubscription(subscription: UnknownRecord): PackageQuota[] {
  const periodDefinitions = [
    { key: "daily", label: "Daily" },
    { key: "weekly", label: "Weekly" },
    { key: "monthly", label: "Monthly" },
  ] as const;

  return periodDefinitions.map(({ key, label }) => {
    const usedUSD = asOptionalNumber(subscription[`${key}_used_usd`]);
    const limitUSD = asOptionalNumber(subscription[`${key}_limit_usd`]);

    return {
      period: key,
      label,
      used_usd: usedUSD,
      limit_usd: limitUSD,
      percentage: usedUSD !== null && limitUSD !== null && limitUSD > 0 ? (usedUSD / limitUSD) * 100 : null,
    };
  });
}

export function parsePackageSummaries(subscriptionPayload: unknown): PackageSummary[] {
  const root = extractEnvelopeOrRoot(subscriptionPayload);
  const subscriptions = Array.isArray(root?.subscriptions) ? root.subscriptions : [];

  return subscriptions
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map((subscription) => {
      const tierCode = String(subscription.group_id ?? "").trim();
      const tierName = asString(subscription.group_name) || asString(asRecord(subscription.group)?.name);
      const quotas = parsePackageQuotaListFromSubscription(subscription);
      const status = asString(subscription.status) || "unconfigured";

      return {
        status,
        tier_code: tierCode || null,
        tier_name: tierName || null,
        subscription_id: asNumber(subscription.id, 0) || null,
        expires_at: asString(subscription.expires_at) || null,
        quotas,
      };
    })
    .filter((item) => item.tier_name || item.subscription_id !== null || item.quotas.some((quota) => quota.limit_usd !== null || quota.used_usd !== null));
}

function parsePackageSummary(subscriptionPayload: unknown): PackageSummary {
  const summaries = parsePackageSummaries(subscriptionPayload);
  const activeSubscription = summaries.find((item) => item.status === "active") ?? summaries[0];

  if (!activeSubscription) {
    return {
      status: "unconfigured",
      tier_code: null,
      tier_name: null,
      subscription_id: null,
      expires_at: null,
      quotas: [],
    };
  }
  return activeSubscription;
}

function parseBalanceSummary(accountPayload: unknown): BalanceSummary {
  const root = asRecord(accountPayload);
  const wallet = asRecord(root?.wallet) ?? root;
  if (!wallet) {
    return {
      balance_micros: 0,
      currency: "CNY",
      updated_at: null,
    };
  }

  return {
    balance_micros: asNumber(wallet.balance_micros),
    currency: asString(wallet.currency, "CNY"),
    updated_at: asString(wallet.updated_at) || null,
  };
}

function parsePurchaseOptions(groupsPayload: unknown, currencyHint: string): PurchaseOptions {
  const root = extractEnvelopeOrRoot(groupsPayload);
  const tiersRaw = Array.isArray(root) ? root : Array.isArray(root?.tiers) ? root.tiers : [];
  const tiers = tiersRaw
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map((item) => ({
      code: asString(item.code) || asString(item.id),
      name: asString(item.name),
    }))
    .filter((item) => item.code && item.name);

  return {
    package_purchase: {
      durations: [
        { code: "one_week", label: "1 week", days: 7 },
        { code: "one_month", label: "1 month", days: 30 },
        { code: "three_months", label: "3 months", days: 90 },
      ],
      tiers,
    },
    prepaid_topup: {
      entry_mode: "redeem_code",
      redeem_endpoint: "/api/wallet/redeem",
      currency_hint: currencyHint,
    },
  };
}

export function parseDashboardHomePayload(
  homePayload: unknown,
  subscriptionPayload: unknown,
  accountPayload: unknown,
  groupsPayload: unknown,
): DashboardHomeResponse {
  const home = asRecord(homePayload);
  const stats = extractEnvelopeOrRoot(homePayload);
  const todayRequests = stats ? asNumber(stats.today_requests) : 0;
  const todayTokens = stats ? asNumber(stats.today_tokens) : 0;
  const requestTrendSource =
    home?.request_trend ??
    home?.requestTrend ??
    home?.requests_trend ??
    home?.api_request_trend ??
    home?.requests ??
    home?.request_points;
  const tokenTrendSource = home?.token_trend ?? home?.tokenTrend ?? home?.tokens_trend ?? home?.token_points;
  const requestTrend =
    requestTrendSource !== undefined
      ? parseTrendSeries(requestTrendSource)
      : buildSinglePointTrendSeries(todayRequests);
  const tokenTrend =
    tokenTrendSource !== undefined
      ? parseTrendSeries(tokenTrendSource)
      : buildSinglePointTrendSeries(todayTokens);

  const packageSummary = parsePackageSummary(subscriptionPayload);
  const balanceSummary = parseBalanceSummary(accountPayload);
  const purchaseOptions = parsePurchaseOptions(groupsPayload, balanceSummary.currency || "CNY");

  return {
    request_trend: requestTrend,
    token_trend: tokenTrend,
    package_summary: packageSummary,
    package_summaries: parsePackageSummaries(subscriptionPayload),
    balance_summary: balanceSummary,
    purchase_options: purchaseOptions,
  };
}

export { parseTokenTrendResponse };

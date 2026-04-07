"use client";

import Link from "next/link";
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";
import { parseDashboardModelsEnvelope, parseDashboardSimpleTrendPoints, parseDashboardTrendEnvelope } from "@/lib/dashboard-analytics-adapter";

const SESSION_TOKEN_STORAGE_KEY = "session_token";
const DASHBOARD_CONFIG_KEY_STORAGE_KEY = "dashboard_config_user_key";
const DASHBOARD_TREND_TIMEZONE = "Asia/Shanghai";

type TrendRange = "7d" | "30d" | "90d";
type TrendGranularity = "day" | "week" | "month";

type ClientTemplateId = "opencode" | "claude" | "codex";
type TemplateFormat = "json" | "yaml" | "shell";

type TrendPoint = {
  bucket_start: string;
  value: number;
};

type TrendSeries = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  interval: TrendGranularity;
  points: TrendPoint[];
};

type TokenTrendResponse = {
  series: TrendSeries;
  start_date: string;
  end_date: string;
  granularity: TrendGranularity;
};

type PackageQuota = {
  period: "daily" | "weekly" | "monthly";
  label: string;
  used_usd: number | null;
  limit_usd: number | null;
  percentage: number | null;
};

type PackageSummary = {
  status: string;
  tier_code: string | null;
  tier_name: string | null;
  subscription_id: number | null;
  expires_at: string | null;
  quotas: PackageQuota[];
};

type BalanceSummary = {
  balance_micros: number;
  currency: string;
  updated_at: string | null;
};

type PurchaseOptions = {
  package_purchase: {
    durations: Array<{
      code: "one_week" | "one_month" | "three_months";
      label: string;
      days: number;
    }>;
    tiers: Array<{
      code: string;
      name: string;
    }>;
  };
  prepaid_topup: {
    entry_mode: "redeem_code";
    redeem_endpoint: "/api/wallet/redeem";
    currency_hint: string;
  };
};

type DashboardHomeResponse = {
  request_trend: TrendSeries;
  token_trend: TrendSeries;
  package_summary: PackageSummary;
  package_summaries: PackageSummary[];
  balance_summary: BalanceSummary;
  purchase_options: PurchaseOptions;
};

type DashboardMetricSummary = {
  balance: number | null;
  today_requests: number | null;
  today_spend: number | null;
  today_token: number | null;
  cumulative_token: number | null;
};

type ModelShareDatum = {
  model: string;
  value: number;
  share: number;
  stroke: string;
};

type UnknownRecord = Record<string, unknown>;

type PurchaseMessageTone = "success" | "error" | "info";
type TicketMessageTone = "success" | "error";

type TemplateDefinition = {
  id: ClientTemplateId;
  labelKey: string;
  helperKey: string;
  supportedFormats: TemplateFormat[];
};

const TEMPLATE_DEFINITIONS: TemplateDefinition[] = [
  {
    id: "opencode",
    labelKey: "templateOpencodeLabel",
    helperKey: "templateOpencodeHelper",
    supportedFormats: ["json"],
  },
  {
    id: "claude",
    labelKey: "templateClaudeLabel",
    helperKey: "templateClaudeHelper",
    supportedFormats: ["shell"],
  },
  {
    id: "codex",
    labelKey: "templateCodexLabel",
    helperKey: "templateCodexHelper",
    supportedFormats: ["json", "yaml"],
  },
];

const TREND_RANGE_OPTIONS: Array<{ value: TrendRange; label: string }> = [
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" },
];

const TREND_GRANULARITY_OPTIONS: Array<{ value: TrendGranularity; labelKey: string }> = [
  { value: "day", labelKey: "dayLabel" },
  { value: "week", labelKey: "weekLabel" },
  { value: "month", labelKey: "monthLabel" },
];

const ALLOWED_TREND_GRANULARITY: Record<TrendRange, TrendGranularity[]> = {
  "7d": ["day"],
  "30d": ["day", "week"],
  "90d": ["day", "week", "month"],
};

function escapeJsonString(value: string) {
  return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
}

function escapeSingleQuotedShell(value: string) {
  return value.replaceAll("'", "'\\''");
}

function formatYamlScalar(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
}

function isTrendGranularity(value: string): value is TrendGranularity {
  return value === "day" || value === "week" || value === "month";
}

function isTrendRange(value: string): value is TrendRange {
  return value === "7d" || value === "30d" || value === "90d";
}

function normalizeTrendGranularity(range: TrendRange, granularity: TrendGranularity) {
  const allowedGranularity = ALLOWED_TREND_GRANULARITY[range];
  if (allowedGranularity.includes(granularity)) {
    return granularity;
  }

  const granularityRank: Record<TrendGranularity, number> = {
    day: 0,
    week: 1,
    month: 2,
  };

  return allowedGranularity.reduce<TrendGranularity>((closest, candidate) => {
    const candidateDistance = Math.abs(granularityRank[candidate] - granularityRank[granularity]);
    const closestDistance = Math.abs(granularityRank[closest] - granularityRank[granularity]);
    return candidateDistance < closestDistance ? candidate : closest;
  }, allowedGranularity[0]);
}

function getTrendRangeDays(range: TrendRange) {
  if (range === "30d") {
    return 30;
  }
  if (range === "90d") {
    return 90;
  }
  return 7;
}

function formatDateParts(date: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);

  const year = parts.find((part) => part.type === "year")?.value ?? "1970";
  const month = parts.find((part) => part.type === "month")?.value ?? "01";
  const day = parts.find((part) => part.type === "day")?.value ?? "01";

  return `${year}-${month}-${day}`;
}

function buildTrendDateRange(range: TrendRange, timeZone: string) {
  const endDate = new Date();
  const startDate = new Date(endDate.getTime() - (getTrendRangeDays(range) - 1) * 24 * 60 * 60 * 1000);

  return {
    start_date: formatDateParts(startDate, timeZone),
    end_date: formatDateParts(endDate, timeZone),
  };
}

function buildTemplateContent(templateId: ClientTemplateId, format: TemplateFormat, userKey: string) {
  const baseUrl = "https://api.aliang.one";
  const escapedKey = escapeJsonString(userKey);
  const escapedBaseUrl = escapeJsonString(baseUrl);
  const yamlKey = formatYamlScalar(userKey);
  const yamlBaseUrl = formatYamlScalar(baseUrl);

  if (templateId === "opencode") {
    return [
      "{",
      '  "provider": "custom",',
      `  "base_url": "${escapedBaseUrl}/v1",`,
      `  "api_key": "${escapedKey}",`,
      '  "model": "claude-sonnet-4-20250514"',
      "}",
    ].join("\n");
  }

  if (templateId === "claude") {
    return [
      `export ANTHROPIC_BASE_URL='${escapeSingleQuotedShell(baseUrl)}'`,
      `export ANTHROPIC_AUTH_TOKEN='${escapeSingleQuotedShell(userKey)}'`,
      "claude",
    ].join("\n");
  }

  if (templateId === "codex") {
    if (format === "yaml") {
      return [
        "provider: openai",
        `base_url: ${yamlBaseUrl}`,
        `api_key: ${yamlKey}`,
        "model: gpt-4.1",
      ].join("\n");
    }

    return [
      "{",
      '  "provider": "openai",',
      `  "base_url": "${escapedBaseUrl}",`,
      `  "api_key": "${escapedKey}",`,
      '  "model": "gpt-4.1"',
      "}",
    ].join("\n");
  }

  return "";
}

function formatShortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(date);
}

function buildPreviewPoints(points: TrendPoint[], fallbackStep: number) {
  if (points.length > 0) {
    return points;
  }

  return Array.from({ length: 7 }, (_, index) => ({
    bucket_start: new Date(Date.now() - (6 - index) * 24 * 60 * 60 * 1000).toISOString(),
    value: fallbackStep * (index + 1),
  }));
}

function asNumber(value: unknown, fallback = 0) {
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

function parseDashboardMetricSummary(homePayload: unknown, accountPayload: unknown): DashboardMetricSummary {
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

function normalizeModelShareData(payload: unknown): { start_date: string; end_date: string; items: ModelShareDatum[] } {
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

function parsePackageSummaries(subscriptionPayload: unknown): PackageSummary[] {
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

function parseDashboardHomePayload(homePayload: unknown, subscriptionPayload: unknown, accountPayload: unknown, groupsPayload: unknown): DashboardHomeResponse {
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

function formatMetricNumber(value: number | null, options?: Intl.NumberFormatOptions) {
  if (value === null) {
    return "--";
  }

  return new Intl.NumberFormat("en-US", options).format(value);
}

function formatMetricCurrency(value: number | null) {
  if (value === null) {
    return "--";
  }

  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function formatUsagePercentage(value: number | null) {
  if (value === null) {
    return "--";
  }

  return `${value.toFixed(1)}%`;
}

function describePercentage(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function polarToCartesian(centerX: number, centerY: number, radius: number, angleInDegrees: number) {
  const angleInRadians = ((angleInDegrees - 90) * Math.PI) / 180;
  return {
    x: centerX + radius * Math.cos(angleInRadians),
    y: centerY + radius * Math.sin(angleInRadians),
  };
}

function buildArcPath(centerX: number, centerY: number, radius: number, startAngle: number, endAngle: number) {
  const start = polarToCartesian(centerX, centerY, radius, endAngle);
  const end = polarToCartesian(centerX, centerY, radius, startAngle);
  const largeArcFlag = endAngle - startAngle > 180 ? 1 : 0;

  return `M ${centerX} ${centerY} L ${start.x} ${start.y} A ${radius} ${radius} 0 ${largeArcFlag} 0 ${end.x} ${end.y} Z`;
}

function ModelSharePieChart({
  items,
  startDate,
  endDate,
}: {
  items: ModelShareDatum[];
  startDate: string;
  endDate: string;
}) {
  const total = items.reduce((sum, item) => sum + item.value, 0);

  if (items.length === 0 || total <= 0) {
    return (
      <div className="mt-4 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 text-sm text-[var(--portal-muted)]">
        No model-share data is available for the selected period yet. The pie stays empty until at least one model reports non-zero total tokens.
      </div>
    );
  }

  const segments = items.reduce<Array<ModelShareDatum & { path: string; startAngle: number; endAngle: number }>>((acc, item) => {
    const startAngle = acc[acc.length - 1]?.endAngle ?? 0;
    const sweepAngle = item.share * 360;
    const endAngle = startAngle + sweepAngle;

    acc.push({
      ...item,
      startAngle,
      endAngle,
      path: buildArcPath(50, 50, 42, startAngle, endAngle),
    });

    return acc;
  }, []);

  return (
    <div className="mt-4 min-w-0 overflow-hidden rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
      <div className="grid min-w-0 gap-4">
        <div className="flex items-center justify-center">
          <svg viewBox="0 0 100 100" className="h-52 w-52" aria-label="Model share pie chart">
            <circle cx="50" cy="50" r="42" fill="rgba(255,255,255,0.45)" className="dark:fill-[rgba(15,23,42,0.4)]" />
            {segments.map((segment) => (
              <path key={segment.model} d={segment.path} fill={segment.stroke} stroke="var(--portal-clay-strong)" strokeWidth="1.4" />
            ))}
            <circle cx="50" cy="50" r="18" fill="var(--portal-clay-strong)" />
            <text x="50" y="46" textAnchor="middle" className="fill-[var(--portal-muted)] text-[5px] uppercase tracking-[0.24em]">
              Tokens
            </text>
            <text x="50" y="56" textAnchor="middle" className="fill-[var(--portal-ink)] text-[8px] font-semibold">
              {formatMetricNumber(total, { notation: "compact", maximumFractionDigits: 1 })}
            </text>
          </svg>
        </div>

        <div className="grid gap-3 min-w-0">
          {segments.map((segment) => (
            <div key={`${segment.model}-legend`} className="min-w-0 rounded-[1rem] border border-[var(--portal-line)] bg-white/55 p-3 dark:bg-slate-950/30">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: segment.stroke }} aria-hidden="true" />
                    <p className="truncate text-sm font-semibold text-[var(--portal-ink)]">{segment.model}</p>
                  </div>
                  <p className="mt-1 text-xs text-[var(--portal-muted)]">
                    {startDate || "--"} → {endDate || "--"}
                  </p>
                </div>
                <p className="text-sm font-semibold text-[var(--portal-ink)]">{describePercentage(segment.share)}</p>
              </div>
              <p className="mt-2 text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                {formatMetricNumber(segment.value)} total tokens
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function TrendPreview({
  points,
  tone,
}: {
  points: TrendPoint[];
  tone: "emerald" | "cyan";
}) {
  const preview = useMemo(() => buildPreviewPoints(points, tone === "emerald" ? 12 : 3200), [points, tone]);
  const maxValue = Math.max(...preview.map((point) => point.value), 1);

  const coordinates = preview
    .map((point, index) => {
      const x = (index / Math.max(preview.length - 1, 1)) * 100;
      const y = 100 - (point.value / maxValue) * 100;
      return `${x},${y}`;
    })
    .join(" ");

  const areaCoordinates = `${coordinates} 100,100 0,100`;
  const stroke = tone === "emerald" ? "#10b981" : "#06b6d4";
  return (
    <div className="mt-4 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
      <svg viewBox="0 0 100 100" className="h-36 w-full overflow-visible" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={`trend-fill-${tone}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.28" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <path d={`M ${areaCoordinates}`} fill={`url(#trend-fill-${tone})`} />
        <polyline fill="none" stroke={stroke} strokeWidth="3" strokeLinejoin="round" strokeLinecap="round" points={coordinates} />
        {preview.map((point, index) => {
          const pointKey = `${point.bucket_start}-${point.value}`;
          const x = (index / Math.max(preview.length - 1, 1)) * 100;
          const y = 100 - (point.value / maxValue) * 100;
          return <circle key={pointKey} cx={x} cy={y} r="2.5" fill={stroke} />;
        })}
      </svg>
      <div className="mt-3 grid grid-cols-3 gap-2 text-xs text-[var(--portal-muted)] sm:grid-cols-7">
        {preview.map((point) => (
          <div key={`${point.bucket_start}-${point.value}-label`} className="min-w-0 rounded-2xl bg-white/50 px-2 py-1 text-center dark:bg-slate-950/30">
            {formatShortDate(point.bucket_start)}
          </div>
        ))}
      </div>
    </div>
  );
}

function DashboardPageContent() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const t = useTranslations("dashboard");
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboard, setDashboard] = useState<DashboardHomeResponse | null>(null);
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<ClientTemplateId>("opencode");
  const [selectedFormat, setSelectedFormat] = useState<TemplateFormat>("json");
  const [userKey, setUserKey] = useState("");
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");
  const [selectedPackageTierCode, setSelectedPackageTierCode] = useState("");
  const [selectedPackageSummaryId, setSelectedPackageSummaryId] = useState<number | null>(null);
  const [packageActionLoading, setPackageActionLoading] = useState(false);
  const [redeemCode, setRedeemCode] = useState("");
  const [prepaidActionLoading, setPrepaidActionLoading] = useState(false);
  const [purchaseMessage, setPurchaseMessage] = useState<{ tone: PurchaseMessageTone; text: string } | null>(null);
  const [ticketTitle, setTicketTitle] = useState("");
  const [ticketCategory, setTicketCategory] = useState("delivery_issue");
  const [ticketMessage, setTicketMessage] = useState("");
  const [ticketSubmitting, setTicketSubmitting] = useState(false);
  const [ticketSubmitMessage, setTicketSubmitMessage] = useState<{ tone: TicketMessageTone; text: string } | null>(null);
  const [tokenTrend, setTokenTrend] = useState<TokenTrendResponse | null>(null);
  const [modelShare, setModelShare] = useState<{ start_date: string; end_date: string; items: ModelShareDatum[] } | null>(null);
  const [metricSummary, setMetricSummary] = useState<DashboardMetricSummary | null>(null);
  const modalRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const configTriggerRef = useRef<HTMLButtonElement | null>(null);
  const hadConfigModalOpenRef = useRef(false);

  const selectedTrendRange = useMemo<TrendRange>(() => {
    const requestedRange = searchParams.get("range");
    return requestedRange && isTrendRange(requestedRange) ? requestedRange : "7d";
  }, [searchParams]);

  const selectedTrendGranularity = useMemo<TrendGranularity>(() => {
    const requestedGranularity = searchParams.get("granularity");
    return requestedGranularity && isTrendGranularity(requestedGranularity) ? requestedGranularity : "day";
  }, [searchParams]);

  const selectedTemplateDefinition = useMemo(
    () => TEMPLATE_DEFINITIONS.find((template) => template.id === selectedTemplate) ?? TEMPLATE_DEFINITIONS[0],
    [selectedTemplate],
  );

  const renderedConfig = useMemo(() => {
    return buildTemplateContent(selectedTemplate, selectedFormat, userKey.trim());
  }, [selectedFormat, selectedTemplate, userKey]);

  const appliedTrendGranularity = useMemo(
    () => normalizeTrendGranularity(selectedTrendRange, selectedTrendGranularity),
    [selectedTrendGranularity, selectedTrendRange],
  );

  const updateTrendSearchParams = useCallback(
    (range: TrendRange, granularity: TrendGranularity, historyMode: "push" | "replace") => {
      const nextParams = new URLSearchParams(searchParams.toString());
      nextParams.set("range", range);
      nextParams.set("granularity", normalizeTrendGranularity(range, granularity));

      const nextQuery = nextParams.toString();
      const nextHref = nextQuery ? `${pathname}?${nextQuery}` : pathname;

      if (historyMode === "replace") {
        router.replace(nextHref);
        return;
      }

      router.push(nextHref);
    },
    [pathname, router, searchParams],
  );

  const trendDateRange = useMemo(
    () => buildTrendDateRange(selectedTrendRange, DASHBOARD_TREND_TIMEZONE),
    [selectedTrendRange],
  );

  const tokenTrendQueryString = useMemo(() => {
    const params = new URLSearchParams({
      start_date: trendDateRange.start_date,
      end_date: trendDateRange.end_date,
      granularity: appliedTrendGranularity,
      timezone: DASHBOARD_TREND_TIMEZONE,
    });

    return params.toString();
  }, [appliedTrendGranularity, trendDateRange.end_date, trendDateRange.start_date]);

  const closeConfigModal = useCallback(() => {
    setIsConfigModalOpen(false);
    setCopyState("idle");
  }, []);

  useEffect(() => {
    setIsHydrated(true);
    const storedSessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    const storedUserKey = localStorage.getItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY) ?? "";
    setSessionToken(storedSessionToken);
    setUserKey(storedUserKey);
  }, []);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    localStorage.setItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY, userKey);
  }, [isHydrated, userKey]);

  useEffect(() => {
    const currentRange = searchParams.get("range");
    const currentGranularity = searchParams.get("granularity");

    if (currentRange === selectedTrendRange && currentGranularity === appliedTrendGranularity) {
      return;
    }

    updateTrendSearchParams(selectedTrendRange, appliedTrendGranularity, "replace");
  }, [appliedTrendGranularity, searchParams, selectedTrendRange, updateTrendSearchParams]);

  const loadDashboard = useCallback(async (signal?: AbortSignal) => {
    if (!sessionToken) {
      setDashboard(null);
      setTokenTrend(null);
      setModelShare(null);
      setMetricSummary(null);
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const commonRequestInit: RequestInit = {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
        signal,
      };

      const [homeResponse, subscriptionResponse, accountResponse, trendResponse, modelsResponse, groupsResponse] = await Promise.all([
        fetch("/api/dashboard/home", commonRequestInit),
        fetch("/api/subscriptions/summary", commonRequestInit),
        fetch("/api/dashboard/account", commonRequestInit),
        fetch(`/api/dashboard/trend?${tokenTrendQueryString}`, commonRequestInit),
        fetch(`/api/dashboard/models?${tokenTrendQueryString}`, commonRequestInit),
        fetch("/api/groups/available", commonRequestInit),
      ]);

      const homePayload = (await homeResponse.json()) as unknown;
      if (!homeResponse.ok) {
        const errorPayload = asRecord(homePayload);
        throw new Error(asString(errorPayload?.error, "Failed to load dashboard home"));
      }

      const trendPayload = (await trendResponse.json()) as unknown;
      if (!trendResponse.ok) {
        const errorPayload = asRecord(trendPayload);
        throw new Error(asString(errorPayload?.error, "Failed to load dashboard token trend"));
      }

      const modelsPayload = (await modelsResponse.json()) as unknown;
      if (!modelsResponse.ok) {
        const errorPayload = asRecord(modelsPayload);
        throw new Error(asString(errorPayload?.error, "Failed to load dashboard model share"));
      }

      let subscriptionPayload: unknown = null;
      if (subscriptionResponse.ok) {
        subscriptionPayload = (await subscriptionResponse.json()) as unknown;
      }

      let accountPayload: unknown = null;
      if (accountResponse.ok) {
        accountPayload = (await accountResponse.json()) as unknown;
      }

      let groupsPayload: unknown = null;
      if (groupsResponse.ok) {
        groupsPayload = (await groupsResponse.json()) as unknown;
      }

      setDashboard(parseDashboardHomePayload(homePayload, subscriptionPayload, accountPayload, groupsPayload));
      setTokenTrend(parseTokenTrendResponse(trendPayload));
      setModelShare(normalizeModelShareData(modelsPayload));
      setMetricSummary(parseDashboardMetricSummary(homePayload, accountPayload));
    } catch (fetchError) {
      if ((fetchError as Error).name === "AbortError") {
        return;
      }
      setDashboard(null);
      setTokenTrend(null);
      setModelShare(null);
      setMetricSummary(null);
      setError(fetchError instanceof Error ? fetchError.message : "Failed to load dashboard home");
    } finally {
      if (!signal?.aborted) {
        setLoading(false);
      }
    }
  }, [sessionToken, tokenTrendQueryString]);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (!sessionToken) {
      setDashboard(null);
      setTokenTrend(null);
      setModelShare(null);
      setMetricSummary(null);
      setLoading(false);
      return;
    }

    const controller = new AbortController();

    void loadDashboard(controller.signal);

    return () => controller.abort();
  }, [isHydrated, loadDashboard, sessionToken]);

  useEffect(() => {
    const tiers = dashboard?.purchase_options.package_purchase.tiers ?? [];
    if (tiers.length === 0) {
      if (selectedPackageTierCode) {
        setSelectedPackageTierCode("");
      }
      return;
    }

    const hasSelectedTier = tiers.some((tier) => tier.code === selectedPackageTierCode);
    if (!hasSelectedTier) {
      setSelectedPackageTierCode(tiers[0].code);
    }
  }, [dashboard, selectedPackageTierCode]);

  useEffect(() => {
    const summaries = dashboard?.package_summaries ?? [];
    if (summaries.length === 0) {
      if (selectedPackageSummaryId !== null) {
        setSelectedPackageSummaryId(null);
      }
      return;
    }

    const hasSelectedSummary = summaries.some((summary) => summary.subscription_id === selectedPackageSummaryId);
    if (hasSelectedSummary) {
      return;
    }

    const preferredSummary = summaries.find((summary) => summary.status === "active") ?? summaries[0];
    setSelectedPackageSummaryId(preferredSummary?.subscription_id ?? null);
  }, [dashboard, selectedPackageSummaryId]);

  useEffect(() => {
    const checkoutState = searchParams.get("checkout");
    if (!checkoutState) {
      return;
    }

    const nextParams = new URLSearchParams(searchParams.toString());
    nextParams.delete("checkout");
    nextParams.delete("session_id");
    const nextQuery = nextParams.toString();
    const nextHref = nextQuery ? `${pathname}?${nextQuery}` : pathname;

    if (checkoutState === "success") {
      setPurchaseMessage({
        tone: "success",
        text: t("stripeSuccess"),
      });
      if (sessionToken) {
        void loadDashboard();
      }
    } else if (checkoutState === "cancelled") {
      setPurchaseMessage({
        tone: "error",
        text: t("stripeCancelled"),
      });
    }

    router.replace(nextHref);
  }, [loadDashboard, pathname, router, searchParams, sessionToken]);

  useEffect(() => {
    const nextFormat = selectedTemplateDefinition.supportedFormats.includes(selectedFormat)
      ? selectedFormat
      : selectedTemplateDefinition.supportedFormats[0];

    if (nextFormat !== selectedFormat) {
      setSelectedFormat(nextFormat);
    }
  }, [selectedFormat, selectedTemplateDefinition]);

  useEffect(() => {
    if (!isConfigModalOpen) {
      if (hadConfigModalOpenRef.current) {
        configTriggerRef.current?.focus();
        hadConfigModalOpenRef.current = false;
      }
      return;
    }

    hadConfigModalOpenRef.current = true;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeConfigModal();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const modal = modalRef.current;
      if (!modal) {
        return;
      }

      const focusable = modal.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) {
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey && activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [closeConfigModal, isConfigModalOpen]);

  const handleCopyConfig = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(renderedConfig);
      setCopyState("copied");
    } catch {
      setCopyState("error");
    }
  }, [renderedConfig]);

  const handlePackagePurchase = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      router.push(`/login?next=${encodeURIComponent("/dashboard")}`);
      return;
    }

    const selectedTier = dashboard?.purchase_options.package_purchase.tiers.find((tier) => tier.code === selectedPackageTierCode);
    if (!selectedTier) {
      setPurchaseMessage({ tone: "error", text: "Choose one package tier before starting checkout." });
      return;
    }

    setPackageActionLoading(true);

    try {
      const response = await fetch("/api/checkout/package", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          tier_code: selectedTier.code,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Package checkout is unavailable right now."));
      }

      const checkout = unwrapData<{ checkout_url?: string }>(payload) ?? (asRecord(payload) as { checkout_url?: string } | null);
      const checkoutURL = asString(checkout?.checkout_url);
      if (!checkoutURL) {
        throw new Error(extractApiError(payload, "Stripe checkout session was created without a redirect URL."));
      }

      window.location.assign(checkoutURL);
      return;

    } catch (packageError) {
      setPurchaseMessage({
        tone: "error",
        text:
          packageError instanceof Error
            ? `Package checkout could not be started: ${packageError.message} You can still review plans on /services or retry later.`
            : "Package checkout could not be started. You can still review plans on /services or retry later.",
      });
    } finally {
      setPackageActionLoading(false);
    }
  }, [dashboard, router, selectedPackageTierCode, sessionToken]);

  const handlePrepaidTopUp = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      setPurchaseMessage({ tone: "error", text: "Your session token is missing. Sign in again before redeeming prepaid credit." });
      return;
    }

    const normalizedCode = redeemCode.trim();
    const redeemEndpoint = dashboard?.purchase_options.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";

    if (!normalizedCode) {
      setPurchaseMessage({ tone: "error", text: "Enter a redeem code before submitting a prepaid top-up attempt." });
      return;
    }

    setPrepaidActionLoading(true);

    try {
      const response = await fetch(redeemEndpoint, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ code: normalizedCode }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Prepaid top-up is unavailable right now."));
      }

      setRedeemCode("");
      setPurchaseMessage({
        tone: "success",
        text: t("prepaidSuccess"),
      });
      await loadDashboard();
    } catch (redeemError) {
      setPurchaseMessage({
        tone: "error",
        text:
          redeemError instanceof Error
            ? `Prepaid top-up could not be completed: ${redeemError.message} No balance was changed locally.`
            : "Prepaid top-up could not be completed. No balance was changed locally.",
      });
    } finally {
      setPrepaidActionLoading(false);
    }
  }, [dashboard, loadDashboard, redeemCode, sessionToken]);

  const handleTicketSubmit = useCallback(async () => {
    setTicketSubmitMessage(null);

    if (!sessionToken) {
      setTicketSubmitMessage({ tone: "error", text: "Your session token is missing. Sign in again before creating a feedback ticket." });
      return;
    }

    const normalizedTitle = ticketTitle.trim();
    const normalizedMessage = ticketMessage.trim();

    if (!normalizedTitle || !ticketCategory.trim() || !normalizedMessage) {
      setTicketSubmitMessage({ tone: "error", text: "Title, category, and message are required before submitting your feedback ticket." });
      return;
    }

    setTicketSubmitting(true);

    try {
      const response = await fetch("/api/dashboard/tickets", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          title: normalizedTitle,
          category: ticketCategory,
          message: normalizedMessage,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Ticket submission is unavailable right now."));
      }

      const ticketEnvelope = unwrapData<{ ticket_id?: string }>(payload);
      const ticketRoot = asRecord(payload);
      const legacyTicketResult = asRecord(ticketRoot?.result);
      const ticketId =
        asString(ticketEnvelope?.ticket_id) ||
        asString(ticketRoot?.ticket_id) ||
        asString(legacyTicketResult?.ticket_id);

      setTicketTitle("");
      setTicketCategory("delivery_issue");
      setTicketMessage("");
      setTicketSubmitMessage({
        tone: "success",
        text: `Feedback ticket submitted successfully${ticketId ? ` (ID: ${ticketId})` : ""}.`,
      });
    } catch (submitError) {
      setTicketSubmitMessage({
        tone: "error",
        text: submitError instanceof Error ? `Ticket submission failed: ${submitError.message}` : "Ticket submission failed.",
      });
    } finally {
      setTicketSubmitting(false);
    }
  }, [sessionToken, ticketCategory, ticketMessage, ticketTitle]);

  const packageSummary = dashboard?.package_summary;
  const packageSummaries = dashboard?.package_summaries ?? [];
  const visiblePackageSummary =
    packageSummaries.find((summary) => summary.subscription_id === selectedPackageSummaryId) ?? packageSummary;
  const purchaseOptions = dashboard?.purchase_options;
  const tokenPoints = tokenTrend?.series.points ?? [];
  const modelShareItems = modelShare?.items ?? [];
  const quotaPreview = visiblePackageSummary?.quotas ?? [];
  const packageTiers = purchaseOptions?.package_purchase.tiers ?? [];
  const redeemEndpoint = purchaseOptions?.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";
  const appliedTrendRangeLabel = TREND_RANGE_OPTIONS.find((option) => option.value === selectedTrendRange)?.label ?? selectedTrendRange;
  const appliedTrendGranularityLabel =
    t(TREND_GRANULARITY_OPTIONS.find((option) => option.value === appliedTrendGranularity)?.labelKey ?? "dayLabel");
  const purchaseMessageClassName =
    purchaseMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : purchaseMessage?.tone === "success"
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";
  const ticketMessageClassName =
    ticketSubmitMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : ticketSubmitMessage?.tone === "success"
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";

  if (!isHydrated || loading) {
    return (
      <section className="portal-shell py-8">
        <div className="clay-panel p-5">
          <p className="text-sm text-[var(--portal-muted)]">{t("loading")}</p>
        </div>
      </section>
    );
  }

  if (!sessionToken) {
    return (
      <section className="portal-shell space-y-6 py-8">
        <div className="clay-panel space-y-2 p-5">
          <h1 className="section-title">
            <span className="gradient-text">{t("title")}</span>
          </h1>
          <p className="section-subtitle">{t("signInPrompt")}</p>
        </div>

        <div className="block-card space-y-4">
          <p className="notice">{t("tokenMissing")}</p>
          <div className="flex flex-wrap gap-3">
            <Link href="/login" className="btn-primary inline-flex items-center justify-center no-underline">
              {t("goToLogin")}
            </Link>
            <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
              {t("viewPackages")}
            </Link>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-6 py-8">
      <div className="portal-header clay-panel p-5">
        <div className="min-w-0 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">{t("headerLabel")}</p>
          <h1 className="section-title">
            <span className="gradient-text">{t("headerTitle")}</span>
          </h1>
          <p className="section-subtitle max-w-2xl">
            {t("headerDescription")}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className="btn-ghost" onClick={() => window.location.reload()}>
            {t("refresh")}
          </button>
          <button
            type="button"
            className="btn-primary"
            onClick={() => {
              localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
              setSessionToken("");
              router.replace("/login");
            }}
          >
            {t("signOut")}
          </button>
        </div>
      </div>

      {error ? <p className="notice">{t("errorPrefix")}{error}</p> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(320px,1fr)]">
        <div className="grid min-w-0 gap-6">
          <div className="grid gap-6 xl:grid-cols-2">
            <article className="block-card min-w-0">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-cyan-500 dark:text-cyan-400">{t("tokenTrend")}</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("consumptionCurve")}</h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">
                    {t("tokenTrendDescription")}
                  </p>
                </div>
                <div className="rounded-full border border-cyan-500/20 bg-cyan-500/10 px-3 py-1 text-xs font-semibold text-cyan-600 dark:text-cyan-300">
                  {tokenPoints.length > 0 ? t("points", { count: tokenPoints.length }) : t("preview")}
                </div>
              </div>

              <div className="mt-4 grid gap-3 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                <div>
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("range")}</p>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {TREND_RANGE_OPTIONS.map((option) => {
                      const isSelected = option.value === selectedTrendRange;
                      return (
                        <button
                          key={option.value}
                          type="button"
                          className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-semibold transition-all duration-200 ${
                            isSelected
                              ? "border-cyan-500/40 bg-cyan-500/10 text-cyan-700 dark:text-cyan-200"
                              : "border-[var(--portal-line)] bg-white/60 text-[var(--portal-ink)] dark:bg-slate-950/30"
                          }`}
                          onClick={() => updateTrendSearchParams(option.value, appliedTrendGranularity, "push")}
                          aria-pressed={isSelected}
                        >
                          {option.label}
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div>
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("granularity")}</p>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {TREND_GRANULARITY_OPTIONS.map((option) => {
                      const isAllowed = ALLOWED_TREND_GRANULARITY[selectedTrendRange].includes(option.value);
                      const isSelected = option.value === appliedTrendGranularity;
                      return (
                        <button
                          key={option.value}
                          type="button"
                          className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-semibold transition-all duration-200 ${
                            isSelected
                              ? "border-cyan-500/40 bg-cyan-500/10 text-cyan-700 dark:text-cyan-200"
                              : isAllowed
                                ? "border-[var(--portal-line)] bg-white/60 text-[var(--portal-ink)] dark:bg-slate-950/30"
                                : "border-[var(--portal-line)] bg-transparent text-[var(--portal-muted)]"
                          }`}
                           onClick={() => updateTrendSearchParams(selectedTrendRange, option.value, "push")}
                          aria-pressed={isSelected}
                          aria-describedby={!isAllowed ? "dashboard-trend-granularity-note" : undefined}
                        >
                          {t(option.labelKey)}
                        </button>
                      );
                    })}
                  </div>
                  <p id="dashboard-trend-granularity-note" className="mt-2 text-xs text-[var(--portal-muted)]">
                    {t("granularityNote")}
                  </p>
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-[var(--portal-muted)]">
                  <span>
                    {t("appliedLabel")} {appliedTrendRangeLabel} · {appliedTrendGranularityLabel}
                  </span>
                  <span>
                    {tokenTrend?.start_date || trendDateRange.start_date} → {tokenTrend?.end_date || trendDateRange.end_date}
                  </span>
                </div>
              </div>

              <TrendPreview points={tokenPoints} tone="cyan" />
            </article>

            <article className="block-card min-w-0">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("modelShare")}</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("tokenDistribution")}</h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">
                    {t("modelShareDescription")}
                  </p>
                </div>
                <div className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
                  {modelShareItems.length > 0 ? t("models", { count: modelShareItems.length }) : t("empty")}
                </div>
              </div>
              <ModelSharePieChart items={modelShareItems} startDate={modelShare?.start_date ?? ""} endDate={modelShare?.end_date ?? ""} />
            </article>
          </div>

          <div className="grid gap-6 md:grid-cols-2">
            <article className="block-card min-w-0 space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("package")}</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">
                    {visiblePackageSummary?.tier_name ?? t("noPackageYet")}
                  </h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">
                    {visiblePackageSummary?.status === "active"
                      ? (visiblePackageSummary.expires_at
                          ? t("subscriptionExpires", { id: visiblePackageSummary.subscription_id ?? "--", date: formatShortDate(visiblePackageSummary.expires_at!) })
                          : t("subscriptionActive", { id: visiblePackageSummary.subscription_id ?? "--" }))
                      : t("noPackageDescription")}
                  </p>
                </div>
                <span className="rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] px-3 py-1 text-xs font-semibold text-[var(--portal-muted)]">
                  {{unconfigured: t("statusUnconfigured"), active: t("statusActive"), expired: t("statusExpired"), cancelled: t("statusCancelled"), suspended: t("statusSuspended"), pending: t("statusPending")}[visiblePackageSummary?.status ?? "unconfigured"] ?? visiblePackageSummary?.status ?? "unconfigured"}
                </span>
              </div>

              {packageSummaries.length > 1 ? (
                <div className="flex flex-wrap gap-2">
                  {packageSummaries.map((summary, index) => {
                    const isSelected = summary.subscription_id === visiblePackageSummary?.subscription_id;
                    return (
                      <button
                        key={summary.subscription_id ?? `${summary.tier_name ?? "subscription"}-${index}`}
                        type="button"
                        onClick={() => setSelectedPackageSummaryId(summary.subscription_id)}
                        className={`rounded-full border px-3 py-1 text-xs font-semibold transition-all duration-200 ${
                          isSelected
                            ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200"
                            : "border-[var(--portal-line)] bg-white/60 text-[var(--portal-ink)] dark:bg-slate-950/30"
                        }`}
                        aria-pressed={isSelected}
                      >
                        {summary.tier_name ?? t("subscriptionN", { index: index + 1 })}
                      </button>
                    );
                  })}
                </div>
              ) : null}

              {quotaPreview.length === 0 ? (
                <p className="rounded-[1rem] border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
                  {t("noSubscriptionLoaded")}
                </p>
              ) : (
                <ul className="grid gap-3">
                  {quotaPreview.map((quota) => (
                    <li key={quota.period} className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-[var(--portal-ink)]">{t("usage", { label: {daily: t("daily"), weekly: t("weekly"), monthly: t("monthly")}[quota.period] ?? quota.period })}</p>
                          <p className="mt-1 text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{quota.period}</p>
                        </div>
                        <p className="text-sm font-semibold text-[var(--portal-ink)]">
                          {formatUsagePercentage(quota.percentage)}
                        </p>
                      </div>
                      <p className="mt-2 text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                        {formatMetricCurrency(quota.used_usd)} / {formatMetricCurrency(quota.limit_usd)}
                      </p>
                    </li>
                  ))}
                </ul>
              )}
            </article>

            <article className="block-card min-w-0 space-y-4">
              <div>
                <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("metrics")}</p>
                <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">
                  {t("accountIndicators")}
                </h2>
                <p className="mt-2 text-sm text-[var(--portal-muted)]">
                  {t("metricsDescription")}
                </p>
              </div>

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-1">
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("balance")}</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{formatMetricCurrency(metricSummary?.balance ?? null)}</p>
                </div>
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("todayRequests")}</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{formatMetricNumber(metricSummary?.today_requests ?? null)}</p>
                </div>
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("todaySpend")}</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{formatMetricCurrency(metricSummary?.today_spend ?? null)}</p>
                </div>
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("todayToken")}</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{formatMetricNumber(metricSummary?.today_token ?? null)}</p>
                </div>
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("cumulativeToken")}</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{formatMetricNumber(metricSummary?.cumulative_token ?? null)}</p>
                </div>
              </div>
            </article>
          </div>
        </div>

        <div className="grid min-w-0 gap-6">
          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("configApiKey")}</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("clientSetupEntry")}</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                {t("configDescription")}
              </p>
            </div>
            <div className="rounded-[1rem] border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
              {t("configHint")}
            </div>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                className="btn-primary"
                ref={configTriggerRef}
                onClick={() => {
                  setIsConfigModalOpen(true);
                  setError(null);
                  setCopyState("idle");
                }}
              >
                {t("openConfigSetup")}
              </button>
              <Link href="/account" className="btn-ghost inline-flex items-center justify-center no-underline">
                {t("manageSessionKeys")}
              </Link>
            </div>
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("purchase")}</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("topUpOrExtend")}</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                {t("purchaseDescription")}
              </p>
            </div>

            <div className="grid gap-3">
              <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("packagePurchase")}</p>
                    <p className="mt-2 text-sm text-[var(--portal-muted)]">
                      {t("packagePurchaseDescription")}
                    </p>
                  </div>
                  <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
                    {t("comparePackages")}
                  </Link>
                </div>

                <div className="mt-4 grid gap-3">
                  <div>
                    <label htmlFor="dashboard-package-tier" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                      {t("packageTier")}
                    </label>
                    <select
                      id="dashboard-package-tier"
                      className="field mt-2"
                      value={selectedPackageTierCode}
                      onChange={(event) => setSelectedPackageTierCode(event.target.value)}
                      disabled={packageTiers.length === 0 || packageActionLoading}
                    >
                      {packageTiers.length === 0 ? <option value="">{t("noPublicTiers")}</option> : null}
                      {packageTiers.map((tier) => (
                        <option key={tier.code} value={tier.code}>
                          {tier.name} ({tier.code})
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button
                      type="button"
                      className="btn-primary w-fit"
                      onClick={() => void handlePackagePurchase()}
                      disabled={packageActionLoading}
                    >
                      {packageActionLoading ? t("redirectingToStripe") : t("checkoutWithStripe")}
                    </button>
                  </div>
                </div>
              </div>

              <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("prepaidTopUp")}</p>
                <p className="mt-2 text-sm text-[var(--portal-ink)]">
                  {t("redeemEndpoint")} <span className="font-mono">{redeemEndpoint}</span>
                </p>
                <p className="mt-2 text-sm text-[var(--portal-muted)]">
                  {t("currencyHint", { currency: purchaseOptions?.prepaid_topup.currency_hint ?? "CNY" })}
                </p>

                <div className="mt-4 grid gap-3">
                  <div>
                    <label htmlFor="dashboard-redeem-code" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                      {t("redeemCode")}
                    </label>
                    <input
                      id="dashboard-redeem-code"
                      className="field mt-2 font-mono"
                      type="text"
                      placeholder="CARD-XXXX-XXXX"
                      value={redeemCode}
                      onChange={(event) => setRedeemCode(event.target.value)}
                      disabled={prepaidActionLoading}
                    />
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button type="button" className="btn-primary w-fit" onClick={() => void handlePrepaidTopUp()} disabled={prepaidActionLoading}>
                      {prepaidActionLoading ? t("submittingTopUp") : t("redeemPrepaidCode")}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {purchaseMessage ? <p className={`text-sm ${purchaseMessageClassName}`}>{purchaseMessage.text}</p> : null}
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("ticketFeedback")}</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("supportEntry")}</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("ticketDescription")}</p>
            </div>

            <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
              <div className="grid gap-3">
                <div>
                  <label htmlFor="dashboard-ticket-title" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    {t("titleLabel")}
                  </label>
                  <input
                    id="dashboard-ticket-title"
                    className="field mt-2"
                    type="text"
                    maxLength={120}
                    placeholder={t("titlePlaceholder")}
                    value={ticketTitle}
                    onChange={(event) => {
                      setTicketTitle(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  />
                </div>

                <div>
                  <label htmlFor="dashboard-ticket-category" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    {t("category")}
                  </label>
                  <select
                    id="dashboard-ticket-category"
                    className="field mt-2"
                    value={ticketCategory}
                    onChange={(event) => {
                      setTicketCategory(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  >
                    <option value="delivery_issue">{t("deliveryIssue")}</option>
                    <option value="model_feedback">{t("modelFeedback")}</option>
                    <option value="billing_question">{t("billingQuestion")}</option>
                    <option value="other">{t("other")}</option>
                  </select>
                </div>

                <div>
                  <label htmlFor="dashboard-ticket-message" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    {t("messageLabel")}
                  </label>
                  <textarea
                    id="dashboard-ticket-message"
                    className="field mt-2 min-h-[108px] resize-y"
                    placeholder={t("messagePlaceholder")}
                    value={ticketMessage}
                    onChange={(event) => {
                      setTicketMessage(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  />
                </div>
              </div>
            </div>

            <div className="flex flex-wrap gap-3">
              <button type="button" className="btn-primary w-fit" onClick={() => void handleTicketSubmit()} disabled={ticketSubmitting}>
                {ticketSubmitting ? t("submittingTicket") : t("createFeedbackTicket")}
              </button>
            </div>

            {ticketSubmitMessage ? <p className={`text-sm ${ticketMessageClassName}`}>{ticketSubmitMessage.text}</p> : null}
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("details")}</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("openDeeperRecords")}</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("detailsDescription")}</p>
            </div>
            <Link href="/dashboard/details" className="btn-primary inline-flex w-fit items-center justify-center no-underline">
              {t("goToDetailsPage")}
            </Link>
          </article>
        </div>
      </div>

      {isConfigModalOpen ? (
        <section
          className="fixed inset-0 z-[70] flex items-center justify-center p-4 sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-labelledby="dashboard-config-modal-title"
        >
          <button
            type="button"
            className="absolute inset-0 bg-slate-950/60 backdrop-blur-sm"
            aria-label={t("closeConfigModal")}
            onClick={closeConfigModal}
          />

          <div
            ref={modalRef}
            className="relative z-[1] flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-[1.4rem] border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] shadow-[var(--portal-shadow)]"
          >
            <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] px-5 py-4 sm:px-6">
              <div className="min-w-0 space-y-2">
                <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">{t("configModal")}</p>
                <h2 id="dashboard-config-modal-title" className="text-2xl font-bold text-[var(--portal-ink)]">
                  {t("singleKeyFourTemplates")}
                </h2>
                <p className="max-w-2xl text-sm text-[var(--portal-muted)]">
                  {t("configModalDescription")}
                </p>
              </div>
              <button
                type="button"
                ref={closeButtonRef}
                className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] text-xl font-semibold text-[var(--portal-ink)] transition-transform duration-200 hover:-translate-y-[1px]"
                aria-label={t("closeConfigModal")}
                onClick={closeConfigModal}
              >
                ×
              </button>
            </div>

            <div className="grid min-h-0 gap-0 overflow-y-auto lg:grid-cols-[280px_minmax(0,1fr)]">
              <div className="border-b border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 lg:border-b-0 lg:border-r">
                <div className="space-y-4">
                  <div className="space-y-2">
                    <label htmlFor="dashboard-user-key" className="text-sm font-semibold text-[var(--portal-ink)]">
                      {t("underlyingUserKey")}
                    </label>
                    <textarea
                      id="dashboard-user-key"
                      className="field min-h-[112px] resize-y font-mono text-sm"
                      placeholder={t("pasteExistingKey")}
                      value={userKey}
                      onChange={(event) => {
                        setUserKey(event.target.value);
                        setCopyState("idle");
                      }}
                    />
                    <p className="text-xs leading-5 text-[var(--portal-muted)]">
                      {t("keySourceDescription")}
                    </p>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button type="button" className="btn-ghost" onClick={() => setUserKey("")}>
                      {t("clearKey")}
                    </button>
                  </div>

                  <div className="rounded-[1rem] border border-amber-400/40 bg-amber-50/80 p-4 text-sm text-amber-900 dark:bg-amber-500/10 dark:text-amber-200">
                    {t("sensitiveKeyWarning")}
                  </div>

                  <div className="space-y-2">
                    <p className="text-sm font-semibold text-[var(--portal-ink)]">{t("template")}</p>
                    <div className="grid gap-2">
                      {TEMPLATE_DEFINITIONS.map((template) => {
                        const isActive = template.id === selectedTemplate;
                        return (
                          <button
                            key={template.id}
                            type="button"
                            className={`rounded-[1rem] border px-4 py-3 text-left transition-all duration-200 ${
                              isActive
                                ? "border-emerald-500/40 bg-emerald-500/10 shadow-[0_12px_24px_rgba(16,185,129,0.12)]"
                                : "border-[var(--portal-line)] bg-[var(--portal-clay-strong)] hover:-translate-y-[1px]"
                            }`}
                            onClick={() => {
                              setSelectedTemplate(template.id);
                              setCopyState("idle");
                            }}
                          >
                            <p className="text-sm font-semibold text-[var(--portal-ink)]">{t(template.labelKey)}</p>
                            <p className="mt-1 text-xs leading-5 text-[var(--portal-muted)]">{t(template.helperKey)}</p>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </div>
              </div>

              <div className="flex min-h-0 flex-col p-5 sm:p-6">
                <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] pb-4">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t(selectedTemplateDefinition.labelKey)}</p>
                    <h3 className="mt-1 text-xl font-bold text-[var(--portal-ink)]">{t("renderedClientConfig")}</h3>
                    <p className="mt-2 max-w-2xl text-sm text-[var(--portal-muted)]">{t(selectedTemplateDefinition.helperKey)}</p>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {selectedTemplateDefinition.supportedFormats.map((format) => (
                      <button
                        key={format}
                        type="button"
                        className={`rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] transition-colors ${
                          selectedFormat === format
                            ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                            : "border-[var(--portal-line)] bg-[var(--portal-clay)] text-[var(--portal-muted)]"
                        }`}
                        onClick={() => {
                          setSelectedFormat(format);
                          setCopyState("idle");
                        }}
                      >
                        {format}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_220px]">
                  <div className="min-w-0 rounded-[1.15rem] border border-[var(--portal-line)] bg-slate-950 p-4 shadow-inner shadow-black/20">
                    <pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-sm leading-6 text-emerald-100">
                      <code>{renderedConfig}</code>
                    </pre>
                  </div>

                  <div className="grid gap-3 self-start">
                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("gatewayBaseUrl")}</p>
                      <p className="mt-2 break-all text-sm font-semibold text-[var(--portal-ink)]">https://api.aliang.one</p>
                    </div>

                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("copy")}</p>
                      <button type="button" className="btn-primary mt-3 w-full" onClick={() => void handleCopyConfig()} disabled={!userKey.trim()}>
                        {t("copyRenderedConfig")}
                      </button>
                      <p className="mt-3 text-xs leading-5 text-[var(--portal-muted)]">
                        {copyState === "copied"
                          ? t("copyCopied")
                          : copyState === "error"
                            ? t("copyError")
                            : t("copyIdle")}
                      </p>
                    </div>

                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4 text-sm text-[var(--portal-muted)]">
                      {userKey.trim()
                        ? t("templateContentLive")
                        : t("addKeyFirst")}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      ) : null}
    </section>
  );
}

function DashboardPageFallback() {
  const t = useTranslations("dashboard");
  return (
    <section className="portal-shell py-8">
      <div className="clay-panel p-5">
        <p className="text-sm text-[var(--portal-muted)]">{t("loading")}</p>
      </div>
    </section>
  );
}

export default function DashboardPage() {
  return (
    <Suspense fallback={<DashboardPageFallback />}>
      <DashboardPageContent />
    </Suspense>
  );
}

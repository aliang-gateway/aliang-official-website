// Dashboard 客户端展示领域类型。
// 从 app/dashboard/page.tsx 提取,保持与原始定义逐字一致。

export type TrendRange = "7d" | "30d" | "90d";
export type TrendGranularity = "day" | "week" | "month";

export type ClientTemplateId = "opencode" | "claude" | "codex";
export type TemplateFormat = "json" | "yaml" | "shell";

export type TrendPoint = {
  bucket_start: string;
  value: number;
};

export type TrendSeries = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  interval: TrendGranularity;
  points: TrendPoint[];
};

export type TokenTrendResponse = {
  series: TrendSeries;
  start_date: string;
  end_date: string;
  granularity: TrendGranularity;
};

export type PackageQuota = {
  period: "daily" | "weekly" | "monthly";
  label: string;
  used_usd: number | null;
  limit_usd: number | null;
  percentage: number | null;
};

export type PackageSummary = {
  status: string;
  tier_code: string | null;
  tier_name: string | null;
  subscription_id: number | null;
  expires_at: string | null;
  quotas: PackageQuota[];
};

export type BalanceSummary = {
  balance_micros: number;
  currency: string;
  updated_at: string | null;
};

export type PurchaseOptions = {
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

export type DashboardHomeResponse = {
  request_trend: TrendSeries;
  token_trend: TrendSeries;
  package_summary: PackageSummary;
  package_summaries: PackageSummary[];
  balance_summary: BalanceSummary;
  purchase_options: PurchaseOptions;
};

export type DashboardMetricSummary = {
  balance: number | null;
  today_requests: number | null;
  today_spend: number | null;
  today_token: number | null;
  cumulative_token: number | null;
};

export type ModelShareDatum = {
  model: string;
  value: number;
  share: number;
  stroke: string;
};

export type UnknownRecord = Record<string, unknown>;

// Dashboard 趋势/格式化/几何纯工具函数与常量。
// 从 app/dashboard/page.tsx 提取,保持行为逐字一致。

import type { TrendGranularity, TrendRange } from "./dashboard-types";

export const TREND_RANGE_OPTIONS: Array<{ value: TrendRange; label: string }> = [
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" },
];

export const TREND_GRANULARITY_OPTIONS: Array<{ value: TrendGranularity; labelKey: string }> = [
  { value: "day", labelKey: "dayLabel" },
  { value: "week", labelKey: "weekLabel" },
  { value: "month", labelKey: "monthLabel" },
];

export const ALLOWED_TREND_GRANULARITY: Record<TrendRange, TrendGranularity[]> = {
  "7d": ["day"],
  "30d": ["day", "week"],
  "90d": ["day", "week", "month"],
};

export function isTrendGranularity(value: string): value is TrendGranularity {
  return value === "day" || value === "week" || value === "month";
}

export function isTrendRange(value: string): value is TrendRange {
  return value === "7d" || value === "30d" || value === "90d";
}

export function normalizeTrendGranularity(range: TrendRange, granularity: TrendGranularity) {
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

export function getTrendRangeDays(range: TrendRange) {
  if (range === "30d") {
    return 30;
  }
  if (range === "90d") {
    return 90;
  }
  return 7;
}

export function formatDateParts(date: Date, timeZone: string) {
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

export function buildTrendDateRange(range: TrendRange, timeZone: string) {
  const endDate = new Date();
  const startDate = new Date(endDate.getTime() - (getTrendRangeDays(range) - 1) * 24 * 60 * 60 * 1000);

  return {
    start_date: formatDateParts(startDate, timeZone),
    end_date: formatDateParts(endDate, timeZone),
  };
}

export function formatShortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(date);
}

export function formatMetricNumber(value: number | null, options?: Intl.NumberFormatOptions) {
  if (value === null) {
    return "--";
  }

  return new Intl.NumberFormat("en-US", options).format(value);
}

export function formatMetricCurrency(value: number | null) {
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

export function formatUsagePercentage(value: number | null) {
  if (value === null) {
    return "--";
  }

  return `${value.toFixed(1)}%`;
}

export function describePercentage(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

export function polarToCartesian(centerX: number, centerY: number, radius: number, angleInDegrees: number) {
  const angleInRadians = ((angleInDegrees - 90) * Math.PI) / 180;
  return {
    x: centerX + radius * Math.cos(angleInRadians),
    y: centerY + radius * Math.sin(angleInRadians),
  };
}

export function buildArcPath(centerX: number, centerY: number, radius: number, startAngle: number, endAngle: number) {
  const start = polarToCartesian(centerX, centerY, radius, endAngle);
  const end = polarToCartesian(centerX, centerY, radius, startAngle);
  const largeArcFlag = endAngle - startAngle > 180 ? 1 : 0;

  return `M ${centerX} ${centerY} L ${start.x} ${start.y} A ${radius} ${radius} 0 ${largeArcFlag} 0 ${end.x} ${end.y} Z`;
}

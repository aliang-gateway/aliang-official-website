"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";

import {
  ALLOWED_TREND_GRANULARITY,
  TREND_GRANULARITY_OPTIONS,
  TREND_RANGE_OPTIONS,
  formatShortDate,
} from "@/lib/dashboard-format";
import type { TokenTrendResponse, TrendGranularity, TrendPoint, TrendRange } from "@/lib/dashboard-types";

function buildPreviewPoints(points: TrendPoint[], fallbackStep: number) {
  if (points.length > 0) {
    return points;
  }

  return Array.from({ length: 7 }, (_, index) => ({
    bucket_start: new Date(Date.now() - (6 - index) * 24 * 60 * 60 * 1000).toISOString(),
    value: fallbackStep * (index + 1),
  }));
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

type TokenTrendCardProps = {
  selectedRange: TrendRange;
  appliedGranularity: TrendGranularity;
  trendDateRange: { start_date: string; end_date: string };
  tokenTrend: TokenTrendResponse | null;
  updateSearchParams: (range: TrendRange, granularity: TrendGranularity, historyMode: "push" | "replace") => void;
};

export function TokenTrendCard({
  selectedRange,
  appliedGranularity,
  trendDateRange,
  tokenTrend,
  updateSearchParams,
}: TokenTrendCardProps) {
  const t = useTranslations("dashboard");
  const tokenPoints = tokenTrend?.series.points ?? [];
  const appliedTrendRangeLabel = TREND_RANGE_OPTIONS.find((option) => option.value === selectedRange)?.label ?? selectedRange;
  const appliedTrendGranularityLabel =
    t(TREND_GRANULARITY_OPTIONS.find((option) => option.value === appliedGranularity)?.labelKey ?? "dayLabel");

  return (
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
              const isSelected = option.value === selectedRange;
              return (
                <button
                  key={option.value}
                  type="button"
                  className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-semibold transition-all duration-200 ${
                    isSelected
                      ? "border-cyan-500/40 bg-cyan-500/10 text-cyan-700 dark:text-cyan-200"
                      : "border-[var(--portal-line)] bg-white/60 text-[var(--portal-ink)] dark:bg-slate-950/30"
                  }`}
                  onClick={() => updateSearchParams(option.value, appliedGranularity, "push")}
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
              const isAllowed = ALLOWED_TREND_GRANULARITY[selectedRange].includes(option.value);
              const isSelected = option.value === appliedGranularity;
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
                   onClick={() => updateSearchParams(selectedRange, option.value, "push")}
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
  );
}

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

// Smooth curve through points (Catmull-Rom → cubic Bézier) so the line reads as
// a real consumption curve instead of a jagged polyline of straight segments.
function buildSmoothPath(pts: { x: number; y: number }[]): string {
  if (pts.length === 0) return "";
  if (pts.length === 1) return `M ${pts[0].x} ${pts[0].y}`;
  const d = [`M ${pts[0].x.toFixed(2)} ${pts[0].y.toFixed(2)}`];
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] ?? pts[i];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[i + 2] ?? p2;
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    d.push(`C ${cp1x.toFixed(2)} ${cp1y.toFixed(2)} ${cp2x.toFixed(2)} ${cp2y.toFixed(2)} ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`);
  }
  return d.join(" ");
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
  // Show at most ~7 x-axis ticks so labels never collide, regardless of how
  // many buckets the range/granularity produces (7d → 7, 90d/day → 90, …).
  const labelStep = Math.max(1, Math.ceil(preview.length / 7));

  const coords = preview.map((point, index) => ({
    x: (index / Math.max(preview.length - 1, 1)) * 100,
    y: 100 - (point.value / maxValue) * 100,
  }));
  const linePath = buildSmoothPath(coords);
  const areaPath = coords.length > 0 ? `${linePath} L 100 100 L 0 100 Z` : "";
  const stroke = "#147a4f";
  return (
    <div className="mt-4 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
      <svg viewBox="0 0 100 100" className="h-40 w-full overflow-visible" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={`trend-fill-${tone}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.22" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0.02" />
          </linearGradient>
        </defs>
        {areaPath ? <path d={areaPath} fill={`url(#trend-fill-${tone})`} /> : null}
        {linePath ? (
          <path
            d={linePath}
            fill="none"
            stroke={stroke}
            strokeWidth="1.6"
            vectorEffect="non-scaling-stroke"
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ) : null}
      </svg>
      <div className="relative mt-3 h-5 w-full text-xs text-[var(--ink-muted)]">
        {preview.map((point, index) => {
          // Thin to labelStep, but always keep the first and last bucket so the
          // axis spans the full selected range edge-to-edge.
          if (index % labelStep !== 0 && index !== preview.length - 1) {
            return null;
          }
          const x = (index / Math.max(preview.length - 1, 1)) * 100;
          return (
            <span
              key={`${point.bucket_start}-${point.value}-label`}
              className="absolute top-0 whitespace-nowrap"
              style={{
                left: `${x}%`,
                transform:
                  index === 0
                    ? "translateX(0)"
                    : index === preview.length - 1
                      ? "translateX(-100%)"
                      : "translateX(-50%)",
              }}
            >
              {formatShortDate(point.bucket_start)}
            </span>
          );
        })}
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
          <p className="text-sm font-semibold text-[var(--accent)]">{t("tokenTrend")}</p>
          <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("consumptionCurve")}</h2>
          <p className="mt-2 text-sm text-[var(--portal-muted)]">
            {t("tokenTrendDescription")}
          </p>
        </div>
        <div className="rounded-full border border-[var(--accent)]/30 bg-[var(--accent-wash)] px-3 py-1 text-xs font-semibold text-[var(--accent-ink)]">
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
                      ? "border-[var(--accent)]/40 bg-[var(--accent-wash)] text-[var(--accent-ink)]"
                      : "border-[var(--line)] bg-[var(--bone)]/60 text-[var(--ink)]"
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
                      ? "border-[var(--accent)]/40 bg-[var(--accent-wash)] text-[var(--accent-ink)]"
                      : isAllowed
                        ? "border-[var(--line)] bg-[var(--bone)]/60 text-[var(--ink)]"
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

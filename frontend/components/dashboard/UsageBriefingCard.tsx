"use client";

import { useTranslations } from "next-intl";

import {
  TREND_RANGE_OPTIONS,
  describePercentage,
  formatMetricCurrency,
  formatMetricNumber,
  formatShortDate,
} from "@/lib/dashboard-format";
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
 *  范围内无数据时指标显示 "--" 并给空态文案,不整卡隐藏(简报是固定信息位)。
 *  总 tokens 口径:沿用上游 total_tokens 字段,与消耗曲线一致;拆分行仅作分项展示。 */
export function UsageBriefingCard({ usageSummary, modelShare, selectedRange }: UsageBriefingCardProps) {
  const t = useTranslations("dashboard");
  const rangeLabel = TREND_RANGE_OPTIONS.find((option) => option.value === selectedRange)?.label ?? selectedRange;
  const hasData = (usageSummary?.requests ?? 0) > 0;

  const headline = [
    { label: t("totalTokens"), value: usageSummary && hasData ? compact(usageSummary.total_tokens) : "--" },
    { label: t("briefingRequests"), value: usageSummary && hasData ? formatMetricNumber(usageSummary.requests) : "--" },
    { label: t("briefingSpend"), value: usageSummary && hasData ? formatMetricCurrency(usageSummary.actual_cost) : "--" },
  ];

  const breakdown = [
    { label: t("briefingInputTokens"), value: hasData ? (usageSummary?.input_tokens ?? null) : null },
    { label: t("briefingOutputTokens"), value: hasData ? (usageSummary?.output_tokens ?? null) : null },
    { label: t("briefingCacheRead"), value: hasData ? (usageSummary?.cache_read_tokens ?? null) : null },
    { label: t("briefingCacheCreation"), value: hasData ? (usageSummary?.cache_creation_tokens ?? null) : null },
  ];

  const topModel = modelShare?.items[0] ?? null;

  return (
    <article className="block-card min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-2xl font-bold text-[var(--portal-ink)]">{t("usageBriefing")}</h2>
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

      <ol className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
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
          {t("topModelLine", { model: topModel.model, share: describePercentage(topModel.share) })}
        </p>
      ) : null}

      {!hasData ? <p className="mt-3 text-sm text-[var(--portal-muted)]">{t("briefingEmpty")}</p> : null}
    </article>
  );
}

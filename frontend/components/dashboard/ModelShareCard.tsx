"use client";

import { useTranslations } from "next-intl";

import { buildArcPath, describePercentage, formatMetricNumber } from "@/lib/dashboard-format";
import type { ModelShareDatum } from "@/lib/dashboard-types";

type ModelShare = { start_date: string; end_date: string; items: ModelShareDatum[] };

type ModelShareCardProps = {
  modelShare: ModelShare | null;
};

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

export function ModelShareCard({ modelShare }: ModelShareCardProps) {
  const t = useTranslations("dashboard");
  const items = modelShare?.items ?? [];

  return (
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
          {items.length > 0 ? t("models", { count: items.length }) : t("empty")}
        </div>
      </div>
      <ModelSharePieChart items={items} startDate={modelShare?.start_date ?? ""} endDate={modelShare?.end_date ?? ""} />
    </article>
  );
}

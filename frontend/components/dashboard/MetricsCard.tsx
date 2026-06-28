"use client";

import { useTranslations } from "next-intl";

import { formatMetricCurrency, formatMetricNumber } from "@/lib/dashboard-format";
import type { DashboardMetricSummary } from "@/lib/dashboard-types";

type MetricsCardProps = {
  metricSummary: DashboardMetricSummary | null;
};

export function MetricsCard({ metricSummary }: MetricsCardProps) {
  const t = useTranslations("dashboard");

  return (
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
  );
}

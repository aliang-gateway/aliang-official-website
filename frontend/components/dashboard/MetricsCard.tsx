"use client";

import { useTranslations } from "next-intl";

import { formatMetricCurrency, formatMetricNumber } from "@/lib/dashboard-format";
import type { DashboardMetricSummary } from "@/lib/dashboard-types";

type MetricsCardProps = {
  metricSummary: DashboardMetricSummary | null;
};

/**
 * Full-width KPI strip — the five account numbers a user wants at a glance
 * (balance, today's requests/spend/tokens, cumulative tokens) laid out as a
 * single divided data row across the top of the dashboard, like a masthead
 * stat band. Replaces the old equal-weight card that buried these numbers in
 * the second row.
 */
export function MetricsCard({ metricSummary }: MetricsCardProps) {
  const t = useTranslations("dashboard");

  const cells = [
    { label: t("balance"), value: formatMetricCurrency(metricSummary?.balance ?? null) },
    { label: t("todayRequests"), value: formatMetricNumber(metricSummary?.today_requests ?? null) },
    { label: t("todaySpend"), value: formatMetricCurrency(metricSummary?.today_spend ?? null) },
    { label: t("todayToken"), value: formatMetricNumber(metricSummary?.today_token ?? null) },
    { label: t("cumulativeToken"), value: formatMetricNumber(metricSummary?.cumulative_token ?? null) },
  ] as const;

  return (
    <div className="clay-panel min-w-0 overflow-hidden">
      <ol className="grid grid-cols-2 divide-[color:var(--line)] divide-y sm:grid-cols-3 sm:divide-y-0 sm:divide-x lg:grid-cols-5">
        {cells.map((cell) => (
          <li key={cell.label} className="min-w-0 px-5 py-4">
            <p
              className="text-[10px] font-extrabold uppercase tracking-[0.18em] text-[var(--ink-muted)]"
              style={{ fontFamily: "var(--font-editorial-mono)" }}
            >
              {cell.label}
            </p>
            <p className="mt-1.5 truncate text-2xl font-extrabold tracking-tight text-[var(--ink)]">
              {cell.value}
            </p>
          </li>
        ))}
      </ol>
    </div>
  );
}

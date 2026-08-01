"use client";

import { useTranslations } from "next-intl";

import {
  formatMetricCurrency,
  formatMetricNumber,
  formatShortDate,
  formatUsagePercentage,
} from "@/lib/dashboard-format";
import type { DashboardHomeResponse, DashboardMetricSummary } from "@/lib/dashboard-types";

type StatusCardProps = {
  metricSummary: DashboardMetricSummary | null;
  dashboard: DashboardHomeResponse | null;
  onPurchase: () => void;
};

const STATUS_LABEL_KEYS: Record<string, string> = {
  unconfigured: "statusUnconfigured",
  active: "statusActive",
  expired: "statusExpired",
  cancelled: "statusCancelled",
  suspended: "statusSuspended",
  pending: "statusPending",
};

const MONO = { fontFamily: "var(--font-editorial-mono)" } as const;

/**
 * Dashboard hero: the one card a user reads on entry. Combines the old KPI
 * strip + package card into a single status surface — balance & today's burn on
 * the left, current plan + quota bar + the top-up CTA on the right.
 */
export function StatusCard({ metricSummary, dashboard, onPurchase }: StatusCardProps) {
  const t = useTranslations("dashboard");

  const summaries = dashboard?.package_summaries ?? [];
  const pkg =
    summaries.find((summary) => summary.status === "active") ??
    summaries[0] ??
    dashboard?.package_summary ??
    null;
  const quotas = pkg?.quotas ?? [];
  // Surface the quota closest to its limit — that's the one a user needs to see.
  const headlineQuota = quotas.slice().sort((a, b) => (b.percentage ?? 0) - (a.percentage ?? 0))[0] ?? null;
  const statusKey = STATUS_LABEL_KEYS[pkg?.status ?? "unconfigured"] ?? "statusUnconfigured";
  const periodLabel = (period: string) =>
    ({ daily: t("daily"), weekly: t("weekly"), monthly: t("monthly") }[period] ?? period);

  const todayStats = [
    { label: t("todayRequests"), value: formatMetricNumber(metricSummary?.today_requests ?? null) },
    { label: t("todaySpend"), value: formatMetricCurrency(metricSummary?.today_spend ?? null) },
    { label: t("todayToken"), value: formatMetricNumber(metricSummary?.today_token ?? null) },
  ];

  const quotaPct = headlineQuota
    ? Math.min(Math.max(headlineQuota.percentage ?? 0, 0), 100)
    : 0;

  return (
    <article className="clay-panel overflow-hidden">
      <div className="grid gap-0 md:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        {/* 余额 + 今日用量 */}
        <div className="space-y-6 p-6">
          <div>
            <p className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]" style={MONO}>
              {t("balance")}
            </p>
            <p className="mt-2 text-4xl font-extrabold tracking-tight text-[var(--ink)]">
              {formatMetricCurrency(metricSummary?.balance ?? null)}
            </p>
          </div>
          <ol className="grid grid-cols-3 gap-3">
            {todayStats.map((stat) => (
              <li key={stat.label} className="min-w-0">
                <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]" style={MONO}>
                  {stat.label}
                </p>
                <p className="mt-1 truncate font-semibold text-[var(--ink)]">{stat.value}</p>
              </li>
            ))}
          </ol>
        </div>

        {/* 套餐 + 额度 + 充值 */}
        <div className="space-y-4 border-t border-[var(--line)] bg-[var(--paper)]/50 p-6 md:border-l md:border-t-0">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]" style={MONO}>
                {t("package")}
              </p>
              <h2 className="mt-2 truncate text-xl font-extrabold text-[var(--ink)]">
                {pkg?.tier_name ?? t("noPackageYet")}
              </h2>
            </div>
            <span className="shrink-0 rounded-full border border-[var(--line)] bg-[var(--paper)] px-3 py-1 text-xs font-bold text-[var(--ink-muted)]">
              {t(statusKey)}
            </span>
          </div>

          <p className="text-sm text-[var(--ink-muted)]">
            {pkg?.status === "active"
              ? pkg.expires_at
                ? t("subscriptionExpires", { id: pkg.subscription_id ?? "--", date: formatShortDate(pkg.expires_at) })
                : t("subscriptionActive", { id: pkg.subscription_id ?? "--" })
              : t("noPackageDescription")}
          </p>

          {headlineQuota ? (
            <div>
              <div className="flex items-center justify-between text-xs text-[var(--ink-muted)]">
                <span>{t("usage", { label: periodLabel(headlineQuota.period) })}</span>
                <span className="tabular-nums">
                  {formatUsagePercentage(headlineQuota.percentage ?? 0)} · {formatMetricCurrency(headlineQuota.used_usd)} /{" "}
                  {formatMetricCurrency(headlineQuota.limit_usd)}
                </span>
              </div>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--line-soft)]">
                <div
                  className="h-full rounded-full bg-[var(--accent)] transition-[width] duration-500"
                  style={{ width: `${quotaPct}%` }}
                />
              </div>
            </div>
          ) : (
            <p className="rounded-[1rem] border border-dashed border-[var(--line)] p-3 text-sm text-[var(--ink-muted)]">
              {t("noSubscriptionLoaded")}
            </p>
          )}

          <button type="button" onClick={onPurchase} className="btn-primary w-full">
            {t("topUpOrExtend")}
          </button>
        </div>
      </div>
    </article>
  );
}

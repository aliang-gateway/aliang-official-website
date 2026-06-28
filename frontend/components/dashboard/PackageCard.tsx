"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import { formatMetricCurrency, formatShortDate, formatUsagePercentage } from "@/lib/dashboard-format";
import type { DashboardHomeResponse } from "@/lib/dashboard-types";

type PackageCardProps = {
  dashboard: DashboardHomeResponse | null;
};

export function PackageCard({ dashboard }: PackageCardProps) {
  const t = useTranslations("dashboard");
  const packageSummary = dashboard?.package_summary;
  const packageSummaries = dashboard?.package_summaries ?? [];
  const [selectedPackageSummaryId, setSelectedPackageSummaryId] = useState<number | null>(null);

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

  const visiblePackageSummary =
    packageSummaries.find((summary) => summary.subscription_id === selectedPackageSummaryId) ?? packageSummary;
  const quotaPreview = visiblePackageSummary?.quotas ?? [];

  return (
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
                  ? t("subscriptionExpires", { id: visiblePackageSummary.subscription_id ?? "--", date: formatShortDate(visiblePackageSummary.expires_at) })
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
  );
}

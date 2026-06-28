"use client";

import { useTranslations } from "next-intl";

type DashboardHeaderProps = {
  onRefresh: () => void;
  onSignOut: () => void;
};

export function DashboardHeader({ onRefresh, onSignOut }: DashboardHeaderProps) {
  const t = useTranslations("dashboard");

  return (
    <div className="portal-header clay-panel p-5">
      <div className="min-w-0 space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">{t("headerLabel")}</p>
        <h1 className="section-title">
          <span className="gradient-text">{t("headerTitle")}</span>
        </h1>
        <p className="section-subtitle max-w-2xl">
          {t("headerDescription")}
        </p>
      </div>
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn-ghost" onClick={onRefresh}>
          {t("refresh")}
        </button>
        <button type="button" className="btn-primary" onClick={onSignOut}>
          {t("signOut")}
        </button>
      </div>
    </div>
  );
}

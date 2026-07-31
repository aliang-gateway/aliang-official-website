"use client";

import { Suspense, useRef } from "react";
import { useTranslations } from "next-intl";

import {
  ConfigEntryCard,
  ConfigModal,
  DashboardHeader,
  DetailsLinkCard,
  MetricsCard,
  ModelShareCard,
  PackageCard,
  PurchaseCard,
  SectionLabel,
  TicketCard,
  TokenTrendCard,
} from "@/components/dashboard";
import { useConfigModal } from "@/lib/hooks/use-config-modal";
import { useDashboardData } from "@/lib/hooks/use-dashboard-data";
import { useTrendControls } from "@/lib/hooks/use-trend-controls";

function DashboardPageContent() {
  const t = useTranslations("dashboard");
  const configTriggerRef = useRef<HTMLButtonElement | null>(null);
  const trend = useTrendControls();
  const data = useDashboardData(trend.queryString);
  const config = useConfigModal();

  if (!data.isHydrated || !data.sessionToken || data.loading) {
    return (
      <section className="portal-shell py-8">
        <div className="clay-panel p-5">
          <p className="text-sm text-[var(--portal-muted)]">{t("loading")}</p>
        </div>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-10 py-10">
      <DashboardHeader onRefresh={() => data.reload()} onSignOut={data.signOut} />

      {data.error ? <p className="notice">{t("errorPrefix")}{data.error}</p> : null}

      {/* 账户概览:关键数字横条(余额 / 今日用量 / 累计) */}
      <div className="space-y-3">
        <SectionLabel kicker={t("sectionOverview")} />
        <MetricsCard metricSummary={data.metricSummary} />
        <p className="text-sm text-[var(--ink-muted)]">{t("metricsDescription")}</p>
      </div>

      {/* 用量趋势 + 模型分布(主列) / 下一步操作(侧列:充值/配置) */}
      <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(300px,340px)]">
        <div className="min-w-0 space-y-8">
          <div className="space-y-4">
            <SectionLabel kicker={t("sectionUsage")} />
            <TokenTrendCard
              selectedRange={trend.selectedRange}
              appliedGranularity={trend.appliedGranularity}
              trendDateRange={trend.trendDateRange}
              tokenTrend={data.tokenTrend}
              updateSearchParams={trend.updateSearchParams}
            />
          </div>
          <div className="space-y-4">
            <SectionLabel kicker={t("modelShare")} />
            <ModelShareCard modelShare={data.modelShare} />
          </div>
        </div>

        <div className="min-w-0 space-y-4">
          <SectionLabel kicker={t("sectionNext")} />
          <div className="space-y-4">
            <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} />
            <ConfigEntryCard onOpen={() => { config.open(); data.clearError(); }} triggerRef={configTriggerRef} />
          </div>
        </div>
      </div>

      {/* 套餐 */}
      <div className="space-y-3">
        <SectionLabel kicker={t("sectionPlan")} />
        <PackageCard dashboard={data.dashboard} />
      </div>

      {/* 更多(次要) */}
      <div className="space-y-3">
        <SectionLabel kicker={t("sectionMore")} />
        <div className="grid gap-6 md:grid-cols-2">
          <TicketCard sessionToken={data.sessionToken} />
          <DetailsLinkCard />
        </div>
      </div>


      <ConfigModal
        isOpen={config.isOpen}
        onClose={config.close}
        userKey={config.userKey}
        onUserKeyChange={config.setUserKey}
        template={config.template}
        onTemplateChange={config.setTemplate}
        format={config.format}
        onFormatChange={config.setFormat}
        templateDefinition={config.templateDefinition}
        renderedConfig={config.renderedConfig}
        copyState={config.copyState}
        onCopy={config.handleCopy}
        triggerRef={configTriggerRef}
        sessionToken={data.sessionToken}
      />
    </section>
  );
}

function DashboardPageFallback() {
  const t = useTranslations("dashboard");
  return (
    <section className="portal-shell py-8">
      <div className="clay-panel p-5">
        <p className="text-sm text-[var(--portal-muted)]">{t("loading")}</p>
      </div>
    </section>
  );
}

export default function DashboardPage() {
  return (
    <Suspense fallback={<DashboardPageFallback />}>
      <DashboardPageContent />
    </Suspense>
  );
}

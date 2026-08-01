"use client";

import { Suspense, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

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
  const [activeTab, setActiveTab] = useState<"usage" | "config" | "support">("usage");

  if (!data.isHydrated || !data.sessionToken || data.loading) {
    return (
      <section className="portal-shell py-8">
        <div className="clay-panel p-5">
          <p className="text-sm text-[var(--portal-muted)]">{t("loading")}</p>
        </div>
      </section>
    );
  }

  const tabs: { id: "usage" | "config" | "support"; label: string }[] = [
    { id: "usage", label: t("tabUsage") },
    { id: "config", label: t("tabConfig") },
    { id: "support", label: t("tabSupport") },
  ];

  return (
    <section className="portal-shell space-y-8 py-10">
      <DashboardHeader onRefresh={() => data.reload()} onSignOut={data.signOut} />

      {data.error ? <p className="notice">{t("errorPrefix")}{data.error}</p> : null}

      {/* Tab 栏 */}
      <div role="tablist" aria-label={t("dashboardTabs")} className="flex w-fit flex-wrap gap-1.5 rounded-full border border-[var(--line)] bg-[var(--paper)] p-1.5">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            role="tab"
            type="button"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              "rounded-full px-5 py-2 text-sm font-bold transition-colors",
              activeTab === tab.id
                ? "bg-[var(--ink)] text-[var(--paper)]"
                : "text-[var(--ink-muted)] hover:text-[var(--ink)]"
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* 用量:KPI + 趋势 + 套餐/分布 + 充值 */}
      {activeTab === "usage" ? (
        <div className="space-y-10">
          <div className="space-y-3">
            <SectionLabel kicker={t("sectionOverview")} />
            <MetricsCard metricSummary={data.metricSummary} />
            <p className="text-sm text-[var(--ink-muted)]">{t("metricsDescription")}</p>
          </div>

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

          <div className="space-y-3">
            <SectionLabel kicker={t("sectionPlan")} />
            <div className="grid gap-6 lg:grid-cols-2">
              <PackageCard dashboard={data.dashboard} />
              <ModelShareCard modelShare={data.modelShare} />
            </div>
          </div>

          <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} />
        </div>
      ) : null}

      {/* 配置:客户端配置入口 */}
      {activeTab === "config" ? (
        <ConfigEntryCard onOpen={() => { config.open(); data.clearError(); }} triggerRef={configTriggerRef} />
      ) : null}

      {/* 支持:工单反馈 + 深度记录 */}
      {activeTab === "support" ? (
        <div className="grid gap-6 md:grid-cols-2">
          <TicketCard sessionToken={data.sessionToken} />
          <DetailsLinkCard />
        </div>
      ) : null}

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

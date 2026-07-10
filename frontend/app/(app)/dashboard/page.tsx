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
    <section className="portal-shell space-y-6 py-8">
      <DashboardHeader onRefresh={() => window.location.reload()} onSignOut={data.signOut} />

      {data.error ? <p className="notice">{t("errorPrefix")}{data.error}</p> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(320px,1fr)]">
        <div className="grid min-w-0 gap-6">
          <div className="grid gap-6 xl:grid-cols-2">
            <TokenTrendCard
              selectedRange={trend.selectedRange}
              appliedGranularity={trend.appliedGranularity}
              trendDateRange={trend.trendDateRange}
              tokenTrend={data.tokenTrend}
              updateSearchParams={trend.updateSearchParams}
            />
            <ModelShareCard modelShare={data.modelShare} />
          </div>

          <div className="grid gap-6 md:grid-cols-2">
            <PackageCard dashboard={data.dashboard} />
            <MetricsCard metricSummary={data.metricSummary} />
          </div>
        </div>

        <div className="grid min-w-0 gap-6">
          <ConfigEntryCard onOpen={() => { config.open(); data.clearError(); }} triggerRef={configTriggerRef} />
          <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} />
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

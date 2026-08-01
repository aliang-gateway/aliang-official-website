"use client";

import { Suspense, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import {
  ConfigModal,
  DashboardHeader,
  ModelShareCard,
  PurchaseCard,
  StatusCard,
  TicketCard,
  TokenTrendCard,
} from "@/components/dashboard";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useConfigModal } from "@/lib/hooks/use-config-modal";
import { useDashboardData } from "@/lib/hooks/use-dashboard-data";
import { useTrendControls } from "@/lib/hooks/use-trend-controls";

function DashboardPageContent() {
  const t = useTranslations("dashboard");
  const configTriggerRef = useRef<HTMLButtonElement | null>(null);
  const trend = useTrendControls();
  const data = useDashboardData(trend.queryString);
  const config = useConfigModal();
  const [showPurchase, setShowPurchase] = useState(false);
  const [showTicket, setShowTicket] = useState(false);

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
    <section className="portal-shell space-y-8 py-10">
      <DashboardHeader onRefresh={() => data.reload()} onSignOut={data.signOut} />

      {data.error ? <p className="notice">{t("errorPrefix")}{data.error}</p> : null}

      {/* ★ 状态卡:余额 / 今日用量 / 套餐额度 / 充值入口 */}
      <StatusCard
        metricSummary={data.metricSummary}
        dashboard={data.dashboard}
        onPurchase={() => setShowPurchase((value) => !value)}
      />

      {/* 用量趋势(全宽) */}
      <TokenTrendCard
        selectedRange={trend.selectedRange}
        appliedGranularity={trend.appliedGranularity}
        trendDateRange={trend.trendDateRange}
        tokenTrend={data.tokenTrend}
        updateSearchParams={trend.updateSearchParams}
      />

      {/* 模型分布 */}
      <ModelShareCard modelShare={data.modelShare} />

      {/* 入口 tile:配置 / 深度记录 / 工单 */}
      <div className="grid gap-4 sm:grid-cols-3">
        <button
          ref={configTriggerRef}
          type="button"
          onClick={() => { config.open(); data.clearError(); }}
          className="clay-panel flex items-center gap-3 p-4 text-left transition-colors hover:bg-[var(--paper)]"
        >
          <MaterialIcon name="key" size={22} className="text-[var(--accent)]" />
          <span className="font-bold text-[var(--ink)]">{t("configApiKey")}</span>
          <MaterialIcon name="arrow_forward" size={18} className="ml-auto text-[var(--ink-faint)]" />
        </button>

        <Link
          href="/dashboard/details"
          className="clay-panel flex items-center gap-3 p-4 transition-colors hover:bg-[var(--paper)]"
        >
          <MaterialIcon name="insights" size={22} className="text-[var(--accent)]" />
          <span className="font-bold text-[var(--ink)]">{t("details")}</span>
          <MaterialIcon name="arrow_forward" size={18} className="ml-auto text-[var(--ink-faint)]" />
        </Link>

        <button
          type="button"
          onClick={() => setShowTicket((value) => !value)}
          aria-expanded={showTicket}
          className="clay-panel flex items-center gap-3 p-4 text-left transition-colors hover:bg-[var(--paper)]"
        >
          <MaterialIcon name="support_agent" size={22} className="text-[var(--accent)]" />
          <span className="font-bold text-[var(--ink)]">{t("ticketFeedback")}</span>
          <MaterialIcon name={showTicket ? "expand_less" : "expand_more"} size={18} className="ml-auto text-[var(--ink-faint)]" />
        </button>
      </div>

      {/* 展开面板(由状态卡 CTA / 工单 tile 触发) */}
      {showPurchase ? (
        <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} />
      ) : null}
      {showTicket ? <TicketCard sessionToken={data.sessionToken} /> : null}

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

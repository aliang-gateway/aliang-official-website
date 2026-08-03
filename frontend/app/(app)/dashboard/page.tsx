"use client";

import { Suspense, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import {
  DashboardHeader,
  ModelShareCard,
  PurchaseCard,
  StatusCard,
  TicketCard,
  TokenTrendCard,
} from "@/components/dashboard";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { Modal } from "@/components/ui/Modal";
import { useDashboardData } from "@/lib/hooks/use-dashboard-data";
import { useTrendControls } from "@/lib/hooks/use-trend-controls";

function DashboardPageContent() {
  const t = useTranslations("dashboard");
  const ticketTriggerRef = useRef<HTMLButtonElement | null>(null);
  const trend = useTrendControls();
  const data = useDashboardData(trend.queryString);
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

      {/* 入口 tile:密钥与配置(跳转页) / 工单(modal) / 详情(跳转,最右) */}
      <div className="grid gap-4 sm:grid-cols-3">
        <Link
          href="/keys"
          className="clay-panel flex items-center gap-3 p-4 transition-colors hover:bg-[var(--paper)]"
        >
          <MaterialIcon name="key" size={22} className="text-[var(--accent)]" />
          <span className="font-bold text-[var(--ink)]">{t("configApiKey")}</span>
          <MaterialIcon name="arrow_forward" size={18} className="ml-auto text-[var(--ink-faint)]" />
        </Link>

        <button
          ref={ticketTriggerRef}
          type="button"
          onClick={() => setShowTicket(true)}
          className="clay-panel flex items-center gap-3 p-4 text-left transition-colors hover:bg-[var(--paper)]"
        >
          <MaterialIcon name="support_agent" size={22} className="text-[var(--accent)]" />
          <span className="font-bold text-[var(--ink)]">{t("ticketFeedback")}</span>
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
      </div>

      {/* 充值面板(状态卡 CTA 触发) */}
      {showPurchase ? (
        <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} />
      ) : null}

      {/* 工单 modal */}
      <Modal
        isOpen={showTicket}
        onClose={() => setShowTicket(false)}
        closeLabel={t("closeConfigModal")}
        triggerRef={ticketTriggerRef}
      >
        <TicketCard sessionToken={data.sessionToken} bare />
      </Modal>
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

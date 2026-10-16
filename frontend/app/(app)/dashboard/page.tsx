"use client";

import { Suspense, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import {
  DashboardHeader,
  ModelShareCard,
  PurchaseCard,
  QuickActionsBar,
  StatusCard,
  TicketCard,
  TokenTrendCard,
  UsageBriefingCard,
  WalletTransactionsCard,
} from "@/components/dashboard";
import { CacheRateCard } from "@/components/dashboard/CacheRateCard";
import { Modal } from "@/components/ui/Modal";
import { useDashboardData } from "@/lib/hooks/use-dashboard-data";
import { useTrendControls } from "@/lib/hooks/use-trend-controls";

function DashboardPageContent() {
  const t = useTranslations("dashboard");
  const ticketTriggerRef = useRef<HTMLButtonElement | null>(null);
  const purchaseTriggerRef = useRef<HTMLButtonElement | null>(null);
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

      <QuickActionsBar onOpenTicket={() => setShowTicket(true)} ticketTriggerRef={ticketTriggerRef} />

      {data.error ? <p className="notice">{t("errorPrefix")}{data.error}</p> : null}

      {/* ★ 状态卡:余额 / 今日用量 / 套餐额度 / 充值入口 */}
      <StatusCard
        metricSummary={data.metricSummary}
        dashboard={data.dashboard}
        onPurchase={() => setShowPurchase(true)}
        purchaseButtonRef={purchaseTriggerRef}
      />

      {/* ★ 用量简报 + 余额变动(一行两列,窄屏堆叠) */}
      <div className="grid gap-4 lg:grid-cols-2">
        <UsageBriefingCard
          usageSummary={data.usageSummary}
          modelShare={data.modelShare}
          selectedRange={trend.selectedRange}
        />
        <WalletTransactionsCard sessionToken={data.sessionToken} />
      </div>

      {/* 缓存命中率(随所选时间范围聚合) */}
      <CacheRateCard sessionToken={data.sessionToken} queryString={trend.queryString} />

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

      {/* 充值 modal(状态卡 CTA 触发) */}
      <Modal
        isOpen={showPurchase}
        onClose={() => setShowPurchase(false)}
        closeLabel={t("closePurchaseModal")}
        triggerRef={purchaseTriggerRef}
        panelClassName="max-w-2xl"
      >
        <PurchaseCard sessionToken={data.sessionToken} dashboard={data.dashboard} onReload={data.loadDashboard} bare />
      </Modal>

      {/* 工单 modal */}
      <Modal
        isOpen={showTicket}
        onClose={() => setShowTicket(false)}
        closeLabel={t("closeTicketModal")}
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

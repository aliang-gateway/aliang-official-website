"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

import { usePurchaseActions } from "@/lib/hooks/use-purchase-actions";
import type { DashboardHomeResponse } from "@/lib/dashboard-types";

type PurchaseCardProps = {
  sessionToken: string;
  dashboard: DashboardHomeResponse | null;
  onReload: () => Promise<void>;
};

export function PurchaseCard({ sessionToken, dashboard, onReload }: PurchaseCardProps) {
  const t = useTranslations("dashboard");
  const {
    selectedTierCode,
    setSelectedTierCode,
    redeemCode,
    setRedeemCode,
    packageActionLoading,
    prepaidActionLoading,
    purchaseMessage,
    handlePackagePurchase,
    handlePrepaidTopUp,
  } = usePurchaseActions({ sessionToken, dashboard, reload: onReload });

  const purchaseOptions = dashboard?.purchase_options;
  const packageTiers = purchaseOptions?.package_purchase.tiers ?? [];
  const redeemEndpoint = purchaseOptions?.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";
  const purchaseMessageClassName =
    purchaseMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : purchaseMessage?.tone === "success"
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";

  return (
    <article className="block-card min-w-0 space-y-4">
      <div>
        <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("purchase")}</p>
        <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("topUpOrExtend")}</h2>
        <p className="mt-2 text-sm text-[var(--portal-muted)]">
          {t("purchaseDescription")}
        </p>
      </div>

      <div className="grid gap-3">
        <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("packagePurchase")}</p>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                {t("packagePurchaseDescription")}
              </p>
            </div>
            <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
              {t("comparePackages")}
            </Link>
          </div>

          <div className="mt-4 grid gap-3">
            <div>
              <label htmlFor="dashboard-package-tier" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                {t("packageTier")}
              </label>
              <select
                id="dashboard-package-tier"
                className="field mt-2"
                value={selectedTierCode}
                onChange={(event) => setSelectedTierCode(event.target.value)}
                disabled={packageTiers.length === 0 || packageActionLoading}
              >
                {packageTiers.length === 0 ? <option value="">{t("noPublicTiers")}</option> : null}
                {packageTiers.map((tier) => (
                  <option key={tier.code} value={tier.code}>
                    {tier.name} ({tier.code})
                  </option>
                ))}
              </select>
            </div>

            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                className="btn-primary w-fit"
                onClick={() => void handlePackagePurchase()}
                disabled={packageActionLoading}
              >
                {packageActionLoading ? t("redirectingToStripe") : t("checkoutWithStripe")}
              </button>
            </div>
          </div>
        </div>

        <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
          <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("prepaidTopUp")}</p>
          <p className="mt-2 text-sm text-[var(--portal-ink)]">
            {t("redeemEndpoint")} <span className="font-mono">{redeemEndpoint}</span>
          </p>
          <p className="mt-2 text-sm text-[var(--portal-muted)]">
            {t("currencyHint", { currency: purchaseOptions?.prepaid_topup.currency_hint ?? "CNY" })}
          </p>

          <div className="mt-4 grid gap-3">
            <div>
              <label htmlFor="dashboard-redeem-code" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                {t("redeemCode")}
              </label>
              <input
                id="dashboard-redeem-code"
                className="field mt-2 font-mono"
                type="text"
                placeholder="CARD-XXXX-XXXX"
                value={redeemCode}
                onChange={(event) => setRedeemCode(event.target.value)}
                disabled={prepaidActionLoading}
              />
            </div>

            <div className="flex flex-wrap gap-3">
              <button type="button" className="btn-primary w-fit" onClick={() => void handlePrepaidTopUp()} disabled={prepaidActionLoading}>
                {prepaidActionLoading ? t("submittingTopUp") : t("redeemPrepaidCode")}
              </button>
            </div>
          </div>
        </div>
      </div>

      {purchaseMessage ? <p className={`text-sm ${purchaseMessageClassName}`}>{purchaseMessage.text}</p> : null}
    </article>
  );
}

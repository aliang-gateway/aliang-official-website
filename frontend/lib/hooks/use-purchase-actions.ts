import { useCallback, useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";
import type { DashboardHomeResponse } from "@/lib/dashboard-types";

export type PurchaseMessageTone = "success" | "error" | "info";

export type PurchaseActions = {
  selectedTierCode: string;
  setSelectedTierCode: (value: string) => void;
  redeemCode: string;
  setRedeemCode: (value: string) => void;
  packageActionLoading: boolean;
  prepaidActionLoading: boolean;
  purchaseMessage: { tone: PurchaseMessageTone; text: string } | null;
  handlePackagePurchase: () => Promise<void>;
  handlePrepaidTopUp: () => Promise<void>;
};

type UsePurchaseActionsArgs = {
  sessionToken: string;
  dashboard: DashboardHomeResponse | null;
  reload: () => Promise<void>;
};

export function usePurchaseActions({ sessionToken, dashboard, reload }: UsePurchaseActionsArgs): PurchaseActions {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const t = useTranslations("dashboard");

  const [selectedTierCode, setSelectedTierCode] = useState("");
  const [redeemCode, setRedeemCode] = useState("");
  const [packageActionLoading, setPackageActionLoading] = useState(false);
  const [prepaidActionLoading, setPrepaidActionLoading] = useState(false);
  const [purchaseMessage, setPurchaseMessage] = useState<{ tone: PurchaseMessageTone; text: string } | null>(null);

  useEffect(() => {
    const tiers = dashboard?.purchase_options.package_purchase.tiers ?? [];
    if (tiers.length === 0) {
      if (selectedTierCode) {
        setSelectedTierCode("");
      }
      return;
    }

    const hasSelectedTier = tiers.some((tier) => tier.code === selectedTierCode);
    if (!hasSelectedTier) {
      setSelectedTierCode(tiers[0].code);
    }
  }, [dashboard, selectedTierCode]);

  useEffect(() => {
    const checkoutState = searchParams.get("checkout");
    if (!checkoutState) {
      return;
    }

    const nextParams = new URLSearchParams(searchParams.toString());
    nextParams.delete("checkout");
    nextParams.delete("session_id");
    const nextQuery = nextParams.toString();
    const nextHref = nextQuery ? `${pathname}?${nextQuery}` : pathname;

    if (checkoutState === "success") {
      setPurchaseMessage({
        tone: "success",
        text: t("stripeSuccess"),
      });
      if (sessionToken) {
        void reload();
      }
    } else if (checkoutState === "cancelled") {
      setPurchaseMessage({
        tone: "error",
        text: t("stripeCancelled"),
      });
    }

    router.replace(nextHref);
  }, [pathname, reload, router, searchParams, sessionToken, t]);

  const handlePackagePurchase = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      router.push(`/login?next=${encodeURIComponent("/dashboard")}`);
      return;
    }

    const selectedTier = dashboard?.purchase_options.package_purchase.tiers.find((tier) => tier.code === selectedTierCode);
    if (!selectedTier) {
      setPurchaseMessage({ tone: "error", text: "Choose one package tier before starting checkout." });
      return;
    }

    setPackageActionLoading(true);

    try {
      const response = await fetch("/api/checkout/package", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          tier_code: selectedTier.code,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Package checkout is unavailable right now."));
      }

      const checkout = unwrapData<{ checkout_url?: string }>(payload) ?? (asRecord(payload) as { checkout_url?: string } | null);
      const checkoutURL = asString(checkout?.checkout_url);
      if (!checkoutURL) {
        throw new Error(extractApiError(payload, "Stripe checkout session was created without a redirect URL."));
      }

      window.location.assign(checkoutURL);
      return;
    } catch (packageError) {
      setPurchaseMessage({
        tone: "error",
        text:
          packageError instanceof Error
            ? `Package checkout could not be started: ${packageError.message} You can still review plans on /services or retry later.`
            : "Package checkout could not be started. You can still review plans on /services or retry later.",
      });
    } finally {
      setPackageActionLoading(false);
    }
  }, [dashboard, router, selectedTierCode, sessionToken]);

  const handlePrepaidTopUp = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      setPurchaseMessage({ tone: "error", text: "Your session token is missing. Sign in again before redeeming prepaid credit." });
      return;
    }

    const normalizedCode = redeemCode.trim();
    const redeemEndpoint = dashboard?.purchase_options.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";

    if (!normalizedCode) {
      setPurchaseMessage({ tone: "error", text: "Enter a redeem code before submitting a prepaid top-up attempt." });
      return;
    }

    setPrepaidActionLoading(true);

    try {
      const response = await fetch(redeemEndpoint, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ code: normalizedCode }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Prepaid top-up is unavailable right now."));
      }

      setRedeemCode("");
      setPurchaseMessage({
        tone: "success",
        text: t("prepaidSuccess"),
      });
      await reload();
    } catch (redeemError) {
      setPurchaseMessage({
        tone: "error",
        text:
          redeemError instanceof Error
            ? `Prepaid top-up could not be completed: ${redeemError.message} No balance was changed locally.`
            : "Prepaid top-up could not be completed. No balance was changed locally.",
      });
    } finally {
      setPrepaidActionLoading(false);
    }
  }, [dashboard, redeemCode, reload, sessionToken, t]);

  return {
    selectedTierCode,
    setSelectedTierCode,
    redeemCode,
    setRedeemCode,
    packageActionLoading,
    prepaidActionLoading,
    purchaseMessage,
    handlePackagePurchase,
    handlePrepaidTopUp,
  };
}

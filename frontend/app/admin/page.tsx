"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord, extractApiError, unwrapData } from "@/lib/api-response";

type AuthMeResponse = {
  id: number;
  email: string;
  name: string;
  role: "user" | "admin";
  created_at: string;
  updated_at: string;
};

type UnitPrice = {
  service_item_code: string;
  tier_code?: string;
  price_per_unit_micros: number;
  currency: string;
  effective_from: string;
};

type UnitPricesResponse = {
  unit_prices: UnitPrice[];
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

function formatMoney(micros: number, currency: string) {
  return `${(micros / 1_000_000).toFixed(6)} ${currency}`;
}

export default function AdminPage() {
  const router = useRouter();
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [authError, setAuthError] = useState<string | null>(null);
  const [adminProfile, setAdminProfile] = useState<AuthMeResponse | null>(null);

  const [serviceItemCode, setServiceItemCode] = useState("chat_input_tokens");
  const [prices, setPrices] = useState<UnitPrice[]>([]);
  const [pricesLoading, setPricesLoading] = useState(false);
  const [pricesError, setPricesError] = useState<string | null>(null);

  useEffect(() => {
    const run = async () => {
      const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
      if (!sessionToken) {
        router.replace("/login");
        return;
      }

      try {
        const meResponse = await fetch("/api/auth/me", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
        });

        const mePayload = (await meResponse.json()) as unknown;
        if (!meResponse.ok) {
          throw new Error(extractApiError(mePayload, "failed to verify session"));
        }

        const profile = unwrapData<AuthMeResponse>(mePayload) ?? (asRecord(mePayload) as AuthMeResponse | null);
        if (!profile) {
          throw new Error("failed to verify session");
        }
        if (profile.role !== "admin") {
          router.replace("/account");
          return;
        }

        setAdminProfile(profile);
      } catch (error) {
        const message = error instanceof Error ? error.message : "failed to verify session";
        setAuthError(message);
        localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
        router.replace("/login");
        return;
      } finally {
        setIsCheckingAuth(false);
      }
    };

    void run();
  }, [router]);

  const handleLoadPrices = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPricesError(null);
    setPrices([]);

    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    if (!sessionToken) {
      setPricesError("session missing, please login again");
      return;
    }

    setPricesLoading(true);
    try {
      const response = await fetch(
        `/api/admin/unit-prices?service_item_code=${encodeURIComponent(serviceItemCode.trim())}`,
        {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
        },
      );

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "failed to load unit prices"));
      }

      const pricePayload = unwrapData<UnitPricesResponse>(payload) ?? (asRecord(payload) as UnitPricesResponse | null);
      setPrices(pricePayload?.unit_prices ?? []);
    } catch (error) {
      setPricesError(error instanceof Error ? error.message : "failed to load unit prices");
    } finally {
      setPricesLoading(false);
    }
  };

  if (isCheckingAuth) {
    return (
      <section className="portal-shell py-10">
        <p className="text-sm text-[var(--stitch-text-muted)]">Checking admin session...</p>
      </section>
    );
  }

  return (
    <section className="portal-shell py-8">
      <div className="rounded-xl border border-[var(--stitch-primary)]/10 bg-[var(--stitch-bg-elevated)] p-6 shadow-sm">
        <div className="mb-6 flex items-center gap-3">
          <span className="rounded-lg bg-[var(--stitch-primary)]/10 p-2 text-[var(--stitch-primary)]">
            <MaterialIcon name="admin_panel_settings" size={20} />
          </span>
          <div>
            <h1 className="text-xl font-bold">Admin Console</h1>
            <p className="text-sm text-[var(--stitch-text-muted)]">
              {adminProfile ? `${adminProfile.name} (${adminProfile.email})` : ""}
            </p>
          </div>
        </div>

        <div className="mb-6 flex flex-wrap gap-2">
          <Link href="/admin/packages" className="nav-pill">
            <MaterialIcon name="inventory_2" size={16} className="mr-1" />
            Packages
          </Link>
          <Link href="/admin/payments" className="nav-pill">
            <MaterialIcon name="receipt_long" size={16} className="mr-1" />
            Payments
          </Link>
          <Link href="/admin/articles" className="nav-pill">
            <MaterialIcon name="article" size={16} className="mr-1" />
            Articles
          </Link>
        </div>

        {authError ? <p className="mb-4 text-sm text-red-500">{authError}</p> : null}

        <form className="mb-6 flex flex-col gap-3 sm:flex-row" onSubmit={handleLoadPrices}>
          <input
            className="field"
            value={serviceItemCode}
            onChange={(event) => setServiceItemCode(event.target.value)}
            placeholder="service item code, e.g. chat_input_tokens"
            required
          />
          <button className="btn-primary w-fit" type="submit" disabled={pricesLoading}>
            {pricesLoading ? "Loading..." : "Load Unit Prices"}
          </button>
        </form>

        {pricesError ? <p className="mb-4 text-sm text-red-500">{pricesError}</p> : null}

        <div className="overflow-x-auto rounded-lg border border-[var(--stitch-border)]">
          <table className="w-full text-left">
            <thead>
              <tr className="bg-[var(--stitch-bg)]">
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">Service Item</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">Tier</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">Price</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">Effective From</th>
              </tr>
            </thead>
            <tbody>
              {prices.map((item) => (
                <tr key={`${item.service_item_code}-${item.tier_code ?? "global"}-${item.effective_from}`}>
                  <td className="px-4 py-3 text-sm">{item.service_item_code}</td>
                  <td className="px-4 py-3 text-sm">{item.tier_code ?? "global"}</td>
                  <td className="px-4 py-3 text-sm">{formatMoney(item.price_per_unit_micros, item.currency)}</td>
                  <td className="px-4 py-3 text-sm">{item.effective_from}</td>
                </tr>
              ))}
              {!pricesLoading && prices.length === 0 ? (
                <tr>
                  <td className="px-4 py-4 text-sm text-[var(--stitch-text-muted)]" colSpan={4}>
                    No data yet.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

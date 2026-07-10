"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";

type Package = {
  code: string;
  name: string;
  price_micros: number;
  value_type: string;
  value_amount: number;
  rate: number;
  min_topup_micros: number;
  max_topup_micros: number;
  description: string;
  features: string[];
  is_published: boolean;
};

type CheckoutResponse = { checkout_url?: string };

const SESSION_TOKEN_KEY = "session_token";

function formatMoneyMicros(value: number) {
  return `¥${((value || 0) / 1000000).toFixed(2)}`;
}

function yuanToMicros(yuan: string): number {
  const n = parseFloat(yuan);
  return !Number.isNaN(n) && n > 0 ? Math.round(n * 1_000_000) : 0;
}

export default function PricePage() {
  const t = useTranslations("editorial.price");
  const router = useRouter();
  const [pkgs, setPkgs] = useState<Package[] | null>(null);
  const [loadingTier, setLoadingTier] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [topupYuan, setTopupYuan] = useState<Record<string, string>>({});
  const [surcharge, setSurcharge] = useState<{
    enabled: boolean;
    amountMicros: number;
    thresholdMicros: number;
  } | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/packages", { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (cancelled) return;
        const list = (Array.isArray(data?.packages) ? (data.packages as Package[]) : []).filter(
          (p) => p.is_published,
        );
        setPkgs(list);
      })
      .catch(() => {
        if (!cancelled) setPkgs([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/public/payment-config", { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (cancelled || !data || typeof data.surcharge_enabled !== "boolean") return;
        setSurcharge({
          enabled: data.surcharge_enabled,
          amountMicros: Number(data.surcharge_amount_micros) || 0,
          thresholdMicros: Number(data.surcharge_threshold_micros) || 0,
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const handleCheckout = async (tierCode: string, amountMicros?: number) => {
    setError(null);
    const token = localStorage.getItem(SESSION_TOKEN_KEY);
    if (!token) {
      router.push(`/login?next=${encodeURIComponent("/price")}`);
      return;
    }
    setLoadingTier(tierCode);
    try {
      const res = await fetch("/api/checkout/package", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          tier_code: tierCode,
          ...(amountMicros ? { amount_micros: amountMicros } : {}),
        }),
      });
      const payload = (await res.json()) as unknown;
      if (!res.ok) {
        throw new Error(extractApiError(payload, "Checkout is unavailable right now."));
      }
      const checkout = unwrapData<CheckoutResponse>(payload) ?? (asRecord(payload) as CheckoutResponse | null);
      const checkoutURL = asString(checkout?.checkout_url);
      if (!checkoutURL) {
        throw new Error(extractApiError(payload, "Stripe checkout session was created without a redirect URL."));
      }
      window.location.assign(checkoutURL);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Checkout failed.");
      setLoadingTier(null);
    }
  };

  const loading = pkgs === null;
  const isTopup = (p: Package) => p.value_type === "balance" && p.rate > 0;
  const subscriptions = (pkgs ?? []).filter((p) => !isTopup(p));
  const topups = (pkgs ?? []).filter((p) => isTopup(p));

  return (
    <div className="page-price">
      <header className="hero" aria-labelledby="price-hero-title">
        <div className="container wide hero-grid">
          <div data-reveal>
            <div className="label">{t("heroLabel")}</div>
            <h1 className="display" id="price-hero-title">
              {t("heroTitle")}
              <span className="dot">.</span>
            </h1>
            <p className="lead">{t("heroLead")}</p>
          </div>
          <figure className="plate" data-reveal>
            <img src="/editorial/cta.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
        </div>
      </header>

      <main>
        <section className="pricing-section" aria-labelledby="price-plans-title">
          <div className="container">
            <h2 id="price-plans-title" className="sr-only">
              {t("heroTitle")}
            </h2>
            {surcharge?.enabled && surcharge.amountMicros > 0 && surcharge.thresholdMicros > 0 ? (
              <p className="surcharge-hint">
                {t("surchargeHint", {
                  amount: `¥${(surcharge.amountMicros / 1_000_000).toFixed(2)}`,
                  threshold: `¥${(surcharge.thresholdMicros / 1_000_000).toFixed(2)}`,
                })}
              </p>
            ) : null}
            {error ? (
              <p className="filter-feedback" role="alert">
                {error}
              </p>
            ) : null}
            {loading ? (
              <p className="filter-feedback">{t("loading")}</p>
            ) : pkgs.length === 0 ? (
              <p className="filter-feedback">{t("empty")}</p>
            ) : (
              <div className="pricing-groups">
                {subscriptions.length > 0 && (
                  <div className="pricing-group">
                    <h3 className="pricing-group-title">{t("groupSubscription")}</h3>
                    <div className="pricing-grid">
                      {subscriptions.map((p) => {
                        const unit =
                          p.value_type === "days"
                            ? t("unitDays")
                            : p.value_type === "tokens"
                              ? t("unitTokens")
                              : p.value_type;
                        return (
                          <article className="pricing-card" key={p.code}>
                            <div className="plan-name">{p.name}</div>
                            <div className="plan-price">
                              <b>{formatMoneyMicros(p.price_micros)}</b>
                              <span>
                                {" / "}
                                {p.value_amount} {unit}
                              </span>
                            </div>
                            {p.description && <p className="plan-desc">{p.description}</p>}
                            {p.features.length > 0 && (
                              <ul className="plan-features">
                                {p.features.map((f, i) => (
                                  <li key={i}>{f}</li>
                                ))}
                              </ul>
                            )}
                            <button
                              type="button"
                              className="btn"
                              onClick={() => void handleCheckout(p.code)}
                              disabled={loadingTier === p.code}
                            >
                              {loadingTier === p.code ? t("redirecting") : t("cta")}
                            </button>
                          </article>
                        );
                      })}
                    </div>
                  </div>
                )}

                {topups.length > 0 && (
                  <div className="pricing-group">
                    <h3 className="pricing-group-title">{t("groupTopup")}</h3>
                    <div className="pricing-grid">
                      {topups.map((p) => {
                        const range =
                          p.min_topup_micros > 0 || p.max_topup_micros > 0
                            ? `${formatMoneyMicros(p.min_topup_micros)} – ${formatMoneyMicros(p.max_topup_micros)}`
                            : "";
                        const micros = yuanToMicros(topupYuan[p.code] ?? "");
                        const withinBounds =
                          (p.min_topup_micros <= 0 || micros >= p.min_topup_micros) &&
                          (p.max_topup_micros <= 0 || micros <= p.max_topup_micros);
                        return (
                          <article className="pricing-card pricing-card-topup" key={p.code}>
                            <div className="plan-name">{p.name}</div>
                            <div className="plan-rate">
                              ¥1 = <b>${p.rate.toFixed(2)}</b>
                            </div>
                            {range && (
                              <div className="plan-range">
                                {t("range")}: {range}
                              </div>
                            )}
                            {p.description && <p className="plan-desc">{p.description}</p>}
                            {p.features.length > 0 && (
                              <ul className="plan-features">
                                {p.features.map((f, i) => (
                                  <li key={i}>{f}</li>
                                ))}
                              </ul>
                            )}
                            <input
                              className="field plan-topup-input"
                              type="number"
                              min={p.min_topup_micros ? p.min_topup_micros / 1_000_000 : undefined}
                              max={p.max_topup_micros ? p.max_topup_micros / 1_000_000 : undefined}
                              step="0.01"
                              placeholder="80"
                              value={topupYuan[p.code] ?? ""}
                              onChange={(e) => setTopupYuan((m) => ({ ...m, [p.code]: e.target.value }))}
                              disabled={loadingTier === p.code}
                            />
                            {micros > 0 ? (() => {
                              const feeMicros =
                                surcharge?.enabled &&
                                surcharge.thresholdMicros > 0 &&
                                micros < surcharge.thresholdMicros &&
                                surcharge.amountMicros > 0
                                  ? surcharge.amountMicros
                                  : 0;
                              const billMicros = micros + feeMicros;
                              return (
                                <div className="plan-credit">
                                  <p>
                                    {t("billLine", { bill: formatMoneyMicros(billMicros) })}
                                    {feeMicros > 0 ? (
                                      <span className="plan-credit-fee"> {t("inclFee", { fee: formatMoneyMicros(feeMicros) })}</span>
                                    ) : null}
                                  </p>
                                  <p>
                                    {t("creditHint", {
                                      paid: formatMoneyMicros(micros),
                                      credit: ((micros / 1_000_000) * p.rate).toFixed(2),
                                    })}
                                  </p>
                                </div>
                              );
                            })() : null}
                            <button
                              type="button"
                              className="btn"
                              onClick={() => void handleCheckout(p.code, micros)}
                              disabled={loadingTier === p.code || micros <= 0 || !withinBounds}
                            >
                              {loadingTier === p.code ? t("redirecting") : t("ctaTopup")}
                            </button>
                          </article>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </section>
      </main>

      <section className="closing" aria-labelledby="price-closing-title">
        <div className="container closing-grid">
          <div>
            <div className="label">{t("closingLabel")}</div>
            <h2 className="display" id="price-closing-title">
              {t("closingTitle")}
              <span className="dot">.</span>
            </h2>
            <p>{t("closingLead")}</p>
          </div>
          <Link className="btn" href="/download">
            {t("closingBtn")}
          </Link>
        </div>
      </section>
    </div>
  );
}

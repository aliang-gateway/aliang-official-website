"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";

type CheckoutStatusPayload = {
  status?: string;
  provider?: string;
  checkout_session_id?: string;
  payment_event_id?: string;
  tier_code?: string;
  package_name?: string;
  amount_minor?: number;
  currency?: string;
  fulfillment_job?: {
    id?: number;
    status?: string;
    error_message?: string | null;
  };
  error?: string;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

function formatAmount(amountMinor?: number, currency?: string) {
  if (!amountMinor || amountMinor <= 0) {
    return "";
  }
  return `${(amountMinor / 100).toFixed(2)} ${(currency ?? "cny").toUpperCase()}`;
}

function CheckoutStatusContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [sessionToken, setSessionToken] = useState("");
  const [payload, setPayload] = useState<CheckoutStatusPayload | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasTimedOut, setHasTimedOut] = useState(false);

  const checkoutState = searchParams.get("checkout")?.trim() ?? "";
  const sessionID = searchParams.get("session_id")?.trim() ?? "";

  useEffect(() => {
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    setSessionToken(token);
    if (!token) {
      const next = `/checkout/status${window.location.search}`;
      router.replace(`/login?next=${encodeURIComponent(next)}`);
    }
  }, [router]);

  useEffect(() => {
    if (!sessionToken || !sessionID || checkoutState !== "success") {
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let timeoutTimer: ReturnType<typeof setTimeout> | null = null;

    setHasTimedOut(false);
    timeoutTimer = setTimeout(() => {
      if (!cancelled) {
        setHasTimedOut(true);
        setIsLoading(false);
      }
    }, 25000);

    const poll = async () => {
      try {
        const response = await fetch(`/api/checkout/package/status?session_id=${encodeURIComponent(sessionID)}`, {
          method: "GET",
          headers: {
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
        });

        const nextPayload = (await response.json()) as CheckoutStatusPayload;
        if (!response.ok) {
          throw new Error(nextPayload.error ?? "Failed to confirm checkout status");
        }
        if (cancelled) {
          return;
        }

        setPayload(nextPayload);
        setError(null);
        const status = String(nextPayload.status ?? "").trim();
        if (status === "fulfilled" || status === "failed") {
          if (timeoutTimer) {
            clearTimeout(timeoutTimer);
          }
          setIsLoading(false);
          return;
        }

        setIsLoading(true);
        timer = setTimeout(() => void poll(), 2500);
      } catch (pollError) {
        if (cancelled) {
          return;
        }
        setError(pollError instanceof Error ? pollError.message : "Failed to confirm checkout status");
        setIsLoading(false);
      }
    };

    void poll();

    return () => {
      cancelled = true;
      if (timer) {
        clearTimeout(timer);
      }
      if (timeoutTimer) {
        clearTimeout(timeoutTimer);
      }
    };
  }, [checkoutState, sessionID, sessionToken]);

  const title = useMemo(() => {
    if (checkoutState === "cancelled") {
      return "Checkout cancelled";
    }
    const status = String(payload?.status ?? "").trim();
    if (status === "fulfilled") {
      return "Payment confirmed";
    }
    if (status === "failed") {
      return "Fulfillment failed";
    }
    return "Confirming your payment";
  }, [checkoutState, payload?.status]);

  const description = useMemo(() => {
    if (checkoutState === "cancelled") {
      return "The Stripe checkout flow was cancelled before payment completed. No package changes were applied.";
    }
    const status = String(payload?.status ?? "").trim();
    if (status === "fulfilled") {
      return "Your payment has been confirmed and the package entitlements were applied successfully.";
    }
    if (status === "failed") {
      return payload?.fulfillment_job?.error_message || "Payment succeeded, but fulfillment failed. Please contact support with your checkout session ID.";
    }
    if (hasTimedOut) {
      return "Payment may still be processing, but webhook confirmation took longer than expected. You can wait and refresh, or contact support with the identifiers below.";
    }
    if (error) {
      return error;
    }
    return "We are waiting for Stripe webhook confirmation and package fulfillment to complete. This page refreshes automatically.";
  }, [checkoutState, error, hasTimedOut, payload?.fulfillment_job?.error_message, payload?.status]);

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // ignore
    }
  };

  const isPendingStatus = checkoutState === "success" && (isLoading || payload?.status === "processing" || payload?.status === "retrying");

  return (
    <section className="portal-shell py-16">
      <div className="mx-auto max-w-2xl rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-8 shadow-sm">
        <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--stitch-primary)]">Stripe checkout</p>
        <h1 className="mt-3 text-3xl font-black text-[var(--stitch-text)]">{title}</h1>
        <p className="mt-4 text-sm leading-7 text-[var(--stitch-text-muted)]">{description}</p>

        {isPendingStatus ? (
          <div className="mt-8 rounded-2xl border border-[var(--stitch-primary)]/15 bg-[var(--stitch-primary)]/5 p-6">
            <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-4">
                <div className="relative flex h-14 w-14 items-center justify-center">
                  <div className="absolute inset-0 rounded-full border-2 border-[var(--stitch-primary)]/20" />
                  <div className="absolute inset-1 rounded-full border-2 border-transparent border-t-[var(--stitch-primary)] animate-spin" />
                  <div className="h-3 w-3 rounded-full bg-[var(--stitch-primary)] animate-pulse" />
                </div>
                <div>
                  <p className="text-sm font-semibold text-[var(--stitch-text)]">Waiting for backend confirmation</p>
                  <p className="mt-1 text-xs leading-6 text-[var(--stitch-text-muted)]">
                    Stripe has returned control to the app. We are now waiting for webhook confirmation and final entitlement fulfillment.
                  </p>
                </div>
              </div>

              <div className="grid gap-2 sm:min-w-[180px]">
                <div className="h-2 overflow-hidden rounded-full bg-[var(--stitch-border)]">
                  <div className="h-full w-1/2 rounded-full bg-[var(--stitch-primary)] animate-pulse" />
                </div>
                <div className="flex gap-2">
                  <span className="h-2 w-2 rounded-full bg-[var(--stitch-primary)] animate-bounce [animation-delay:-0.2s]" />
                  <span className="h-2 w-2 rounded-full bg-[var(--stitch-primary)] animate-bounce [animation-delay:-0.1s]" />
                  <span className="h-2 w-2 rounded-full bg-[var(--stitch-primary)] animate-bounce" />
                </div>
              </div>
            </div>
          </div>
        ) : null}

        {checkoutState === "success" ? (
          <div className="mt-8 rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-5">
            <dl className="grid gap-3 text-sm">
              <div className="grid gap-2 border-b border-[var(--stitch-border)] pb-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                <dt className="text-[var(--stitch-text-muted)]">Session</dt>
                <dd className="flex min-w-0 flex-col items-start gap-2 text-[var(--stitch-text)]">
                  <span className="w-full break-all rounded-lg bg-[var(--stitch-bg-elevated)] px-3 py-2 font-mono text-xs leading-6">
                    {sessionID || "--"}
                  </span>
                  {sessionID ? (
                    <button type="button" className="btn-ghost px-2 py-1 text-xs" onClick={() => void handleCopy(sessionID)}>
                      Copy
                    </button>
                  ) : null}
                </dd>
              </div>
              <div className="grid gap-2 border-b border-[var(--stitch-border)] pb-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                <dt className="text-[var(--stitch-text-muted)]">Package</dt>
                <dd className="text-[var(--stitch-text)]">{payload?.package_name || payload?.tier_code || "--"}</dd>
              </div>
              <div className="grid gap-2 border-b border-[var(--stitch-border)] pb-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                <dt className="text-[var(--stitch-text-muted)]">Amount</dt>
                <dd className="text-[var(--stitch-text)]">{formatAmount(payload?.amount_minor, payload?.currency) || "--"}</dd>
              </div>
              <div className="grid gap-2 border-b border-[var(--stitch-border)] pb-3 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                <dt className="text-[var(--stitch-text-muted)]">Status</dt>
                <dd className="text-[var(--stitch-text)]">
                  <span className={`inline-flex rounded-full px-3 py-1 text-xs font-semibold ${
                    payload?.status === "fulfilled"
                      ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                      : payload?.status === "failed"
                        ? "bg-red-500/10 text-red-700 dark:text-red-300"
                        : "bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)]"
                  }`}>
                    {payload?.status || (isLoading ? "processing" : "--")}
                  </span>
                </dd>
              </div>
              <div className="grid gap-2 sm:grid-cols-[120px_minmax(0,1fr)] sm:items-start">
                <dt className="text-[var(--stitch-text-muted)]">Payment Event</dt>
                <dd className="flex min-w-0 flex-col items-start gap-2 text-[var(--stitch-text)]">
                  <span className="w-full break-all rounded-lg bg-[var(--stitch-bg-elevated)] px-3 py-2 font-mono text-xs leading-6">
                    {payload?.payment_event_id || "--"}
                  </span>
                  {payload?.payment_event_id ? (
                    <button type="button" className="btn-ghost px-2 py-1 text-xs" onClick={() => void handleCopy(payload.payment_event_id!)}>
                      Copy
                    </button>
                  ) : null}
                </dd>
              </div>
            </dl>
          </div>
        ) : null}

        {hasTimedOut ? (
          <div className="mt-6 rounded-xl border border-amber-400/35 bg-amber-500/10 p-4 text-sm text-amber-700 dark:border-amber-400/45 dark:bg-amber-500/20 dark:text-amber-300">
            Contact support if this remains unresolved. Include your Stripe checkout session ID and payment event ID so we can locate the fulfillment job quickly.
          </div>
        ) : null}

        <div className="mt-8 flex flex-wrap gap-3">
          <Link href="/dashboard" className="btn-primary inline-flex items-center justify-center no-underline">
            Go to dashboard
          </Link>
          <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
            Back to packages
          </Link>
        </div>
      </div>
    </section>
  );
}

export default function CheckoutStatusPage() {
  return (
    <Suspense>
      <CheckoutStatusContent />
    </Suspense>
  );
}

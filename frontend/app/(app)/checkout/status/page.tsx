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

const MONO = { fontFamily: "var(--font-editorial-mono)" } as const;
const SERIF = { fontFamily: "var(--font-editorial-serif)" } as const;

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
  const statusTone =
    payload?.status === "fulfilled"
      ? { bg: "rgba(20,122,79,0.1)", color: "var(--accent-ink)" }
      : payload?.status === "failed"
        ? { bg: "rgba(220,38,38,0.1)", color: "#b91c1c" }
        : { bg: "rgba(20,122,79,0.1)", color: "var(--accent-ink)" };

  return (
    <section className="min-h-[78vh]" style={{ background: "var(--bone)" }}>
      <div className="mx-auto max-w-2xl px-6 py-20">
        <p className="text-xs font-bold uppercase tracking-[0.22em]" style={{ color: "var(--ink-faint)", ...MONO }}>
          Stripe Checkout · 支付确认
        </p>
        <h1 className="mt-3 text-4xl font-black leading-[1.05] md:text-5xl" style={{ color: "var(--ink)", ...SERIF }}>
          {title}
          <span style={{ color: "var(--accent)" }}>.</span>
        </h1>
        <p className="mt-4 text-base leading-7" style={{ color: "var(--ink-muted)" }}>
          {description}
        </p>

        {isPendingStatus ? (
          <div
            className="mt-8 rounded-lg border p-6"
            style={{ borderColor: "rgba(20,122,79,0.25)", background: "rgba(20,122,79,0.06)" }}
          >
            <div className="flex items-center gap-4">
              <div className="relative flex h-12 w-12 items-center justify-center">
                <div className="absolute inset-0 rounded-full border-2" style={{ borderColor: "rgba(20,122,79,0.2)" }} />
                <div
                  className="absolute inset-0 rounded-full border-2 border-transparent animate-spin"
                  style={{ borderTopColor: "var(--accent)" }}
                />
                <div className="h-2 w-2 rounded-full animate-pulse" style={{ background: "var(--accent)" }} />
              </div>
              <div className="min-w-0">
                <p className="text-sm font-bold" style={{ color: "var(--ink)" }}>
                  等待后端确认 / Waiting for confirmation
                </p>
                <p className="mt-1 text-xs leading-6" style={{ color: "var(--ink-muted)", ...MONO }}>
                  Stripe has returned control. Awaiting webhook + entitlement fulfillment.
                </p>
              </div>
            </div>
            <div className="mt-5 flex items-center gap-2">
              <span className="h-1.5 w-1.5 rounded-full animate-bounce" style={{ background: "var(--accent)", animationDelay: "-0.2s" }} />
              <span className="h-1.5 w-1.5 rounded-full animate-bounce" style={{ background: "var(--accent)", animationDelay: "-0.1s" }} />
              <span className="h-1.5 w-1.5 rounded-full animate-bounce" style={{ background: "var(--accent)" }} />
            </div>
          </div>
        ) : null}

        {checkoutState === "success" ? (
          <div
            className="mt-8 overflow-hidden rounded-lg border"
            style={{ borderColor: "var(--line)", background: "rgba(255,255,255,0.45)" }}
          >
            <dl className="divide-y" style={{ borderColor: "var(--line)" }}>
              <Row label="Session">
                <span
                  className="block w-full break-all rounded px-3 py-2 text-xs leading-6"
                  style={{ background: "var(--bone)", color: "var(--ink)", ...MONO }}
                >
                  {sessionID || "--"}
                </span>
                {sessionID ? (
                  <button
                    type="button"
                    className="mt-1 text-xs underline"
                    style={{ color: "var(--accent-ink)" }}
                    onClick={() => void handleCopy(sessionID)}
                  >
                    Copy
                  </button>
                ) : null}
              </Row>
              <Row label="Package">
                <span style={{ color: "var(--ink)" }}>{payload?.package_name || payload?.tier_code || "--"}</span>
              </Row>
              <Row label="Amount">
                <span style={{ color: "var(--ink)" }}>{formatAmount(payload?.amount_minor, payload?.currency) || "--"}</span>
              </Row>
              <Row label="Status">
                <span
                  className="inline-flex rounded-full px-3 py-1 text-xs font-bold uppercase tracking-wider"
                  style={{ background: statusTone.bg, color: statusTone.color }}
                >
                  {payload?.status || (isLoading ? "processing" : "--")}
                </span>
              </Row>
              <Row label="Payment Event" last>
                <span
                  className="block w-full break-all rounded px-3 py-2 text-xs leading-6"
                  style={{ background: "var(--bone)", color: "var(--ink)", ...MONO }}
                >
                  {payload?.payment_event_id || "--"}
                </span>
                {payload?.payment_event_id ? (
                  <button
                    type="button"
                    className="mt-1 text-xs underline"
                    style={{ color: "var(--accent-ink)" }}
                    onClick={() => void handleCopy(payload.payment_event_id!)}
                  >
                    Copy
                  </button>
                ) : null}
              </Row>
            </dl>
          </div>
        ) : null}

        {hasTimedOut ? (
          <div
            className="mt-6 rounded-lg border p-4 text-sm"
            style={{ borderColor: "rgba(180,83,9,0.35)", background: "rgba(245,158,11,0.1)", color: "#92400e" }}
          >
            Contact support if this remains unresolved. Include your Stripe checkout session ID and payment event ID so we can locate the fulfillment job quickly.
          </div>
        ) : null}

        <div className="mt-10 flex flex-wrap gap-3">
          <Link
            href="/dashboard"
            className="inline-flex items-center justify-center rounded-full no-underline transition-transform hover:-translate-y-0.5"
            style={{ background: "var(--ink)", color: "var(--bone)", padding: "12px 28px", fontWeight: 800, fontSize: 13 }}
          >
            Go to dashboard
          </Link>
          <Link
            href="/services"
            className="inline-flex items-center justify-center rounded-full border no-underline transition-colors"
            style={{ borderColor: "var(--ink)", color: "var(--ink)", padding: "12px 28px", fontWeight: 800, fontSize: 13 }}
          >
            Back to packages
          </Link>
        </div>
      </div>
    </section>
  );
}

function Row({ label, children, last }: { label: string; children: React.ReactNode; last?: boolean }) {
  return (
    <div className="grid gap-2 px-6 py-4 sm:grid-cols-[140px_minmax(0,1fr)] sm:items-start" style={{ borderColor: "var(--line)", borderTopWidth: last ? 0 : undefined }}>
      <dt className="text-xs uppercase tracking-wider" style={{ color: "var(--ink-faint)", ...MONO }}>
        {label}
      </dt>
      <dd className="flex min-w-0 flex-col items-start" style={{ color: "var(--ink)" }}>
        {children}
      </dd>
    </div>
  );
}

export default function CheckoutStatusPage() {
  return (
    <Suspense>
      <CheckoutStatusContent />
    </Suspense>
  );
}

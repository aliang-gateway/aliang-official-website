"use client";

import { useEffect, useState } from "react";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type AdminPaymentRecord = {
  id: number;
  provider: string;
  checkout_session_id: string;
  payment_event_id?: string;
  user_id: number;
  tier_code: string;
  package_name: string;
  amount_minor: number;
  currency: string;
  status: string;
  order_status: string;
  replayable: boolean;
  fulfillment_job?: {
    id?: number;
    status?: string;
    error_message?: string | null;
  };
};

type PaymentRecordsResponse = {
  records?: AdminPaymentRecord[];
  error?: string;
};

function formatMoney(amountMinor: number, currency: string) {
  return `${(amountMinor / 100).toFixed(2)} ${(currency || "cny").toUpperCase()}`;
}

export default function AdminPaymentsPage() {
  const [records, setRecords] = useState<AdminPaymentRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [busyReplayID, setBusyReplayID] = useState<number | null>(null);

  const loadRecords = async (showSpinner: boolean) => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    if (!sessionToken) {
      setError("Missing session token. Please login first.");
      setIsLoading(false);
      return;
    }

    if (showSpinner) {
      setIsRefreshing(true);
    }

    try {
      const response = await fetch("/api/admin/payments?limit=100", {
        method: "GET",
        headers: {
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
      });
      const payload = (await response.json()) as PaymentRecordsResponse;
      if (!response.ok) {
        throw new Error(payload.error ?? "Failed to load payment records");
      }
      setRecords(Array.isArray(payload.records) ? payload.records : []);
      setError(null);
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "Failed to load payment records");
    } finally {
      setIsLoading(false);
      if (showSpinner) {
        setIsRefreshing(false);
      }
    }
  };

  useEffect(() => {
    void loadRecords(false);
  }, []);

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // ignore
    }
  };

  const handleReplay = async (record: AdminPaymentRecord) => {
    if (!record.fulfillment_job?.id) {
      return;
    }

    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    if (!sessionToken) {
      setError("Missing session token. Please login first.");
      return;
    }

    setBusyReplayID(record.id);
    try {
      const response = await fetch(`/api/admin/fulfillment/jobs/${record.fulfillment_job.id}/replay`, {
        method: "POST",
        headers: {
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
      });
      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "Failed to replay fulfillment job");
      }
      await loadRecords(false);
    } catch (replayError) {
      setError(replayError instanceof Error ? replayError.message : "Failed to replay fulfillment job");
    } finally {
      setBusyReplayID(null);
    }
  };

  const renderStatusPill = (value?: string | null) => {
    const label = String(value ?? "").trim() || "--";
    const normalized = label.toLowerCase();
    const className =
      normalized === "fulfilled"
        ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
        : normalized === "failed"
          ? "bg-red-500/10 text-red-700 dark:text-red-300"
          : normalized === "retrying"
            ? "bg-amber-500/10 text-amber-700 dark:text-amber-300"
            : "bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)]";
    return <span className={`inline-flex rounded-full px-3 py-1 text-xs font-semibold ${className}`}>{label}</span>;
  };

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-3 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-2">
            <h1 className="section-title">
              <span className="gradient-text">Payment Records</span>
            </h1>
            <p className="section-subtitle">
              Review Stripe checkout sessions and their fulfillment status.
            </p>
          </div>
          <button type="button" className="btn-ghost" onClick={() => void loadRecords(true)} disabled={isRefreshing}>
            {isRefreshing ? "Refreshing..." : "Refresh"}
          </button>
        </div>
      </div>

      <div className="block-card space-y-4">
        {error ? (
          <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300">
            {error}
          </div>
        ) : null}

        {isLoading ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading payment records...</p>
        ) : records.length === 0 ? (
          <p className="text-sm text-[var(--portal-muted)]">No payment records yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-[var(--portal-muted)]">
                  <th className="px-2 py-1">Package</th>
                  <th className="px-2 py-1">Amount</th>
                  <th className="px-2 py-1">Checkout Session</th>
                  <th className="px-2 py-1">Payment Event</th>
                  <th className="px-2 py-1">Fulfillment</th>
                  <th className="px-2 py-1">Payment</th>
                  <th className="px-2 py-1">Order</th>
                  <th className="px-2 py-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                {records.map((record) => (
                  <tr key={record.id} className="rounded-lg bg-[var(--portal-clay)] align-top">
                    <td className="px-2 py-2">
                      <div className="font-medium text-[var(--portal-ink)]">{record.package_name || record.tier_code}</div>
                      <div className="font-mono text-xs text-[var(--portal-muted)]">{record.tier_code}</div>
                    </td>
                    <td className="px-2 py-2 text-[var(--portal-ink)]">{formatMoney(record.amount_minor, record.currency)}</td>
                    <td className="px-2 py-2">
                      <div className="rounded-lg bg-[var(--portal-clay-strong)] px-2 py-2 font-mono text-xs break-all text-[var(--portal-muted)]">
                        {record.checkout_session_id}
                      </div>
                      <button type="button" className="btn-ghost mt-2 px-2 py-1 text-xs" onClick={() => void handleCopy(record.checkout_session_id)}>
                        Copy
                      </button>
                    </td>
                    <td className="px-2 py-2">
                      <div className="rounded-lg bg-[var(--portal-clay-strong)] px-2 py-2 font-mono text-xs break-all text-[var(--portal-muted)]">
                        {record.payment_event_id || "-"}
                      </div>
                      {record.payment_event_id ? (
                        <button type="button" className="btn-ghost mt-2 px-2 py-1 text-xs" onClick={() => void handleCopy(record.payment_event_id!)}>
                          Copy
                        </button>
                      ) : null}
                    </td>
                    <td className="px-2 py-2">
                      {record.fulfillment_job ? (
                        <div className="space-y-1">
                          <div className="font-mono text-xs text-[var(--portal-ink)]">#{record.fulfillment_job.id}</div>
                          <div className="text-xs text-[var(--portal-muted)]">{record.fulfillment_job.status || "-"}</div>
                          {record.fulfillment_job.error_message ? (
                            <div className="max-w-[280px] text-xs text-red-600 dark:text-red-300">{record.fulfillment_job.error_message}</div>
                          ) : null}
                        </div>
                      ) : (
                        <span className="text-xs text-[var(--portal-muted)]">Not created</span>
                      )}
                    </td>
                    <td className="px-2 py-2">{renderStatusPill(record.status)}</td>
                    <td className="px-2 py-2">{renderStatusPill(record.order_status)}</td>
                    <td className="px-2 py-2">
                      {record.replayable && record.fulfillment_job?.id ? (
                        <button
                          type="button"
                          className="btn-ghost"
                          disabled={busyReplayID === record.id}
                          onClick={() => void handleReplay(record)}
                        >
                          {busyReplayID === record.id ? "Replaying..." : "Replay"}
                        </button>
                      ) : (
                        <span className="text-xs text-[var(--portal-muted)]">-</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </section>
  );
}

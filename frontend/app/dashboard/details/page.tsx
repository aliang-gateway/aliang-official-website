"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type TrendPoint = {
  bucket_start: string;
  value: number;
};

type TrendSeries = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  interval: "day";
  points: TrendPoint[];
};

type RequestRecord = {
  request_id: string;
  occurred_at: string;
  endpoint: string;
  status: "success" | "error";
  request_count: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  latency_ms: number;
};

type RequestRecordList = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  records: RequestRecord[];
};

type DashboardDetailsResponse = {
  request_records: RequestRecordList;
  token_trend: TrendSeries;
  api_frequency_trend: TrendSeries;
};

function formatShortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }

  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(date);
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }

  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

function buildPreviewPoints(points: TrendPoint[], fallbackStep: number) {
  if (points.length > 0) {
    return points;
  }

  return Array.from({ length: 7 }, (_, index) => ({
    bucket_start: new Date(Date.now() - (6 - index) * 24 * 60 * 60 * 1000).toISOString(),
    value: fallbackStep * (index + 1),
  }));
}

function TrendDetail({
  points,
  tone,
  label,
}: {
  points: TrendPoint[];
  tone: "cyan" | "emerald";
  label: string;
}) {
  const preview = useMemo(() => buildPreviewPoints(points, tone === "emerald" ? 14 : 2800), [points, tone]);
  const maxValue = Math.max(...preview.map((point) => point.value), 1);
  const stroke = tone === "emerald" ? "#10b981" : "#06b6d4";

  const coordinates = preview
    .map((point, index) => {
      const x = (index / Math.max(preview.length - 1, 1)) * 100;
      const y = 100 - (point.value / maxValue) * 100;
      return `${x},${y}`;
    })
    .join(" ");

  const areaCoordinates = `${coordinates} 100,100 0,100`;

  return (
    <div className="mt-4 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
      <svg viewBox="0 0 100 100" className="h-40 w-full overflow-visible" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={`trend-detail-fill-${tone}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.28" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0.03" />
          </linearGradient>
        </defs>
        <path d={`M ${areaCoordinates}`} fill={`url(#trend-detail-fill-${tone})`} />
        <polyline fill="none" stroke={stroke} strokeWidth="3" strokeLinejoin="round" strokeLinecap="round" points={coordinates} />
        {preview.map((point, index) => {
          const x = (index / Math.max(preview.length - 1, 1)) * 100;
          const y = 100 - (point.value / maxValue) * 100;
          return <circle key={`${point.bucket_start}-${point.value}`} cx={x} cy={y} r="2.5" fill={stroke} />;
        })}
      </svg>

      <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
        <div className="grid min-w-0 flex-1 grid-cols-3 gap-2 text-xs text-[var(--portal-muted)] sm:grid-cols-7">
          {preview.map((point) => (
            <div
              key={`${point.bucket_start}-${point.value}-label`}
              className="min-w-0 rounded-2xl bg-white/50 px-2 py-1 text-center dark:bg-slate-950/30"
            >
              {formatShortDate(point.bucket_start)}
            </div>
          ))}
        </div>
        <div className="rounded-full border border-[var(--portal-line)] bg-white/60 px-3 py-1 text-xs font-semibold text-[var(--portal-muted)] dark:bg-slate-950/30">
          {label}
        </div>
      </div>
    </div>
  );
}

export default function DashboardDetailsPage() {
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [details, setDetails] = useState<DashboardDetailsResponse | null>(null);

  useEffect(() => {
    setIsHydrated(true);
    setSessionToken(localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "");
  }, []);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (!sessionToken) {
      setDetails(null);
      setLoading(false);
      return;
    }

    const controller = new AbortController();

    const loadDetails = async () => {
      setLoading(true);
      setError(null);

      try {
        const response = await fetch("/api/dashboard/details", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });

        const payload = (await response.json()) as DashboardDetailsResponse | { error?: string };
        if (!response.ok) {
          throw new Error((payload as { error?: string }).error ?? "Failed to load dashboard details");
        }

        setDetails(payload as DashboardDetailsResponse);
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }

        setDetails(null);
        setError(fetchError instanceof Error ? fetchError.message : "Failed to load dashboard details");
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    };

    void loadDetails();

    return () => controller.abort();
  }, [isHydrated, sessionToken]);

  const requestRecords = details?.request_records.records ?? [];
  const tokenPoints = details?.token_trend.points ?? [];
  const frequencyPoints = details?.api_frequency_trend.points ?? [];

  if (!isHydrated || loading) {
    return (
      <section className="portal-shell space-y-6 py-8">
        <div className="portal-header clay-panel p-5">
          <div className="min-w-0 space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">Dashboard details</p>
            <h1 className="section-title">
              <span className="gradient-text">Request records</span>
            </h1>
          </div>
        </div>

        <div className="block-card p-5">
          <p className="text-sm text-[var(--portal-muted)]">Loading deeper trends and recent request history...</p>
        </div>
      </section>
    );
  }

  if (!sessionToken) {
    return (
      <section className="portal-shell space-y-6 py-8">
        <div className="portal-header clay-panel p-5">
          <div className="min-w-0 space-y-3">
            <Link
              href="/dashboard"
              className="inline-flex items-center text-sm text-[var(--portal-muted)] transition-colors hover:text-[var(--portal-ink)]"
            >
              <svg aria-hidden="true" className="mr-2 h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              Back to dashboard
            </Link>
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">Dashboard details</p>
              <h1 className="section-title">
                <span className="gradient-text">Request records</span>
              </h1>
              <p className="section-subtitle max-w-2xl">
                Sign in again to view recent request records, token movement, and API request frequency trends.
              </p>
            </div>
          </div>
        </div>

        <div className="block-card space-y-4">
          <p className="notice">Your session token is missing. Return to the main dashboard or sign in again to load private request history.</p>
          <div className="flex flex-wrap gap-3">
            <Link href="/dashboard" className="btn-ghost inline-flex items-center justify-center no-underline">
              Back to dashboard
            </Link>
            <Link href="/login" className="btn-primary inline-flex items-center justify-center no-underline">
              Go to login
            </Link>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-6 py-8">
      <div className="portal-header clay-panel p-5">
        <div className="min-w-0 space-y-3">
          <Link
            href="/dashboard"
            className="inline-flex items-center text-sm text-[var(--portal-muted)] transition-colors hover:text-[var(--portal-ink)]"
          >
            <svg aria-hidden="true" className="mr-2 h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to dashboard
          </Link>

          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">Dashboard details</p>
            <h1 className="section-title">
              <span className="gradient-text">Request records & trends</span>
            </h1>
            <p className="section-subtitle max-w-2xl">
              Review recent request activity and the two deeper trend surfaces exposed by the simplified dashboard contract.
            </p>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Link href="/dashboard" className="btn-ghost inline-flex items-center justify-center no-underline">
            Home overview
          </Link>
        </div>
      </div>

      {error ? <p className="notice">Dashboard details are temporarily unavailable: {error}</p> : null}

      <div className="grid gap-6 xl:grid-cols-2">
        <article className="block-card min-w-0">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-cyan-500 dark:text-cyan-400">Token trend</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Usage drift</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                Daily token totals from the app-owned aggregation layer, kept inline so this page stays in the same dashboard family.
              </p>
            </div>
            <div className="rounded-full border border-cyan-500/20 bg-cyan-500/10 px-3 py-1 text-xs font-semibold text-cyan-600 dark:text-cyan-300">
              {tokenPoints.length > 0 ? `${tokenPoints.length} points` : "empty-safe"}
            </div>
          </div>
          <TrendDetail points={tokenPoints} tone="cyan" label="Daily token buckets" />
        </article>

        <article className="block-card min-w-0">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">API request frequency</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Request tempo</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                A simple request-frequency trend for deeper reading than the home page, without expanding into full analytics tooling.
              </p>
            </div>
            <div className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
              {frequencyPoints.length > 0 ? `${frequencyPoints.length} points` : "empty-safe"}
            </div>
          </div>
          <TrendDetail points={frequencyPoints} tone="emerald" label="Daily request buckets" />
        </article>
      </div>

      <article className="block-card min-w-0 space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-sm font-semibold text-[var(--portal-muted)]">Request records</p>
            <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Recent requests</h2>
            <p className="mt-2 max-w-2xl text-sm text-[var(--portal-muted)]">
              This table stays explicit when there are no records yet, so sparse upstream data never collapses into a broken layout.
            </p>
          </div>
          <div className="rounded-full border border-[var(--portal-line)] bg-white/60 px-3 py-1 text-xs font-semibold text-[var(--portal-muted)] dark:bg-slate-950/30">
            {requestRecords.length > 0 ? `${requestRecords.length} records` : "No records yet"}
          </div>
        </div>

        <div className="overflow-x-auto rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)]">
          <table className="w-full min-w-[860px] text-left">
            <thead>
              <tr className="border-b border-[var(--portal-line)] bg-white/40 dark:bg-slate-950/20">
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">When</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Request</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Endpoint</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Status</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Count</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Tokens</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Latency</th>
              </tr>
            </thead>
            <tbody>
              {requestRecords.map((record) => (
                <tr key={record.request_id} className="border-b border-[var(--portal-line)] last:border-b-0">
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{formatDateTime(record.occurred_at)}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">
                    <div className="font-medium">{record.request_id}</div>
                  </td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-muted)]">{record.endpoint || "--"}</td>
                  <td className="px-4 py-3 text-sm">
                    <span
                      className={
                        record.status === "success"
                          ? "inline-flex rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300"
                          : "inline-flex rounded-full border border-amber-500/25 bg-amber-500/10 px-2.5 py-1 text-xs font-semibold text-amber-700 dark:text-amber-300"
                      }
                    >
                      {record.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{formatNumber(record.request_count)}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{formatNumber(record.total_tokens)}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{formatNumber(record.latency_ms)} ms</td>
                </tr>
              ))}

              {!error && requestRecords.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8">
                    <div className="space-y-2 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-white/30 p-5 text-sm dark:bg-slate-950/20">
                      <p className="font-semibold text-[var(--portal-ink)]">No request records yet.</p>
                      <p className="text-[var(--portal-muted)]">
                        The dashboard contract is working, but there are no aggregated request entries to show right now. Once traffic arrives,
                        the latest records will appear here without changing this layout.
                      </p>
                    </div>
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </article>
    </section>
  );
}

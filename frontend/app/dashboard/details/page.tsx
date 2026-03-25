"use client";

import Link from "next/link";
import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { parseDashboardSimpleTrendPoints, parseDashboardUsageEnvelope } from "@/lib/dashboard-analytics-adapter";

const SESSION_TOKEN_STORAGE_KEY = "session_token";
const USAGE_RECORDS_PER_PAGE = 20;

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

type UsageRecord = {
  id: number;
  request_id: string;
  model: string;
  inbound_endpoint: string;
  total_tokens: number;
  actual_cost: number;
  duration_ms: number;
  created_at: string;
};

type UsagePagination = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

type DashboardDetailsResponse = {
  token_trend: TrendSeries;
  api_frequency_trend: TrendSeries;
};

type DashboardUsageResponse = {
  data: UsageRecord[];
  pagination: UsagePagination;
};

type UnknownRecord = Record<string, unknown>;

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

function formatCost(value: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 4,
    maximumFractionDigits: 4,
  }).format(value);
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

function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as UnknownRecord;
}

function asString(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

function parsePositiveInteger(value: string | null, fallback: number) {
  if (!value) {
    return fallback;
  }

  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function parseTrendSeries(value: unknown): TrendSeries {
  const points = parseDashboardSimpleTrendPoints(value);
  return {
    aggregation_owner: "dashboard_app",
    aggregation_reason: "upstream_raw_logs_incomplete",
    interval: "day",
    points,
  };
}

function parseDashboardDetailsPayload(payload: unknown): DashboardDetailsResponse {
  const root = asRecord(payload);
  const envelopeData = asRecord(root?.data);
  const tokenTrendSource =
    root?.token_trend ??
    root?.tokenTrend ??
    root?.tokens_trend ??
    root?.token_points ??
    envelopeData?.token_trend ??
    envelopeData?.tokenTrend ??
    envelopeData?.tokens_trend ??
    envelopeData?.token_points ??
    envelopeData?.trend;
  const apiFrequencySource =
    root?.api_frequency_trend ??
    root?.apiFrequencyTrend ??
    root?.request_trend ??
    root?.requestTrend ??
    root?.request_points ??
    envelopeData?.api_frequency_trend ??
    envelopeData?.apiFrequencyTrend ??
    envelopeData?.request_trend ??
    envelopeData?.requestTrend ??
    envelopeData?.request_points ??
    envelopeData?.trend;

  return {
    token_trend: parseTrendSeries(tokenTrendSource),
    api_frequency_trend: parseTrendSeries(apiFrequencySource),
  };
}

function parseDashboardUsagePayload(payload: unknown): DashboardUsageResponse {
  const usageEnvelope = parseDashboardUsageEnvelope(payload);

  return {
    data: usageEnvelope.data.map((record) => ({
      id: record.id,
      request_id: record.request_id || "--",
      model: record.model || "",
      inbound_endpoint: record.inbound_endpoint || "",
      total_tokens: record.total_tokens,
      actual_cost: record.actual_cost,
      duration_ms: record.duration_ms,
      created_at: record.created_at || "",
    })),
    pagination: usageEnvelope.pagination,
  };
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

function DashboardDetailsPageContent() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [details, setDetails] = useState<DashboardDetailsResponse | null>(null);
  const [usageLoading, setUsageLoading] = useState(true);
  const [usageError, setUsageError] = useState<string | null>(null);
  const [usage, setUsage] = useState<DashboardUsageResponse | null>(null);

  const usagePage = useMemo(() => parsePositiveInteger(searchParams.get("page"), 1), [searchParams]);
  const usagePerPage = useMemo(() => {
    const requestedPerPage = parsePositiveInteger(searchParams.get("per_page"), USAGE_RECORDS_PER_PAGE);
    return requestedPerPage === USAGE_RECORDS_PER_PAGE ? requestedPerPage : USAGE_RECORDS_PER_PAGE;
  }, [searchParams]);

  const updateUsageSearchParams = useCallback(
    (page: number, historyMode: "push" | "replace") => {
      const nextParams = new URLSearchParams(searchParams.toString());
      nextParams.set("page", String(Math.max(1, page)));
      nextParams.set("per_page", String(USAGE_RECORDS_PER_PAGE));

      const nextQuery = nextParams.toString();
      const nextHref = nextQuery ? `${pathname}?${nextQuery}` : pathname;

      if (historyMode === "replace") {
        router.replace(nextHref);
        return;
      }

      router.push(nextHref);
    },
    [pathname, router, searchParams],
  );

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

        const payload = (await response.json()) as unknown;
        if (!response.ok) {
          const errorPayload = asRecord(payload);
          throw new Error(asString(errorPayload?.error, "Failed to load dashboard details"));
        }

        setDetails(parseDashboardDetailsPayload(payload));
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

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    const currentPage = searchParams.get("page");
    const currentPerPage = searchParams.get("per_page");

    if (currentPage === String(usagePage) && currentPerPage === String(usagePerPage)) {
      return;
    }

    updateUsageSearchParams(usagePage, "replace");
  }, [isHydrated, searchParams, updateUsageSearchParams, usagePage, usagePerPage]);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (!sessionToken) {
      setUsage(null);
      setUsageLoading(false);
      return;
    }

    const controller = new AbortController();

    const loadUsage = async () => {
      setUsageLoading(true);
      setUsageError(null);

      try {
        const searchParams = new URLSearchParams({
          page: String(usagePage),
          per_page: String(usagePerPage),
        });

        const response = await fetch(`/api/dashboard/usage?${searchParams.toString()}`, {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });

        const payload = (await response.json()) as unknown;
        if (!response.ok) {
          const errorPayload = asRecord(payload);
          throw new Error(asString(errorPayload?.error, "Failed to load usage records"));
        }

        setUsage(parseDashboardUsagePayload(payload));
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }

        setUsage(null);
        setUsageError(fetchError instanceof Error ? fetchError.message : "Failed to load usage records");
      } finally {
        if (!controller.signal.aborted) {
          setUsageLoading(false);
        }
      }
    };

    void loadUsage();

    return () => controller.abort();
  }, [isHydrated, sessionToken, usagePage, usagePerPage]);

  const usagePagination =
    usage?.pagination ??
    ({
      page: usagePage,
        per_page: usagePerPage,
      total: 0,
      total_pages: 1,
      has_next: false,
      has_prev: usagePage > 1,
    } satisfies UsagePagination);

  const usageRecords = usage?.data ?? [];
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
      {usageError ? <p className="notice">Usage records are temporarily unavailable: {usageError}</p> : null}

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
            <p className="text-sm font-semibold text-[var(--portal-muted)]">Usage records</p>
            <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Paginated usage log</h2>
            <p className="mt-2 max-w-2xl text-sm text-[var(--portal-muted)]">
              This table reads from the dedicated usage relay and stays deterministic when the payload is sparse, partial, or fully empty.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="rounded-full border border-[var(--portal-line)] bg-white/60 px-3 py-1 text-xs font-semibold text-[var(--portal-muted)] dark:bg-slate-950/30">
              {usageLoading ? "Loading records" : usagePagination.total > 0 ? `${formatNumber(usagePagination.total)} total` : "No records yet"}
            </div>
            <div className="rounded-full border border-[var(--portal-line)] bg-white/60 px-3 py-1 text-xs font-semibold text-[var(--portal-muted)] dark:bg-slate-950/30">
              Page {formatNumber(usagePagination.page)} / {formatNumber(Math.max(usagePagination.total_pages, 1))}
            </div>
          </div>
        </div>

        <div className="overflow-x-auto rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)]">
          <table className="w-full min-w-[960px] text-left">
            <thead>
              <tr className="border-b border-[var(--portal-line)] bg-white/40 dark:bg-slate-950/20">
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Created</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Request ID</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Endpoint</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Model</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Total tokens</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Actual cost</th>
                <th className="px-4 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--portal-muted)]">Duration</th>
              </tr>
            </thead>
            <tbody>
              {usageRecords.map((record, index) => (
                <tr key={`${record.request_id}-${record.id || index}`} className="border-b border-[var(--portal-line)] last:border-b-0">
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{formatDateTime(record.created_at)}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">
                    <div className="font-medium">{record.request_id || "--"}</div>
                  </td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-muted)]">{record.inbound_endpoint || "--"}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{record.model || "--"}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{record.total_tokens > 0 ? formatNumber(record.total_tokens) : "--"}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{record.actual_cost > 0 ? formatCost(record.actual_cost) : "--"}</td>
                  <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{record.duration_ms > 0 ? `${formatNumber(record.duration_ms)} ms` : "--"}</td>
                </tr>
              ))}

              {!usageError && !usageLoading && usageRecords.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8">
                    <div className="space-y-2 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-white/30 p-5 text-sm dark:bg-slate-950/20">
                      <p className="font-semibold text-[var(--portal-ink)]">No usage records yet.</p>
                      <p className="text-[var(--portal-muted)]">
                        The usage relay is working, but there are no aggregated rows to show for this page yet. Once traffic arrives, the latest
                        records will appear here without changing the layout.
                      </p>
                    </div>
                  </td>
                </tr>
              ) : null}

              {usageLoading ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8">
                    <div className="space-y-2 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-white/30 p-5 text-sm dark:bg-slate-950/20">
                      <p className="font-semibold text-[var(--portal-ink)]">Loading usage records...</p>
                      <p className="text-[var(--portal-muted)]">Fetching the current pagination window from the dashboard usage relay.</p>
                    </div>
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-3">
          <p className="text-sm text-[var(--portal-muted)]">
            Showing {usageRecords.length > 0 ? `${formatNumber((usagePagination.page - 1) * usagePagination.per_page + 1)}-${formatNumber((usagePagination.page - 1) * usagePagination.per_page + usageRecords.length)}` : "0-0"} of {formatNumber(usagePagination.total)} records
          </p>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              className="btn-ghost inline-flex items-center justify-center disabled:cursor-not-allowed disabled:opacity-60"
               onClick={() => updateUsageSearchParams(usagePage - 1, "push")}
              disabled={usageLoading || !usagePagination.has_prev}
            >
              Previous
            </button>
            <button
              type="button"
              className="btn-primary inline-flex items-center justify-center disabled:cursor-not-allowed disabled:opacity-60"
               onClick={() => updateUsageSearchParams(usagePage + 1, "push")}
              disabled={usageLoading || !usagePagination.has_next}
            >
              Next page
            </button>
          </div>
        </div>
      </article>
    </section>
  );
}

function DashboardDetailsPageFallback() {
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

export default function DashboardDetailsPage() {
  return (
    <Suspense fallback={<DashboardDetailsPageFallback />}>
      <DashboardDetailsPageContent />
    </Suspense>
  );
}

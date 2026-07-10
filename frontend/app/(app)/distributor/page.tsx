"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const SESSION_TOKEN_STORAGE_KEY = "session_token";
const LIST_PAGE_SIZE = 20;

type PaginationInfo = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

type DistributorUser = {
  user_id: number;
  upstream_user_id?: number;
  email: string;
  name: string;
  package_code?: string;
  package_name?: string;
  subscription_status?: string;
  total_tokens: number;
  active_days: number;
  actual_cost_micros: number;
  last_active_date?: string;
};

type DistributorPackage = {
  code: string;
  name: string;
  price_micros: number;
  is_enabled: boolean;
  is_published?: boolean;
};

type DistributorInvitation = {
  id: number;
  distributor_user_id: number;
  user_id: number;
  upstream_user_id?: number;
  email: string;
  name: string;
  source: string;
  created_at: string;
  updated_at?: string;
};

type AssignResult = {
  payment_event_id?: string;
  tier_code?: string;
  fulfillment_job?: {
    id?: number;
    status?: string;
    error_message?: string | null;
  };
};

type QuickCreateResult = {
  id: number;
  distributor_binding_id?: number;
  email: string;
  name: string;
  password: string;
  created_at: string;
};

type AssignmentStats = {
  totals?: {
    assignment_count?: number;
    unique_user_count?: number;
    total_price_micros?: number;
  };
  daily?: Array<{
    date: string;
    assignment_count: number;
    total_price_micros: number;
  }>;
  packages?: Array<{
    tier_code: string;
    package_name?: string;
    assignment_count: number;
    total_price_micros: number;
    latest_assigned_at?: string;
  }>;
  users?: Array<{
    user_id: number;
    email: string;
    name?: string;
    assignment_count: number;
    total_price_micros: number;
    latest_assigned_at?: string;
  }>;
};

const defaultPagination: PaginationInfo = {
  page: 1,
  per_page: LIST_PAGE_SIZE,
  total: 0,
  total_pages: 0,
  has_next: false,
  has_prev: false,
};

function parsePagination(value: unknown, fallbackPage = 1): PaginationInfo {
  const raw = value && typeof value === "object" ? value as Partial<PaginationInfo> : {};
  const total = Math.max(0, Number(raw.total ?? 0) || 0);
  const perPage = Math.max(1, Number(raw.per_page ?? LIST_PAGE_SIZE) || LIST_PAGE_SIZE);
  const page = Math.max(1, Number(raw.page ?? fallbackPage) || fallbackPage);
  const totalPages = Math.max(0, Number(raw.total_pages ?? Math.ceil(total / perPage)) || 0);
  return {
    page,
    per_page: perPage,
    total,
    total_pages: totalPages,
    has_next: Boolean(raw.has_next ?? (totalPages > 0 && page < totalPages)),
    has_prev: Boolean(raw.has_prev ?? page > 1),
  };
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("en-US").format(value || 0);
}

function formatMoneyMicros(value: number) {
  return `¥${((value || 0) / 1000000).toFixed(2)}`;
}

function formatDateTime(value?: string) {
  if (!value) return "--";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function StatsTable({ headers, rows, emptyLabel }: { headers: string[]; rows: string[][]; emptyLabel: string }) {
  if (rows.length === 0) {
    return <p className="rounded-lg border border-[var(--portal-line)] p-3 text-sm text-[var(--portal-muted)]">{emptyLabel}</p>;
  }
  return (
    <div className="overflow-x-auto rounded-lg border border-[var(--portal-line)]">
      <table className="min-w-full text-left text-sm">
        <thead className="bg-[var(--portal-clay)] text-xs uppercase text-[var(--portal-muted)]">
          <tr>
            {headers.map((header) => (
              <th key={header} className="px-3 py-2">{header}</th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--portal-line)]">
          {rows.map((row, rowIndex) => (
            <tr key={`${row[0]}-${rowIndex}`}>
              {row.map((cell, cellIndex) => (
                <td key={`${cell}-${cellIndex}`} className="px-3 py-2 text-[var(--portal-ink)]">{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PaginationControls({
  pagination,
  isLoading,
  onPageChange,
  labels,
}: {
  pagination: PaginationInfo;
  isLoading: boolean;
  onPageChange: (page: number) => void;
  labels: {
    previous: string;
    next: string;
    pageSummary: string;
    totalRecords: string;
  };
}) {
  if (pagination.total <= pagination.per_page && pagination.total_pages <= 1) {
    return null;
  }
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
      <span>{labels.totalRecords}</span>
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="btn-ghost min-h-11 px-3 py-2 text-sm"
          onClick={() => onPageChange(pagination.page - 1)}
          disabled={isLoading || !pagination.has_prev}
        >
          {labels.previous}
        </button>
        <span className="min-w-24 text-center font-medium text-[var(--portal-ink)]">{labels.pageSummary}</span>
        <button
          type="button"
          className="btn-ghost min-h-11 px-3 py-2 text-sm"
          onClick={() => onPageChange(pagination.page + 1)}
          disabled={isLoading || !pagination.has_next}
        >
          {labels.next}
        </button>
      </div>
    </div>
  );
}

export default function DistributorPage() {
  const t = useTranslations("distributor");
  const router = useRouter();
  const [sessionToken, setSessionToken] = useState("");
  const [users, setUsers] = useState<DistributorUser[]>([]);
  const [invitations, setInvitations] = useState<DistributorInvitation[]>([]);
  const [usersPagination, setUsersPagination] = useState<PaginationInfo>(defaultPagination);
  const [invitationsPagination, setInvitationsPagination] = useState<PaginationInfo>(defaultPagination);
  const [packages, setPackages] = useState<DistributorPackage[]>([]);
  const [selectedUserID, setSelectedUserID] = useState("");
  const [selectedTierCode, setSelectedTierCode] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isAssigning, setIsAssigning] = useState(false);
  const [createEmail, setCreateEmail] = useState("");
  const [isCreating, setIsCreating] = useState(false);
  const [createResult, setCreateResult] = useState<QuickCreateResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [assignResult, setAssignResult] = useState<AssignResult | null>(null);
  const [assignmentStats, setAssignmentStats] = useState<AssignmentStats | null>(null);
  const [isLoadingAssignmentStats, setIsLoadingAssignmentStats] = useState(false);
  const [statsFrom, setStatsFrom] = useState("");
  const [statsTo, setStatsTo] = useState("");

  const headers = useMemo(() => ({
    "content-type": "application/json",
    accept: "application/json",
    Authorization: `Bearer ${sessionToken}`,
  }), [sessionToken]);

  const totals = useMemo(() => users.reduce(
    (acc, user) => ({
      users: acc.users + 1,
      tokens: acc.tokens + (user.total_tokens || 0),
      activeDays: acc.activeDays + (user.active_days || 0),
      costMicros: acc.costMicros + (user.actual_cost_micros || 0),
    }),
    { users: 0, tokens: 0, activeDays: 0, costMicros: 0 },
  ), [users]);

  const loadAssignmentStats = useCallback(async (token: string, filters?: { from?: string; to?: string }) => {
    setIsLoadingAssignmentStats(true);
    try {
      const query = new URLSearchParams();
      if (filters?.from) query.set("from", filters.from);
      if (filters?.to) query.set("to", filters.to);
      const response = await fetch(`/api/distributor/stats${query.size > 0 ? `?${query.toString()}` : ""}`, {
        headers: { accept: "application/json", Authorization: `Bearer ${token}` },
        cache: "no-store",
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload?.error ?? t("loadStatsFailed"));
      }
      setAssignmentStats(payload ?? null);
    } finally {
      setIsLoadingAssignmentStats(false);
    }
  }, [t]);

  const loadData = useCallback(async (token: string, pages?: { usersPage?: number; invitationsPage?: number }) => {
    setIsLoading(true);
    setError(null);
    try {
      const usersPage = pages?.usersPage ?? 1;
      const invitationsPage = pages?.invitationsPage ?? 1;
      const usersQuery = new URLSearchParams({ page: String(usersPage), per_page: String(LIST_PAGE_SIZE) });
      const invitationsQuery = new URLSearchParams({ page: String(invitationsPage), per_page: String(LIST_PAGE_SIZE) });
      const [meResponse, usersResponse, invitationsResponse, packagesResponse] = await Promise.all([
        fetch("/api/auth/me", { headers: { accept: "application/json", Authorization: `Bearer ${token}` }, cache: "no-store" }),
        fetch(`/api/distributor/users?${usersQuery.toString()}`, { headers: { accept: "application/json", Authorization: `Bearer ${token}` }, cache: "no-store" }),
        fetch(`/api/distributor/invitations?${invitationsQuery.toString()}`, { headers: { accept: "application/json", Authorization: `Bearer ${token}` }, cache: "no-store" }),
        fetch("/api/distributor/packages", { headers: { accept: "application/json", Authorization: `Bearer ${token}` }, cache: "no-store" }),
      ]);

      const mePayload = await meResponse.json();
      const role = mePayload?.data?.role ?? mePayload?.role;
      if (!meResponse.ok || role !== "distributor") {
        router.replace("/account");
        return;
      }

      const usersPayload = await usersResponse.json();
      if (!usersResponse.ok) {
        throw new Error(usersPayload?.error ?? t("loadUsersFailed"));
      }
      const invitationsPayload = await invitationsResponse.json();
      if (!invitationsResponse.ok) {
        throw new Error(invitationsPayload?.error ?? t("loadInvitationsFailed"));
      }
      const packagesPayload = await packagesResponse.json();
      if (!packagesResponse.ok) {
        throw new Error(packagesPayload?.error ?? t("loadPackagesFailed"));
      }

      setUsers(Array.isArray(usersPayload?.users) ? usersPayload.users : []);
      setInvitations(Array.isArray(invitationsPayload?.invitations) ? invitationsPayload.invitations : []);
      setUsersPagination(parsePagination(usersPayload?.pagination, usersPage));
      setInvitationsPagination(parsePagination(invitationsPayload?.pagination, invitationsPage));
      setPackages(Array.isArray(packagesPayload?.packages) ? packagesPayload.packages : []);
      await loadAssignmentStats(token);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : t("loadDashboardFailed"));
    } finally {
      setIsLoading(false);
    }
  }, [loadAssignmentStats, router, t]);

  useEffect(() => {
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    if (!token) {
      router.replace("/login");
      return;
    }
    setSessionToken(token);
    void loadData(token);
  }, [loadData, router]);

  const handleUsersPageChange = (page: number) => {
    if (!sessionToken) return;
    void loadData(sessionToken, { usersPage: page, invitationsPage: invitationsPagination.page });
  };

  const handleInvitationsPageChange = (page: number) => {
    if (!sessionToken) return;
    void loadData(sessionToken, { usersPage: usersPagination.page, invitationsPage: page });
  };

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // Clipboard availability depends on browser permissions; keep the generated value visible.
    }
  };

  const handleQuickCreate = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);
    setCreateResult(null);

    const email = createEmail.trim();
    if (!email) {
      setError(t("emailRequired"));
      return;
    }
    if (!sessionToken) {
      setError(t("missingSession"));
      return;
    }

    setIsCreating(true);
    try {
      const response = await fetch("/api/distributor/users/quick-create", {
        method: "POST",
        headers,
        body: JSON.stringify({ email }),
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload?.error ?? t("createUserFailed"));
      }

      setCreateResult(payload);
      setCreateEmail("");
      if (payload?.id) {
        setSelectedUserID(String(payload.id));
      }
      setSuccess(t("createdSuccess", { email: payload?.email ?? email }));
      await loadData(sessionToken);
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : t("createUserFailed"));
    } finally {
      setIsCreating(false);
    }
  };

  const handleAssign = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);
    setAssignResult(null);

    const userID = Number(selectedUserID);
    if (!userID || userID <= 0) {
      setError(t("selectUserRequired"));
      return;
    }
    if (!selectedTierCode) {
      setError(t("selectPackageRequired"));
      return;
    }

    setIsAssigning(true);
    try {
      const response = await fetch("/api/distributor/assign-package", {
        method: "POST",
        headers,
        body: JSON.stringify({ user_id: userID, tier_code: selectedTierCode }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const jobMessage = payload?.fulfillment_job?.error_message;
        throw new Error(jobMessage ?? payload?.error ?? t("assignFailed"));
      }
      setAssignResult(payload);
      setSuccess(t("assignSuccess"));
      await loadData(sessionToken);
    } catch (assignError) {
      setError(assignError instanceof Error ? assignError.message : t("assignFailed"));
    } finally {
      setIsAssigning(false);
    }
  };

  return (
    <section className="portal-shell space-y-6 py-8">
      <div className="clay-panel space-y-3 p-5">
        <div className="space-y-2">
          <h1 className="section-title">
            <span className="gradient-text">{t("title")}</span>
          </h1>
          <p className="section-subtitle">{t("subtitle")}</p>
        </div>
      </div>

      {error ? <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700" role="alert">{error}</div> : null}
      {success ? <div className="rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-700" aria-live="polite">{success}</div> : null}

      <div className="block-card space-y-4 p-4">
        <div className="flex items-center gap-2">
          <span className="rounded-lg bg-[var(--portal-accent)]/10 p-2 text-[var(--portal-accent)]">
            <MaterialIcon name="person_add" size={18} />
          </span>
          <div>
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("quickCreateUser")}</h2>
            <p className="mt-1 text-sm text-[var(--portal-muted)]">{t("quickCreateUserDescription")}</p>
          </div>
        </div>
        <form className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]" onSubmit={handleQuickCreate}>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>{t("email")}</span>
            <input
              className="field"
              type="email"
              placeholder="user@example.com"
              value={createEmail}
              onChange={(event) => setCreateEmail(event.target.value)}
              disabled={isCreating}
              required
            />
          </label>
          <button type="submit" className="btn-primary self-end" disabled={isCreating}>
            {isCreating ? t("creating") : t("createUser")}
          </button>
        </form>
        {createResult ? (
          <div className="grid gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/5 p-4 text-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="font-semibold text-emerald-700 dark:text-emerald-300">{t("userCreated")}</h3>
              <button
                type="button"
                className="btn-ghost px-3 py-1 text-xs"
                onClick={() => {
                  const text = [
                    `Sub2API ID: ${createResult.id}`,
                    `${t("email")}: ${createResult.email}`,
                    `${t("password")}: ${createResult.password}`,
                    `${t("name")}: ${createResult.name}`,
                  ].filter(Boolean).join("\n");
                  void handleCopy(text);
                }}
              >
                {t("copyAll")}
              </button>
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              <div>
                <span className="text-[var(--portal-muted)]">{t("email")}: </span>
                <span className="text-[var(--portal-ink)]">{createResult.email}</span>
              </div>
              <div>
                <span className="text-[var(--portal-muted)]">{t("name")}: </span>
                <span className="text-[var(--portal-ink)]">{createResult.name}</span>
              </div>
              <div>
                <span className="text-[var(--portal-muted)]">Sub2API ID: </span>
                <span className="font-mono text-[var(--portal-ink)]">{createResult.id}</span>
              </div>
              <div className="md:col-span-2">
                <span className="text-[var(--portal-muted)]">{t("password")}: </span>
                <code className="rounded bg-[var(--portal-clay-strong)] px-2 py-0.5 font-mono text-xs text-[var(--portal-ink)]">
                  {createResult.password}
                </code>
                <button
                  type="button"
                  className="btn-ghost ml-2 px-2 py-0.5 text-xs"
                  onClick={() => void handleCopy(createResult.password)}
                >
                  {t("copy")}
                </button>
              </div>
            </div>
          </div>
        ) : null}
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <div className="block-card p-4">
          <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("users")}</p>
          <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatNumber(totals.users)}</p>
        </div>
        <div className="block-card p-4">
          <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("totalTokens")}</p>
          <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatNumber(totals.tokens)}</p>
        </div>
        <div className="block-card p-4">
          <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("activeDays")}</p>
          <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatNumber(totals.activeDays)}</p>
        </div>
        <div className="block-card p-4">
          <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("spend")}</p>
          <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatMoneyMicros(totals.costMicros)}</p>
        </div>
      </div>

      <div className="block-card overflow-hidden">
        <div className="border-b border-[var(--portal-line)] p-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("assignmentStats")}</h2>
          <p className="mt-1 text-sm text-[var(--portal-muted)]">{t("assignmentStatsDescription")}</p>
        </div>
        <form className="grid gap-3 border-b border-[var(--portal-line)] p-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end" onSubmit={(event) => {
          event.preventDefault();
          if (!sessionToken) return;
          void loadAssignmentStats(sessionToken, { from: statsFrom, to: statsTo });
        }}>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>{t("fromDate")}</span>
            <input className="field" type="date" value={statsFrom} onChange={(event) => setStatsFrom(event.target.value)} disabled={isLoadingAssignmentStats} />
          </label>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>{t("toDate")}</span>
            <input className="field" type="date" value={statsTo} onChange={(event) => setStatsTo(event.target.value)} disabled={isLoadingAssignmentStats} />
          </label>
          <button className="btn-primary" type="submit" disabled={isLoadingAssignmentStats}>
            {isLoadingAssignmentStats ? t("loadingStats") : t("queryStats")}
          </button>
        </form>
        <div className="grid gap-4 p-4 md:grid-cols-3">
          <div className="rounded-lg border border-[var(--portal-line)] p-3">
            <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("assignedPackages")}</p>
            <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatNumber(assignmentStats?.totals?.assignment_count ?? 0)}</p>
          </div>
          <div className="rounded-lg border border-[var(--portal-line)] p-3">
            <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("assignedUsers")}</p>
            <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatNumber(assignmentStats?.totals?.unique_user_count ?? 0)}</p>
          </div>
          <div className="rounded-lg border border-[var(--portal-line)] p-3">
            <p className="text-xs font-semibold uppercase text-[var(--portal-muted)]">{t("assignmentRevenue")}</p>
            <p className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{formatMoneyMicros(assignmentStats?.totals?.total_price_micros ?? 0)}</p>
          </div>
        </div>
        <div className="grid gap-4 border-t border-[var(--portal-line)] p-4 lg:grid-cols-3">
          <StatsTable
            emptyLabel={t("emptyStats")}
            headers={[t("date"), t("count"), t("amount")]}
            rows={(assignmentStats?.daily ?? []).slice(0, 10).map((item) => [
              item.date,
              formatNumber(item.assignment_count),
              formatMoneyMicros(item.total_price_micros),
            ])}
          />
          <StatsTable
            emptyLabel={t("emptyStats")}
            headers={[t("package"), t("count"), t("amount")]}
            rows={(assignmentStats?.packages ?? []).slice(0, 10).map((item) => [
              item.package_name || item.tier_code,
              formatNumber(item.assignment_count),
              formatMoneyMicros(item.total_price_micros),
            ])}
          />
          <StatsTable
            emptyLabel={t("emptyStats")}
            headers={[t("user"), t("count"), t("amount")]}
            rows={(assignmentStats?.users ?? []).slice(0, 10).map((item) => [
              item.name || item.email || `#${item.user_id}`,
              formatNumber(item.assignment_count),
              formatMoneyMicros(item.total_price_micros),
            ])}
          />
        </div>
      </div>

      <div className="block-card overflow-hidden">
        <div className="border-b border-[var(--portal-line)] p-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("invitationRecords")}</h2>
          <p className="mt-1 text-sm text-[var(--portal-muted)]">{t("invitationRecordsDescription")}</p>
        </div>
        {isLoading ? (
          <p className="p-4 text-sm text-[var(--portal-muted)]">{t("loadingInvitations")}</p>
        ) : invitations.length === 0 ? (
          <p className="p-4 text-sm text-[var(--portal-muted)]">{t("emptyInvitations")}</p>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-left">
                <thead className="bg-[var(--portal-clay)] text-xs uppercase text-[var(--portal-muted)]">
                  <tr>
                    <th className="px-4 py-3">{t("user")}</th>
                    <th className="px-4 py-3">{t("source")}</th>
                    <th className="px-4 py-3">{t("createdAt")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--portal-line)]">
                  {invitations.map((item) => (
                    <tr key={item.id}>
                      <td className="px-4 py-3">
                        <p className="text-sm font-semibold text-[var(--portal-ink)]">{item.name || item.email}</p>
                        <p className="text-xs text-[var(--portal-muted)]">{item.email}</p>
                        {item.upstream_user_id != null && (
                          <p className="mt-0.5 flex items-center gap-1 text-xs text-[var(--portal-muted)]">
                            <code className="font-mono">sub2api #{item.upstream_user_id}</code>
                            <button type="button" className="btn-ghost px-1.5 py-0 text-xs" onClick={() => void handleCopy(String(item.upstream_user_id))}>
                              {t("copy")}
                            </button>
                          </p>
                        )}
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{item.source || "--"}</td>
                      <td className="px-4 py-3 text-sm text-[var(--portal-muted)]">{formatDateTime(item.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <PaginationControls
              pagination={invitationsPagination}
              isLoading={isLoading}
              onPageChange={handleInvitationsPageChange}
              labels={{
                previous: t("previous"),
                next: t("nextPage"),
                pageSummary: t("pageN", { current: formatNumber(invitationsPagination.page), total: formatNumber(Math.max(invitationsPagination.total_pages, 1)) }),
                totalRecords: t("totalRecords", { count: formatNumber(invitationsPagination.total) }),
              }}
            />
          </>
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
        <div className="block-card overflow-hidden">
          <div className="border-b border-[var(--portal-line)] p-4">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("boundUsers")}</h2>
          </div>
          {isLoading ? (
            <p className="p-4 text-sm text-[var(--portal-muted)]">{t("loadingUsers")}</p>
          ) : users.length === 0 ? (
            <p className="p-4 text-sm text-[var(--portal-muted)]">{t("emptyUsers")}</p>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="min-w-full text-left">
                  <thead className="bg-[var(--portal-clay)] text-xs uppercase text-[var(--portal-muted)]">
                    <tr>
                      <th className="px-4 py-3">{t("user")}</th>
                      <th className="px-4 py-3">{t("package")}</th>
                      <th className="px-4 py-3">{t("tokens")}</th>
                      <th className="px-4 py-3">{t("activeDays")}</th>
                      <th className="px-4 py-3">{t("spend")}</th>
                      <th className="px-4 py-3">{t("lastActive")}</th>
                      <th className="px-4 py-3"></th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--portal-line)]">
                    {users.map((user) => (
                      <tr key={user.user_id}>
                        <td className="px-4 py-3">
                          <p className="text-sm font-semibold text-[var(--portal-ink)]">{user.name || user.email}</p>
                          <p className="text-xs text-[var(--portal-muted)]">{user.email}</p>
                          {user.upstream_user_id != null && (
                            <p className="mt-0.5 flex items-center gap-1 text-xs text-[var(--portal-muted)]">
                              <code className="font-mono">sub2api #{user.upstream_user_id}</code>
                              <button type="button" className="btn-ghost px-1.5 py-0 text-xs" onClick={() => void handleCopy(String(user.upstream_user_id))}>
                                {t("copy")}
                              </button>
                            </p>
                          )}
                        </td>
                        <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{user.package_name || user.package_code || "--"}</td>
                        <td className="px-4 py-3 font-mono text-sm text-[var(--portal-ink)]">{formatNumber(user.total_tokens)}</td>
                        <td className="px-4 py-3 font-mono text-sm text-[var(--portal-ink)]">{formatNumber(user.active_days)}</td>
                        <td className="px-4 py-3 font-mono text-sm text-[var(--portal-ink)]">{formatMoneyMicros(user.actual_cost_micros)}</td>
                        <td className="px-4 py-3 text-sm text-[var(--portal-muted)]">{user.last_active_date || "--"}</td>
                        <td className="px-4 py-3">
                          <button
                            type="button"
                            className="btn-ghost px-3 py-1 text-xs"
                            onClick={() => setSelectedUserID(String(user.user_id))}
                          >
                            {t("select")}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <PaginationControls
                pagination={usersPagination}
                isLoading={isLoading}
                onPageChange={handleUsersPageChange}
                labels={{
                  previous: t("previous"),
                  next: t("nextPage"),
                  pageSummary: t("pageN", { current: formatNumber(usersPagination.page), total: formatNumber(Math.max(usersPagination.total_pages, 1)) }),
                  totalRecords: t("totalRecords", { count: formatNumber(usersPagination.total) }),
                }}
              />
            </>
          )}
        </div>

        <div className="block-card space-y-4 p-4">
          <div className="flex items-center gap-2">
            <span className="rounded-lg bg-[var(--portal-accent)]/10 p-2 text-[var(--portal-accent)]">
              <MaterialIcon name="assignment_ind" size={18} />
            </span>
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("assignPackage")}</h2>
          </div>
          <form className="grid gap-4" onSubmit={handleAssign}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("user")}</span>
              <select className="field" value={selectedUserID} onChange={(event) => setSelectedUserID(event.target.value)} disabled={isAssigning}>
                <option value="">{t("selectBoundUser")}</option>
                {users.map((user) => (
                  <option key={user.user_id} value={user.user_id}>
                    {user.email}
                  </option>
                ))}
              </select>
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("package")}</span>
              <select className="field" value={selectedTierCode} onChange={(event) => setSelectedTierCode(event.target.value)} disabled={isAssigning}>
                <option value="">{t("selectPackage")}</option>
                {packages.filter((item) => (item.is_published ?? item.is_enabled) !== false).map((item) => (
                  <option key={item.code} value={item.code}>
                    {item.name} ({item.code})
                  </option>
                ))}
              </select>
            </label>
            <button type="submit" className="btn-primary" disabled={isAssigning}>
              {isAssigning ? t("assigning") : t("assignPackage")}
            </button>
          </form>
          {assignResult?.fulfillment_job ? (
            <p className="text-sm text-[var(--portal-muted)]">
              {t("fulfillmentStatus")}<span className="font-semibold text-[var(--portal-ink)]">{assignResult.fulfillment_job.status ?? t("unknown")}</span>
            </p>
          ) : null}
        </div>
      </div>
    </section>
  );
}

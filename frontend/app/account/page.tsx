"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord, asString, extractApiError } from "@/lib/api-response";

/* ------------------------------------------------------------------ */
/*  Types                                                                */
/* ------------------------------------------------------------------ */

type UserProfile = {
  id: number;
  email: string;
  name: string;
  role: string;
  balance: number;
};

type ApiKeyItem = {
  id: number;
  name: string;
  key: string;
  group_id: number;
  group_name: string;
  status: string;
  quota: number;
  quota_used: number;
  expires_at: string;
  created_at: string;
};

type GroupItem = {
  id: number;
  name: string;
  platform: string;
  status: string;
};

type SubscriptionRow = {
  id: number;
  group_id: number;
  group_name: string;
  status: string;
  daily_used_usd: number;
  daily_limit_usd: number;
  weekly_used_usd: number;
  weekly_limit_usd: number;
  monthly_used_usd: number;
  monthly_limit_usd: number;
  expires_at: string;
};

type SubscriptionSummary = {
  active_count: number;
  total_used_usd: number;
  subscriptions: SubscriptionRow[];
};

type UsageStats = {
  today_requests: number;
  today_cost: number;
  today_tokens: number;
  total_tokens: number;
};

type PaginationInfo = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

const SESSION_TOKEN_KEY = "session_token";

/* ------------------------------------------------------------------ */
/*  Helpers                                                              */
/* ------------------------------------------------------------------ */

function asNumber(value: unknown, fallback = 0) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function authHeaders(sessionToken: string) {
  return {
    "content-type": "application/json",
    accept: "application/json",
    Authorization: `Bearer ${sessionToken}`,
  };
}

function parsePagination(payload: unknown): PaginationInfo {
  const root = asRecord(payload);
  const pag = asRecord(root?.pagination);
  const total = asNumber(pag?.total ?? root?.total);
  const page = Math.max(1, asNumber(pag?.page ?? root?.page, 1));
  const per_page = Math.max(1, asNumber(pag?.per_page ?? pag?.page_size ?? root?.per_page, 20));
  const total_pages = Math.max(1, Math.ceil(total / per_page));
  return { page, per_page, total, total_pages, has_next: page < total_pages, has_prev: page > 1 };
}

function parseApiKeysList(payload: unknown): { keys: ApiKeyItem[]; pagination: PaginationInfo } {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  const list = Array.isArray(inner?.data) ? inner.data : Array.isArray(inner?.api_keys) ? inner.api_keys : Array.isArray(root?.data) ? root.data : [];
  const keys = list
    .map((item: unknown) => asRecord(item))
    .filter((item): item is Record<string, unknown> => Boolean(item))
    .map((item) => {
      const group = asRecord(item.group);
      return {
        id: asNumber(item.id),
        name: asString(item.name) || asString(item.label),
        key: asString(item.key) || asString(item.api_key),
        group_id: asNumber(item.group_id),
        group_name: asString(group?.name) || asString(item.group_name) || `Group #${item.group_id}`,
        status: asString(item.status, "active"),
        quota: asNumber(item.quota),
        quota_used: asNumber(item.quota_used),
        expires_at: asString(item.expires_at),
        created_at: asString(item.created_at),
      };
    });
  const pagination = parsePagination(root?.data ?? inner ?? payload);
  return { keys, pagination };
}

function parseGroupsList(payload: unknown): GroupItem[] {
  const root = asRecord(payload);
  const data = Array.isArray(root?.data) ? root.data : Array.isArray(payload) ? payload : [];
  return data
    .map((item: unknown) => asRecord(item))
    .filter((item): item is Record<string, unknown> => Boolean(item))
    .map((item) => ({
      id: asNumber(item.id),
      name: asString(item.name),
      platform: asString(item.platform),
      status: asString(item.status, "active"),
    }))
    .filter((g) => g.status === "active");
}

function parseSubscriptionSummary(payload: unknown): SubscriptionSummary | null {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  const subs = Array.isArray(inner?.subscriptions) ? inner.subscriptions : [];
  if (subs.length === 0 && asNumber(inner?.active_count) === 0) return null;
  return {
    active_count: asNumber(inner?.active_count),
    total_used_usd: asNumber(inner?.total_used_usd),
    subscriptions: subs
      .map((s: unknown) => asRecord(s))
      .filter((s): s is Record<string, unknown> => Boolean(s))
      .map((s) => ({
        id: asNumber(s.id),
        group_id: asNumber(s.group_id),
        group_name: asString(s.group_name),
        status: asString(s.status, "active"),
        daily_used_usd: asNumber(s.daily_used_usd),
        daily_limit_usd: asNumber(s.daily_limit_usd),
        weekly_used_usd: asNumber(s.weekly_used_usd),
        weekly_limit_usd: asNumber(s.weekly_limit_usd),
        monthly_used_usd: asNumber(s.monthly_used_usd),
        monthly_limit_usd: asNumber(s.monthly_limit_usd),
        expires_at: asString(s.expires_at),
      })),
  };
}

function parseProfileWithBalance(payload: unknown): UserProfile | null {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  if (!inner?.email) return null;
  return {
    id: asNumber(inner.id),
    email: asString(inner.email),
    name: asString(inner.username) || asString(inner.name),
    role: asString(inner.role),
    balance: asNumber(inner.balance),
  };
}

function parseUsageStats(payload: unknown): UsageStats | null {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  if (!inner) return null;
  return {
    today_requests: asNumber(inner.today_requests),
    today_cost: asNumber(inner.today_actual_cost || inner.today_cost),
    today_tokens: asNumber(inner.today_tokens),
    total_tokens: asNumber(inner.total_tokens),
  };
}

/* ------------------------------------------------------------------ */
/*  Component                                                            */
/* ------------------------------------------------------------------ */

export default function AccountPage() {
  const router = useRouter();
  const [sessionToken, setSessionToken] = useState("");
  const [isReady, setIsReady] = useState(false);

  // Profile
  const [profile, setProfile] = useState<UserProfile | null>(null);

  // API keys
  const [apiKeys, setApiKeys] = useState<ApiKeyItem[]>([]);
  const [keyPagination, setKeyPagination] = useState<PaginationInfo>({ page: 1, per_page: 20, total: 0, total_pages: 1, has_next: false, has_prev: false });
  const [keyName, setKeyName] = useState("");
  const [keyGroupId, setKeyGroupId] = useState<number | "">("");
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<string | null>(null);
  const [apiKeyLoading, setApiKeyLoading] = useState(false);
  const [apiKeyError, setApiKeyError] = useState<string | null>(null);

  // Groups
  const [groups, setGroups] = useState<GroupItem[]>([]);

  // Change password
  const [oldPwd, setOldPwd] = useState("");
  const [newPwd, setNewPwd] = useState("");
  const [confirmPwd, setConfirmPwd] = useState("");
  const [pwdSubmitting, setPwdSubmitting] = useState(false);
  const [pwdError, setPwdError] = useState<string | null>(null);
  const [pwdSuccess, setPwdSuccess] = useState<string | null>(null);

  // Subscription & usage
  const [subscription, setSubscription] = useState<SubscriptionSummary | null>(null);
  const [usage, setUsage] = useState<UsageStats | null>(null);
  const [subLoading, setSubLoading] = useState(true);
  const [subError, setSubError] = useState<string | null>(null);

  // Loading indicator
  const [pageLoading, setPageLoading] = useState(true);

  /* --- Session & profile ------------------------------------------- */

  useEffect(() => {
    const token = localStorage.getItem(SESSION_TOKEN_KEY) ?? "";
    setSessionToken(token);
    if (!token) {
      setIsReady(true);
      setPageLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!sessionToken) return;

    const load = async () => {
      try {
        const res = await fetch("/api/dashboard/account", {
          headers: authHeaders(sessionToken),
          cache: "no-store",
        });
        const data = await res.json();
        const parsed = parseProfileWithBalance(data);
        if (parsed) setProfile(parsed);
      } catch {}

      setIsReady(true);
      setPageLoading(false);
    };
    void load();
  }, [sessionToken]);

  /* --- API keys ------------------------------------------------ */

  const loadApiKeys = useCallback(async (page = 1) => {
    if (!sessionToken) return;
    setApiKeyLoading(true);
    setApiKeyError(null);
    try {
      const res = await fetch(`/api/api-keys?page=${page}&per_page=20`, {
        headers: authHeaders(sessionToken),
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        setApiKeyError(extractApiError(payload, "Failed to load API keys"));
        return;
      }
      const payload = await res.json();
      const { keys, pagination } = parseApiKeysList(payload);
      setApiKeys(keys);
      setKeyPagination(pagination);
    } catch {
      setApiKeyError("Failed to load API keys");
    } finally {
      setApiKeyLoading(false);
    }
  }, [sessionToken]);

  const loadGroups = useCallback(async () => {
    if (!sessionToken) return;
    try {
      const res = await fetch("/api/groups/available", {
        headers: authHeaders(sessionToken),
        cache: "no-store",
      });
      if (!res.ok) return;
      const data = await res.json();
      setGroups(parseGroupsList(data));
    } catch {}
  }, [sessionToken]);

  useEffect(() => {
    if (isReady && sessionToken) {
      void loadApiKeys(1);
      void loadGroups();
    }
  }, [isReady, sessionToken, loadApiKeys, loadGroups]);

  const handleCreateApiKey = async (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setApiKeyError(null);
    setNewlyCreatedKey(null);
    try {
      const body: Record<string, unknown> = {};
      if (keyName) body.name = keyName;
      if (typeof keyGroupId === "number") body.group_id = keyGroupId;

      const res = await fetch("/api/api-keys", {
        method: "POST",
        headers: authHeaders(sessionToken),
        body: JSON.stringify(body),
        cache: "no-store",
      });
      const payload = await res.json();
      if (!res.ok) throw new Error(extractApiError(payload, "Failed to create API key"));
      const created = asRecord(payload?.data) ?? asRecord(payload);
      const keyValue = asString(created?.key) || asString(created?.api_key);
      if (!keyValue) throw new Error("Incomplete response");
      setNewlyCreatedKey(keyValue);
      setKeyName("");
      setKeyGroupId("");
      void loadApiKeys(1);
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "Failed to create API key");
    }
  };

  const handleToggleApiKey = async (keyId: number, currentStatus: string) => {
    setApiKeyError(null);
    const newStatus = currentStatus === "active" ? "inactive" : "active";
    try {
      const res = await fetch(`/api/api-keys/${keyId}`, {
        method: "PUT",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ status: newStatus }),
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, `Failed to ${newStatus === "active" ? "enable" : "disable"} API key`));
      }
      setApiKeys((prev) => prev.map((k) => (k.id === keyId ? { ...k, status: newStatus } : k)));
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "Failed to update API key");
    }
  };

  const handleDeleteApiKey = async (keyId: number) => {
    setApiKeyError(null);
    try {
      const res = await fetch(`/api/api-keys/${keyId}`, {
        method: "DELETE",
        headers: authHeaders(sessionToken),
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, "Failed to delete API key"));
      }
      setApiKeys((prev) => prev.filter((k) => k.id !== keyId));
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : "Failed to delete API key");
    }
  };

  /* --- Change password -------------------------------------------- */

  const handleChangePassword = async (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setPwdError(null);
    setPwdSuccess(null);
    if (newPwd !== confirmPwd) { setPwdError("New passwords do not match"); return; }
    if (newPwd.length < 6) { setPwdError("Password must be at least 6 characters"); return; }
    setPwdSubmitting(true);
    try {
      const res = await fetch("/api/user/password", {
        method: "PUT",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ old_password: oldPwd, new_password: newPwd }),
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, "Failed to change password"));
      }
      setPwdSuccess("Password changed successfully");
      setOldPwd("");
      setNewPwd("");
      setConfirmPwd("");
    } catch (err) {
      setPwdError(err instanceof Error ? err.message : "Failed to change password");
    } finally {
      setPwdSubmitting(false);
    }
  };

  /* --- Subscription & usage ---------------------------------------- */

  const loadSubscription = useCallback(async () => {
    if (!sessionToken) { setSubscription(null); setUsage(null); return; }
    setSubLoading(true);
    setSubError(null);
    try {
      const [subRes, usageRes] = await Promise.all([
        fetch("/api/subscriptions/summary", { headers: authHeaders(sessionToken), cache: "no-store" }),
        fetch("/api/dashboard/home", { headers: authHeaders(sessionToken), cache: "no-store" }),
      ]);
      if (subRes.ok) {
        const subData = await subRes.json();
        setSubscription(parseSubscriptionSummary(subData));
      }

      if (usageRes.ok) {
        const usagePayload = await usageRes.json();
        setUsage(parseUsageStats(usagePayload));
      }
    } catch {
      setSubError("Failed to load subscription data");
    } finally {
      setSubLoading(false);
    }
  }, [sessionToken]);

  useEffect(() => {
    if (isReady && sessionToken) void loadSubscription();
  }, [isReady, sessionToken, loadSubscription]);

  /* --- Logout -------------------------------------------------- */

  const handleLogout = () => {
    localStorage.removeItem(SESSION_TOKEN_KEY);
    setSessionToken("");
    setProfile(null);
    setApiKeys([]);
    setSubscription(null);
    setUsage(null);
    router.replace("/login");
  };

  /* --- Render --------------------------------------------------- */

  if (!sessionToken) {
    return (
      <section className="portal-shell flex items-center justify-center" style={{ minHeight: "60vh" }}>
        <div className="clay-panel p-10 text-center space-y-4">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-[var(--portal-accent)]/10">
            <MaterialIcon name="lock" size={28} className="text-[var(--portal-accent)]" />
          </div>
          <p className="text-sm text-[var(--portal-muted)]">Please log in to manage your account.</p>
          <button type="button" onClick={() => router.replace("/login")} className="btn-primary">
            Go to Login
          </button>
        </div>
      </section>
    );
  }

  if (pageLoading) {
    return (
      <section className="portal-shell flex items-center justify-center" style={{ minHeight: "60vh" }}>
        <div className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
          <span className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--portal-accent)]/30 border-t-[var(--portal-accent)]" />
          Loading...
        </div>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-5 py-8">
      {/* ── Header ── */}
      <div className="clay-panel space-y-1.5 p-5">
        <h2 className="section-title"><span className="gradient-text">Account</span></h2>
        <p className="section-subtitle">Manage your profile, API keys, subscription, and security settings.</p>
      </div>

      {/* ── Profile ── */}
      <div className="block-card">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
          {/* Avatar */}
          <div className="flex flex-col items-center gap-2">
            <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl text-2xl font-bold text-white" style={{ background: "var(--portal-gradient)" }}>
              {(profile?.email ?? "?")[0].toUpperCase()}
            </div>
            <span className="text-[10px] font-bold uppercase tracking-widest text-[var(--portal-accent)]">
              {profile?.role ?? "user"}
            </span>
          </div>
          {/* Info grid */}
          <div className="flex-1">
            <h3 className="mb-3 text-base font-bold text-[var(--portal-ink)]">Profile</h3>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <MetricBox label="Name" value={profile?.name || "\u2014"} />
              <MetricBox label="Email" value={profile?.email || "\u2014"} />
              <MetricBox
                label="Balance"
                value={typeof profile?.balance === "number" ? `$${profile.balance.toFixed(2)}` : "\u2014"}
                highlight
              />
            </div>
          </div>
        </div>
      </div>

      {/* ── API Keys ── */}
      <div className="block-card space-y-5">
        <h3 className="text-base font-bold text-[var(--portal-ink)]">
          <span className="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-lg text-xs" style={{ background: "var(--portal-gradient)" }}>
            <MaterialIcon name="vpn_key" size={14} className="text-white" />
          </span>
          API Keys
          {!apiKeyLoading && keyPagination.total > 0 && (
            <span className="ml-2 text-sm font-normal text-[var(--portal-muted)]">({keyPagination.total})</span>
          )}
        </h3>

        {/* Create form */}
        <form className="clay-panel flex flex-col gap-3 p-4 sm:flex-row sm:items-end" onSubmit={handleCreateApiKey}>
          <div className="min-w-[160px] flex-1 space-y-1.5">
            <label htmlFor="ak-name" className="text-xs font-semibold uppercase tracking-wider text-[var(--portal-muted)]">Name</label>
            <input id="ak-name" className="field" type="text" placeholder="e.g. Production" value={keyName} onChange={(e) => setKeyName(e.target.value)} />
          </div>
          {groups.length > 0 && (
            <div className="min-w-[180px] space-y-1.5">
              <label htmlFor="ak-group" className="text-xs font-semibold uppercase tracking-wider text-[var(--portal-muted)]">Group</label>
              <select
                id="ak-group"
                className="field"
                value={keyGroupId}
                onChange={(e) => setKeyGroupId(e.target.value ? Number(e.target.value) : "")}
              >
                <option value="">Auto select</option>
                {groups.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name}{g.platform ? ` (${g.platform})` : ""}
                  </option>
                ))}
              </select>
            </div>
          )}
          <button type="submit" className="btn-primary whitespace-nowrap">
            <MaterialIcon name="add" size={16} className="mr-1" />
            Create Key
          </button>
        </form>

        {/* Newly created key alert */}
        {newlyCreatedKey && (
          <div className="relative overflow-hidden rounded-xl border border-[var(--portal-accent)]/20 p-4" style={{ background: "linear-gradient(135deg, rgba(16,185,129,0.06), rgba(16,185,129,0.02))" }}>
            <div className="absolute -right-4 -top-4 h-24 w-24 rounded-full" style={{ background: "radial-gradient(circle, rgba(16,185,129,0.1), transparent 70%)" }} />
            <p className="relative mb-2 flex items-center gap-2 text-sm font-bold text-[var(--portal-accent)]">
              <MaterialIcon name="check_circle" size={16} />
              New API Key Created
            </p>
            <p className="relative mb-2 text-xs text-[var(--portal-muted)]">Save it now — this is the only time the key will be shown.</p>
            <div className="relative flex items-center gap-2">
              <code className="flex-1 break-all rounded-lg bg-[var(--portal-ink)]/5 px-3 py-2 font-mono text-xs text-[var(--portal-ink)]">{newlyCreatedKey}</code>
              <button
                type="button"
                className="btn-ghost shrink-0 rounded-lg border border-[var(--portal-line)] px-3 py-2 text-xs font-medium"
                onClick={() => { navigator.clipboard.writeText(newlyCreatedKey); }}
              >
                <MaterialIcon name="content_copy" size={14} />
              </button>
            </div>
          </div>
        )}

        {apiKeyError && <p className="rounded-lg bg-red-500/5 px-4 py-2.5 text-sm text-red-500">{apiKeyError}</p>}

        {/* Key list */}
        {apiKeyLoading ? (
          <div className="flex items-center gap-2 py-6 text-sm text-[var(--portal-muted)]">
            <span className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--portal-accent)]/30 border-t-[var(--portal-accent)]" />
            Loading API keys...
          </div>
        ) : apiKeys.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[var(--portal-accent)]/5">
              <MaterialIcon name="key_off" size={22} className="text-[var(--portal-muted)]" />
            </div>
            <p className="text-sm text-[var(--portal-muted)]">No API keys yet. Create one above to get started.</p>
          </div>
        ) : (
          <>
            <ul className="space-y-2">
              {apiKeys.map((key) => (
                <li key={key.id} className="group flex items-center justify-between gap-3 rounded-xl border border-[var(--portal-line)] px-4 py-3 transition-colors hover:border-[var(--portal-accent)]/30" style={{ background: "var(--portal-clay-strong, var(--portal-clay))" }}>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate text-sm font-semibold text-[var(--portal-ink)]">{key.name || `Key #${key.id}`}</p>
                      <span className={`shrink-0 inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${
                        key.status === "active"
                          ? "bg-emerald-500/10 text-emerald-500"
                          : "bg-red-500/10 text-red-500"
                      }`}>
                        <span className={`inline-block h-1.5 w-1.5 rounded-full ${key.status === "active" ? "bg-emerald-500" : "bg-red-500"}`} />
                        {key.status}
                      </span>
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[11px] text-[var(--portal-muted)]">
                      <span className="flex items-center gap-1">
                        <MaterialIcon name="group" size={12} />
                        {key.group_name}
                      </span>
                      <span>ID: {key.id}</span>
                      <span>Created: {key.created_at?.split("T")[0] ?? "\u2014"}</span>
                      {key.quota > 0 && (
                        <span className="font-mono">${key.quota_used.toFixed(2)} / ${key.quota.toFixed(2)}</span>
                      )}
                      {key.expires_at && <span>Expires: {key.expires_at.split("T")[0]}</span>}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      type="button"
                      onClick={() => void handleToggleApiKey(key.id, key.status)}
                      className="rounded-lg border border-[var(--portal-line)] px-2.5 py-1.5 text-xs transition-colors hover:border-[var(--portal-accent)]/40 hover:text-[var(--portal-accent)]"
                      title={key.status === "active" ? "Disable key" : "Enable key"}
                    >
                      <MaterialIcon name={key.status === "active" ? "toggle_on" : "toggle_off"} size={16} className={key.status === "active" ? "text-emerald-500" : "text-red-400"} />
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDeleteApiKey(key.id)}
                      className="rounded-lg border border-[var(--portal-line)] px-2.5 py-1.5 text-xs text-red-500 transition-colors hover:border-red-500/40 hover:bg-red-500/5"
                      title="Delete key"
                    >
                      <MaterialIcon name="delete_outline" size={16} />
                    </button>
                  </div>
                </li>
              ))}
            </ul>

            {/* Pagination */}
            {keyPagination.total_pages > 1 && (
              <div className="flex items-center justify-center gap-2 pt-2">
                <button
                  type="button"
                  disabled={!keyPagination.has_prev || apiKeyLoading}
                  onClick={() => void loadApiKeys(keyPagination.page - 1)}
                  className="btn-ghost rounded-lg border border-[var(--portal-line)] px-4 py-1.5 text-xs disabled:opacity-40"
                >
                  <MaterialIcon name="chevron_left" size={14} />
                  Prev
                </button>
                <span className="px-2 text-xs text-[var(--portal-muted)]">
                  {keyPagination.page} / {keyPagination.total_pages}
                </span>
                <button
                  type="button"
                  disabled={!keyPagination.has_next || apiKeyLoading}
                  onClick={() => void loadApiKeys(keyPagination.page + 1)}
                  className="btn-ghost rounded-lg border border-[var(--portal-line)] px-4 py-1.5 text-xs disabled:opacity-40"
                >
                  Next
                  <MaterialIcon name="chevron_right" size={14} />
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* ── Subscription & Usage ── */}
      <div className="block-card space-y-5">
        <h3 className="text-base font-bold text-[var(--portal-ink)]">
          <span className="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-lg text-xs" style={{ background: "var(--portal-gradient)" }}>
            <MaterialIcon name="card_membership" size={14} className="text-white" />
          </span>
          Subscription &amp; Usage
        </h3>

        {subLoading ? (
          <div className="flex items-center gap-2 py-6 text-sm text-[var(--portal-muted)]">
            <span className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--portal-accent)]/30 border-t-[var(--portal-accent)]" />
            Loading...
          </div>
        ) : subError ? (
          <p className="rounded-lg bg-red-500/5 px-4 py-2.5 text-sm text-red-500">{subError}</p>
        ) : !subscription ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[var(--portal-accent)]/5">
              <MaterialIcon name="subscription_off" size={22} className="text-[var(--portal-muted)]" />
            </div>
            <p className="text-sm text-[var(--portal-muted)]">No active subscription found.</p>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Subscription cards */}
            {subscription.subscriptions.map((sub) => (
              <div key={sub.id} className="clay-panel space-y-4 p-5">
                {/* Card header */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--portal-accent)]/10">
                      <MaterialIcon name="workspace_premium" size={16} className="text-[var(--portal-accent)]" />
                    </div>
                    <div>
                      <p className="text-sm font-bold text-[var(--portal-ink)]">{sub.group_name}</p>
                      {sub.expires_at && (
                        <p className="text-[11px] text-[var(--portal-muted)]">Expires {sub.expires_at.split("T")[0]}</p>
                      )}
                    </div>
                  </div>
                  <span className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider ${
                    sub.status === "active"
                      ? "bg-emerald-500/10 text-emerald-500"
                      : "bg-red-500/10 text-red-500"
                  }`}>
                    <span className={`h-1.5 w-1.5 rounded-full ${sub.status === "active" ? "bg-emerald-500" : "bg-red-500"}`} />
                    {sub.status}
                  </span>
                </div>

                {/* Progress bars */}
                <div className="space-y-3 rounded-lg border border-[var(--portal-line)] bg-[var(--portal-ink)]/[0.02] p-4">
                  <ProgressRow label="Daily" used={sub.daily_used_usd} limit={sub.daily_limit_usd} />
                  <ProgressRow label="Weekly" used={sub.weekly_used_usd} limit={sub.weekly_limit_usd} />
                  <ProgressRow label="Monthly" used={sub.monthly_used_usd} limit={sub.monthly_limit_usd} />
                </div>
              </div>
            ))}

            {/* Usage stats grid */}
            {usage && (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <StatCard icon="bolt" label="Today Requests" value={usage.today_requests.toLocaleString()} />
                <StatCard icon="token" label="Today Tokens" value={usage.today_tokens.toLocaleString()} />
                <StatCard icon="payments" label="Today Cost" value={`$${usage.today_cost.toFixed(4)}`} />
                <StatCard icon="data_usage" label="Total Tokens" value={usage.total_tokens.toLocaleString()} />
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── Security ── */}
      <div className="block-card space-y-5">
        <h3 className="text-base font-bold text-[var(--portal-ink)]">
          <span className="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-lg text-xs" style={{ background: "var(--portal-gradient)" }}>
            <MaterialIcon name="shield" size={14} className="text-white" />
          </span>
          Security
        </h3>

        <form className="clay-panel space-y-4 p-5" onSubmit={handleChangePassword}>
          <p className="text-sm font-semibold text-[var(--portal-ink)]">Change Password</p>
          <div>
            <label htmlFor="old-pwd" className="text-xs font-semibold uppercase tracking-wider text-[var(--portal-muted)]">Current Password</label>
            <input id="old-pwd" className="field mt-1.5" type="password" value={oldPwd} onChange={(e) => setOldPwd(e.target.value)} placeholder="Enter current password" required />
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label htmlFor="new-pwd" className="text-xs font-semibold uppercase tracking-wider text-[var(--portal-muted)]">New Password</label>
              <input id="new-pwd" className="field mt-1.5" type="password" value={newPwd} onChange={(e) => setNewPwd(e.target.value)} placeholder="Min. 6 characters" required />
            </div>
            <div>
              <label htmlFor="confirm-pwd" className="text-xs font-semibold uppercase tracking-wider text-[var(--portal-muted)]">Confirm New Password</label>
              <input id="confirm-pwd" className="field mt-1.5" type="password" value={confirmPwd} onChange={(e) => setConfirmPwd(e.target.value)} placeholder="Re-enter new password" required />
            </div>
          </div>
          {pwdError && <p className="rounded-lg bg-red-500/5 px-4 py-2.5 text-sm text-red-500">{pwdError}</p>}
          {pwdSuccess && (
            <p className="flex items-center gap-2 rounded-lg bg-emerald-500/5 px-4 py-2.5 text-sm text-emerald-500">
              <MaterialIcon name="check_circle" size={14} />
              {pwdSuccess}
            </p>
          )}
          <button type="submit" disabled={pwdSubmitting} className="btn-primary">
            {pwdSubmitting ? (
              <><span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/30 border-t-white" /> Updating...</>
            ) : (
              <><MaterialIcon name="lock_reset" size={16} className="mr-1" /> Update Password</>
            )}
          </button>
        </form>
      </div>

      {/* ── Footer actions ── */}
      <div className="flex items-center justify-between rounded-xl border border-[var(--portal-line)] px-5 py-3.5" style={{ background: "var(--portal-clay, transparent)" }}>
        <p className="text-xs text-[var(--portal-muted)]">
          Signed in as <span className="font-medium text-[var(--portal-ink)]">{profile?.email}</span>
        </p>
        <button
          type="button"
          onClick={handleLogout}
          className="flex items-center gap-1.5 rounded-lg border border-red-500/20 bg-red-500/5 px-4 py-2 text-xs font-medium text-red-500 transition-colors hover:bg-red-500/10"
        >
          <MaterialIcon name="logout" size={14} />
          Log out
        </button>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                       */
/* ------------------------------------------------------------------ */

function MetricBox({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="rounded-lg border border-[var(--portal-line)] px-3.5 py-2.5" style={{ background: highlight ? "linear-gradient(135deg, rgba(16,185,129,0.06), rgba(16,185,129,0.02))" : undefined }}>
      <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--portal-muted)]">{label}</p>
      <p className={`mt-0.5 text-sm font-bold ${highlight ? "text-[var(--portal-accent)]" : "text-[var(--portal-ink)]"}`}>
        {value}
      </p>
    </div>
  );
}

function StatCard({ icon, label, value }: { icon: string; label: string; value: string }) {
  return (
    <div className="clay-panel flex items-center gap-3 p-4">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--portal-accent)]/10">
        <MaterialIcon name={icon} size={18} className="text-[var(--portal-accent)]" />
      </div>
      <div className="min-w-0">
        <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--portal-muted)]">{label}</p>
        <p className="truncate text-lg font-bold text-[var(--portal-ink)]">{value}</p>
      </div>
    </div>
  );
}

function ProgressRow({ label, used, limit }: { label: string; used: number; limit: number }) {
  const hasLimit = limit > 0;
  const pct = hasLimit ? Math.min(100, (used / limit) * 100) : 0;
  const isOver = hasLimit && used > limit;

  return (
    <div className="flex items-center gap-3">
      <span className="w-14 shrink-0 text-[11px] font-semibold text-[var(--portal-muted)]">{label}</span>
      <div className="h-2.5 flex-1 overflow-hidden rounded-full bg-[var(--portal-line)]">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            isOver
              ? "bg-gradient-to-r from-red-500 to-red-400"
              : pct > 80
                ? "bg-gradient-to-r from-amber-500 to-amber-400"
                : "bg-gradient-to-r from-emerald-500 to-emerald-400"
          }`}
          style={{ width: `${hasLimit ? pct : 0}%` }}
        />
      </div>
      <span className={`shrink-0 text-[11px] font-mono font-semibold tabular-nums ${isOver ? "text-red-500" : "text-[var(--portal-ink)]"}`}>
        ${used.toFixed(2)}
        {hasLimit ? (
          <span className="text-[var(--portal-muted)]"> / ${limit.toFixed(2)}</span>
        ) : (
          <span className="font-normal text-[var(--portal-muted)]"> (no limit)</span>
        )}
      </span>
    </div>
  );
}

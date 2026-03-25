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
        <div className="block-card p-8 text-center">
          <MaterialIcon name="lock" size={40} className="mb-3 text-[var(--stitch-text-muted)]" />
          <p className="text-[var(--stitch-text-muted)]">Please log in to manage your account.</p>
          <button type="button" onClick={() => router.replace("/login")} className="btn-primary mt-4">
            Go to Login
          </button>
        </div>
      </section>
    );
  }

  if (pageLoading) {
    return (
      <section className="portal-shell flex items-center justify-center" style={{ minHeight: "60vh" }}>
        <p className="text-sm text-[var(--stitch-text-muted)]">Loading...</p>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-6">
      {/* Header */}
      <div className="clay-panel space-y-2 p-5">
        <h2 className="section-title"><span className="gradient-text">Account</span></h2>
        <p className="section-subtitle">
          Manage your profile, API keys, subscription, and security settings.
        </p>
      </div>

      {/* Profile overview */}
      <div className="block-card">
        <h3 className="mb-4 text-lg font-semibold text-emerald-500 dark:text-emerald-400">Profile</h3>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-[var(--stitch-text-muted)]">Name</p>
            <p className="mt-1 text-sm font-semibold text-[var(--stitch-text)]">{profile?.name || "—"}</p>
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-[var(--stitch-text-muted)]">Email</p>
            <p className="mt-1 text-sm font-semibold text-[var(--stitch-text)]">{profile?.email || "—"}</p>
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-[var(--stitch-text-muted)]">Role</p>
            <p className="mt-1 text-sm font-semibold text-[var(--stitch-text)]">{profile?.role || "—"}</p>
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-[var(--stitch-text-muted)]">Balance</p>
            <p className="mt-1 text-sm font-semibold text-[var(--stitch-text)]">
              ${typeof profile?.balance === "number" ? profile.balance.toFixed(2) : "—"}
            </p>
          </div>
        </div>
      </div>

      {/* API Keys */}
      <div className="block-card">
        <h3 className="mb-4 text-lg font-semibold text-emerald-500 dark:text-emerald-400">API Keys</h3>

        <form className="mb-4 flex flex-wrap items-end gap-3" onSubmit={handleCreateApiKey}>
          <div className="min-w-[160px] flex-1 space-y-1">
            <label htmlFor="ak-name" className="text-sm font-medium text-[var(--stitch-text)]">Name (optional)</label>
            <input id="ak-name" className="field" type="text" placeholder="e.g. Production" value={keyName} onChange={(e) => setKeyName(e.target.value)} />
          </div>
          {groups.length > 0 && (
            <div className="min-w-[180px] space-y-1">
              <label htmlFor="ak-group" className="text-sm font-medium text-[var(--stitch-text)]">Group</label>
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
          <button type="submit" className="btn-primary">Create Key</button>
        </form>

        {newlyCreatedKey && (
          <div className="mb-4 rounded-lg border border-[var(--stitch-primary)]/30 bg-[var(--stitch-primary)]/5 p-3 text-sm">
            <p className="mb-1 font-semibold text-[var(--stitch-primary)]">New API Key (save it now — shown only once)</p>
            <code className="block break-all rounded bg-[var(--stitch-bg)] px-2 py-1 font-mono text-xs">{newlyCreatedKey}</code>
          </div>
        )}

        {apiKeyError && <p className="mb-3 text-sm text-red-500">{apiKeyError}</p>}

        {apiKeyLoading ? (
          <p className="text-sm text-[var(--stitch-text-muted)]">Loading API keys...</p>
        ) : apiKeys.length === 0 ? (
          <p className="text-sm text-[var(--stitch-text-muted)]">No API keys yet. Create one above to get started.</p>
        ) : (
          <>
            <ul className="space-y-2">
              {apiKeys.map((key) => (
                <li key={key.id} className="flex items-center justify-between gap-3 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-semibold text-[var(--stitch-text)] truncate">{key.name || `Key #${key.id}`}</p>
                      <span className={`shrink-0 inline-flex rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${
                        key.status === "active"
                          ? "bg-emerald-500/10 text-emerald-500"
                          : "bg-red-500/10 text-red-500"
                      }`}>
                        {key.status}
                      </span>
                    </div>
                    <p className="mt-0.5 text-xs text-[var(--stitch-text-muted)]">
                      {key.group_name} · ID: {key.id} · Created: {key.created_at?.split("T")[0] ?? "—"}
                    </p>
                    {(key.quota > 0 || key.expires_at) && (
                      <p className="mt-0.5 text-xs text-[var(--stitch-text-muted)]">
                        {key.quota > 0 && <span>Quota: ${key.quota_used.toFixed(2)} / ${key.quota.toFixed(2)}</span>}
                        {key.quota > 0 && key.expires_at && " · "}
                        {key.expires_at && <span>Expires: {key.expires_at.split("T")[0]}</span>}
                      </p>
                    )}
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <button
                      type="button"
                      onClick={() => void handleToggleApiKey(key.id, key.status)}
                      className="btn-ghost px-3 py-1.5 text-xs"
                      title={key.status === "active" ? "Disable key" : "Enable key"}
                    >
                      <MaterialIcon name={key.status === "active" ? "visibility_off" : "visibility"} size={14} />
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDeleteApiKey(key.id)}
                      className="btn-ghost px-3 py-1.5 text-xs text-red-500 hover:text-red-400"
                      title="Delete key"
                    >
                      <MaterialIcon name="delete" size={14} />
                    </button>
                  </div>
                </li>
              ))}
            </ul>

            {/* Pagination */}
            {keyPagination.total_pages > 1 && (
              <div className="mt-4 flex items-center justify-center gap-2">
                <button
                  type="button"
                  disabled={!keyPagination.has_prev || apiKeyLoading}
                  onClick={() => void loadApiKeys(keyPagination.page - 1)}
                  className="btn-ghost px-3 py-1.5 text-xs disabled:opacity-40"
                >
                  Previous
                </button>
                <span className="text-xs text-[var(--stitch-text-muted)]">
                  Page {keyPagination.page} of {keyPagination.total_pages} ({keyPagination.total} keys)
                </span>
                <button
                  type="button"
                  disabled={!keyPagination.has_next || apiKeyLoading}
                  onClick={() => void loadApiKeys(keyPagination.page + 1)}
                  className="btn-ghost px-3 py-1.5 text-xs disabled:opacity-40"
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Subscription & Usage */}
      <div className="block-card">
        <h3 className="mb-4 text-lg font-semibold text-emerald-500 dark:text-emerald-400">Subscription & Usage</h3>

        {subLoading ? (
          <p className="text-sm text-[var(--stitch-text-muted)]">Loading...</p>
        ) : subError ? (
          <p className="text-sm text-red-500">{subError}</p>
        ) : !subscription ? (
          <p className="text-sm text-[var(--stitch-text-muted)]">No active subscription found.</p>
        ) : (
          <div className="space-y-5">
            {/* Summary */}
            <div className="flex items-center gap-3">
              <span className="inline-flex rounded-lg bg-[var(--stitch-primary)]/10 px-3 py-1 text-xs font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
                {subscription.active_count} Active Subscription{subscription.active_count !== 1 ? "s" : ""}
              </span>
              {subscription.total_used_usd > 0 && (
                <span className="text-xs text-[var(--stitch-text-muted)]">
                  Total Used: ${subscription.total_used_usd.toFixed(2)}
                </span>
              )}
            </div>

            {/* Subscription cards with progress bars */}
            {subscription.subscriptions.map((sub) => (
              <div key={sub.id} className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-semibold text-[var(--stitch-text)]">{sub.group_name}</p>
                  <span className={`text-[10px] font-bold uppercase tracking-wider ${
                    sub.status === "active" ? "text-emerald-500" : "text-red-500"
                  }`}>{sub.status}</span>
                </div>
                {sub.expires_at && (
                  <p className="text-xs text-[var(--stitch-text-muted)]">Expires: {sub.expires_at.split("T")[0]}</p>
                )}
                <div className="space-y-2">
                  <ProgressBar label="Daily" used={sub.daily_used_usd} limit={sub.daily_limit_usd} />
                  <ProgressBar label="Weekly" used={sub.weekly_used_usd} limit={sub.weekly_limit_usd} />
                  <ProgressBar label="Monthly" used={sub.monthly_used_usd} limit={sub.monthly_limit_usd} />
                </div>
              </div>
            ))}

            {/* Usage summary */}
            {usage && (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3 text-center">
                  <p className="text-xs uppercase tracking-wider text-[var(--stitch-text-muted)]">Today Requests</p>
                  <p className="mt-1 text-xl font-bold text-[var(--stitch-text)]">{usage.today_requests.toLocaleString()}</p>
                </div>
                <div className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3 text-center">
                  <p className="text-xs uppercase tracking-wider text-[var(--stitch-text-muted)]">Today Tokens</p>
                  <p className="mt-1 text-xl font-bold text-[var(--stitch-text)]">{usage.today_tokens.toLocaleString()}</p>
                </div>
                <div className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3 text-center">
                  <p className="text-xs uppercase tracking-wider text-[var(--stitch-text-muted)]">Today Cost</p>
                  <p className="mt-1 text-xl font-bold text-[var(--stitch-text)]">${usage.today_cost.toFixed(4)}</p>
                </div>
                <div className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3 text-center">
                  <p className="text-xs uppercase tracking-wider text-[var(--stitch-text-muted)]">Total Tokens</p>
                  <p className="mt-1 text-xl font-bold text-[var(--stitch-text)]">{usage.total_tokens.toLocaleString()}</p>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Change Password */}
      <div className="block-card">
        <h3 className="mb-4 text-lg font-semibold text-emerald-500 dark:text-emerald-400">Change Password</h3>
        <form className="space-y-3" onSubmit={handleChangePassword}>
          <div>
            <label htmlFor="old-pwd" className="text-sm font-medium text-[var(--stitch-text)]">Current Password</label>
            <input id="old-pwd" className="field" type="password" value={oldPwd} onChange={(e) => setOldPwd(e.target.value)} placeholder="Enter current password" required />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label htmlFor="new-pwd" className="text-sm font-medium text-[var(--stitch-text)]">New Password</label>
              <input id="new-pwd" className="field" type="password" value={newPwd} onChange={(e) => setNewPwd(e.target.value)} placeholder="Min. 6 characters" required />
            </div>
            <div>
              <label htmlFor="confirm-pwd" className="text-sm font-medium text-[var(--stitch-text)]">Confirm New Password</label>
              <input id="confirm-pwd" className="field" type="password" value={confirmPwd} onChange={(e) => setConfirmPwd(e.target.value)} placeholder="Re-enter new password" required />
            </div>
          </div>
          {pwdError && <p className="text-sm text-red-500">{pwdError}</p>}
          {pwdSuccess && <p className="text-sm text-emerald-500">{pwdSuccess}</p>}
          <button type="submit" disabled={pwdSubmitting} className="btn-primary w-fit">
            {pwdSubmitting ? "Changing..." : "Update Password"}
          </button>
        </form>
      </div>

      {/* Logout */}
      <div className="block-card">
        <button type="button" onClick={handleLogout} className="btn-ghost text-red-500 hover:text-red-400">
          <MaterialIcon name="logout" size={16} /> Log out
        </button>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                       */
/* ------------------------------------------------------------------ */

function ProgressBar({ label, used, limit }: { label: string; used: number; limit: number }) {
  const hasLimit = limit > 0;
  const pct = hasLimit ? Math.min(100, (used / limit) * 100) : 0;
  const isOver = hasLimit && used > limit;

  return (
    <div className="flex items-center gap-3">
      <span className="w-16 shrink-0 text-xs text-[var(--stitch-text-muted)]">{label}</span>
      <div className="flex-1">
        <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--stitch-border)]">
          <div
            className={`h-full rounded-full transition-all ${isOver ? "bg-red-500" : pct > 80 ? "bg-amber-500" : "bg-emerald-500"}`}
            style={{ width: `${hasLimit ? pct : 0}%` }}
          />
        </div>
      </div>
      <span className={`shrink-0 text-xs font-mono ${isOver ? "text-red-500" : "text-[var(--stitch-text)]"}`}>
        ${used.toFixed(2)}{hasLimit ? ` / $${limit.toFixed(2)}` : " (unlimited)"}
      </span>
    </div>
  );
}

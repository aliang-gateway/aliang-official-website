"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type AdminPackage = {
  code: string;
  name: string;
  price_micros: number;
  value_type: string;
  value_amount: number;
  is_enabled: boolean;
  is_published?: boolean;
};

type QuickCreateResult = {
  id: number;
  local_id?: number;
  email: string;
  name: string;
  password: string;
  created_at: string;
};

type AssignPackageResult = {
  payment_event_id: string;
  tier_code: string;
  fulfillment_job?: {
    id?: number;
    status?: string;
    error_message?: string | null;
    retry_count?: number;
  };
};

export default function AdminUsersPage() {
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);

  const [packages, setPackages] = useState<AdminPackage[]>([]);
  const [isLoadingPackages, setIsLoadingPackages] = useState(false);

  const [authBlocked, setAuthBlocked] = useState<string | null>(null);
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null);

  // Quick Create
  const [createEmail, setCreateEmail] = useState("");
  const [isCreating, setIsCreating] = useState(false);
  const [createResult, setCreateResult] = useState<QuickCreateResult | null>(null);

  // Assign Package
  const [assignUserId, setAssignUserId] = useState("");
  const [assignTierCode, setAssignTierCode] = useState("");
  const [assignPassword, setAssignPassword] = useState("");
  const [isAssigning, setIsAssigning] = useState(false);
  const [assignResult, setAssignResult] = useState<AssignPackageResult | null>(null);

  useEffect(() => {
    setIsHydrated(true);
    setSessionToken(localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "");
  }, []);

  const buildHeaders = useCallback(() => {
    const headers: Record<string, string> = {
      "content-type": "application/json",
      accept: "application/json",
    };
    if (sessionToken) {
      headers.Authorization = `Bearer ${sessionToken}`;
    }
    return headers;
  }, [sessionToken]);

  const handleAuthFailure = useCallback((status: number, message?: string) => {
    if (status === 401 || status === 403) {
      setAuthBlocked(message ?? "Unauthorized. Admin permission is required.");
      return true;
    }
    return false;
  }, []);

  const loadPackages = useCallback(async () => {
    if (!sessionToken) return;
    setIsLoadingPackages(true);
    try {
      const response = await fetch("/api/admin/packages", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload?.error ?? "Failed to load packages";
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setPackages(Array.isArray(payload?.packages) ? payload.packages : []);
    } catch {
      // ignore
    } finally {
      setIsLoadingPackages(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  useEffect(() => {
    if (isHydrated) void loadPackages();
  }, [isHydrated, loadPackages]);

  const handleQuickCreate = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError(null);
    setGlobalSuccess(null);
    setCreateResult(null);

    const email = createEmail.trim();
    if (!email) {
      setGlobalError("Email is required");
      return;
    }
    if (!sessionToken) {
      setGlobalError("Missing session token. Please login first.");
      return;
    }

    setIsCreating(true);
    try {
      const response = await fetch("/api/admin/users/quick-create", {
        method: "POST",
        headers: buildHeaders(),
        body: JSON.stringify({ email }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload?.error ?? "Failed to create user";
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setCreateResult(payload);
      setAssignUserId(String(payload.id));
      setGlobalSuccess(`Sub2API user #${payload.id} created`);
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : "Failed to create user");
    } finally {
      setIsCreating(false);
    }
  };

  const handleAssignPackage = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError(null);
    setGlobalSuccess(null);
    setAssignResult(null);

    const userId = parseInt(assignUserId.trim(), 10);
    const tierCode = assignTierCode.trim();
    const latestPassword = assignPassword.trim();
    if (!userId || userId <= 0) {
      setGlobalError("Valid user ID is required");
      return;
    }
    if (!tierCode) {
      setGlobalError("Please select a package");
      return;
    }
    if (!sessionToken) {
      setGlobalError("Missing session token. Please login first.");
      return;
    }

    setIsAssigning(true);
    try {
      const response = await fetch("/api/admin/users/assign-package", {
        method: "POST",
        headers: buildHeaders(),
        body: JSON.stringify({
          user_id: userId,
          tier_code: tierCode,
          ...(latestPassword ? { password: latestPassword } : {}),
        }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const jobMessage = payload?.fulfillment_job?.error_message;
        const message = jobMessage ?? payload?.error ?? "Failed to assign package";
        if (payload?.fulfillment_job) {
          setAssignResult(payload);
        }
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setAssignResult(payload);
      const status = payload?.fulfillment_job?.status ?? "unknown";
      setGlobalSuccess(`Package assigned. Fulfillment status: ${status}`);
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : "Failed to assign package");
    } finally {
      setIsAssigning(false);
    }
  };

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // ignore
    }
  };

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-3 p-5">
        <div className="space-y-2">
          <h1 className="section-title">
            <span className="gradient-text">User Management</span>
          </h1>
          <p className="section-subtitle">Quick-create users and assign packages in one click.</p>
        </div>
      </div>

      {/* Session & Alerts */}
      <div className="block-card space-y-3">
        <p className="text-sm text-[var(--portal-muted)]">
          Session token: {isHydrated && sessionToken ? "Loaded from localStorage" : "Not found"}
        </p>
        {authBlocked ? (
          <div className="rounded-xl border border-red-400/40 bg-red-500/10 p-3 text-sm text-red-700 dark:border-red-400/60 dark:bg-red-500/20 dark:text-red-300" role="alert">
            {authBlocked}
          </div>
        ) : null}
        {globalSuccess ? (
          <div className="rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-700 dark:border-emerald-400/60 dark:bg-emerald-500/20 dark:text-emerald-300" aria-live="polite">
            {globalSuccess}
          </div>
        ) : null}
        {globalError ? (
          <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
            {globalError}
          </div>
        ) : null}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Quick Create User */}
        <div className="block-card space-y-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Quick Create User</h2>
          <form className="grid gap-4" onSubmit={handleQuickCreate}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Email</span>
              <input
                className="field"
                type="email"
                placeholder="user@example.com"
                value={createEmail}
                onChange={(e) => setCreateEmail(e.target.value)}
                disabled={isBlocked || isCreating}
                required
              />
            </label>
            <button className="btn-primary" type="submit" disabled={isBlocked || isCreating}>
              {isCreating ? "Creating..." : "Create User"}
            </button>
          </form>

          {createResult ? (
            <div className="space-y-2 rounded-xl border border-emerald-400/40 bg-emerald-500/5 p-4">
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">User Created</h3>
                <button
                  type="button"
                  className="btn-ghost px-3 py-1 text-xs"
                  onClick={() => {
                    const text = [
                      `Sub2API ID: ${createResult.id}`,
                      createResult.local_id ? `Local ID: ${createResult.local_id}` : "",
                      `邮箱: ${createResult.email}`,
                      `密码: ${createResult.password}`,
                      `姓名: ${createResult.name}`,
                    ].filter(Boolean).join("\n");
                    void handleCopy(text);
                  }}
                >
                  Copy All
                </button>
              </div>
              <div className="grid gap-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Sub2API ID:</span>
                  <span className="font-mono text-[var(--portal-ink)]">{createResult.id}</span>
                </div>
                {createResult.local_id ? (
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[var(--portal-muted)]">Local ID:</span>
                    <span className="font-mono text-[var(--portal-muted)]">{createResult.local_id}</span>
                  </div>
                ) : null}
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Email:</span>
                  <span className="text-[var(--portal-ink)]">{createResult.email}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Name:</span>
                  <span className="text-[var(--portal-ink)]">{createResult.name}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Password:</span>
                  <div className="flex items-center gap-2">
                    <code className="rounded bg-[var(--portal-clay-strong)] px-2 py-0.5 font-mono text-xs text-[var(--portal-ink)]">
                      {createResult.password}
                    </code>
                    <button
                      type="button"
                      className="btn-ghost px-2 py-0.5 text-xs"
                      onClick={() => void handleCopy(createResult.password)}
                    >
                      Copy
                    </button>
                  </div>
                </div>
              </div>
            </div>
          ) : null}
        </div>

        {/* Assign Package */}
        <div className="block-card space-y-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Assign Package</h2>
          <form className="grid gap-4" onSubmit={handleAssignPackage}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Sub2API User ID</span>
              <input
                className="field font-mono"
                type="number"
                min="1"
                placeholder="Sub2API user ID from creation result"
                value={assignUserId}
                onChange={(e) => setAssignUserId(e.target.value)}
                disabled={isBlocked || isAssigning}
                required
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Package</span>
              <select
                className="field"
                value={assignTierCode}
                onChange={(e) => setAssignTierCode(e.target.value)}
                disabled={isBlocked || isAssigning || isLoadingPackages}
                required
              >
                <option value="">Select a package...</option>
	                {packages
	                  .filter((p) => (p.is_published ?? p.is_enabled) !== false)
	                  .map((p) => (
                    <option key={p.code} value={p.code}>
                      {p.name} ({p.code}) — {p.price_micros > 0 ? `¥${(p.price_micros / 1000000).toFixed(2)}` : "Free"}
                    </option>
	                  ))}
	              </select>
	            </label>
	            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
	              <span>Latest password (optional)</span>
	              <input
	                className="field"
	                type="password"
	                placeholder="Refresh Sub2API token before assigning"
	                value={assignPassword}
	                onChange={(e) => setAssignPassword(e.target.value)}
	                disabled={isBlocked || isAssigning}
	                autoComplete="new-password"
	              />
	            </label>
	            <button className="btn-primary" type="submit" disabled={isBlocked || isAssigning}>
              {isAssigning ? "Assigning..." : "Assign Package"}
            </button>
          </form>

          {assignResult ? (
            <div className="space-y-2 rounded-xl border border-emerald-400/40 bg-emerald-500/5 p-4">
              <h3 className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">Package Assigned</h3>
              <div className="grid gap-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Payment Event:</span>
                  <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.payment_event_id}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Tier Code:</span>
                  <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.tier_code}</span>
                </div>
                {assignResult.fulfillment_job ? (
                  <>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-[var(--portal-muted)]">Job ID:</span>
                      <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.fulfillment_job.id}</span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-[var(--portal-muted)]">Status:</span>
                      <span
                        className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${
                          assignResult.fulfillment_job.status === "fulfilled"
                            ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                            : assignResult.fulfillment_job.status?.includes("failed")
                              ? "bg-red-500/10 text-red-700 dark:text-red-300"
                              : "bg-amber-500/10 text-amber-700 dark:text-amber-300"
                        }`}
                      >
                        {assignResult.fulfillment_job.status}
                      </span>
                    </div>
                    {assignResult.fulfillment_job.error_message ? (
                      <div className="text-xs text-red-600 dark:text-red-400">
                        Error: {assignResult.fulfillment_job.error_message}
                      </div>
                    ) : null}
                  </>
                ) : null}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

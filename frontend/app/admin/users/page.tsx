"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslations } from "next-intl";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type AdminPackage = {
  code: string;
  name: string;
  level?: "admin" | "distributor";
  price_micros: number;
  value_type: string;
  value_amount: number;
  is_enabled: boolean;
  is_published?: boolean;
};

type QuickCreateResult = {
  id: number;
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

type UpdateRoleResult = {
  id: number;
  sub2api_user_id?: number;
  email: string;
  name: string;
  role: "user" | "admin" | "distributor";
  updated_at: string;
};

type BindDistributorResult = {
  id: number;
  distributor_user_id: number;
  user_id: number;
  email: string;
  name: string;
  source: string;
  created_at?: string;
};

type DistributorBinding = {
  id: number;
  distributor_user_id: number;
  distributor_email?: string;
  distributor_name?: string;
  user_id: number;
  email: string;
  name: string;
  source: string;
  created_at: string;
  updated_at?: string;
};

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

export default function AdminUsersPage() {
  const t = useTranslations("adminUsers");
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);
  const [currentRole, setCurrentRole] = useState<"user" | "admin" | "distributor" | "">("");

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
  const [assignEmail, setAssignEmail] = useState("");
  const [assignTierCode, setAssignTierCode] = useState("");
  const [assignPassword, setAssignPassword] = useState("");
  const [isAssigning, setIsAssigning] = useState(false);
  const [assignResult, setAssignResult] = useState<AssignPackageResult | null>(null);

  // Local Role
  const [roleUserId, setRoleUserId] = useState("");
  const [roleEmail, setRoleEmail] = useState("");
  const [roleValue, setRoleValue] = useState<"user" | "admin" | "distributor">("user");
  const [isUpdatingRole, setIsUpdatingRole] = useState(false);
  const [roleResult, setRoleResult] = useState<UpdateRoleResult | null>(null);

  const [bindDistributorEmail, setBindDistributorEmail] = useState("");
  const [bindUserEmail, setBindUserEmail] = useState("");
  const [isBindingDistributor, setIsBindingDistributor] = useState(false);
  const [bindResult, setBindResult] = useState<BindDistributorResult | null>(null);
  const [distributorBindings, setDistributorBindings] = useState<DistributorBinding[]>([]);
  const [isLoadingDistributorBindings, setIsLoadingDistributorBindings] = useState(false);

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
      setAuthBlocked(message ?? t("unauthorized"));
      return true;
    }
    return false;
  }, [t]);

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
        const message = payload?.error ?? t("loadPackagesFailed");
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
  }, [buildHeaders, handleAuthFailure, sessionToken, t]);

  const loadDistributorBindings = useCallback(async () => {
    if (!sessionToken) return;
    setIsLoadingDistributorBindings(true);
    try {
      const response = await fetch("/api/admin/distributor/users", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload?.error ?? t("loadDistributorBindingsFailed");
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setDistributorBindings(Array.isArray(payload?.invitations) ? payload.invitations : []);
    } catch {
      // keep the rest of the admin page usable if this side panel cannot load
    } finally {
      setIsLoadingDistributorBindings(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken, t]);

  const loadCurrentRole = useCallback(async () => {
    if (!sessionToken) return;
    try {
      const response = await fetch("/api/auth/me", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });
      const payload = await response.json();
      if (!response.ok) return;
      const role = payload?.data?.role ?? payload?.role;
      if (role === "admin" || role === "distributor" || role === "user") {
        setCurrentRole(role);
      }
    } catch {
      // ignore
    }
  }, [buildHeaders, sessionToken]);

  useEffect(() => {
    if (isHydrated) void Promise.all([loadPackages(), loadCurrentRole()]);
  }, [isHydrated, loadCurrentRole, loadPackages]);

  useEffect(() => {
    if (isHydrated && currentRole === "admin") void loadDistributorBindings();
  }, [currentRole, isHydrated, loadDistributorBindings]);

  const handleQuickCreate = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError(null);
    setGlobalSuccess(null);
    setCreateResult(null);

    const email = createEmail.trim();
    if (!email) {
      setGlobalError(t("emailRequired"));
      return;
    }
    if (!sessionToken) {
      setGlobalError(t("missingSession"));
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
        const message = payload?.error ?? t("createUserFailed");
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setCreateResult(payload);
      setAssignUserId(String(payload.id));
      setAssignEmail(payload.email);
      setRoleUserId(String(payload.id));
      setRoleEmail(payload.email);
      setGlobalSuccess(t("createdSuccess", { id: payload.id }));
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : t("createUserFailed"));
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
    const email = assignEmail.trim();
    const tierCode = assignTierCode.trim();
    const latestPassword = assignPassword.trim();
    if ((!userId || userId <= 0) && !email) {
      setGlobalError(t("userIdOrEmailRequired"));
      return;
    }
    if (!tierCode) {
      setGlobalError(t("packageRequired"));
      return;
    }
    if (!sessionToken) {
      setGlobalError(t("missingSession"));
      return;
    }

    setIsAssigning(true);
    try {
      const response = await fetch("/api/admin/users/assign-package", {
        method: "POST",
        headers: buildHeaders(),
        body: JSON.stringify({
          ...(userId > 0 ? { user_id: userId } : {}),
          ...(email ? { email } : {}),
          tier_code: tierCode,
          ...(latestPassword ? { password: latestPassword } : {}),
        }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const jobMessage = payload?.fulfillment_job?.error_message;
        const message = jobMessage ?? payload?.error ?? t("assignPackageFailed");
        if (payload?.fulfillment_job) {
          setAssignResult(payload);
        }
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setAssignResult(payload);
      const status = payload?.fulfillment_job?.status ?? "unknown";
      setGlobalSuccess(t("assignedSuccess", { status }));
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : t("assignPackageFailed"));
    } finally {
      setIsAssigning(false);
    }
  };

  const handleUpdateRole = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError(null);
    setGlobalSuccess(null);
    setRoleResult(null);

    const userId = parseInt(roleUserId.trim(), 10);
    const email = roleEmail.trim();
    if ((!userId || userId <= 0) && !email) {
      setGlobalError(t("roleUserIdOrEmailRequired"));
      return;
    }
    if (!sessionToken) {
      setGlobalError(t("missingSession"));
      return;
    }

    setIsUpdatingRole(true);
    try {
      const response = await fetch("/api/admin/users/role", {
        method: "PUT",
        headers: buildHeaders(),
        body: JSON.stringify({
          ...(userId > 0 ? { user_id: userId } : {}),
          ...(email ? { email } : {}),
          role: roleValue,
        }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload?.error ?? t("updateRoleFailed");
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setRoleResult(payload);
      setGlobalSuccess(t("roleUpdatedSuccess", { role: payload.role }));
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : t("updateRoleFailed"));
    } finally {
      setIsUpdatingRole(false);
    }
  };

  const handleBindDistributor = async (e: FormEvent) => {
    e.preventDefault();
    setGlobalError(null);
    setGlobalSuccess(null);
    setBindResult(null);

    const distributorEmail = bindDistributorEmail.trim();
    const userEmail = bindUserEmail.trim();
    if (!distributorEmail || !userEmail) {
      setGlobalError(t("distributorEmailsRequired"));
      return;
    }
    if (!sessionToken) {
      setGlobalError(t("missingSession"));
      return;
    }

    setIsBindingDistributor(true);
    try {
      const response = await fetch("/api/admin/distributor/users", {
        method: "POST",
        headers: buildHeaders(),
        body: JSON.stringify({
          distributor_email: distributorEmail,
          email: userEmail,
          source: "manual",
        }),
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload?.error ?? t("bindDistributorFailed");
        if (handleAuthFailure(response.status, message)) return;
        throw new Error(message);
      }
      setAuthBlocked(null);
      setBindResult(payload);
      setGlobalSuccess(t("bindSuccess", { email: payload.email, id: payload.distributor_user_id }));
      void loadDistributorBindings();
    } catch (err) {
      setGlobalError(err instanceof Error ? err.message : t("bindDistributorFailed"));
    } finally {
      setIsBindingDistributor(false);
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
            <span className="gradient-text">{t("title")}</span>
          </h1>
          <p className="section-subtitle">{t("subtitle")}</p>
        </div>
      </div>

      {/* Session & Alerts */}
      <div className="block-card space-y-3">
        <p className="text-sm text-[var(--portal-muted)]">
          {t("sessionToken")}{isHydrated && sessionToken ? t("tokenLoaded") : t("tokenMissing")}
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

      {currentRole === "admin" ? (
        <div className="block-card space-y-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("roleOverlay")}</h2>
          <form className="grid gap-4 md:grid-cols-[1fr_1fr_180px_auto] md:items-end" onSubmit={handleUpdateRole}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("sub2apiUserId")}</span>
              <input
                className="field font-mono"
                type="number"
                min="1"
                placeholder={t("upstreamUserId")}
                value={roleUserId}
                onChange={(e) => setRoleUserId(e.target.value)}
                disabled={isBlocked || isUpdatingRole}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("email")}</span>
              <input
                className="field"
                type="email"
                placeholder={t("importedEmailOnly")}
                value={roleEmail}
                onChange={(e) => setRoleEmail(e.target.value)}
                disabled={isBlocked || isUpdatingRole}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("role")}</span>
              <select
                className="field"
                value={roleValue}
                onChange={(e) => {
                  const value = e.target.value;
                  setRoleValue(value === "admin" || value === "distributor" ? value : "user");
                }}
                disabled={isBlocked || isUpdatingRole}
              >
                <option value="user">{t("userRole")}</option>
                <option value="distributor">{t("distributorRole")}</option>
                <option value="admin">{t("adminRole")}</option>
              </select>
            </label>
            <button className="btn-primary" type="submit" disabled={isBlocked || isUpdatingRole}>
              {isUpdatingRole ? t("updating") : t("updateRole")}
            </button>
          </form>
          {roleResult ? (
            <p className="text-sm text-[var(--portal-muted)]">
              {t.rich("roleResult", {
                email: roleResult.email,
                role: roleResult.role,
                strong: (chunks) => <span className="font-semibold text-[var(--portal-ink)]">{chunks}</span>,
                suffix: roleResult.sub2api_user_id ? t("forSub2api", { id: roleResult.sub2api_user_id }) : "",
              })}
            </p>
          ) : null}
        </div>
      ) : null}

      {currentRole === "admin" ? (
        <div className="block-card space-y-4">
          <div>
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("bindDistributorUser")}</h2>
            <p className="mt-1 text-sm text-[var(--portal-muted)]">{t("bindDistributorDescription")}</p>
          </div>
          <form className="grid gap-4 md:grid-cols-[1fr_1fr_auto] md:items-end" onSubmit={handleBindDistributor}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("distributorEmail")}</span>
              <input
                className="field"
                type="email"
                placeholder="distributor@example.com"
                value={bindDistributorEmail}
                onChange={(e) => setBindDistributorEmail(e.target.value)}
                disabled={isBlocked || isBindingDistributor}
                required
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("userEmail")}</span>
              <input
                className="field"
                type="email"
                placeholder="user@example.com"
                value={bindUserEmail}
                onChange={(e) => setBindUserEmail(e.target.value)}
                disabled={isBlocked || isBindingDistributor}
                required
              />
            </label>
            <button className="btn-primary" type="submit" disabled={isBlocked || isBindingDistributor}>
              {isBindingDistributor ? t("binding") : t("bindUser")}
            </button>
          </form>
          {bindResult ? (
            <p className="text-sm text-[var(--portal-muted)]">
              {t("bindResult", { email: bindResult.email, id: bindResult.distributor_user_id })}
            </p>
          ) : null}
        </div>
      ) : null}

      {currentRole === "admin" ? (
        <div className="block-card overflow-hidden">
          <div className="border-b border-[var(--portal-line)] p-4">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("distributorBindingRecords")}</h2>
            <p className="mt-1 text-sm text-[var(--portal-muted)]">{t("distributorBindingRecordsDescription")}</p>
          </div>
          {isLoadingDistributorBindings ? (
            <p className="p-4 text-sm text-[var(--portal-muted)]">{t("loadingDistributorBindings")}</p>
          ) : distributorBindings.length === 0 ? (
            <p className="p-4 text-sm text-[var(--portal-muted)]">{t("emptyDistributorBindings")}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full text-left">
                <thead className="bg-[var(--portal-clay)] text-xs uppercase text-[var(--portal-muted)]">
                  <tr>
                    <th className="px-4 py-3">{t("distributor")}</th>
                    <th className="px-4 py-3">{t("user")}</th>
                    <th className="px-4 py-3">{t("source")}</th>
                    <th className="px-4 py-3">{t("createdAt")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--portal-line)]">
                  {distributorBindings.map((item) => (
                    <tr key={item.id}>
                      <td className="px-4 py-3">
                        <p className="text-sm font-semibold text-[var(--portal-ink)]">{item.distributor_name || item.distributor_email || `#${item.distributor_user_id}`}</p>
                        <p className="text-xs text-[var(--portal-muted)]">{item.distributor_email || `#${item.distributor_user_id}`}</p>
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-sm font-semibold text-[var(--portal-ink)]">{item.name || item.email}</p>
                        <p className="text-xs text-[var(--portal-muted)]">{item.email}</p>
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--portal-ink)]">{item.source || "--"}</td>
                      <td className="px-4 py-3 text-sm text-[var(--portal-muted)]">{formatDateTime(item.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Quick Create User */}
        <div className="block-card space-y-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("quickCreateUser")}</h2>
          <form className="grid gap-4" onSubmit={handleQuickCreate}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("email")}</span>
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
              {isCreating ? t("creating") : t("createUser")}
            </button>
          </form>

          {createResult ? (
            <div className="space-y-2 rounded-xl border border-emerald-400/40 bg-emerald-500/5 p-4">
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">{t("userCreated")}</h3>
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
              <div className="grid gap-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">Sub2API ID:</span>
                  <span className="font-mono text-[var(--portal-ink)]">{createResult.id}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">{t("email")}:</span>
                  <span className="text-[var(--portal-ink)]">{createResult.email}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">{t("name")}:</span>
                  <span className="text-[var(--portal-ink)]">{createResult.name}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">{t("password")}:</span>
                  <div className="flex items-center gap-2">
                    <code className="rounded bg-[var(--portal-clay-strong)] px-2 py-0.5 font-mono text-xs text-[var(--portal-ink)]">
                      {createResult.password}
                    </code>
                    <button
                      type="button"
                      className="btn-ghost px-2 py-0.5 text-xs"
                      onClick={() => void handleCopy(createResult.password)}
                    >
                      {t("copy")}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          ) : null}
        </div>

        {/* Assign Package */}
        <div className="block-card space-y-4">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">{t("assignPackage")}</h2>
          <form className="grid gap-4" onSubmit={handleAssignPackage}>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("sub2apiUserId")}</span>
              <input
                className="field font-mono"
                type="number"
                min="1"
                placeholder={t("creationResultUserId")}
                value={assignUserId}
                onChange={(e) => setAssignUserId(e.target.value)}
                disabled={isBlocked || isAssigning}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("email")}</span>
              <input
                className="field"
                type="email"
                placeholder={t("importedUserEmail")}
                value={assignEmail}
                onChange={(e) => setAssignEmail(e.target.value)}
                disabled={isBlocked || isAssigning}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>{t("package")}</span>
              <select
                className="field"
                value={assignTierCode}
                onChange={(e) => setAssignTierCode(e.target.value)}
                disabled={isBlocked || isAssigning || isLoadingPackages}
                required
              >
                <option value="">{t("selectPackage")}</option>
	                {packages
	                  .filter((p) => (p.is_published ?? p.is_enabled) !== false)
	                  .map((p) => (
                    <option key={p.code} value={p.code}>
                      {p.name} ({p.code}) - {p.level === "distributor" ? t("distributorLevel") : t("adminLevel")} - {p.price_micros > 0 ? `¥${(p.price_micros / 1000000).toFixed(2)}` : t("free")}
                    </option>
	                  ))}
	              </select>
	            </label>
	            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
	              <span>{t("latestPassword")}</span>
	              <input
	                className="field"
	                type="password"
	                placeholder={t("latestPasswordPlaceholder")}
	                value={assignPassword}
	                onChange={(e) => setAssignPassword(e.target.value)}
	                disabled={isBlocked || isAssigning}
	                autoComplete="new-password"
	              />
	            </label>
	            <button className="btn-primary" type="submit" disabled={isBlocked || isAssigning}>
              {isAssigning ? t("assigning") : t("assignPackage")}
            </button>
          </form>

          {assignResult ? (
            <div className="space-y-2 rounded-xl border border-emerald-400/40 bg-emerald-500/5 p-4">
              <h3 className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">{t("packageAssigned")}</h3>
              <div className="grid gap-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">{t("paymentEvent")}:</span>
                  <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.payment_event_id}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[var(--portal-muted)]">{t("tierCode")}:</span>
                  <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.tier_code}</span>
                </div>
                {assignResult.fulfillment_job ? (
                  <>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-[var(--portal-muted)]">{t("jobId")}:</span>
                      <span className="font-mono text-xs text-[var(--portal-ink)]">{assignResult.fulfillment_job.id}</span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-[var(--portal-muted)]">{t("status")}:</span>
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
                        {t("error")}: {assignResult.fulfillment_job.error_message}
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

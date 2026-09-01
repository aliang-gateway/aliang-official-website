"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { ConfigPanel } from "@/components/dashboard/ConfigPanel";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { extractApiError } from "@/lib/api-response";
import { useConfigModal } from "@/lib/hooks/use-config-modal";
import {
  authHeaders,
  isProtectedApiKeyName,
  matchesFormatFilter,
  parseApiKeysList,
  parseAvailableGroups,
  platformBadgeLabel,
  type ApiKeyFormatFilter,
  type ApiKeyItem,
  type AvailableGroup,
} from "@/lib/api-keys";

const SESSION_TOKEN_KEY = "session_token";

type Tab = "keys" | "config";

export default function KeysPage() {
  const t = useTranslations("dashboard");
  const router = useRouter();
  const config = useConfigModal();
  const [activeTab, setActiveTab] = useState<Tab>("keys");
  const [sessionToken, setSessionToken] = useState("");
  const [isReady, setIsReady] = useState(false);

  // API keys tab state
  const [keys, setKeys] = useState<ApiKeyItem[]>([]);
  const [groups, setGroups] = useState<AvailableGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState<ApiKeyFormatFilter>("all");
  const [groupFilter, setGroupFilter] = useState<number | "all">("all");
  const [busyKeyId, setBusyKeyId] = useState<number | null>(null);
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyGroupId, setNewKeyGroupId] = useState<number | null>(null);
  const [creatingKey, setCreatingKey] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [createSuccess, setCreateSuccess] = useState<string | null>(null);
  const [mutateError, setMutateError] = useState<string | null>(null);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copiedCreatedKey, setCopiedCreatedKey] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);

  useEffect(() => {
    setSessionToken(localStorage.getItem(SESSION_TOKEN_KEY) ?? "");
    setIsReady(true);
  }, []);

  const loadAll = useCallback(async () => {
    if (!sessionToken) return;
    setLoading(true);
    setError(null);
    try {
      const headers = authHeaders(sessionToken);
      // Best-effort: idempotently provision auto keys before listing, so a
      // fresh user sees keys on first visit. Failure must not block listing.
      // 10s client cap so a degraded backend can't hold the page's loading
      // state — but AbortSignal.timeout is unavailable on older Safari (<16),
      // where calling it throws synchronously and escapes the .catch; guard it.
      const ensureSignal = typeof AbortSignal !== "undefined" && "timeout" in AbortSignal ? AbortSignal.timeout(10_000) : undefined;
      await fetch("/api-keys/ensure-auto", { method: "POST", headers, cache: "no-store", signal: ensureSignal }).catch(() => null);
      const [keysRes, groupsRes] = await Promise.all([
        fetch("/api-keys?page=1&per_page=100", { headers, cache: "no-store" }),
        fetch("/api/groups/available", { headers, cache: "no-store" }),
      ]);
      if (keysRes.status === 401 || keysRes.status === 403) {
        // 会话已过期/无效：清除本地凭证并回登录页（与 dashboard 的 401 处理一致）。
        localStorage.removeItem(SESSION_TOKEN_KEY);
        setSessionToken("");
        router.replace(`/login?next=${encodeURIComponent("/keys")}`);
        return;
      }
      const keysPayload = keysRes.ok ? await keysRes.json() : null;
      const groupsPayload = groupsRes.ok ? await groupsRes.json() : null;
      setKeys(parseApiKeysList(keysPayload));
      setGroups(parseAvailableGroups(groupsPayload));
    } catch (e) {
      setError(e instanceof Error ? e.message : t("errorPrefix"));
    } finally {
      setLoading(false);
    }
  }, [router, sessionToken, t]);

  useEffect(() => {
    if (!isReady) return;
    if (!sessionToken) {
      // 本地无凭证：直接送登录页，登录后回跳（避免误伤首帧尚未读到 token 的情况）。
      router.replace(`/login?next=${encodeURIComponent("/keys")}`);
      return;
    }
    void loadAll();
  }, [isReady, loadAll, router, sessionToken]);

  const filteredKeys = useMemo(() => {
    return keys.filter((key) => {
      if (!matchesFormatFilter(key.group_platform, typeFilter)) return false;
      if (groupFilter !== "all" && key.group_id !== groupFilter) return false;
      return true;
    });
  }, [keys, typeFilter, groupFilter]);

  const handleToggle = async (keyId: number, status: string) => {
    if (!sessionToken) return;
    const nextStatus = status === "active" ? "inactive" : "active";
    setBusyKeyId(keyId);
    setMutateError(null);
    try {
      const res = await fetch(`/api-keys/${keyId}`, {
        method: "PUT",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ status: nextStatus }),
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, t("errorPrefix")));
      }
      await loadAll();
    } catch (err) {
      setMutateError(err instanceof Error ? err.message : t("errorPrefix"));
    } finally {
      setBusyKeyId(null);
    }
  };

  const handleDelete = async (keyId: number, name: string) => {
    if (!sessionToken || isProtectedApiKeyName(name)) return;
    if (!window.confirm(t("confirmDeleteKey"))) return;
    setBusyKeyId(keyId);
    setMutateError(null);
    try {
      const res = await fetch(`/api-keys/${keyId}`, {
        method: "DELETE",
        headers: authHeaders(sessionToken),
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, t("errorPrefix")));
      }
      await loadAll();
    } catch (err) {
      setMutateError(err instanceof Error ? err.message : t("errorPrefix"));
    } finally {
      setBusyKeyId(null);
    }
  };

  const handleCreateKey = async (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setCreateError(null);
    setCreateSuccess(null);
    setMutateError(null);
    if (!sessionToken || !newKeyName.trim() || !newKeyGroupId) return;
    setCreatingKey(true);
    try {
      const res = await fetch("/api-keys", {
        method: "POST",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ name: newKeyName.trim(), group_id: newKeyGroupId }),
        cache: "no-store",
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        throw new Error(extractApiError(payload, t("createKeyError")));
      }
      const record = (payload as { data?: { key?: unknown } } | null)?.data;
      const plaintext = typeof record?.key === "string" ? record.key : "";
      if (plaintext) {
        setCreatedKey(plaintext);
      }
      setCreateSuccess(plaintext ? null : t("createKeySuccess"));
      setNewKeyName("");
      setNewKeyGroupId(null);
      await loadAll();
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : t("createKeyError"));
    } finally {
      setCreatingKey(false);
    }
  };

  const handleCopyCreatedKey = async () => {
    if (!createdKey) return;
    try {
      await navigator.clipboard.writeText(createdKey);
      setCopiedCreatedKey(true);
      setCopyFailed(false);
      window.setTimeout(() => setCopiedCreatedKey(false), 1500);
    } catch {
      // clipboard unavailable — tell the user instead of failing silently
      setCopiedCreatedKey(false);
      setCopyFailed(true);
    }
  };

  const tabs: { id: Tab; label: string }[] = [
    { id: "keys", label: t("tabApiKeys") },
    { id: "config", label: t("tabConfig") },
  ];

  return (
    <section className="portal-shell space-y-8 py-10">
      <div className="space-y-2">
        <p
          className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]"
          style={{ fontFamily: "var(--font-editorial-mono)" }}
        >
          {t("keysAndConfigTitle")}
        </p>
        <h1 className="font-[var(--font-editorial)] text-3xl font-extrabold tracking-tight text-[var(--ink)]">
          {t("keysAndConfigTitle")}
        </h1>
      </div>

      {/* Tab 栏 */}
      <div
        role="tablist"
        aria-label={t("keysAndConfigTitle")}
        className="flex w-fit flex-wrap gap-1.5 rounded-full border border-[var(--line)] bg-[var(--paper)] p-1.5"
      >
        {tabs.map((tab) => (
          <button
            key={tab.id}
            role="tab"
            type="button"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`rounded-full px-5 py-2 text-sm font-bold transition-colors ${
              activeTab === tab.id
                ? "bg-[var(--ink)] text-[var(--paper)]"
                : "text-[var(--ink-muted)] hover:text-[var(--ink)]"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* API 密钥 tab */}
      {activeTab === "keys" ? (
        <div className="space-y-5">
          {createdKey ? (
            <div className="clay-panel space-y-3 border-2 border-[var(--accent)]/40 p-4">
              <p className="text-sm font-bold text-[var(--ink)]">{t("createKeyRevealTitle")}</p>
              <p className="text-xs text-[var(--ink-muted)]">{t("createKeyRevealHint")}</p>
              <div className="flex flex-wrap items-center gap-2">
                <code className="min-w-0 flex-1 break-all rounded-lg border border-[var(--line)] bg-[var(--paper)] px-3 py-2 font-mono text-xs text-[var(--ink)]">{createdKey}</code>
                <button type="button" onClick={() => void handleCopyCreatedKey()} className="rounded-full bg-[var(--ink)] px-4 py-2 text-xs font-bold text-[var(--paper)]">
                  {copiedCreatedKey ? t("copied") : t("copy")}
                </button>
                <button type="button" onClick={() => { setCreatedKey(null); setCopiedCreatedKey(false); setCopyFailed(false); }} className="rounded-full border border-[var(--line)] px-4 py-2 text-xs font-bold text-[var(--ink)]">
                  {t("closePanel")}
                </button>
              </div>
              {copyFailed ? <p className="text-xs font-bold text-red-500">{t("copyFailedHint")}</p> : null}
            </div>
          ) : null}

          {/* 创建 key */}
          <form onSubmit={handleCreateKey} aria-label={t("createKeyTitle")} className="clay-panel flex flex-wrap items-end gap-3 p-4">
            <div className="flex flex-col gap-1">
              <label htmlFor="new-key-name" className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("createKeyNameLabel")}
              </label>
              <input
                id="new-key-name"
                value={newKeyName}
                onChange={(event) => setNewKeyName(event.target.value)}
                placeholder={t("createKeyNamePlaceholder")}
                maxLength={100}
                className="field w-56"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="new-key-group" className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("createKeyGroupLabel")}
              </label>
              <select
                id="new-key-group"
                value={newKeyGroupId === null ? "" : String(newKeyGroupId)}
                onChange={(event) => setNewKeyGroupId(event.target.value ? Number(event.target.value) : null)}
                className="field w-56"
              >
                <option value="">—</option>
                {groups.map((group) => (
                  <option key={group.id} value={String(group.id)}>
                    {group.name}
                    {group.platform ? ` · ${platformBadgeLabel(group.platform)}` : ""}
                  </option>
                ))}
              </select>
            </div>
            <button
              type="submit"
              disabled={creatingKey || !sessionToken || !newKeyName.trim() || !newKeyGroupId}
              className="rounded-full bg-[var(--ink)] px-5 py-2 text-sm font-bold text-[var(--paper)] transition-opacity disabled:opacity-50"
            >
              {creatingKey ? t("createKeyCreating") : t("createKeySubmit")}
            </button>
            {createSuccess ? <span role="status" className="text-xs font-bold text-emerald-500">{createSuccess}</span> : null}
            {createError ? <span role="status" className="text-xs font-bold text-red-500">{createError}</span> : null}
          </form>

          {/* 筛选 */}
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("filterByType")}
              </span>
              <div className="flex gap-1.5 rounded-full border border-[var(--line)] bg-[var(--paper)] p-1">
                {(["all", "openai", "anthropic"] as const).map((value) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setTypeFilter(value)}
                    aria-pressed={typeFilter === value}
                    className={`rounded-full px-3 py-1 text-xs font-bold transition-colors ${
                      typeFilter === value
                        ? "bg-[var(--ink)] text-[var(--paper)]"
                        : "text-[var(--ink-muted)] hover:text-[var(--ink)]"
                    }`}
                  >
                    {value === "all" ? t("filterAll") : platformBadgeLabel(value)}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("filterByGroup")}
              </span>
              <select
                value={groupFilter === "all" ? "all" : String(groupFilter)}
                onChange={(event) =>
                  setGroupFilter(event.target.value === "all" ? "all" : Number(event.target.value))
                }
                className="field w-auto py-1.5 text-sm"
              >
                <option value="all">{t("filterAll")}</option>
                {groups.map((group) => (
                  <option key={group.id} value={String(group.id)}>
                    {group.name}
                    {group.platform ? ` · ${platformBadgeLabel(group.platform)}` : ""}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {mutateError ? <p className="notice">{mutateError}</p> : null}

          {/* 列表 */}
          {loading ? (
            <div className="clay-panel p-5">
              <p className="text-sm text-[var(--ink-muted)]">{t("loading")}</p>
            </div>
          ) : error ? (
            <p className="notice">{t("errorPrefix")}{error}</p>
          ) : filteredKeys.length === 0 ? (
            <div className="clay-panel p-5">
              <p className="text-sm text-[var(--ink-muted)]">{t("noKeysYet")}</p>
            </div>
          ) : (
            <ul className="grid gap-3">
              {filteredKeys.map((apiKey) => (
                <KeyRow
                  key={apiKey.id}
                  apiKey={apiKey}
                  busyKeyId={busyKeyId}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  t={t}
                />
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {/* 配置 tab */}
      {activeTab === "config" ? (
        <div className="clay-panel overflow-hidden">
          <ConfigPanel
            userKey={config.userKey}
            onUserKeyChange={config.setUserKey}
            template={config.template}
            onTemplateChange={config.setTemplate}
            format={config.format}
            onFormatChange={config.setFormat}
            templateDefinition={config.templateDefinition}
            renderedConfig={config.renderedConfig}
            copyState={config.copyState}
            onCopy={config.handleCopy}
            sessionToken={sessionToken}
          />
        </div>
      ) : null}
    </section>
  );
}

type KeyRowProps = {
  apiKey: ApiKeyItem;
  busyKeyId: number | null;
  onToggle: (keyId: number, status: string) => void;
  onDelete: (keyId: number, name: string) => void;
  t: (key: string) => string;
};

function KeyRow({ apiKey, busyKeyId, onToggle, onDelete, t }: KeyRowProps) {
  const isProtected = isProtectedApiKeyName(apiKey.name);
  const isActive = apiKey.status === "active";
  const created = apiKey.created_at?.split("T")[0] ?? "—";
  const expires = apiKey.expires_at?.split("T")[0];

  return (
    <li className="clay-panel p-4">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate text-sm font-bold text-[var(--ink)]">{apiKey.name || `Key #${apiKey.id}`}</p>
            <span
              className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${
                isActive ? "bg-[var(--accent-wash)] text-[var(--accent-ink)]" : "bg-red-500/10 text-red-600"
              }`}
            >
              <span className={`inline-block h-1.5 w-1.5 rounded-full ${isActive ? "bg-[var(--accent)]" : "bg-red-500"}`} />
              {apiKey.status}
            </span>
            {apiKey.group_platform ? (
              <span className="rounded-md bg-[var(--accent-wash)] px-2 py-0.5 text-[10px] font-bold text-[var(--accent-ink)]">
                {platformBadgeLabel(apiKey.group_platform)}
              </span>
            ) : null}
          </div>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-0.5 text-[11px] text-[var(--ink-muted)]">
            <span className="flex items-center gap-1 font-mono">
              <MaterialIcon name="key" size={12} />
              {apiKey.key}
            </span>
            <span className="flex items-center gap-1">
              <MaterialIcon name="group" size={12} />
              {apiKey.group_name}
            </span>
            <span className="flex items-center gap-1">
              <MaterialIcon name="schedule" size={12} />
              {t("createdLabel")}: {created}
            </span>
            {apiKey.quota > 0 ? (
              <span className="font-mono">
                ${apiKey.quota_used.toFixed(2)} / ${apiKey.quota.toFixed(2)}
              </span>
            ) : null}
            {expires ? (
              <span>
                {t("expiresLabel")}: {expires}
              </span>
            ) : null}
            <span>ID: {apiKey.id}</span>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-1.5">
          <button
            type="button"
            onClick={() => onToggle(apiKey.id, apiKey.status)}
            disabled={busyKeyId === apiKey.id}
            className="rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
            title={isActive ? t("disableKey") : t("enableKey")}
          >
            <MaterialIcon name={isActive ? "toggle_on" : "toggle_off"} size={16} className={isActive ? "text-[var(--accent)]" : "text-red-500"} />
          </button>
          <button
            type="button"
            onClick={() => onDelete(apiKey.id, apiKey.name)}
            disabled={isProtected || busyKeyId === apiKey.id}
            className="rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-red-600 transition-colors hover:border-red-500/40 hover:bg-red-500/5 disabled:cursor-not-allowed disabled:opacity-40"
            title={isProtected ? t("protectedKey") : t("deleteKey")}
          >
            <MaterialIcon name={isProtected ? "lock" : "delete_outline"} size={16} />
          </button>
        </div>
      </div>
    </li>
  );
}

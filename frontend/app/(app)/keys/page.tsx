"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";

import { ConfigPanel } from "@/components/dashboard/ConfigPanel";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useConfigModal } from "@/lib/hooks/use-config-modal";
import {
  authHeaders,
  isProtectedApiKeyName,
  maskApiKey,
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
  const config = useConfigModal();
  const [activeTab, setActiveTab] = useState<Tab>("keys");
  const [sessionToken, setSessionToken] = useState("");

  // API keys tab state
  const [keys, setKeys] = useState<ApiKeyItem[]>([]);
  const [groups, setGroups] = useState<AvailableGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState<ApiKeyFormatFilter>("all");
  const [groupFilter, setGroupFilter] = useState<number | "all">("all");
  const [copiedKeyId, setCopiedKeyId] = useState<number | null>(null);
  const [busyKeyId, setBusyKeyId] = useState<number | null>(null);

  useEffect(() => {
    setSessionToken(localStorage.getItem(SESSION_TOKEN_KEY) ?? "");
  }, []);

  const loadAll = useCallback(async () => {
    if (!sessionToken) return;
    setLoading(true);
    setError(null);
    try {
      const headers = authHeaders(sessionToken);
      const [keysRes, groupsRes] = await Promise.all([
        fetch("/api-keys?page=1&per_page=100", { headers, cache: "no-store" }),
        fetch("/api/groups/available", { headers, cache: "no-store" }),
      ]);
      const keysPayload = keysRes.ok ? await keysRes.json() : null;
      const groupsPayload = groupsRes.ok ? await groupsRes.json() : null;
      setKeys(parseApiKeysList(keysPayload));
      setGroups(parseAvailableGroups(groupsPayload));
    } catch (e) {
      setError(e instanceof Error ? e.message : t("errorPrefix"));
    } finally {
      setLoading(false);
    }
  }, [sessionToken, t]);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const filteredKeys = useMemo(() => {
    return keys.filter((key) => {
      if (!matchesFormatFilter(key.group_platform, typeFilter)) return false;
      if (groupFilter !== "all" && key.group_id !== groupFilter) return false;
      return true;
    });
  }, [keys, typeFilter, groupFilter]);

  const handleCopy = async (keyId: number, keyValue: string) => {
    if (!keyValue) return;
    try {
      await navigator.clipboard.writeText(keyValue);
      setCopiedKeyId(keyId);
      window.setTimeout(() => setCopiedKeyId((current) => (current === keyId ? null : current)), 1500);
    } catch {
      // clipboard unavailable
    }
  };

  const handleToggle = async (keyId: number, status: string) => {
    if (!sessionToken) return;
    const nextStatus = status === "active" ? "revoked" : "active";
    setBusyKeyId(keyId);
    try {
      const res = await fetch(`/api-keys/${keyId}`, {
        method: "PUT",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ status: nextStatus }),
      });
      if (!res.ok) throw new Error(t("errorPrefix"));
      await loadAll();
    } catch {
      // ignore — keep current state
    } finally {
      setBusyKeyId(null);
    }
  };

  const handleDelete = async (keyId: number, name: string) => {
    if (!sessionToken || isProtectedApiKeyName(name)) return;
    if (!window.confirm(t("confirmDeleteKey"))) return;
    setBusyKeyId(keyId);
    try {
      const res = await fetch(`/api-keys/${keyId}`, {
        method: "DELETE",
        headers: authHeaders(sessionToken),
      });
      if (!res.ok) throw new Error(t("errorPrefix"));
      await loadAll();
    } catch {
      // ignore
    } finally {
      setBusyKeyId(null);
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
                  copiedKeyId={copiedKeyId}
                  busyKeyId={busyKeyId}
                  onCopy={handleCopy}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  t={t}
                />
              ))}
            </ul>
          )}

          {/* 操作提示 */}
          <p className="text-xs text-[var(--ink-muted)]">
            <MaterialIcon name="info" size={14} className="mr-1 align-[-2px]" />
            {t("modelsHint")}
          </p>
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
  copiedKeyId: number | null;
  busyKeyId: number | null;
  onCopy: (keyId: number, keyValue: string) => void;
  onToggle: (keyId: number, status: string) => void;
  onDelete: (keyId: number, name: string) => void;
  t: (key: string) => string;
};

function KeyRow({ apiKey, copiedKeyId, busyKeyId, onCopy, onToggle, onDelete, t }: KeyRowProps) {
  const isProtected = isProtectedApiKeyName(apiKey.name);
  const isActive = apiKey.status === "active";
  const created = apiKey.created_at?.split("T")[0] ?? "—";
  const expires = apiKey.expires_at?.split("T")[0];

  // Per-key supported models — lazy-loaded on expand via the gateway /v1/models.
  const [showModels, setShowModels] = useState(false);
  const [models, setModels] = useState<string[] | null>(null);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [modelsError, setModelsError] = useState<string | null>(null);

  const toggleModels = async () => {
    if (showModels) {
      setShowModels(false);
      return;
    }
    setShowModels(true);
    if (models !== null || !apiKey.key) return;
    setModelsLoading(true);
    setModelsError(null);
    try {
      const res = await fetch("/api/models", {
        headers: { "x-api-key": apiKey.key },
        cache: "no-store",
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const payload = (await res.json()) as { data?: unknown };
      const list = Array.isArray(payload?.data) ? payload.data : [];
      const names = list
        .map((item) =>
          typeof item === "string"
            ? item
            : (item as { id?: string; name?: string })?.id ?? (item as { name?: string })?.name,
        )
        .filter((name): name is string => Boolean(name));
      setModels(names);
    } catch (e) {
      setModelsError(e instanceof Error ? e.message : "error");
      setModels([]);
    } finally {
      setModelsLoading(false);
    }
  };

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
              {maskApiKey(apiKey.key)}
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
            onClick={() => void toggleModels()}
            disabled={!isActive || !apiKey.key}
            className="rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
            title={t("supportedModels")}
            aria-expanded={showModels}
          >
            <MaterialIcon name={showModels ? "expand_less" : "expand_more"} size={16} />
          </button>
          <button
            type="button"
            onClick={() => onCopy(apiKey.id, apiKey.key)}
            disabled={!apiKey.key}
            className="rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
            title={t("copy")}
          >
            <MaterialIcon name={copiedKeyId === apiKey.id ? "check" : "content_copy"} size={16} />
          </button>
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

      {showModels ? (
        <div className="mt-3 w-full border-t border-[var(--line)] pt-3">
          <p
            className="mb-2 text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]"
            style={{ fontFamily: "var(--font-editorial-mono)" }}
          >
            {t("supportedModels")}
          </p>
          {modelsLoading ? (
            <p className="text-xs text-[var(--ink-muted)]">{t("loading")}</p>
          ) : modelsError ? (
            <p className="text-xs text-red-600">
              {t("modelsLoadFailed")}({modelsError})
            </p>
          ) : models && models.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {models.map((name) => (
                <span
                  key={name}
                  className="rounded-md border border-[var(--line)] bg-[var(--paper)] px-2 py-0.5 font-mono text-[11px] text-[var(--ink)]"
                >
                  {name}
                </span>
              ))}
            </div>
          ) : (
            <p className="text-xs text-[var(--ink-muted)]">{t("noModels")}</p>
          )}
        </div>
      ) : null}
    </li>
  );
}

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { ConfigPanel } from "@/components/dashboard/ConfigPanel";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { Modal } from "@/components/ui/Modal";
import { extractApiError } from "@/lib/api-response";
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
  const [copiedKeyId, setCopiedKeyId] = useState<number | null>(null);
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyGroupId, setNewKeyGroupId] = useState<number | null>(null);
  const [creatingKey, setCreatingKey] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [createSuccess, setCreateSuccess] = useState<string | null>(null);
  const [mutateError, setMutateError] = useState<string | null>(null);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copyFailed, setCopyFailed] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createKeyCopied, setCreateKeyCopied] = useState(false);
  const [groupMenuRect, setGroupMenuRect] = useState<{ left: number; width: number; top: number | null; bottom: number | null } | null>(null);
  const createKeyTriggerRef = useRef<HTMLButtonElement | null>(null);
  const groupTriggerRef = useRef<HTMLButtonElement | null>(null);

  // 分组下拉用 fixed 定位(按触发按钮的视口坐标):面板容器有 overflow-hidden/
  // overflow-y-auto,absolute 定位的列表会被裁掉;祖先链无 transform,fixed 可
  // 逃逸裁剪。下方空间不足(max-h-60 = 240px + 间距)时自动向上弹出。
  const toggleGroupMenu = () => {
    setGroupMenuRect((current) => {
      if (current) return null;
      const el = groupTriggerRef.current;
      if (!el) return null;
      const rect = el.getBoundingClientRect();
      const openUp = window.innerHeight - rect.bottom < 248;
      return {
        left: rect.left,
        width: rect.width,
        top: openUp ? null : rect.bottom + 4,
        bottom: openUp ? window.innerHeight - rect.top + 4 : null,
      };
    });
  };

  useEffect(() => {
    if (!groupMenuRect) return;
    const close = () => setGroupMenuRect(null);
    window.addEventListener("resize", close);
    return () => window.removeEventListener("resize", close);
  }, [groupMenuRect]);

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

  const handleCopy = async (keyId: number, keyValue: string) => {
    if (!keyValue) return;
    try {
      await navigator.clipboard.writeText(keyValue);
      setCopiedKeyId(keyId);
      window.setTimeout(() => setCopiedKeyId((current) => (current === keyId ? null : current)), 1500);
    } catch (err) {
      setMutateError(err instanceof Error ? err.message : t("copyFailedHint"));
    }
  };

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
      // 10s 客户端上限（旧 Safari 守卫同 ensure-auto）：创建请求挂死时超时报错并解除弹窗的关闭禁用，
      // 避免用户被"创建中"锁死在弹窗里。
      const createSignal = typeof AbortSignal !== "undefined" && "timeout" in AbortSignal ? AbortSignal.timeout(10_000) : undefined;
      const res = await fetch("/api-keys", {
        method: "POST",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ name: newKeyName.trim(), group_id: newKeyGroupId }),
        cache: "no-store",
        signal: createSignal,
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
      setCreateSuccess(plaintext ? null : t("createKeyNoPlaintext"));
      setNewKeyName("");
      setNewKeyGroupId(null);
      // 不在此处刷新列表:明文只在弹窗里展示一次,等用户读完并关闭弹窗后再 loadAll。
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : t("createKeyError"));
    } finally {
      setCreatingKey(false);
    }
  };

  const handleCloseCreateModal = useCallback(() => {
    // 创建请求仍在进行时禁止关闭:若此刻放行,请求完成后明文会落在已关闭的
    // 弹窗里,用户再也看不到(列表里只有打码值),密钥等于白创建一次。
    if (creatingKey) return;
    // createdKey(明文已展示)或 createSuccess(后端成功但响应缺明文的兜底)
    // 都说明刚才创建过密钥,关闭时刷新列表让它出现。
    const didCreate = createdKey !== null || createSuccess !== null;
    setShowCreateModal(false);
    setCreatedKey(null);
    setCreateKeyCopied(false);
    setCopyFailed(false);
    setCreateError(null);
    setCreateSuccess(null);
    setNewKeyName("");
    setNewKeyGroupId(null);
    setGroupMenuRect(null);
    // 用户处理完明文后再刷新列表,新建的密钥才会出现。
    if (didCreate) void loadAll();
  }, [createdKey, createSuccess, creatingKey, loadAll]);

  const handleCopyCreatedKey = async () => {
    if (!createdKey) return;
    try {
      await navigator.clipboard.writeText(createdKey);
      setCreateKeyCopied(true);
      setCopyFailed(false);
      window.setTimeout(() => setCreateKeyCopied(false), 1500);
    } catch {
      // clipboard unavailable — tell the user instead of failing silently
      setCreateKeyCopied(false);
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
          {/* 创建 key:点击打开弹窗,分组+名字在弹窗内填写,明文仅在弹窗内一次性展示 */}
          <button
            ref={createKeyTriggerRef}
            type="button"
            onClick={() => setShowCreateModal(true)}
            disabled={!sessionToken}
            className="rounded-full bg-[var(--ink)] px-5 py-2 text-sm font-bold text-[var(--paper)] transition-opacity disabled:opacity-50"
          >
            {t("createKey")}
          </button>

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
                  copiedKeyId={copiedKeyId}
                  onCopy={handleCopy}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  t={t}
                />
              ))}
            </ul>
          )}
        </div>
      ) : null}

      {/* 创建密钥弹窗:成功后切换为明文一次性展示视图,关闭时统一复位并刷新列表 */}
      <Modal
        isOpen={showCreateModal}
        onClose={handleCloseCreateModal}
        closeLabel={t("closePanel")}
        triggerRef={createKeyTriggerRef}
        panelClassName="max-w-md"
      >
        {createdKey ? (
          <div className="space-y-3 p-6 pr-12">
            <p className="text-lg font-extrabold text-[var(--ink)]">{t("createKeyRevealTitle")}</p>
            <p className="text-xs leading-5 text-[var(--ink-muted)]">{t("createKeyRevealHint")}</p>
            <code className="block break-all rounded-lg border border-[var(--line)] bg-[var(--paper)] px-3 py-2 font-mono text-xs text-[var(--ink)]">
              {createdKey}
            </code>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => void handleCopyCreatedKey()}
                className="rounded-full bg-[var(--ink)] px-4 py-2 text-xs font-bold text-[var(--paper)]"
              >
                {createKeyCopied ? t("copied") : t("copy")}
              </button>
              <button
                type="button"
                onClick={handleCloseCreateModal}
                className="rounded-full border border-[var(--line)] px-4 py-2 text-xs font-bold text-[var(--ink)]"
              >
                {t("createKeyDone")}
              </button>
            </div>
            {copyFailed ? <p className="text-xs font-bold text-red-500">{t("copyFailedHint")}</p> : null}
          </div>
        ) : (
          <form onSubmit={handleCreateKey} aria-label={t("createKeyTitle")} className="space-y-4 p-6 pr-12">
            <p className="text-lg font-extrabold text-[var(--ink)]">{t("createKeyTitle")}</p>
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
                className="field w-full"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="new-key-group" className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("createKeyGroupLabel")}
              </label>
              {/* 自绘下拉:原生 <select> 的弹出层由浏览器绘制,CSS 管不到其在
                  fixed 弹窗内的锚定位置(部分环境会错锚到屏幕角落),普通 DOM
                  元素则永远留在弹窗内。 */}
              <div className="relative">
                <button
                  type="button"
                  ref={groupTriggerRef}
                  id="new-key-group"
                  aria-haspopup="listbox"
                  aria-expanded={groupMenuRect !== null}
                  onClick={toggleGroupMenu}
                  onKeyDown={(event) => {
                    if (event.key === "Escape" && groupMenuRect) {
                      // Esc 先只关下拉菜单,不冒泡给 Modal 关掉整个弹窗。
                      event.stopPropagation();
                      setGroupMenuRect(null);
                    }
                  }}
                  className="field flex w-full items-center justify-between text-left"
                >
                  <span className={newKeyGroupId === null ? "text-[var(--ink-muted)]" : ""}>
                    {(() => {
                      if (newKeyGroupId === null) return "—";
                      const picked = groups.find((group) => group.id === newKeyGroupId);
                      if (!picked) return "—";
                      return `${picked.name}${picked.platform ? ` · ${platformBadgeLabel(picked.platform)}` : ""}`;
                    })()}
                  </span>
                  <MaterialIcon name={groupMenuRect ? "expand_less" : "expand_more"} size={18} className="text-[var(--ink-muted)]" />
                </button>
                {groupMenuRect ? (
                  <>
                    <button
                      type="button"
                      aria-hidden
                      tabIndex={-1}
                      className="fixed inset-0 z-10 cursor-default"
                      onClick={() => setGroupMenuRect(null)}
                    />
                    <ul
                      role="listbox"
                      className="fixed z-20 max-h-60 overflow-y-auto rounded-xl border border-[var(--line)] bg-[var(--paper)] py-1 shadow-[var(--shadow)]"
                      style={
                        groupMenuRect.top !== null
                          ? { left: groupMenuRect.left, width: groupMenuRect.width, top: groupMenuRect.top }
                          : { left: groupMenuRect.left, width: groupMenuRect.width, bottom: groupMenuRect.bottom ?? 0 }
                      }
                    >
                      {groups.map((group) => {
                        const active = group.id === newKeyGroupId;
                        return (
                          <li key={group.id}>
                            <button
                              type="button"
                              role="option"
                              aria-selected={active}
                              onClick={() => {
                                setNewKeyGroupId(group.id);
                                setGroupMenuRect(null);
                              }}
                              className={`flex w-full items-center justify-between gap-2 px-4 py-2 text-left text-sm transition-colors hover:bg-[var(--accent-wash)] ${
                                active ? "bg-[var(--accent-wash)] font-bold text-[var(--accent-ink)]" : "text-[var(--ink)]"
                              }`}
                            >
                              <span className="truncate">{group.name}</span>
                              {group.platform ? <span className="shrink-0 text-xs text-[var(--ink-muted)]">{platformBadgeLabel(group.platform)}</span> : null}
                            </button>
                          </li>
                        );
                      })}
                    </ul>
                  </>
                ) : null}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="submit"
                disabled={creatingKey || !sessionToken || !newKeyName.trim() || !newKeyGroupId}
                className="rounded-full bg-[var(--ink)] px-5 py-2 text-sm font-bold text-[var(--paper)] transition-opacity disabled:opacity-50"
              >
                {creatingKey ? t("createKeyCreating") : t("createKeySubmit")}
              </button>
              {createSuccess ? <span role="status" className="text-xs font-bold text-emerald-500">{createSuccess}</span> : null}
              {createError ? <span role="status" className="text-xs font-bold text-red-500">{createError}</span> : null}
            </div>
          </form>
        )}
      </Modal>

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
  copiedKeyId: number | null;
  onCopy: (keyId: number, keyValue: string) => void;
  onToggle: (keyId: number, status: string) => void;
  onDelete: (keyId: number, name: string) => void;
  t: (key: string) => string;
};

function KeyRow({ apiKey, busyKeyId, copiedKeyId, onCopy, onToggle, onDelete, t }: KeyRowProps) {
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
            onClick={() => onCopy(apiKey.id, apiKey.key)}
            disabled={!apiKey.key}
            className="rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
            title={t("copy")}
          >
            <MaterialIcon name={copiedKeyId === apiKey.id ? "check" : "content_copy"} size={16} className={copiedKeyId === apiKey.id ? "text-[var(--accent)]" : ""} />
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
    </li>
  );
}

"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import { GATEWAY_BASE_URL, TEMPLATE_DEFINITIONS, type TemplateDefinition } from "@/lib/dashboard-template";
import type { ClientTemplateId, TemplateFormat } from "@/lib/dashboard-types";
import { maskApiKey, parseApiKeysList, type ApiKeyItem } from "@/lib/api-keys";

type ConfigPanelProps = {
  userKey: string;
  onUserKeyChange: (value: string) => void;
  template: ClientTemplateId;
  onTemplateChange: (value: ClientTemplateId) => void;
  format: TemplateFormat;
  onFormatChange: (value: TemplateFormat) => void;
  templateDefinition: TemplateDefinition;
  renderedConfig: string;
  copyState: "idle" | "copied" | "error";
  onCopy: () => void;
  sessionToken: string;
};

/**
 * The configuration UI (user key + existing keys + template selector on the
 * left, rendered client config + copy on the right). Shared by the dashboard
 * ConfigModal and the /keys page's "配置" tab so both stay in sync.
 */
export function ConfigPanel({
  userKey,
  onUserKeyChange,
  template,
  onTemplateChange,
  format,
  onFormatChange,
  templateDefinition,
  renderedConfig,
  copyState,
  onCopy,
  sessionToken,
}: ConfigPanelProps) {
  const t = useTranslations("dashboard");
  const [creatingKey, setCreatingKey] = useState(false);
  const [createKeyError, setCreateKeyError] = useState<string | null>(null);
  const [existingKeys, setExistingKeys] = useState<ApiKeyItem[]>([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const [copiedKeyId, setCopiedKeyId] = useState<number | null>(null);

  const handleCreateKey = async () => {
    setCreateKeyError(null);
    setCreatingKey(true);
    try {
      const res = await fetch("/api-keys", {
        method: "POST",
        headers: { "content-type": "application/json", Authorization: `Bearer ${sessionToken}` },
        body: JSON.stringify({ name: t("defaultKeyName") }),
      });
      const payload = (await res.json()) as { data?: { key?: string }; key?: string; error?: string };
      if (!res.ok) {
        throw new Error(payload?.error ?? t("createKeyFailed"));
      }
      const createdKey = payload?.data?.key ?? payload?.key ?? "";
      if (createdKey) onUserKeyChange(createdKey);
    } catch (e) {
      setCreateKeyError(e instanceof Error ? e.message : t("createKeyFailed"));
    } finally {
      setCreatingKey(false);
    }
  };

  // Load existing keys on mount so users can retrieve / reuse one.
  useEffect(() => {
    if (!sessionToken) {
      setExistingKeys([]);
      return;
    }
    let cancelled = false;
    const loadKeys = async () => {
      setKeysLoading(true);
      try {
        const res = await fetch("/api-keys?page=1&per_page=20", {
          headers: { accept: "application/json", Authorization: `Bearer ${sessionToken}` },
          cache: "no-store",
        });
        const payload = (await res.json()) as unknown;
        if (cancelled) return;
        setExistingKeys(parseApiKeysList(payload));
      } catch {
        // key list is a convenience — fail silently
      } finally {
        if (!cancelled) setKeysLoading(false);
      }
    };
    void loadKeys();
    return () => {
      cancelled = true;
    };
  }, [sessionToken]);

  const handleCopyKey = async (keyId: number, keyValue: string) => {
    if (!keyValue) return;
    try {
      await navigator.clipboard.writeText(keyValue);
      setCopiedKeyId(keyId);
      window.setTimeout(() => setCopiedKeyId((current) => (current === keyId ? null : current)), 1500);
    } catch {
      // clipboard unavailable — ignore
    }
  };

  return (
    <div className="grid min-h-0 gap-0 overflow-y-auto lg:grid-cols-[280px_minmax(0,1fr)]">
      <div className="border-b border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 lg:border-b-0 lg:border-r">
        <div className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="dashboard-user-key" className="text-sm font-semibold text-[var(--portal-ink)]">
              {t("underlyingUserKey")}
            </label>
            <textarea
              id="dashboard-user-key"
              className="field min-h-[112px] resize-y font-mono text-sm"
              placeholder={t("pasteExistingKey")}
              value={userKey}
              onChange={(event) => onUserKeyChange(event.target.value)}
            />
            <p className="text-xs leading-5 text-[var(--portal-muted)]">{t("keySourceDescription")}</p>
          </div>

          <div className="flex flex-wrap gap-3">
            <button type="button" className="btn-primary" onClick={() => void handleCreateKey()} disabled={creatingKey || !sessionToken}>
              {creatingKey ? t("creatingKey") : t("createKey")}
            </button>
            <button type="button" className="btn-ghost" onClick={() => onUserKeyChange("")}>
              {t("clearKey")}
            </button>
          </div>
          {createKeyError ? <p className="text-xs leading-5 text-red-500">{createKeyError}</p> : null}

          {/* 已有 API 密钥:列出 / 复制 / 一键填入 */}
          <div className="space-y-2">
            <p className="text-sm font-semibold text-[var(--portal-ink)]">{t("existingKeys")}</p>
            {keysLoading ? (
              <p className="text-xs text-[var(--portal-muted)]">{t("loading")}</p>
            ) : existingKeys.length === 0 ? (
              <p className="text-xs text-[var(--portal-muted)]">{t("noExistingKeys")}</p>
            ) : (
              <ul className="grid gap-2">
                {existingKeys.map((existingKey) => (
                  <li
                    key={existingKey.id}
                    className="flex items-center gap-2 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] px-3 py-2"
                  >
                    <span
                      className={`inline-block h-1.5 w-1.5 shrink-0 rounded-full ${existingKey.status === "active" ? "bg-[var(--accent)]" : "bg-red-500"}`}
                      aria-hidden
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-xs font-semibold text-[var(--portal-ink)]">{existingKey.name}</p>
                      <p className="truncate font-mono text-[11px] text-[var(--portal-muted)]">{maskApiKey(existingKey.key)}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => void handleCopyKey(existingKey.id, existingKey.key)}
                      disabled={!existingKey.key}
                      className="shrink-0 rounded-lg border border-[var(--portal-line)] px-2 py-1 text-xs text-[var(--portal-ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
                      title={t("copy")}
                    >
                      {copiedKeyId === existingKey.id ? t("copiedKey") : t("copy")}
                    </button>
                    <button
                      type="button"
                      onClick={() => existingKey.key && onUserKeyChange(existingKey.key)}
                      disabled={!existingKey.key}
                      className="shrink-0 rounded-lg border border-[var(--portal-line)] px-2 py-1 text-xs text-[var(--portal-ink)] transition-colors hover:border-[var(--accent)]/40 hover:text-[var(--accent)] disabled:opacity-40"
                      title={t("useKey")}
                    >
                      {t("useKey")}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <a href="/keys" className="inline-block text-xs font-semibold text-[var(--accent-ink)] hover:underline">
              {t("manageSessionKeys")} →
            </a>
          </div>

          <div className="rounded-[1rem] border border-[var(--line)] border-l-[3px] border-l-[var(--mustard)] bg-[var(--paper-warm)] p-4 text-sm text-[var(--ink-soft)]">
            {t("sensitiveKeyWarning")}
          </div>

          <div className="space-y-2">
            <p className="text-sm font-semibold text-[var(--portal-ink)]">{t("template")}</p>
            <div className="grid gap-2">
              {TEMPLATE_DEFINITIONS.map((templateDef) => {
                const isActive = templateDef.id === template;
                return (
                  <button
                    key={templateDef.id}
                    type="button"
                    className={`rounded-[1rem] border px-4 py-3 text-left transition-all duration-200 ${
                      isActive
                        ? "border-[var(--accent)]/40 bg-[var(--accent-wash)]"
                        : "border-[var(--portal-line)] bg-[var(--portal-clay-strong)] hover:-translate-y-[1px]"
                    }`}
                    onClick={() => onTemplateChange(templateDef.id)}
                  >
                    <p className="text-sm font-semibold text-[var(--portal-ink)]">{t(templateDef.labelKey)}</p>
                    <p className="mt-1 text-xs leading-5 text-[var(--portal-muted)]">{t(templateDef.helperKey)}</p>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-col p-5 sm:p-6">
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] pb-4">
          <div className="min-w-0">
            <p className="text-sm font-semibold text-[var(--accent)]">{t(templateDefinition.labelKey)}</p>
            <h3 className="mt-1 text-xl font-bold text-[var(--portal-ink)]">{t("renderedClientConfig")}</h3>
            <p className="mt-2 max-w-2xl text-sm text-[var(--portal-muted)]">{t(templateDefinition.helperKey)}</p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {templateDefinition.supportedFormats.map((formatOption) => (
              <button
                key={formatOption}
                type="button"
                className={`rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] transition-colors ${
                  format === formatOption
                    ? "border-[var(--accent)]/40 bg-[var(--accent-wash)] text-[var(--accent-ink)]"
                    : "border-[var(--portal-line)] bg-[var(--portal-clay)] text-[var(--portal-muted)]"
                }`}
                onClick={() => onFormatChange(formatOption)}
              >
                {formatOption}
              </button>
            ))}
          </div>
        </div>

        <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_220px]">
          <div className="min-w-0 rounded-[1.15rem] border border-[#0d0b06] bg-[#0d0b06] p-4 shadow-inner shadow-black/30">
            <pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-sm leading-6 text-[#bfe3c4]">
              <code>{renderedConfig}</code>
            </pre>
          </div>

          <div className="grid gap-3 self-start">
            <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("gatewayBaseUrl")}</p>
              <p className="mt-2 break-all text-sm font-semibold text-[var(--portal-ink)]">{GATEWAY_BASE_URL}</p>
            </div>

            <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("copy")}</p>
              <button type="button" className="btn-primary mt-3 w-full" onClick={() => void onCopy()} disabled={!userKey.trim()}>
                {t("copyRenderedConfig")}
              </button>
              <p className="mt-3 text-xs leading-5 text-[var(--portal-muted)]">
                {copyState === "copied"
                  ? t("copyCopied")
                  : copyState === "error"
                    ? t("copyError")
                    : t("copyIdle")}
              </p>
            </div>

            <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4 text-sm text-[var(--portal-muted)]">
              {userKey.trim() ? t("templateContentLive") : t("addKeyFirst")}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

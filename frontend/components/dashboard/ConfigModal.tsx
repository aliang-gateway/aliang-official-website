"use client";

import { useEffect, useRef, type RefObject } from "react";
import { useTranslations } from "next-intl";

import { TEMPLATE_DEFINITIONS, type TemplateDefinition } from "@/lib/dashboard-template";
import type { ClientTemplateId, TemplateFormat } from "@/lib/dashboard-types";

type ConfigModalProps = {
  isOpen: boolean;
  onClose: () => void;
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
  triggerRef: RefObject<HTMLButtonElement | null>;
};

export function ConfigModal({
  isOpen,
  onClose,
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
  triggerRef,
}: ConfigModalProps) {
  const t = useTranslations("dashboard");
  const modalRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const hadOpenRef = useRef(false);

  useEffect(() => {
    if (!isOpen) {
      if (hadOpenRef.current) {
        triggerRef.current?.focus();
        hadOpenRef.current = false;
      }
      return;
    }

    hadOpenRef.current = true;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const modal = modalRef.current;
      if (!modal) {
        return;
      }

      const focusable = modal.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) {
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey && activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, onClose, triggerRef]);

  if (!isOpen) {
    return null;
  }

  return (
    <section
      className="fixed inset-0 z-[70] flex items-center justify-center p-4 sm:p-6"
      role="dialog"
      aria-modal="true"
      aria-labelledby="dashboard-config-modal-title"
    >
      <button
        type="button"
        className="absolute inset-0 bg-slate-950/60 backdrop-blur-sm"
        aria-label={t("closeConfigModal")}
        onClick={onClose}
      />

      <div
        ref={modalRef}
        className="relative z-[1] flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-[1.4rem] border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] shadow-[var(--portal-shadow)]"
      >
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] px-5 py-4 sm:px-6">
          <div className="min-w-0 space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">{t("configModal")}</p>
            <h2 id="dashboard-config-modal-title" className="text-2xl font-bold text-[var(--portal-ink)]">
              {t("singleKeyFourTemplates")}
            </h2>
            <p className="max-w-2xl text-sm text-[var(--portal-muted)]">
              {t("configModalDescription")}
            </p>
          </div>
          <button
            type="button"
            ref={closeButtonRef}
            className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] text-xl font-semibold text-[var(--portal-ink)] transition-transform duration-200 hover:-translate-y-[1px]"
            aria-label={t("closeConfigModal")}
            onClick={onClose}
          >
            ×
          </button>
        </div>

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
                <p className="text-xs leading-5 text-[var(--portal-muted)]">
                  {t("keySourceDescription")}
                </p>
              </div>

              <div className="flex flex-wrap gap-3">
                <button type="button" className="btn-ghost" onClick={() => onUserKeyChange("")}>
                  {t("clearKey")}
                </button>
              </div>

              <div className="rounded-[1rem] border border-amber-400/40 bg-amber-50/80 p-4 text-sm text-amber-900 dark:bg-amber-500/10 dark:text-amber-200">
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
                            ? "border-emerald-500/40 bg-emerald-500/10 shadow-[0_12px_24px_rgba(16,185,129,0.12)]"
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
                <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t(templateDefinition.labelKey)}</p>
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
                        ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
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
              <div className="min-w-0 rounded-[1.15rem] border border-[var(--portal-line)] bg-slate-950 p-4 shadow-inner shadow-black/20">
                <pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-sm leading-6 text-emerald-100">
                  <code>{renderedConfig}</code>
                </pre>
              </div>

              <div className="grid gap-3 self-start">
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{t("gatewayBaseUrl")}</p>
                  <p className="mt-2 break-all text-sm font-semibold text-[var(--portal-ink)]">https://api.aliang.one</p>
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
                  {userKey.trim()
                    ? t("templateContentLive")
                    : t("addKeyFirst")}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

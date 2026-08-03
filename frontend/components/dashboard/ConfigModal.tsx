"use client";

import type { RefObject } from "react";
import { useTranslations } from "next-intl";

import { Modal } from "@/components/ui/Modal";
import { ConfigPanel } from "./ConfigPanel";
import type { TemplateDefinition } from "@/lib/dashboard-template";
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
  sessionToken: string;
};

/**
 * Dashboard quick-config modal. Thin wrapper around the reusable Modal +
 * ConfigPanel so the same config UI is shared with the /keys page's 配置 tab.
 */
export function ConfigModal({ isOpen, onClose, triggerRef, ...panelProps }: ConfigModalProps) {
  const t = useTranslations("dashboard");
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      closeLabel={t("closeConfigModal")}
      triggerRef={triggerRef}
      panelClassName="max-w-5xl"
    >
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--line)] bg-[var(--paper-warm)] px-5 py-4 pr-14 sm:px-6">
        <div className="min-w-0 space-y-2">
          <p
            className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]"
            style={{ fontFamily: "var(--font-editorial-mono)" }}
          >
            {t("configModal")}
          </p>
          <h2 className="text-2xl font-bold text-[var(--ink)]">{t("singleKeyFourTemplates")}</h2>
          <p className="max-w-2xl text-sm text-[var(--ink-muted)]">{t("configModalDescription")}</p>
        </div>
      </div>
      <ConfigPanel {...panelProps} />
    </Modal>
  );
}

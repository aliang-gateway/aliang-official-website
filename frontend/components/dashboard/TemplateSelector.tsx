"use client";

import { useTranslations } from "next-intl";

import { TEMPLATE_DEFINITIONS } from "@/lib/dashboard-template";
import type { ClientTemplateId } from "@/lib/dashboard-types";

type TemplateSelectorProps = {
  template: ClientTemplateId;
  onTemplateChange: (value: ClientTemplateId) => void;
};

/** 面板顶部全宽模板选择条:先选客户端(opencode/claude/codex)再配密钥。
 *  ConfigPanel 的两个消费方(dashboard 弹窗、/keys 配置 tab)共用。
 *  卡片样式沿用原左栏模板块,仅由纵列改为横排。 */
export function TemplateSelector({ template, onTemplateChange }: TemplateSelectorProps) {
  const t = useTranslations("dashboard");

  return (
    <div className="space-y-2">
      <p className="text-sm font-semibold text-[var(--portal-ink)]">{t("template")}</p>
      <div className="grid gap-2 sm:grid-cols-3">
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
  );
}

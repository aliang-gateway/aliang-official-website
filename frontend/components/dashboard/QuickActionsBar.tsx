"use client";

import type { RefObject } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { MaterialIcon } from "@/components/ui/MaterialIcon";

type QuickActionsBarProps = {
  onOpenTicket: () => void;
  /** 工单格 ref:modal 关闭后焦点还原(沿用 page 既有机制)。 */
  ticketTriggerRef: RefObject<HTMLButtonElement | null>;
};

const CELL_CLASS =
  "group flex min-w-0 items-center gap-3 rounded-xl px-4 py-3 text-left transition-colors hover:bg-[var(--accent-wash)]";

const ARROW_CLASS =
  "shrink-0 text-[var(--ink-faint)] transition-all duration-200 group-hover:translate-x-0.5 group-hover:text-[var(--accent)]";

/** Header 正下方的快捷入口条:配置密钥 / 工单反馈 / 历史记录。
 *  行为与原页面底部三格逐字一致(两个跳转 + 工单 modal 触发),仅位置上移、形态改紧凑 pill。 */
export function QuickActionsBar({ onOpenTicket, ticketTriggerRef }: QuickActionsBarProps) {
  const t = useTranslations("dashboard");

  return (
    <nav className="clay-panel p-2">
      <div className="grid divide-y divide-[var(--line)] sm:grid-cols-3 sm:divide-x sm:divide-y-0">
        <Link href="/keys" className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="key" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("configApiKey")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </Link>

        <button ref={ticketTriggerRef} type="button" onClick={onOpenTicket} className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="support_agent" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("ticketFeedback")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </button>

        <Link href="/dashboard/details" className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="insights" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("details")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </Link>
      </div>
    </nav>
  );
}

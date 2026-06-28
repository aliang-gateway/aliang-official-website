"use client";

import type { RefObject } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

type ConfigEntryCardProps = {
  onOpen: () => void;
  triggerRef: RefObject<HTMLButtonElement | null>;
};

export function ConfigEntryCard({ onOpen, triggerRef }: ConfigEntryCardProps) {
  const t = useTranslations("dashboard");

  return (
    <article className="block-card min-w-0 space-y-4">
      <div>
        <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("configApiKey")}</p>
        <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("clientSetupEntry")}</h2>
        <p className="mt-2 text-sm text-[var(--portal-muted)]">
          {t("configDescription")}
        </p>
      </div>
      <div className="rounded-[1rem] border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
        {t("configHint")}
      </div>
      <div className="flex flex-wrap gap-3">
        <button type="button" className="btn-primary" ref={triggerRef} onClick={onOpen}>
          {t("openConfigSetup")}
        </button>
        <Link href="/account" className="btn-ghost inline-flex items-center justify-center no-underline">
          {t("manageSessionKeys")}
        </Link>
      </div>
    </article>
  );
}

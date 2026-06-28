"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

export function DetailsLinkCard() {
  const t = useTranslations("dashboard");

  return (
    <article className="block-card min-w-0 space-y-4">
      <div>
        <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("details")}</p>
        <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("openDeeperRecords")}</h2>
        <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("detailsDescription")}</p>
      </div>
      <Link href="/dashboard/details" className="btn-primary inline-flex w-fit items-center justify-center no-underline">
        {t("goToDetailsPage")}
      </Link>
    </article>
  );
}

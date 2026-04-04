"use client";

import { useLocale, useTranslations } from "next-intl";

export function LanguageSwitcher() {
  const t = useTranslations("languageSwitcher");
  const currentLocale = useLocale();

  const toggleLocale = () => {
    const newLocale = currentLocale === "zh" ? "en" : "zh";
    document.cookie = `NEXT_LOCALE=${newLocale}; path=/; max-age=31536000`;
    window.location.reload();
  };

  const label = currentLocale === "zh" ? t("en") : t("zh");

  return (
    <button
      type="button"
      onClick={toggleLocale}
      className="flex size-9 items-center justify-center rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] text-xs font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)] hover:text-[var(--stitch-primary)]"
      aria-label="Switch language"
    >
      {label}
    </button>
  );
}

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { LanguageSwitcher } from "@/components/ui/LanguageSwitcher";
import { useSessionProfile } from "@/lib/useSessionProfile";

export function EditorialHeader() {
  const t = useTranslations("editorial");
  const h = useTranslations("header");
  const sv = useTranslations("editorial.services");
  const dl = useTranslations("editorial.download");
  const pathname = usePathname();
  const { isLoggedIn } = useSessionProfile();

  const isServices = pathname === "/services" || pathname.startsWith("/services/");
  const isDownload = pathname === "/download" || pathname.startsWith("/download/");
  const topbarCtx = isServices ? sv("topbarCtx") : isDownload ? dl("topbarCtx") : t("topbarLead");
  const topbarMid = (
    isServices ? sv.raw("topbarMid") : isDownload ? dl.raw("topbarMid") : t.raw("topbarMid")
  ) as string[];

  const pages = [
    { href: "/", label: t("navHome"), num: "01", exact: true },
    { href: "/services", label: t("navServices"), num: "02", exact: false },
    { href: "/download", label: t("navDownload"), num: "03", exact: false },
    { href: "/docs", label: t("navDoc"), num: "04", exact: false },
  ];

  const isCurrent = (href: string, exact: boolean) =>
    exact ? pathname === href : pathname === href || pathname.startsWith(`${href}/`);

  const isAuthPage = pathname === "/login" || pathname === "/register";
  const cta = isLoggedIn
    ? { href: "/dashboard", label: h("dashboard") }
    : isAuthPage
      ? { href: "/", label: t("auth.backHome") }
      : { href: "/login", label: h("login") };

  return (
    <>
      <div className="topbar">
        <div className="container wide topbar-inner">
          <span>
            <b>{t("brandName")}</b> / {topbarCtx}
          </span>
          <span className="mid">
            {topbarMid.map((item) => (
              <span key={item}>{item}</span>
            ))}
          </span>
          <span className="right">
            <span>
              <i className="pulse" /> {t("statusLive")}
            </span>
            <span>
              {t("edition")} / {t("year")}
            </span>
          </span>
        </div>
      </div>

      <nav className="nav" aria-label={t("navAria")}>
        <div className="container wide nav-inner">
          <Link className="brand" href="/" aria-label={t("brandAria")}>
            <span className="brand-mark">{t("brandMark")}</span>
            <span className="brand-text">
              {t("brandName")}
              <small>{t("brandSmall")}</small>
            </span>
          </Link>

          <div className="page-links" aria-label={t("pagesAria")}>
            {pages.map((page) => (
              <Link
                key={page.href}
                href={page.href}
                aria-current={isCurrent(page.href, page.exact) ? "page" : undefined}
              >
                <span className="num">{page.num}</span>
                {page.label}
              </Link>
            ))}
          </div>

          <div className="nav-actions">
            <LanguageSwitcher />
            <Link className="nav-cta" href={cta.href}>
              {cta.label}
            </Link>
          </div>
        </div>
      </nav>
    </>
  );
}

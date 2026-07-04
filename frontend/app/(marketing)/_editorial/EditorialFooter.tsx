"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

export function EditorialFooter() {
  const t = useTranslations("editorial");
  const nav = useTranslations("header");

  const siteLinks = [
    { href: "/", label: t("navHome") },
    { href: "/services", label: t("navServices") },
    { href: "/about", label: t("navAbout") },
  ];

  const resourceLinks = [
    { href: "/docs", label: nav("document") },
    { href: "/dashboard", label: nav("dashboard") },
    { href: "/login", label: nav("login") },
  ];

  const companyLinks = [
    { href: "/about", label: t("navAbout") },
    { href: "/security", label: t("footerSecurity") },
    { href: "/privacy", label: t("footerPrivacy") },
    { href: "/terms", label: t("footerTerms") },
  ];

  const products = [
    { label: "VibeCoding", href: "/download" },
    { label: "Cursor", href: "/download" },
    { label: "VS Code", href: "/download" },
    { label: "Claude Code", href: "/download" },
  ];

  return (
    <footer className="footer">
      <div className="container wide">
        <div className="footer-grid">
          <div>
            <h4>{t("brandName")}</h4>
            <p>{t("footerAbout")}</p>
          </div>
          <div>
            <h4>{t("footerSite")}</h4>
            <ul>
              {siteLinks.map((l) => (
                <li key={l.href}>
                  <Link href={l.href}>{l.label}</Link>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h4>{t("footerResources")}</h4>
            <ul>
              {resourceLinks.map((l) => (
                <li key={l.href}>
                  <Link href={l.href}>{l.label}</Link>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h4>{t("footerProduct")}</h4>
            <ul>
              {products.map((p) => (
                <li key={p.label}>
                  <Link href={p.href}>{p.label}</Link>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h4>{t("footerCompany")}</h4>
            <ul>
              {companyLinks.map((l) => (
                <li key={l.href}>
                  <Link href={l.href}>{l.label}</Link>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <div className="footer-mega display">
          {t("footerMegaPre")} <em>{t("footerMegaEm")}</em> {t("footerMegaPost")}
          <span className="dot">.</span>
        </div>
      </div>
    </footer>
  );
}

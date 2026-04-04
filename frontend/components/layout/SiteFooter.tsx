"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useTranslations } from "next-intl";

type FooterGroup = {
  title: string;
  links: Array<{ href: string; label: string }>;
};

function FooterBrand({ title, description }: { title: string; description: string }) {
  return (
    <div className="col-span-1 space-y-4 md:col-span-1">
      <div className="flex items-center gap-3">
        <div className="flex size-6 items-center justify-center rounded bg-[var(--stitch-primary)] text-white">
          <MaterialIcon name="hub" size={14} />
        </div>
        <h2 className="text-lg font-bold text-[var(--stitch-text)]">{title}</h2>
      </div>
      <p className="text-sm text-[var(--stitch-text-muted)]">{description}</p>
    </div>
  );
}

function FooterColumns({ groups }: { groups: FooterGroup[] }) {
  return (
    <>
      {groups.map((group) => (
        <div key={group.title}>
          <h4 className="mb-6 text-xs font-bold uppercase tracking-widest text-[var(--stitch-text)]">
            {group.title}
          </h4>
          <ul className="space-y-4 text-sm text-[var(--stitch-text-muted)]">
            {group.links.map((link) => (
              <li key={link.label}>
                <Link href={link.href} className="transition-colors hover:text-[var(--stitch-primary)]">
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </>
  );
}

export function SiteFooter() {
  const pathname = usePathname();
  const t = useTranslations("footer");

  const isBlog = pathname.startsWith("/blog");
  const isServices = pathname === "/services";
  const isCompactHome = pathname === "/compact";

  if (isBlog) {
    const groups: FooterGroup[] = [
      {
        title: t("blog.resourcesTitle"),
        links: [
          { href: "/docs", label: t("blog.documentation") },
          { href: "#", label: t("blog.apiReference") },
          { href: "#", label: t("blog.communityForum") },
          { href: "#", label: t("blog.openSource") },
        ],
      },
      {
        title: t("blog.platformTitle"),
        links: [
          { href: "#", label: t("blog.aiRoutingEngine") },
          { href: "#", label: t("blog.tunProxyService") },
          { href: "#", label: t("blog.edgeLocations") },
          { href: "#", label: t("blog.statusPage") },
        ],
      },
      {
        title: t("blog.companyTitle"),
        links: [
          { href: "#", label: t("blog.aboutUs") },
          { href: "#", label: t("blog.careers") },
          { href: "#", label: t("blog.securityPolicy") },
          { href: "#", label: t("blog.contact") },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-6 py-12 md:px-20">
        <div className="mx-auto grid max-w-7xl grid-cols-1 gap-12 md:grid-cols-4">
          <FooterBrand
            title={t("blog.brandTitle")}
            description={t("blog.brandDescription")}
          />
          <FooterColumns groups={groups} />
        </div>

        <div className="mx-auto mt-12 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] pt-8 md:flex-row">
          <p className="text-xs text-[var(--stitch-text-muted)]">{t("blog.copyright")}</p>
          <div className="flex gap-6 text-[var(--stitch-text-muted)]">
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="share" size={20} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="terminal" size={20} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
          </div>
        </div>
      </footer>
    );
  }

  if (isServices) {
    const groups: FooterGroup[] = [
      {
        title: t("services.resourcesTitle"),
        links: [
          { href: "/docs", label: t("services.documentation") },
          { href: "#", label: t("services.apiReference") },
          { href: "#", label: t("services.communityForum") },
          { href: "#", label: t("services.statusPage") },
        ],
      },
      {
        title: t("services.platformTitle"),
        links: [
          { href: "#", label: t("services.macosClient") },
          { href: "#", label: t("services.windowsClient") },
          { href: "#", label: t("services.linuxClient") },
          { href: "#", label: t("services.browserExtension") },
        ],
      },
      {
        title: t("services.companyTitle"),
        links: [
          { href: "#", label: t("services.aboutUs") },
          { href: "#", label: t("services.careers") },
          { href: "#", label: t("services.privacyPolicy") },
          { href: "#", label: t("services.termsOfService") },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-4 py-16">
        <div className="mx-auto grid max-w-7xl grid-cols-2 gap-12 md:grid-cols-4 lg:grid-cols-5">
          <div className="col-span-2">
            <div className="mb-6 flex items-center gap-2">
              <MaterialIcon name="hub" size={28} className="text-[var(--stitch-primary)]" />
              <span className="text-xl font-bold tracking-tight text-[var(--stitch-text)]">{t("services.brandTitle")}</span>
            </div>
            <p className="mb-6 max-w-xs text-sm leading-relaxed text-[var(--stitch-text-muted)]">
              {t("services.brandDescription")}
            </p>
            <div className="flex gap-4 text-[var(--stitch-text-muted)]">
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="language" size={20} /></Link>
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="share" size={20} /></Link>
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
            </div>
          </div>
          <FooterColumns groups={groups} />
        </div>
        <div className="mx-auto mt-16 max-w-7xl border-t border-[var(--stitch-border)] pt-8 text-center text-xs text-[var(--stitch-text-muted)]">
          {t("services.copyright")}
        </div>
      </footer>
    );
  }

  if (isCompactHome) {
    const groups: FooterGroup[] = [
      {
        title: t("compact.productTitle"),
        links: [
          { href: "/#features", label: t("compact.features") },
          { href: "/services", label: t("compact.pricing") },
        ],
      },
      {
        title: t("compact.resourcesTitle"),
        links: [
          { href: "/docs", label: t("compact.documentation") },
          { href: "/blog", label: t("compact.blog") },
        ],
      },
      {
        title: t("compact.companyTitle"),
        links: [
          { href: "#", label: t("compact.security") },
          { href: "#", label: t("compact.privacy") },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-8">
        <div className="mx-auto grid max-w-7xl grid-cols-1 gap-8 px-6 md:grid-cols-4 md:px-20">
          <div className="col-span-1 space-y-3">
            <div className="flex items-center gap-3">
              <div className="flex size-5 items-center justify-center rounded bg-[var(--stitch-primary)] text-white">
                <MaterialIcon name="hub" size={10} />
              </div>
              <h2 className="text-base font-bold text-[var(--stitch-text)]">{t("compact.brandTitle")}</h2>
            </div>
            <p className="text-xs text-[var(--stitch-text-muted)]">{t("compact.brandDescription")}</p>
          </div>
          {groups.map((group) => (
            <div key={group.title}>
              <h4 className="mb-4 text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text)]">{group.title}</h4>
              <ul className="space-y-2 text-xs text-[var(--stitch-text-muted)]">
                {group.links.map((link) => (
                  <li key={link.label}>
                    <Link href={link.href} className="transition-colors hover:text-[var(--stitch-primary)]">
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mx-auto mt-8 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] px-6 pt-6 md:flex-row md:px-20">
          <p className="text-[10px] text-[var(--stitch-text-muted)]">{t("compact.copyright")}</p>
          <div className="flex gap-4 text-[var(--stitch-text-muted)]">
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={16} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="alternate_email" size={16} /></Link>
          </div>
        </div>
      </footer>
    );
  }

  const groups: FooterGroup[] = [
    {
      title: t("default.productTitle"),
      links: [
        { href: "/#features", label: t("default.features") },
        { href: "/#integrations", label: t("default.integrations") },
        { href: "/services", label: t("default.pricing") },
        { href: "#", label: t("default.apiReference") },
      ],
    },
    {
      title: t("default.resourcesTitle"),
      links: [
        { href: "/docs", label: t("default.documentation") },
        { href: "/blog", label: t("default.blog") },
        { href: "#", label: t("default.support") },
        { href: "#", label: t("default.status") },
      ],
    },
    {
      title: t("default.companyTitle"),
      links: [
        { href: "#", label: t("default.about") },
        { href: "#", label: t("default.security") },
        { href: "#", label: t("default.privacy") },
        { href: "#", label: t("default.terms") },
      ],
    },
  ];

  return (
    <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-12">
      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-12 px-6 md:grid-cols-4 md:px-20">
        <FooterBrand
          title={t("default.brandTitle")}
          description={t("default.brandDescription")}
        />
        <FooterColumns groups={groups} />
      </div>

      <div className="mx-auto mt-12 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] px-6 pt-8 md:flex-row md:px-20">
        <p className="text-xs text-[var(--stitch-text-muted)]">{t("default.copyright")}</p>
        <div className="flex gap-6 text-[var(--stitch-text-muted)]">
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="alternate_email" size={20} /></Link>
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="rss_feed" size={20} /></Link>
        </div>
      </div>
    </footer>
  );
}

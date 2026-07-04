"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";

type DocsSidebarCategory = {
  slug: string;
  title: string;
  pages: { slug: string; title: string }[];
};

type DocsSidebarProps = {
  categories: DocsSidebarCategory[];
  activeSlug?: string;
};

/**
 * Editorial docs sidebar: category groups (mono labels + accent active state)
 * and a help card. Active page is derived from the URL unless `activeSlug` is
 * supplied. Renders inside the `.editorial` wrapper.
 */
export function DocsSidebar({ categories, activeSlug }: DocsSidebarProps) {
  const t = useTranslations("editorial.docs");
  const pathname = usePathname();
  const computedActive =
    activeSlug ?? (pathname?.startsWith("/docs/") ? pathname.split("/")[2] : undefined);

  return (
    <aside className="docs-side" aria-label="Documentation navigation">
      {categories.map((category) => (
        <div key={category.slug}>
          <div className="docs-cat-title">{category.title}</div>
          <ul>
            {category.pages.map((page) => {
              const isActive = page.slug === computedActive;
              return (
                <li key={page.slug}>
                  <Link href={`/docs/${page.slug}`} aria-current={isActive ? "page" : undefined}>
                    {page.title}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}

      <div className="content-card docs-help">
        <p className="docs-help-title">{t("sidebarHelpTitle")}</p>
        <p className="docs-help-body">{t("sidebarHelpBody")}</p>
        <Link href="/register" className="btn primary">
          {t("sidebarHelpCta")}
        </Link>
      </div>
    </aside>
  );
}

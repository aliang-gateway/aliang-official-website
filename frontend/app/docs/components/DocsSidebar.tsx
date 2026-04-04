"use client";

import { useState } from "react";
import Link from "next/link";

type DocsSidebarCategory = {
  slug: string;
  title: string;
  pages: { slug: string; title: string }[];
};

type DocsSidebarProps = {
  categories: DocsSidebarCategory[];
  activeSlug?: string;
};

export function DocsSidebar({ categories, activeSlug }: DocsSidebarProps) {
  const [mobileOpen, setMobileOpen] = useState(false);

  const navContent = (
    <nav className="flex flex-col gap-8" aria-label="Documentation navigation">
      {categories.map((category) => (
        <div key={category.slug}>
          <h3 className="mb-3 text-xs font-bold uppercase tracking-wider text-[var(--stitch-text-muted)]">
            {category.title}
          </h3>
          <ul className="flex flex-col gap-2">
            {category.pages.map((page) => {
              const isActive = page.slug === activeSlug;
              return (
                <li key={page.slug}>
                  <Link
                    href={`/docs/${page.slug}`}
                    aria-current={isActive ? "page" : undefined}
                    className={
                      isActive
                        ? "text-sm font-semibold text-[var(--stitch-primary)]"
                        : "text-sm text-[var(--stitch-text-muted)] transition-colors hover:text-[var(--stitch-text)]"
                    }
                  >
                    {page.title}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}

      <div className="rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3">
        <p className="text-xs font-semibold text-[var(--stitch-text)]">
          Need account setup help?
        </p>
        <p className="mt-1 text-xs leading-5 text-[var(--stitch-text-muted)]">
          Start from the account page and issue your first API key in under 2
          minutes.
        </p>
        <Link
          href="/register"
          className="mt-3 inline-flex rounded-lg bg-[var(--stitch-primary)] px-3 py-1.5 text-xs font-semibold text-white transition-opacity hover:opacity-90"
        >
          Open Account
        </Link>
      </div>
    </nav>
  );

  return (
    <>
      {/* Mobile toggle button */}
      <button
        type="button"
        className="mb-4 inline-flex items-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-4 py-2 text-sm font-medium text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)] lg:hidden"
        onClick={() => setMobileOpen((prev) => !prev)}
        aria-expanded={mobileOpen}
        aria-controls="docs-sidebar-nav"
      >
        <svg
          aria-hidden="true"
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          {mobileOpen ? (
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          ) : (
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 6h16M4 12h16M4 18h16"
            />
          )}
        </svg>
        {mobileOpen ? "Hide Menu" : "Show Menu"}
      </button>

      {/* Mobile sidebar (collapsible) */}
      {mobileOpen && (
        <aside className="mb-6 rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-5 lg:hidden">
          {navContent}
        </aside>
      )}

      {/* Desktop sidebar (always visible on lg+) */}
      <aside className="sticky top-24 hidden h-fit w-64 shrink-0 self-start rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-5 lg:block">
        {navContent}
      </aside>
    </>
  );
}

"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import ThemeToggle from "@/app/components/ThemeToggle";
import { MobileMenu } from "@/components/ui/MobileMenu";
import { cn } from "@/lib/utils";

export function SiteHeader() {
  const pathname = usePathname();
  const activePath = pathname;
  const showSearch = pathname === "/blog";
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const isHome = pathname === "/";
  const isCompactHome = pathname === "/compact";
  const isServices = pathname === "/services";
  const navLinks = useMemo(
    () =>
      isHome
        ? [
            { href: "/#features", label: "Features" },
            { href: "/#integrations", label: "Integrations" },
            { href: "/docs", label: "Documentation" },
            { href: "/services", label: "Pricing" },
          ]
        : [
            { href: "/blog", label: "Blog" },
            { href: "/docs", label: "Document" },
            { href: "/services", label: "Service" },
          ],
    [isHome]
  );

  const brandLabel = isHome ? "ALiang AI Services" : "ALiang Gateway";
  const primaryCta = isHome
    ? { href: "/register", label: "Get Started" }
    : { href: "/login", label: "Login" };
  const secondaryCta = isHome ? { href: "/login", label: "Login" } : undefined;

  const isLinkActive = (href: string) => {
    // Hash links on home page should NOT be pre-highlighted
    if (href.startsWith("/#")) {
      return false;
    }
    // Blog and services pages should NOT show active nav highlighting
    if (pathname === "/blog" || pathname === "/services") {
      return false;
    }
    return activePath === href;
  };

  return (
    <>
      <header className={cn(
        "sticky top-0 z-50 flex items-center justify-between whitespace-nowrap border-b border-[var(--stitch-border)] px-6 backdrop-blur-md md:px-20",
        isHome
          ? "bg-[var(--stitch-bg)]/95 py-4"
          : isCompactHome
            ? "bg-[var(--stitch-bg)]/95 py-3"
            : "bg-[var(--stitch-bg)]/80 py-4"
      )}>
      <div className="flex items-center gap-8">
        <Link href="/" className="flex items-center gap-3">
          {isServices ? (
            <MaterialIcon name="hub" size={28} className="text-[var(--stitch-primary)]" />
          ) : (
            <div
              className={cn(
                "flex items-center justify-center bg-[var(--stitch-primary)] text-white",
                isHome ? "size-8 rounded" : isCompactHome ? "size-7 rounded" : "size-8 rounded-lg"
              )}
            >
              <MaterialIcon name="hub" size={isCompactHome ? 18 : 20} />
            </div>
          )}
          <h2
            className={cn(
              "font-bold leading-tight tracking-tight text-[var(--stitch-text)]",
              isCompactHome ? "text-lg" : "text-xl"
            )}
          >
            {brandLabel}
          </h2>
        </Link>

        <nav className="hidden items-center gap-8 md:flex">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={cn(
                `text-sm ${isServices || isHome || isCompactHome ? "font-medium" : "font-semibold"} transition-colors`,
                isLinkActive(link.href)
                  ? "text-[var(--stitch-primary)]"
                  : "text-[var(--stitch-text-muted)] hover:text-[var(--stitch-primary)]"
              )}
            >
              {link.label}
            </Link>
          ))}
        </nav>
      </div>

      <div className="flex flex-1 items-center justify-end gap-3 md:gap-6">
        {showSearch && (
          <label className="hidden lg:flex items-center relative min-w-40 max-w-64">
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              className="absolute left-3 size-[18px] text-[var(--stitch-text-muted)]"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <circle cx="11" cy="11" r="7" />
              <line x1="16.65" y1="16.65" x2="21" y2="21" />
            </svg>
            <input
              className="w-full rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-2 pl-10 pr-4 text-sm text-[var(--stitch-text)] outline-none transition-all placeholder:text-[var(--stitch-text-muted)] focus:border-[var(--stitch-primary)] focus:ring-1 focus:ring-[var(--stitch-primary)]"
              placeholder="Search architecture..."
              type="search"
            />
          </label>
        )}
        <ThemeToggle />
        <Link
          href={primaryCta.href}
          className={cn(
            "hidden h-10 cursor-pointer items-center justify-center rounded bg-[var(--stitch-primary)] text-sm font-bold text-white shadow-sm transition-all hover:bg-[var(--stitch-primary)]/90 md:flex",
            isHome ? "min-w-[100px] px-4" : isCompactHome ? "h-9 min-w-[80px] px-4" : "min-w-[100px] px-6 rounded-lg"
          )}
        >
          {primaryCta.label}
        </Link>
        {secondaryCta && (
          <Link
            href={secondaryCta.href}
            className="hidden h-10 min-w-[80px] items-center justify-center rounded border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-4 text-sm font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)] md:flex"
          >
            {secondaryCta.label}
          </Link>
        )}
        <button
          type="button"
          onClick={() => setIsMobileMenuOpen(true)}
          className="flex size-9 items-center justify-center rounded md:hidden text-[var(--stitch-text)]"
          aria-label="Open menu"
        >
          <MaterialIcon name="menu" size={24} />
        </button>
      </div>
      </header>
      <MobileMenu
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        activePath={activePath}
        links={navLinks}
        primaryAction={primaryCta}
        secondaryAction={secondaryCta}
      />
    </>
  );
}

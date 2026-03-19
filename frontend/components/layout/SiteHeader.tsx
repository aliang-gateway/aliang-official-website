"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
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
    ? { href: "/account", label: "Get Started" }
    : { href: "/account", label: "Login" };
  const secondaryCta = isHome ? { href: "/account", label: "Login" } : undefined;

  const isLinkActive = (href: string) => {
    if (href.startsWith("/#")) {
      return pathname === "/";
    }
    return activePath === href;
  };

  return (
    <>
    <header className={cn(
      "sticky top-0 z-50 flex items-center justify-between whitespace-nowrap border-b border-[var(--stitch-border)] px-6 backdrop-blur-md md:px-20",
      isHome
        ? "bg-white py-4 dark:bg-slate-900"
        : isCompactHome
          ? "bg-white py-3 dark:bg-slate-900"
          : "bg-white/80 py-4 dark:bg-[var(--stitch-bg)]/80"
    )}>
      <div className="flex items-center gap-8">
        <Link href="/" className="flex items-center gap-3">
          {isServices ? (
            <MaterialIcon name="hub" size={28} className="text-[var(--stitch-primary)]" />
          ) : (
            <div
              className={cn(
                "flex items-center justify-center bg-[var(--stitch-primary)] text-white",
                isCompactHome ? "size-7 rounded" : "size-8 rounded-lg"
              )}
            >
              <MaterialIcon name="hub" size={isCompactHome ? 18 : 20} />
            </div>
          )}
          <h2 className={cn(
            "font-bold leading-tight tracking-tight text-[var(--stitch-text)]",
            isCompactHome ? "text-lg" : "text-xl"
          )}>
            {brandLabel}
          </h2>
        </Link>

        <nav className="hidden items-center gap-8 md:flex">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={cn(
                `text-sm ${isHome || isCompactHome ? "font-medium" : "font-semibold"} transition-colors`,
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
            <MaterialIcon name="search" size={18} className="absolute left-3 text-[var(--stitch-text-muted)]" />
            <input
              className="w-full rounded-lg border border-[var(--stitch-border)] bg-slate-50 py-2 pl-10 pr-4 text-sm outline-none transition-all focus:border-[var(--stitch-primary)] focus:ring-1 focus:ring-[var(--stitch-primary)] dark:bg-slate-800"
              placeholder="Search architecture..."
              type="search"
            />
          </label>
        )}
        <Link
          href={primaryCta.href}
          className={cn(
            "hidden h-10 cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] text-sm font-bold text-white shadow-sm transition-all hover:bg-[var(--stitch-primary)]/90 md:flex",
            isHome ? "min-w-[100px] px-4" : isCompactHome ? "h-9 min-w-[80px] rounded px-4" : "min-w-[100px] px-6"
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

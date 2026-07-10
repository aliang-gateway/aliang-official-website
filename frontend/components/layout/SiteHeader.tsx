"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import ThemeToggle from "@/app/components/ThemeToggle";
import { MobileMenu } from "@/components/ui/MobileMenu";
import { LanguageSwitcher } from "@/components/ui/LanguageSwitcher";
import { cn } from "@/lib/utils";
import {
  useSessionProfile,
  hasAdminAccess,
  adminEntryHref,
} from "@/lib/useSessionProfile";

export function SiteHeader() {
  const t = useTranslations("header");
  const pathname = usePathname();
  const activePath = pathname;
  const showSearch = pathname === "/blog";
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [avatarMenuOpen, setAvatarMenuOpen] = useState(false);
  const avatarMenuRef = useRef<HTMLDivElement>(null);

  const { user, logout, avatarLabel } = useSessionProfile();
  const isLoggedIn = user !== null;

  const isServices = pathname === "/services";

  const primaryCta = isLoggedIn
    ? { href: "/dashboard", label: t("dashboard") }
    : { href: "/login", label: t("login") };
  const adminCta = user && hasAdminAccess(user.role)
    ? { href: adminEntryHref(user.role), label: user.role === "distributor" ? t("distributor") : t("admin") }
    : undefined;

  const navLinks = [
    { href: "/docs", label: t("document") },
    { href: "/services", label: t("service") },
  ];

  useEffect(() => {
    if (!avatarMenuOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (avatarMenuRef.current && !avatarMenuRef.current.contains(e.target as Node)) {
        setAvatarMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [avatarMenuOpen]);

  const handleLogout = async () => {
    setAvatarMenuOpen(false);
    await logout();
  };

  const isLinkActive = (href: string) => {
    if (pathname === "/blog" || pathname === "/services") {
      return false;
    }
    return activePath === href;
  };

  return (
    <>
      <header className="sticky top-0 z-50 flex items-center justify-between whitespace-nowrap border-b border-[var(--stitch-border)] bg-[var(--stitch-bg)]/80 px-6 py-4 backdrop-blur-md md:px-20">
        <div className="flex items-center gap-8">
          <Link href="/" className="flex items-center gap-3">
            {isServices ? (
              <MaterialIcon name="hub" size={28} className="text-[var(--stitch-primary)]" />
            ) : (
              <div className="flex size-8 items-center justify-center rounded-lg bg-[var(--stitch-primary)] text-white">
                <MaterialIcon name="hub" size={20} />
              </div>
            )}
            <h2 className="font-bold text-xl leading-tight tracking-tight text-[var(--stitch-text)]">
              ALiang Gateway
            </h2>
          </Link>

          <nav className="hidden items-center gap-8 md:flex">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={cn(
                  "text-sm font-semibold transition-colors",
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
                placeholder={t("searchPlaceholder")}
                type="search"
              />
            </label>
          )}
          <LanguageSwitcher />
          <ThemeToggle />

          {isLoggedIn ? (
            <div ref={avatarMenuRef} className="relative hidden md:block">
              <button
                type="button"
                onClick={() => setAvatarMenuOpen(!avatarMenuOpen)}
                className="flex size-10 items-center justify-center rounded-full bg-[var(--stitch-primary)] text-[11px] font-bold text-white shadow-sm transition-all hover:ring-2 hover:ring-[var(--stitch-primary)]/40"
                aria-label={t("userMenu")}
              >
                {avatarLabel}
              </button>
              {avatarMenuOpen && (
                <div className="absolute right-0 top-full mt-2 w-56 overflow-hidden rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-1 shadow-lg">
                  <div className="border-b border-[var(--stitch-border)] px-4 py-3">
                    <p className="text-sm font-semibold text-[var(--stitch-text)] truncate">{user.name || user.email}</p>
                    <p className="text-xs text-[var(--stitch-text-muted)] truncate">{user.email}</p>
                  </div>
                  {hasAdminAccess(user.role) && (
                    <Link
                      href={adminEntryHref(user.role)}
                      onClick={() => setAvatarMenuOpen(false)}
                      className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                    >
                      <MaterialIcon name="admin_panel_settings" size={18} />
                      {user.role === "distributor" ? t("distributor") : t("admin")}
                    </Link>
                  )}
                  <Link
                    href="/dashboard"
                    onClick={() => setAvatarMenuOpen(false)}
                    className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="dashboard" size={18} />
                    {t("dashboard")}
                  </Link>
                  <Link
                    href="/account"
                    onClick={() => setAvatarMenuOpen(false)}
                    className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="person" size={18} />
                    {t("account")}
                  </Link>
                  <button
                    type="button"
                    onClick={handleLogout}
                    className="flex w-full items-center gap-3 px-4 py-2.5 text-sm text-red-500 transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="logout" size={18} />
                    {t("logout")}
                  </button>
                </div>
              )}
            </div>
          ) : (
            <Link
              href={primaryCta.href}
              className="hidden h-10 min-w-[100px] cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] px-6 text-sm font-bold text-white shadow-sm transition-all hover:bg-[var(--stitch-primary)]/90 md:flex"
            >
              {primaryCta.label}
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
        secondaryAction={adminCta}
      />
    </>
  );
}

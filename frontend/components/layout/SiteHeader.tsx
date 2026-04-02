"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import ThemeToggle from "@/app/components/ThemeToggle";
import { MobileMenu } from "@/components/ui/MobileMenu";
import { cn } from "@/lib/utils";

const SESSION_TOKEN_KEY = "session_token";

const navLinks = [
  { href: "/blog", label: "Blog" },
  { href: "/docs", label: "Document" },
  { href: "/services", label: "Service" },
];

type UserProfile = {
  email: string;
  name: string;
  role: string;
};

function buildAvatarLabel(user: UserProfile | null) {
  if (!user) {
    return "??";
  }

  const source = (user.name || user.email.split("@")[0] || "").trim();
  if (!source) {
    return "??";
  }

  const compact = source.replace(/\s+/g, "");
  return Array.from(compact).slice(0, 2).join("").toUpperCase() || "??";
}

export function SiteHeader() {
  const router = useRouter();
  const pathname = usePathname();
  const activePath = pathname;
  const showSearch = pathname === "/blog";
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [user, setUser] = useState<UserProfile | null>(null);
  const [avatarMenuOpen, setAvatarMenuOpen] = useState(false);
  const avatarMenuRef = useRef<HTMLDivElement>(null);

  const isServices = pathname === "/services";

  const isLoggedIn = user !== null;
  const primaryCta = isLoggedIn
    ? { href: "/dashboard", label: "Dashboard" }
    : { href: "/login", label: "Login" };

  useEffect(() => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_KEY);
    if (!sessionToken) return;

    fetch("/api/auth/me", {
      method: "GET",
      headers: {
        "content-type": "application/json",
        accept: "application/json",
        Authorization: `Bearer ${sessionToken}`,
      },
      cache: "no-store",
    })
      .then((res) => {
        if (!res.ok) {
          localStorage.removeItem(SESSION_TOKEN_KEY);
          return null;
        }
        return res.json();
      })
      .then((data) => {
        if (!data) return;
        const profile = data?.data ?? data;
        if (profile?.email) {
          setUser({ email: profile.email, name: profile.name ?? "", role: profile.role ?? "user" });
        } else {
          localStorage.removeItem(SESSION_TOKEN_KEY);
        }
      })
      .catch(() => {
        localStorage.removeItem(SESSION_TOKEN_KEY);
      });
  }, [pathname]);

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
    const sessionToken = localStorage.getItem(SESSION_TOKEN_KEY);
    try {
      await fetch("/api/auth/logout", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          ...(sessionToken ? { Authorization: `Bearer ${sessionToken}` } : {}),
        },
        cache: "no-store",
      });
    } catch {}
    localStorage.removeItem(SESSION_TOKEN_KEY);
    setUser(null);
    router.replace("/login");
  };

  const isLinkActive = (href: string) => {
    if (pathname === "/blog" || pathname === "/services") {
      return false;
    }
    return activePath === href;
  };

  const avatarLabel = buildAvatarLabel(user);

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
                placeholder="Search architecture..."
                type="search"
              />
            </label>
          )}
          <ThemeToggle />

          {isLoggedIn ? (
            <div ref={avatarMenuRef} className="relative hidden md:block">
              <button
                type="button"
                onClick={() => setAvatarMenuOpen(!avatarMenuOpen)}
                className="flex size-10 items-center justify-center rounded-full bg-[var(--stitch-primary)] text-[11px] font-bold text-white shadow-sm transition-all hover:ring-2 hover:ring-[var(--stitch-primary)]/40"
                aria-label="User menu"
              >
                {avatarLabel}
              </button>
              {avatarMenuOpen && (
                <div className="absolute right-0 top-full mt-2 w-56 overflow-hidden rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-1 shadow-lg">
                  <div className="border-b border-[var(--stitch-border)] px-4 py-3">
                    <p className="text-sm font-semibold text-[var(--stitch-text)] truncate">{user.name || user.email}</p>
                    <p className="text-xs text-[var(--stitch-text-muted)] truncate">{user.email}</p>
                  </div>
                  {user.role === "admin" && (
                    <Link
                      href="/admin"
                      onClick={() => setAvatarMenuOpen(false)}
                      className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                    >
                      <MaterialIcon name="admin_panel_settings" size={18} />
                      Admin
                    </Link>
                  )}
                  <Link
                    href="/dashboard"
                    onClick={() => setAvatarMenuOpen(false)}
                    className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="dashboard" size={18} />
                    Dashboard
                  </Link>
                  <Link
                    href="/account"
                    onClick={() => setAvatarMenuOpen(false)}
                    className="flex items-center gap-3 px-4 py-2.5 text-sm text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="person" size={18} />
                    Account
                  </Link>
                  <button
                    type="button"
                    onClick={handleLogout}
                    className="flex w-full items-center gap-3 px-4 py-2.5 text-sm text-red-500 transition-colors hover:bg-[var(--stitch-bg)]"
                  >
                    <MaterialIcon name="logout" size={18} />
                    Log out
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
      />
    </>
  );
}

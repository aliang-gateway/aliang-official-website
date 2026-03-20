"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import ThemeToggle from "@/app/components/ThemeToggle";
import { cn } from "@/lib/utils";

const navLinks = [
  { href: "/blog", label: "Blog" },
  { href: "/docs", label: "Document" },
  { href: "/services", label: "Service" },
];

export function SiteHeader() {
  const pathname = usePathname();
  const activePath = pathname;
  const showSearch = pathname === "/blog";

  return (
    <header
      className="sticky top-0 z-50 flex items-center justify-between whitespace-nowrap border-b border-[var(--stitch-border)] px-6 py-4 backdrop-blur-md md:px-20"
      style={{ backgroundColor: "color-mix(in srgb, var(--stitch-bg) 80%, transparent)" }}
    >
      <div className="flex items-center gap-8">
        <Link href="/" className="flex items-center gap-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-[var(--stitch-primary)] text-white dark:text-white">
            <MaterialIcon name="hub" size={20} />
          </div>
          <h2 className="text-xl font-bold leading-tight tracking-tight text-[var(--stitch-text)]">
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
                activePath === link.href
                  ? "text-[var(--stitch-primary)]"
                  : "text-[var(--stitch-text-muted)] hover:text-[var(--stitch-primary)]"
              )}
            >
              {link.label}
            </Link>
          ))}
        </nav>
      </div>

      <div className="flex flex-1 justify-end gap-6 items-center">
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
          href="/account"
          className="flex h-10 min-w-[100px] cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] px-6 text-sm font-bold text-white dark:text-white shadow-sm transition-all hover:bg-[var(--stitch-primary)]/90"
        >
          Login
        </Link>
        <button
          type="button"
          className="flex size-9 items-center justify-center rounded md:hidden text-[var(--stitch-text)]"
          aria-label="Open menu"
        >
          <MaterialIcon name="menu" size={24} />
        </button>
      </div>
    </header>
  );
}

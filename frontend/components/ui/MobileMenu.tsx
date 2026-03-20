"use client";

import Link from "next/link";
import { useEffect, useRef } from "react";
import { MaterialIcon } from "./MaterialIcon";
import { cn } from "@/lib/utils";

interface MobileMenuProps {
  isOpen: boolean;
  onClose: () => void;
  links: Array<{ href: string; label: string }>;
  activePath?: string;
  primaryAction?: { href: string; label: string };
  secondaryAction?: { href: string; label: string };
}

export function MobileMenu({
  isOpen,
  onClose,
  links,
  activePath,
  primaryAction,
  secondaryAction,
}: MobileMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    
    if (isOpen) {
      document.addEventListener("keydown", handleEscape);
      document.body.style.overflow = "hidden";
    }
    
    return () => {
      document.removeEventListener("keydown", handleEscape);
      document.body.style.overflow = "";
    };
  }, [isOpen, onClose]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
      onClose();
    }
  };

  return (
    <>
      <div
        className={cn(
          "fixed inset-0 z-50 bg-black/50 dark:bg-black/70 transition-opacity md:hidden",
          isOpen ? "opacity-100" : "pointer-events-none opacity-0"
        )}
        onClick={handleBackdropClick}
        aria-hidden="true"
      />

      <div
        ref={menuRef}
        className={cn(
          "fixed right-0 top-0 z-50 h-full w-72 transform bg-[var(--stitch-bg)] shadow-xl transition-transform duration-300 ease-in-out md:hidden",
          isOpen ? "translate-x-0" : "translate-x-full"
        )}
      >
        <div className="flex items-center justify-between border-b border-[var(--stitch-border)] p-4">
          <span className="font-bold text-[var(--stitch-text)]">Menu</span>
          <button
            type="button"
            onClick={onClose}
            className="flex size-8 items-center justify-center rounded text-[var(--stitch-text-muted)] transition-colors hover:bg-[var(--stitch-bg-elevated)] hover:text-[var(--stitch-text)]"
            aria-label="Close menu"
          >
            <MaterialIcon name="close" size={24} />
          </button>
        </div>

        <nav className="p-4">
          <ul className="space-y-1">
            {links.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  onClick={onClose}
                  className={cn(
                    "block rounded-lg px-4 py-3 text-base font-medium transition-colors",
                    activePath === link.href
                      ? "bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)]"
                      : "text-[var(--stitch-text-muted)] hover:bg-[var(--stitch-bg-elevated)] hover:text-[var(--stitch-text)]"
                  )}
                >
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>

        {(secondaryAction || primaryAction) && (
          <div className="space-y-3 border-t border-[var(--stitch-border)] p-4">
            {secondaryAction && (
              <Link
                href={secondaryAction.href}
                onClick={onClose}
                className="block w-full rounded border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-3 text-center text-sm font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
              >
                {secondaryAction.label}
              </Link>
            )}
            {primaryAction && (
              <Link
                href={primaryAction.href}
                onClick={onClose}
                className="block w-full rounded bg-[var(--stitch-primary)] py-3 text-center text-sm font-bold text-white transition-opacity hover:opacity-90"
              >
                {primaryAction.label}
              </Link>
            )}
          </div>
        )}
      </div>
    </>
  );
}

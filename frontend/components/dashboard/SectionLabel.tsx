"use client";

import type { ReactNode } from "react";

/**
 * Editorial section heading for the dashboard: a small mono accent "eyebrow"
 * followed by a thin rule that fills the remaining width — the masthead
 * section-divider look from the marketing paper theme, so the console reads
 * like grouped newspaper sections instead of a flat grid of equal cards.
 */
export function SectionLabel({ kicker, children }: { kicker: string; children?: ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span
        className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]"
        style={{ fontFamily: "var(--font-editorial-mono)" }}
      >
        {kicker}
      </span>
      {children ? (
        <span className="text-sm font-semibold text-[var(--ink-muted)]">{children}</span>
      ) : null}
      <span className="h-px flex-1 bg-[var(--line)]" aria-hidden />
    </div>
  );
}

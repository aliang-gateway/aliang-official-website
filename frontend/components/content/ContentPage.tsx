"use client";

import type { ReactNode } from "react";

type ContentPageProps = {
  badge?: string;
  title: string;
  subtitle?: string;
  lastUpdated?: string;
  children: ReactNode;
};

/**
 * Shared legal / about page shell: a centered hero (badge + title + subtitle)
 * followed by a narrow column of content sections. Reused by /about, /security,
 * /privacy and /terms so every static content page reads the same.
 */
export function ContentPage({ badge, title, subtitle, lastUpdated, children }: ContentPageProps) {
  return (
    <div className="stitch-section">
      <div className="stitch-container">
        <div className="mx-auto max-w-3xl text-center">
          {badge ? (
            <span className="stitch-badge mb-5 inline-flex">{badge}</span>
          ) : null}
          <h1 className="text-4xl font-black leading-tight tracking-tight text-[var(--stitch-text)] md:text-5xl">
            {title}
          </h1>
          {subtitle ? (
            <p className="mx-auto mt-5 max-w-2xl text-lg leading-relaxed text-[var(--stitch-text-muted)]">
              {subtitle}
            </p>
          ) : null}
          {lastUpdated ? (
            <p className="mt-4 text-sm text-[var(--stitch-text-muted)] opacity-80">{lastUpdated}</p>
          ) : null}
        </div>
        <div className="mx-auto mt-12 max-w-3xl space-y-6 md:mt-16">{children}</div>
      </div>
    </div>
  );
}

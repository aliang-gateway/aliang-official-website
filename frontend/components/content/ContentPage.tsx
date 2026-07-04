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
 * Editorial legal / about shell: centered hero (label + display title + lead)
 * above a narrow column of content cards. Renders inside the `.editorial`
 * wrapper, where tokens + `.display/.label/.lead` resolve. Reused by /about,
 * /security, /privacy and /terms so every static content page reads the same.
 */
export function ContentPage({ badge, title, subtitle, lastUpdated, children }: ContentPageProps) {
  return (
    <div className="container content-page">
      <header className="content-head" data-reveal>
        {badge ? <span className="label">{badge}</span> : null}
        <h1 className="display">
          {title}
          <span className="dot">.</span>
        </h1>
        {subtitle ? <p className="lead">{subtitle}</p> : null}
        {lastUpdated ? <p className="content-updated">{lastUpdated}</p> : null}
      </header>
      <div className="content-body">{children}</div>
    </div>
  );
}

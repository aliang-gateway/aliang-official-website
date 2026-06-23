"use client";

import type { ReactNode } from "react";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

export type ContentSectionData = {
  icon?: string;
  heading: string;
  paragraphs?: string[];
  points?: string[];
  children?: ReactNode;
};

type ContentSectionProps = ContentSectionData;

/**
 * A single content card: optional material icon + heading, followed by
 * paragraph text and/or a check-marked bullet list. Driven by i18n data so
 * the four content pages only need to map over their `sections` array.
 */
export function ContentSection({ icon, heading, paragraphs, points, children }: ContentSectionProps) {
  const hasBody = Boolean(paragraphs?.length || points?.length);

  return (
    <section className="stitch-card">
      <div className="flex items-start gap-3">
        {icon ? (
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)]">
            <MaterialIcon name={icon} size={22} />
          </div>
        ) : null}
        <h2 className="text-xl font-bold tracking-tight text-[var(--stitch-text)] md:text-2xl">
          {heading}
        </h2>
      </div>

      {paragraphs?.length ? (
        <div className={hasBody ? (icon ? "mt-4" : "mt-3") : ""}>
          <div className="space-y-3 text-[15px] leading-relaxed text-[var(--stitch-text-muted)]">
            {paragraphs.map((p, i) => (
              <p key={i}>{p}</p>
            ))}
          </div>
        </div>
      ) : null}

      {points?.length ? (
        <ul
          className={`space-y-2.5 text-[15px] leading-relaxed text-[var(--stitch-text-muted)] ${
            paragraphs?.length ? "mt-4" : icon ? "mt-4" : "mt-3"
          }`}
        >
          {points.map((pt, i) => (
            <li key={i} className="flex gap-2.5">
              <MaterialIcon name="check_circle" size={18} className="mt-0.5 shrink-0 text-[var(--stitch-primary)]" />
              <span>{pt}</span>
            </li>
          ))}
        </ul>
      ) : null}

      {children}
    </section>
  );
}

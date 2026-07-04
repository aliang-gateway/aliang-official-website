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
 * A single editorial content card: optional accent icon-chip + heading,
 * followed by paragraph prose and/or a check-marked bullet list. Driven by
 * i18n data so the content pages only map over their `sections` array.
 * Renders inside the `.editorial` wrapper.
 */
export function ContentSection({ icon, heading, paragraphs, points, children }: ContentSectionProps) {
  const hasBody = Boolean(paragraphs?.length || points?.length);

  return (
    <section className="content-card">
      <div className="content-card-head">
        {icon ? (
          <span className="icon-chip">
            <MaterialIcon name={icon} size={22} />
          </span>
        ) : null}
        <h2>{heading}</h2>
      </div>

      {paragraphs?.length ? (
        <div className={`prose${hasBody ? " content-card-body" : ""}`}>
          {paragraphs.map((p, i) => (
            <p key={i}>{p}</p>
          ))}
        </div>
      ) : null}

      {points?.length ? (
        <ul className={`prose${paragraphs?.length || icon ? " content-card-body" : ""}`}>
          {points.map((pt, i) => (
            <li key={i}>
              <span>{pt}</span>
            </li>
          ))}
        </ul>
      ) : null}

      {children}
    </section>
  );
}

import type { ReactNode } from "react";

/**
 * Wraps the MDX announcements page in editorial prose typography. The MDX
 * element overrides are pass-throughs, so `.prose` owns all styling.
 */
export default function AnnouncementsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="container">
      <article className="prose docs-announcements">{children}</article>
    </div>
  );
}

import type { ReactNode } from "react";
import { EditorialShell } from "./_editorial/EditorialShell";
import "./editorial.css";

export default function MarketingLayout({ children }: { children: ReactNode }) {
  return <EditorialShell>{children}</EditorialShell>;
}

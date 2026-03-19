import type { ReactNode } from "react";
import { DocsSidebar } from "./components/DocsSidebar";

type DocsLayoutProps = {
  children: ReactNode;
};

export default function DocsLayout({ children }: DocsLayoutProps) {
  return (
    <section className="bg-[var(--stitch-bg)] text-[var(--stitch-text)]">
      <div className="mx-auto flex w-full max-w-7xl gap-10 px-6 py-10 md:px-10 lg:px-12">
        <DocsSidebar />
        <article className="min-w-0 flex-1 pb-8">{children}</article>
      </div>
    </section>
  );
}

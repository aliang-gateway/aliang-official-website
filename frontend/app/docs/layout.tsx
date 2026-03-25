import type { ReactNode } from "react";

type DocsLayoutProps = {
  children: ReactNode;
};

export default function DocsLayout({ children }: DocsLayoutProps) {
  return (
    <section className="bg-[var(--stitch-bg)] text-[var(--stitch-text)]">
      <div className="mx-auto w-full max-w-5xl px-6 py-10 md:px-10 lg:px-12">
        <article className="min-w-0 pb-8">{children}</article>
      </div>
    </section>
  );
}

import Link from 'next/link';

export function DocsSidebar() {
  return (
    <aside className="sticky top-24 hidden h-fit w-64 shrink-0 self-start rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-5 lg:block">
      <nav className="flex flex-col gap-8" aria-label="Documentation navigation">
        <div>
          <h3 className="mb-3 text-xs font-bold uppercase tracking-wider text-[var(--stitch-text-muted)]">Getting Started</h3>
          <ul className="flex flex-col gap-2">
            <li>
              <Link href="/docs" aria-current="page" className="text-sm font-semibold text-[var(--stitch-primary)]">
                Introduction
              </Link>
            </li>
            <li>
              <Link href="/docs#getting-started" className="text-sm text-[var(--stitch-text-muted)] transition-colors hover:text-[var(--stitch-text)]">
                Quickstart
              </Link>
            </li>
          </ul>
        </div>
        
        <div>
          <h3 className="mb-3 text-xs font-bold uppercase tracking-wider text-[var(--stitch-text-muted)]">Core Concepts</h3>
          <ul className="flex flex-col gap-2">
            <li>
              <Link href="/docs#getting-started" className="text-sm text-[var(--stitch-text-muted)] transition-colors hover:text-[var(--stitch-text)]">
                Authentication
              </Link>
            </li>
            <li>
              <Link href="/docs#public-pricing" className="text-sm text-[var(--stitch-text-muted)] transition-colors hover:text-[var(--stitch-text)]">
                Pricing & Limits
              </Link>
            </li>
            <li>
              <Link href="/docs#api-quick-note" className="text-sm text-[var(--stitch-text-muted)] transition-colors hover:text-[var(--stitch-text)]">
                Managing API Keys
              </Link>
            </li>
          </ul>
        </div>

        <div className="rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3">
          <p className="text-xs font-semibold text-[var(--stitch-text)]">Need account setup help?</p>
          <p className="mt-1 text-xs leading-5 text-[var(--stitch-text-muted)]">
            Start from the account page and issue your first API key in under 2 minutes.
          </p>
          <Link
            href="/register"
            className="mt-3 inline-flex rounded-lg bg-[var(--stitch-primary)] px-3 py-1.5 text-xs font-semibold text-white transition-opacity hover:opacity-90"
          >
            Open Account
          </Link>
        </div>
      </nav>
    </aside>
  );
}

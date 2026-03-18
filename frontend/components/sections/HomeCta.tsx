import Link from "next/link";

export function HomeCta() {
  return (
    <section className="px-6 py-12">
      <div className="relative mx-auto max-w-5xl overflow-hidden rounded-xl border border-[var(--stitch-border-dark)] bg-[#0f172a] p-8 md:p-12 dark:border-[var(--stitch-primary)]/20 dark:bg-[var(--stitch-primary)]/10">
        <div className="absolute -right-20 -top-20 h-64 w-64 rounded-full bg-[var(--stitch-primary)]/20 blur-[100px]"></div>
        <div className="relative z-10 space-y-6 text-center">
          <h2 className="text-3xl font-black leading-tight text-white dark:text-[var(--stitch-primary)] md:text-4xl">
            Ready to switch to ALiang Gateway?
          </h2>
          <p className="mx-auto max-w-xl text-base leading-relaxed text-slate-400 dark:text-slate-300">
            Join thousands of developers building the future. Start today for free.
          </p>
          <div className="flex flex-col justify-center gap-3 sm:flex-row">
            <Link
              href="/account"
              className="flex h-12 min-w-[180px] cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] px-6 text-base font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/40 transition-transform hover:scale-[1.02]"
            >
              Get Started Now
            </Link>
            <Link
              href="/pricing"
              className="flex h-12 min-w-[180px] cursor-pointer items-center justify-center rounded-lg border border-slate-700 bg-slate-800 px-6 text-base font-bold text-white transition-colors hover:bg-slate-700"
            >
              Talk to Sales
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

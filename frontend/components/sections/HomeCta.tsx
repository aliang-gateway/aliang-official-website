import Link from "next/link";

type HomeVariant = "full" | "compact";

interface HomeCtaProps {
  variant?: HomeVariant;
}

export function HomeCta({ variant = "full" }: HomeCtaProps) {
  const isCompact = variant === "compact";

  return (
    <section className={`px-6 ${isCompact ? "py-12" : "py-20"}`}>
      <div className={`relative mx-auto max-w-5xl overflow-hidden border border-[var(--stitch-border-dark)] bg-[#0f172a] dark:border-[var(--stitch-primary)]/20 dark:bg-[var(--stitch-primary)]/10 ${isCompact ? "rounded-xl p-8 md:p-12" : "rounded-2xl p-10 md:p-16"}`}>
        <div className={`absolute -right-20 -top-20 rounded-full bg-[var(--stitch-primary)]/20 blur-[100px] ${isCompact ? "h-64 w-64" : "h-80 w-80"}`}></div>
        <div className={`relative z-10 text-center ${isCompact ? "space-y-6" : "space-y-8"}`}>
          <h2 className={`${isCompact ? "text-3xl md:text-4xl" : "text-4xl md:text-5xl"} font-black leading-tight text-white dark:text-[var(--stitch-primary)]`}>
            {isCompact
              ? "Ready to switch to ALiang Gateway?"
              : "Ready to switch to the most stable AI Gateway?"}
          </h2>
          <p className={`mx-auto leading-relaxed text-slate-400 dark:text-slate-300 ${isCompact ? "max-w-xl text-base" : "max-w-2xl text-lg"}`}>
            {isCompact
              ? "Join thousands of developers building the future. Start today for free."
              : "Join thousands of developers building the future with ALiang AI Services. Start today for free, no credit card required."}
          </p>
          <div className={`flex flex-col justify-center sm:flex-row ${isCompact ? "gap-3" : "gap-4"}`}>
            <Link
              href="/register"
              className={`flex cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/40 transition-transform hover:scale-[1.02] ${isCompact ? "h-12 min-w-[180px] px-6 text-base" : "h-14 min-w-[200px] px-8 text-lg"}`}
            >
              Get Started Now
            </Link>
            <Link
              href="/services"
              className={`flex cursor-pointer items-center justify-center rounded-lg border border-slate-700 bg-slate-800 font-bold text-white transition-colors hover:bg-slate-700 ${isCompact ? "h-12 min-w-[180px] px-6 text-base" : "h-14 min-w-[200px] px-8 text-lg"}`}
            >
              Talk to Sales
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

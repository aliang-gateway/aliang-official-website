import Link from "next/link";
import Image from "next/image";

export function HomeHero() {
  return (
    <section className="mx-auto max-w-7xl px-6 py-10 md:px-20 md:py-16">
      <div className="grid grid-cols-1 items-center gap-10 lg:grid-cols-2">
        <div className="flex flex-col gap-6">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-[var(--stitch-primary)]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75"></span>
              <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]"></span>
            </span>
            99.9% Uptime Guaranteed
          </div>

          <div className="flex flex-col gap-3">
            <h1 className="text-4xl font-black leading-tight tracking-tight text-[var(--stitch-text)] md:text-5xl">
              Next-Gen AI Gateway for{" "}
              <span className="text-[var(--stitch-primary)]">Unmatched Stability</span>
            </h1>
            <p className="max-w-xl text-base leading-relaxed text-[var(--stitch-text-muted)] md:text-lg">
              Experience official API support with high-performance routing. Your all-in-one bridge to leading AI models, engineered for scale.
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link
              href="/account"
              className="flex h-12 min-w-[140px] cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] px-5 text-sm font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-transform hover:scale-[1.02]"
            >
              Get Started Free
            </Link>
            <Link
              href="/docs"
              className="flex h-12 min-w-[140px] cursor-pointer items-center justify-center rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-5 text-sm font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)]"
            >
              Documentation
            </Link>
          </div>

          <div className="flex items-center gap-4 pt-2">
          <div className="flex -space-x-3">
            <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCuvpyv70_JvmcOYPve3_zTuCQQ7hOFr8xcOVUteql0Uuo4mGXQhNxuJUtQItOaK7ojcekgjXV-W2fXkLRrqGl-6dkUQDQNgT1ji8KOHdiAnSgc_exYA_LtPy4qFXpvVws642CP5F9ddOsdomb1oa149NjsMmCN3F1hIJJTK9urLkUEYns57sK81JR3dd04GuYs6hkDbH2M2h64kKKUT0P1w9MS5TwkHuRe-_7mqJhtwHpZ4k_zQ38ZqZ0J8spvxi5_8TSYaTwBqZE"
                alt=""
                width={40}
                height={40}
                className="h-10 w-10 rounded-full border-2 border-[var(--stitch-bg-elevated)]"
                unoptimized
              />
              <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCy7q4jWjPyyT-yOMgPnkOmunOC7S7XNdo13P58ZFCYW3g-RU5o_XuPF1jhyRsMefOqHab4hbQy9tGE6nvXxfMW1q1pcgpBIPMITjmmC6RiG_26rfh8hxUpkmj2vAijoEDYwwmY5xA_rfLall8F7C5D5TqXt1RGrRMBpg9nCEY0FpdnEXr0_Os6Aib7Eqe__x6uqyj4ZqLn9abEbvLwKI_ZmsXISrXEtPJAcvA6fn6zqrX7lhhi1pXU84OeKtomSmPmHtwKeycZhN4"
                alt=""
                width={40}
                height={40}
                className="h-10 w-10 rounded-full border-2 border-[var(--stitch-bg-elevated)]"
                unoptimized
              />
              <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBf5PKY_gGJuKfBH4H76cKrzVAmx_y49u0VqMYB0RpsQDbT49cRd1bvc_6s-9dKg-j2yKOJOG8TXcfTecCVbbxXN6wsKQ3J2VwmSfaoTc9I-m_DHjaApJk-XryoG3Dr6NMi-NwWNk0Le8YZs6JwevrPf_CrfzQSSf0d7mjUixhcMbBkkk7nShtO1l0_CrWFdSy5mOwrfaWXQhEzvKPNVpoxJIE_HiOvlDTNSTi-NVxaAIZF2cKqUrXXkPi75IrszT7y6chc84GLTws"
                alt=""
                width={40}
                height={40}
                className="h-10 w-10 rounded-full border-2 border-[var(--stitch-bg-elevated)]"
                unoptimized
              />
            </div>
            <p className="text-sm font-medium text-[var(--stitch-text-muted)]">
              Trusted by 500+ engineering teams
            </p>
          </div>
        </div>

        <div className="relative">
          <div className="absolute -inset-4 rounded-xl bg-[var(--stitch-primary)]/10 blur-3xl"></div>
          <div className="relative overflow-hidden rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] shadow-xl">
            <div className="flex items-center gap-1.5 border-b border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 py-2">
              <div className="h-2.5 w-2.5 rounded-full bg-red-500/50 dark:bg-red-500/70"></div>
              <div className="h-2.5 w-2.5 rounded-full bg-yellow-500/50 dark:bg-yellow-500/70"></div>
              <div className="h-2.5 w-2.5 rounded-full bg-green-500/50 dark:bg-green-500/70"></div>
              <div className="ml-3 text-[9px] font-mono uppercase tracking-widest text-[var(--stitch-text-muted)]">
                ai-gateway
              </div>
            </div>
            <div className="space-y-1.5 p-5 font-mono text-[11px] text-[var(--stitch-text)] md:text-xs">
              <div className="flex gap-2">
                <span className="text-[var(--stitch-primary)]">$</span>
                <span>curl -X POST https://api.aliang.ai/v1/chat</span>
              </div>
              <div className="flex gap-2 text-[var(--stitch-text-muted)]">
                <span>&gt;</span>
                <span>Model: claude-3-5-sonnet</span>
              </div>
              <div className="pt-2 text-[var(--stitch-primary)]">
                {"// Response received in 142ms"}
              </div>
              <div className="text-emerald-600 dark:text-emerald-300">{"{"}</div>
              <div className="pl-4 text-emerald-600 dark:text-emerald-300">"status": "success",</div>
              <div className="pl-4 text-emerald-600 dark:text-emerald-300">"provider": "anthropic-official",</div>
              <div className="pl-4 text-emerald-600 dark:text-emerald-300">"latency": "142ms"</div>
              <div className="text-emerald-600 dark:text-emerald-300">{"}"}</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

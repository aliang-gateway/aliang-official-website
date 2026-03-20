import Link from "next/link";
import Image from "next/image";

type HomeVariant = "full" | "compact";

interface HomeHeroProps {
  variant?: HomeVariant;
}

export function HomeHero({ variant = "full" }: HomeHeroProps) {
  const isCompact = variant === "compact";

  return (
    <section className={`mx-auto max-w-7xl px-6 md:px-20 ${isCompact ? "py-10 md:py-16" : "py-16 md:py-24"}`}>
      <div className={`grid grid-cols-1 items-center lg:grid-cols-2 ${isCompact ? "gap-10" : "gap-12"}`}>
        <div className={`flex flex-col ${isCompact ? "gap-6" : "gap-8"}`}>
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-[var(--stitch-primary)]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75"></span>
              <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]"></span>
            </span>
            99.9% Uptime Guaranteed
          </div>

          <div className={`flex flex-col ${isCompact ? "gap-3" : "gap-4"}`}>
            <h1 className={`${isCompact ? "text-4xl md:text-5xl" : "text-5xl md:text-6xl"} font-black leading-tight tracking-tight text-[var(--stitch-text)]`}>
              Next-Gen AI Gateway for{" "}
              <span className="text-[var(--stitch-primary)]">Unmatched Stability</span>
            </h1>
            <p className={`max-w-xl leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "text-base md:text-lg" : "text-lg md:text-xl"}`}>
              {isCompact
                ? "Experience official API support with high-performance routing. Your all-in-one bridge to leading AI models, engineered for scale."
                : "Experience the power of official API support with high-performance routing. Your all-in-one bridge to the world&apos;s leading AI models, engineered for enterprise-scale workloads."}
            </p>
          </div>

          <div className={`flex flex-wrap ${isCompact ? "gap-3" : "gap-4"}`}>
            <Link
              href="/register"
              className={`flex cursor-pointer items-center justify-center rounded-lg bg-[var(--stitch-primary)] font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-transform hover:scale-[1.02] ${isCompact ? "h-12 min-w-[140px] px-5 text-sm" : "h-14 min-w-[160px] px-6 text-base"}`}
            >
              Get Started Free
            </Link>
            <Link
              href="/docs"
              className={`flex cursor-pointer items-center justify-center rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)] ${isCompact ? "h-12 min-w-[140px] px-5 text-sm" : "h-14 min-w-[160px] px-6 text-base"}`}
            >
              {isCompact ? "Documentation" : "View Documentation"}
            </Link>
          </div>

          <div className={`flex items-center ${isCompact ? "gap-4 pt-2" : "gap-6 pt-4"}`}>
          <div className="flex -space-x-3">
              <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCuvpyv70_JvmcOYPve3_zTuCQQ7hOFr8xcOVUteql0Uuo4mGXQhNxuJUtQItOaK7ojcekgjXV-W2fXkLRrqGl-6dkUQDQNgT1ji8KOHdiAnSgc_exYA_LtPy4qFXpvVws642CP5F9ddOsdomb1oa149NjsMmCN3F1hIJJTK9urLkUEYns57sK81JR3dd04GuYs6hkDbH2M2h64kKKUT0P1w9MS5TwkHuRe-_7mqJhtwHpZ4k_zQ38ZqZ0J8spvxi5_8TSYaTwBqZE"
                alt=""
                width={isCompact ? 32 : 40}
                height={isCompact ? 32 : 40}
                className={`${isCompact ? "h-8 w-8" : "h-10 w-10"} rounded-full border-2 border-white dark:border-slate-900`}
                unoptimized
              />
              <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBf5PKY_gGJuKfBH4H76cKrzVAmx_y49u0VqMYB0RpsQDbT49cRd1bvc_6s-9dKg-j2yKOJOG8TXcfTecCVbbxXN6wsKQ3J2VwmSfaoTc9I-m_DHjaApJk-XryoG3Dr6NMi-NwWNk0Le8YZs6JwevrPf_CrfzQSSf0d7mjUixhcMbBkkk7nShtO1l0_CrWFdSy5mOwrfaWXQhEzvKPNVpoxJIE_HiOvlDTNSTi-NVxaAIZF2cKqUrXXkPi75IrszT7y6chc84GLTws"
                alt=""
                width={isCompact ? 32 : 40}
                height={isCompact ? 32 : 40}
                className={`${isCompact ? "h-8 w-8" : "h-10 w-10"} rounded-full border-2 border-white dark:border-slate-900`}
                unoptimized
              />
              <Image 
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCy7q4jWjPyyT-yOMgPnkOmunOC7S7XNdo13P58ZFCYW3g-RU5o_XuPF1jhyRsMefOqHab4hbQy9tGE6nvXxfMW1q1pcgpBIPMITjmmC6RiG_26rfh8hxUpkmj2vAijoEDYwwmY5xA_rfLall8F7C5D5TqXt1RGrRMBpg9nCEY0FpdnEXr0_Os6Aib7Eqe__x6uqyj4ZqLn9abEbvLwKI_ZmsXISrXEtPJAcvA6fn6zqrX7lhhi1pXU84OeKtomSmPmHtwKeycZhN4"
                alt=""
                width={isCompact ? 32 : 40}
                height={isCompact ? 32 : 40}
                className={`${isCompact ? "h-8 w-8" : "h-10 w-10"} rounded-full border-2 border-white dark:border-slate-900`}
                unoptimized
              />
            </div>
             <p className={`${isCompact ? "text-xs" : "text-sm"} font-medium text-slate-500 dark:text-slate-400`}>
               {isCompact ? "Trusted by 500+ teams" : "Trusted by 500+ engineering teams"}
            </p>
          </div>
        </div>

        <div className="relative">
          <div className="absolute -inset-4 rounded-xl bg-[var(--stitch-primary)]/10 blur-3xl"></div>
          <div className={`relative overflow-hidden border border-[var(--stitch-border-dark)] bg-[#0f172a] ${isCompact ? "rounded-lg shadow-xl" : "rounded-xl shadow-2xl"}`}>
            <div className={`flex items-center gap-1.5 border-b border-[var(--stitch-border-dark)] bg-slate-800/50 px-4 ${isCompact ? "py-2" : "py-3"}`}>
              <div className={`${isCompact ? "h-2.5 w-2.5" : "h-3 w-3"} rounded-full bg-red-500/50`}></div>
              <div className={`${isCompact ? "h-2.5 w-2.5" : "h-3 w-3"} rounded-full bg-yellow-500/50`}></div>
              <div className={`${isCompact ? "h-2.5 w-2.5" : "h-3 w-3"} rounded-full bg-green-500/50`}></div>
              <div className={`${isCompact ? "ml-3 text-[9px]" : "ml-4 text-[10px]"} font-mono uppercase tracking-widest text-slate-500`}>
                {isCompact ? "ai-gateway" : "ai-gateway-dashboard"}
              </div>
            </div>
            <div className={`${isCompact ? "space-y-1.5 p-5 text-[11px] md:text-xs" : "space-y-2 p-6 text-xs md:text-sm"} font-mono text-slate-300`}>
              <div className={`flex ${isCompact ? "gap-2" : "gap-3"}`}>
                <span className="text-[var(--stitch-primary)]">$</span>
                <span>curl -X POST https://api.aliang.ai/v1/chat</span>
              </div>
              {!isCompact && (
                <div className="flex gap-3 text-slate-500">
                  <span>&gt;</span>
                  <span>Authorization: Bearer AL_KEY_...</span>
                </div>
              )}
              <div className={`flex ${isCompact ? "gap-2" : "gap-3"} text-slate-500`}>
                <span>&gt;</span>
                <span>Model: claude-3-5-sonnet</span>
              </div>
              <div className={`${isCompact ? "pt-2" : "pt-4"} text-[var(--stitch-primary)]`}>
                {"// Response received in 142ms"}
              </div>
              <div className="text-green-400">{"{"}</div>
              <div className="pl-4 text-green-400">"status": "success",</div>
              <div className="pl-4 text-green-400">"provider": "anthropic-official",</div>
              <div className="pl-4 text-green-400">"latency": "142ms"{!isCompact ? "," : ""}</div>
              {!isCompact && <div className="pl-4 text-green-400">"tokens": {`{ "prompt": 1024, "completion": 512 }`}</div>}
              <div className="text-green-400">{"}"}</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

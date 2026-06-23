"use client";

import Link from "next/link";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeHeroProps {
  variant?: HomeVariant;
}

export function HomeHero({ variant = "full" }: HomeHeroProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeHero");

  return (
    <section
      data-od-id="home-hero"
      className={`relative overflow-hidden mx-auto max-w-7xl px-6 md:px-20 ${isCompact ? "py-10 md:py-16" : "py-16 md:py-28"}`}
    >
      {/* 背景装饰：克制的双辉光，制造空间感 */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -top-40 right-0 hidden lg:block"
        style={{
          width: 600,
          height: 600,
          borderRadius: "50%",
          background:
            "radial-gradient(circle, rgba(33,196,93,0.08) 0%, transparent 60%)",
        }}
      />

      <div className={`relative grid grid-cols-1 items-center lg:grid-cols-2 ${isCompact ? "gap-10" : "gap-16"}`}>
        <div className={`flex flex-col ${isCompact ? "gap-6" : "gap-8"}`}>
          {/* 品牌 badge：实时状态 */}
          <div className="inline-flex w-fit items-center gap-2 rounded-full border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-3 py-1.5 text-[10px] font-bold uppercase tracking-[0.08em] text-[var(--stitch-primary)]">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]" />
            </span>
            {t("badge")}
          </div>

          {/* 主标题：加大字号对比、负字距 */}
          <div className={`flex flex-col ${isCompact ? "gap-3" : "gap-5"}`}>
            <h1
              className={`${isCompact ? "text-4xl md:text-5xl" : "text-5xl md:text-6xl lg:text-7xl"} font-black leading-[1.05] tracking-[-0.025em] text-[var(--stitch-text)]`}
            >
              {t("title")}
              <span className="block text-[var(--stitch-primary)]">
                {t("titleHighlight")}
              </span>
            </h1>
            <p
              className={`max-w-xl leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "text-base md:text-lg" : "text-lg md:text-xl"}`}
            >
              {isCompact ? t("descriptionCompact") : t("descriptionFull")}
            </p>
          </div>

          {/* CTA 按钮组 */}
          <div className={`flex flex-wrap ${isCompact ? "gap-3" : "gap-4"}`}>
            <Link
              href="/register"
              className={`group flex cursor-pointer items-center justify-center gap-2 rounded-lg bg-[var(--stitch-primary)] font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:shadow-xl hover:shadow-[var(--stitch-primary)]/30 hover:scale-[1.02] active:scale-[0.98] ${isCompact ? "h-12 min-w-[140px] px-5 text-sm" : "h-14 min-w-[160px] px-7 text-base"}`}
            >
              {t("getStarted")}
              <MaterialIcon
                name="arrow_forward"
                size={isCompact ? 14 : 16}
                className="transition-transform group-hover:translate-x-0.5"
              />
            </Link>
            <Link
              href="/docs"
              className={`flex cursor-pointer items-center justify-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg)] ${isCompact ? "h-12 min-w-[140px] px-5 text-sm" : "h-14 min-w-[160px] px-7 text-base"}`}
            >
              <MaterialIcon name="menu_book" size={isCompact ? 16 : 18} />
              {isCompact ? t("documentation") : t("viewDocumentation")}
            </Link>
          </div>

          {/* 社交证明 */}
          <div className={`flex items-center ${isCompact ? "gap-4 pt-2" : "gap-5 pt-4"}`}>
            <div className="flex -space-x-3">
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCuvpyv70_JvmcOYPve3_zTuCQQ7hOFr8xcOVUteql0Uuo4mGXQhNxuJUtQItOaK7ojcekgjXV-W2fXkLRrqGl-6dkUQDQNgT1ji8KOHdiAnSgc_exYA_LtPy4qFXpvVws642CP5F9ddOsdomb1oa149NjsMmCN3F1hIJJTK9urLkUEYns57sK81JR3dd04GuYs6hkDbH2M2h64kKKUT0P1w9MS5TwkHuRe-_7mqJhtwHpZ4k_zQ38ZqZ0J8spvxi5_8TSYaTwBqZE"
                alt=""
                width={isCompact ? 32 : 36}
                height={isCompact ? 32 : 36}
                className={`${isCompact ? "h-8 w-8" : "h-9 w-9"} rounded-full border-2 border-[var(--stitch-bg)]`}
                unoptimized
              />
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBf5PKY_gGJuKfBH4H76cKrzVAmx_y49u0VqMYB0RpsQDbT49cRd1bvc_6s-9dKg-j2yKOJOG8TXcfTecCVbbxXN6wsKQ3J2VwmSfaoTc9I-m_DHjaApJk-XryoG3Dr6NMi-NwWNk0Le8YZs6JwevrPf_CrfzQSSf0d7mjUixhcMbBkkk7nShtO1l0_CrWFdSy5mOwrfaWXQhEzvKPNVpoxJIE_HiOvlDTNSTi-NVxaAIZF2cKqUrXXkPi75IrszT7y6chc84GLTws"
                alt=""
                width={isCompact ? 32 : 36}
                height={isCompact ? 32 : 36}
                className={`${isCompact ? "h-8 w-8" : "h-9 w-9"} rounded-full border-2 border-[var(--stitch-bg)]`}
                unoptimized
              />
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuCy7q4jWjPyyT-yOMgPnkOmunOC7S7XNdo13P58ZFCYW3g-RU5o_XuPF1jhyRsMefOqHab4hbQy9tGE6nvXxfMW1q1pcgpBIPMITjmmC6RiG_26rfh8hxUpkmj2vAijoEDYwwmY5xA_rfLall8F7C5D5TqXt1RGrRMBpg9nCEY0FpdnEXr0_Os6Aib7Eqe__x6uqyj4ZqLn9abEbvLwKI_ZmsXISrXEtPJAcvA6fn6zqrX7lhhi1pXU84OeKtomSmPmHtwKeycZhN4"
                alt=""
                width={isCompact ? 32 : 36}
                height={isCompact ? 32 : 36}
                className={`${isCompact ? "h-8 w-8" : "h-9 w-9"} rounded-full border-2 border-[var(--stitch-bg)]`}
                unoptimized
              />
            </div>
            <div className="flex flex-col gap-0.5">
              <div className={`flex items-center gap-0.5 ${isCompact ? "" : ""}`}>
                {Array.from({ length: 5 }).map((_, i) => (
                  <MaterialIcon
                    key={i}
                    name="star"
                    size={isCompact ? 11 : 13}
                    className="text-[var(--stitch-primary)]"
                  />
                ))}
              </div>
              <p className={`${isCompact ? "text-xs" : "text-sm"} font-medium text-[var(--stitch-text-muted)]`}>
                {isCompact ? t("trustedCompact") : t("trustedFull")}
              </p>
            </div>
          </div>
        </div>

        {/* 右侧：网关仪表盘 mockup */}
        <div className="relative">
          <div
            aria-hidden="true"
            className="absolute -inset-4 rounded-2xl opacity-60 blur-3xl"
            style={{
              background:
                "radial-gradient(circle at 70% 30%, rgba(33,196,93,0.12) 0%, transparent 60%)",
            }}
          />
          <div
            className={`relative overflow-hidden rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] shadow-2xl`}
            style={{
              boxShadow:
                "0 20px 50px -12px rgba(0,0,0,0.15), 0 0 0 1px var(--stitch-border)",
            }}
          >
            {/* 浏览器/窗口顶栏 */}
            <div className={`flex items-center gap-2 border-b border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 ${isCompact ? "py-2" : "py-3"}`}>
              <div className="flex gap-1.5">
                <div className="size-2.5 rounded-full bg-red-400/60" />
                <div className="size-2.5 rounded-full bg-yellow-400/60" />
                <div className="size-2.5 rounded-full bg-green-400/60" />
              </div>
              <div className={`ml-3 flex items-center gap-1.5 rounded-md bg-[var(--stitch-bg-elevated)] px-2 py-0.5`}>
                <MaterialIcon name="lock" size={10} className="text-[var(--stitch-text-muted)]" />
                <span className={`font-mono text-[9px] tracking-wide text-[var(--stitch-text-muted)]`}>
                  {isCompact ? "api.aliang.ai" : "dashboard.aliang.ai/overview"}
                </span>
              </div>
            </div>

            {/* 仪表盘内容 */}
            <div className={`grid grid-cols-3 gap-px bg-[var(--stitch-border)] ${isCompact ? "p-px" : "p-px"}`}>
              {/* 指标卡 1 */}
              <div className="flex flex-col gap-1 bg-[var(--stitch-bg-elevated)] p-3">
                <span className="text-[9px] font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">
                  Requests
                </span>
                <span className={`font-mono font-bold tracking-tight text-[var(--stitch-text)] ${isCompact ? "text-base" : "text-lg"}`}>
                  1.2M
                </span>
                <span className="flex items-center gap-0.5 text-[9px] font-semibold text-[var(--stitch-primary)]">
                  <MaterialIcon name="trending_up" size={10} />
                  +12.4%
                </span>
              </div>
              {/* 指标卡 2 */}
              <div className="flex flex-col gap-1 bg-[var(--stitch-bg-elevated)] p-3">
                <span className="text-[9px] font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">
                  Latency
                </span>
                <span className={`font-mono font-bold tracking-tight text-[var(--stitch-text)] ${isCompact ? "text-base" : "text-lg"}`}>
                  142<span className="text-[10px] font-normal text-[var(--stitch-text-muted)]">ms</span>
                </span>
                <span className="flex items-center gap-0.5 text-[9px] font-semibold text-[var(--stitch-primary)]">
                  <MaterialIcon name="trending_down" size={10} />
                  -8ms
                </span>
              </div>
              {/* 指标卡 3 */}
              <div className="flex flex-col gap-1 bg-[var(--stitch-bg-elevated)] p-3">
                <span className="text-[9px] font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">
                  Uptime
                </span>
                <span className={`font-mono font-bold tracking-tight text-[var(--stitch-text)] ${isCompact ? "text-base" : "text-lg"}`}>
                  99.9<span className="text-[10px] font-normal text-[var(--stitch-text-muted)]">%</span>
                </span>
                <span className="flex items-center gap-0.5 text-[9px] font-semibold text-[var(--stitch-text-muted)]">
                  <MaterialIcon name="check_circle" size={10} />
                  30d
                </span>
              </div>
            </div>

            {/* 图表区：纯 CSS 柱状图 */}
            <div className={`bg-[var(--stitch-bg-elevated)] ${isCompact ? "p-3" : "p-4"}`}>
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--stitch-text-muted)]">
                  Token Trend · 7d
                </span>
                <div className="flex items-center gap-1">
                  <span className="size-1.5 rounded-full bg-[var(--stitch-primary)]" />
                  <span className="text-[9px] text-[var(--stitch-text-muted)]">live</span>
                </div>
              </div>
              <div className="flex items-end justify-between gap-1.5" style={{ height: isCompact ? 60 : 80 }}>
                {[45, 62, 38, 78, 55, 88, 72].map((h, i) => (
                  <div key={i} className="flex flex-1 flex-col items-center gap-1">
                    <div
                      className="w-full rounded-t-sm transition-all"
                      style={{
                        height: `${h}%`,
                        background:
                          i === 5
                            ? "var(--stitch-primary)"
                            : "color-mix(in oklch, var(--stitch-primary) 25%, var(--stitch-bg-elevated))",
                      }}
                    />
                    <span className="text-[8px] text-[var(--stitch-text-muted)]">
                      {["一", "二", "三", "四", "五", "六", "日"][i]}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* 底部：模型路由状态条 */}
            {!isCompact && (
              <div className="flex items-center gap-2 border-t border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 py-2.5">
                <span className="relative flex h-1.5 w-1.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75" />
                  <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-[var(--stitch-primary)]" />
                </span>
                <span className="font-mono text-[10px] text-[var(--stitch-text-muted)]">
                  claude-sonnet-4 · routed
                </span>
                <div className="ml-auto flex items-center gap-1">
                  <span className="rounded-sm bg-[var(--stitch-primary)]/10 px-1.5 py-0.5 font-mono text-[9px] font-bold text-[var(--stitch-primary)]">
                    healthy
                  </span>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

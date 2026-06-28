"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeCtaProps {
  variant?: HomeVariant;
}

export function HomeCta({ variant = "full" }: HomeCtaProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeCta");

  return (
    <section
      data-od-id="home-cta"
      className={`px-6 md:px-20 ${isCompact ? "py-12" : "py-20 md:py-28"}`}
    >
      <div
        className={`relative mx-auto max-w-5xl overflow-hidden rounded-2xl border border-[var(--stitch-border)] ${isCompact ? "p-8 md:p-12" : "p-10 md:p-16 lg:p-20"}`}
        style={{
          background:
            "linear-gradient(135deg, var(--stitch-bg-elevated) 0%, var(--stitch-bg) 100%)",
        }}
      >
        {/* 双辉光装饰：制造空间深度 */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-20 -top-20"
          style={{
            width: isCompact ? 240 : 320,
            height: isCompact ? 240 : 320,
            borderRadius: "50%",
            background: "radial-gradient(circle, rgba(33,196,93,0.15) 0%, transparent 70%)",
          }}
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -bottom-16 -left-16 opacity-50"
          style={{
            width: isCompact ? 200 : 280,
            height: isCompact ? 200 : 280,
            borderRadius: "50%",
            background: "radial-gradient(circle, rgba(33,196,93,0.08) 0%, transparent 70%)",
          }}
        />

        <div className={`relative z-10 flex flex-col items-center text-center ${isCompact ? "gap-5" : "gap-7"}`}>
          {/* 品牌 badge */}
          <span className={`inline-flex items-center gap-1.5 rounded-full bg-[var(--stitch-primary)]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--stitch-primary)]`}>
            <MaterialIcon name="rocket_launch" size={11} />
            Get Started
          </span>

          <h2 className={`${isCompact ? "text-3xl md:text-4xl" : "text-4xl md:text-5xl lg:text-6xl"} font-black leading-[1.05] tracking-[-0.025em] text-[var(--stitch-text)]`}>
            {isCompact ? t("titleCompact") : t("titleFull")}
          </h2>

          <p className={`mx-auto max-w-2xl leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "max-w-xl text-base" : "text-lg"}`}>
            {isCompact ? t("descriptionCompact") : t("descriptionFull")}
          </p>

          <div className={`flex flex-col justify-center gap-3 sm:flex-row ${isCompact ? "" : "gap-4 pt-2"}`}>
            <Link
              href="/register"
              className={`group flex cursor-pointer items-center justify-center gap-2 rounded-lg bg-[var(--stitch-primary)] font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/25 transition-all hover:shadow-xl hover:shadow-[var(--stitch-primary)]/35 hover:scale-[1.02] active:scale-[0.98] ${isCompact ? "h-12 min-w-[180px] px-6 text-base" : "h-14 min-w-[200px] px-8 text-lg"}`}
            >
              {t("getStarted")}
              <MaterialIcon
                name="arrow_forward"
                size={isCompact ? 16 : 18}
                className="transition-transform group-hover:translate-x-0.5"
              />
            </Link>
            <Link
              href="/services"
              className={`flex cursor-pointer items-center justify-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg-elevated)] ${isCompact ? "h-12 min-w-[180px] px-6 text-base" : "h-14 min-w-[200px] px-8 text-lg"}`}
            >
              {t("talkToSales")}
            </Link>
          </div>

          {/* 底部信任行 */}
          {!isCompact && (
            <div className="flex items-center gap-2 pt-4 text-sm text-[var(--stitch-text-muted)]">
              <MaterialIcon name="check_circle" size={14} className="text-[var(--stitch-primary)]" />
              无需信用卡 · 随时取消 · 免费额度
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

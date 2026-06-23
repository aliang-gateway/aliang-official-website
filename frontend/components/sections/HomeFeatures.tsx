"use client";

import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeFeaturesProps {
  variant?: HomeVariant;
}

const features = [
  { icon: "verified_user", titleKey: "feature1Title", descriptionKey: "feature1Description", accent: true },
  { icon: "shield_with_heart", titleKey: "feature2Title", descriptionKey: "feature2Description", accent: false },
  { icon: "analytics", titleKey: "feature3Title", descriptionKey: "feature3Description", accent: false },
] as const;

export function HomeFeatures({ variant = "full" }: HomeFeaturesProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeFeatures");

  return (
    <section
      id="features"
      data-od-id="home-features"
      className={`bg-[var(--stitch-bg)] ${isCompact ? "py-12" : "py-20 md:py-24"}`}
    >
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        {/* 标题区：左对齐而非居中，制造编辑式节奏 */}
        <div className={`mb-10 flex flex-col gap-3 md:mb-14 ${isCompact ? "" : "max-w-2xl"}`}>
          <span className={`inline-flex w-fit items-center gap-1.5 rounded-full bg-[var(--stitch-primary)]/10 px-2.5 py-1 text-[9px] font-bold uppercase tracking-[0.1em] text-[var(--stitch-primary)]`}>
            <MaterialIcon name="auto_awesome" size={10} />
            Features
          </span>
          <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl lg:text-5xl"} font-black tracking-[-0.025em] text-[var(--stitch-text)]`}>
            {t("title")}
          </h2>
          <p className={`max-w-xl leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "text-sm" : "text-lg"}`}>
            {isCompact ? t("subtitleCompact") : t("subtitleFull")}
          </p>
        </div>

        {/* Bento 布局：首卡占双列作为视觉锚点 */}
        <div
          className={`grid grid-cols-1 gap-4 md:grid-cols-6 md:gap-5 ${isCompact ? "" : "lg:gap-6"}`}
        >
          {features.map((feature) => (
            <div
              key={feature.titleKey}
              className={[
                "group relative flex flex-col rounded-2xl border transition-all duration-300 hover:-translate-y-1",
                feature.accent
                  ? "md:col-span-6 lg:col-span-3 lg:row-span-2"
                  : "md:col-span-3 lg:col-span-3",
                feature.accent && !isCompact ? "p-8 lg:p-10" : isCompact ? "p-5" : "p-6 lg:p-7",
                "border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)]",
              ].join(" ")}
            >
              {/* Accent 卡的背景装饰 */}
              {feature.accent && (
                <div
                  aria-hidden="true"
                  className="pointer-events-none absolute -right-10 -top-10 opacity-40 transition-opacity duration-300 group-hover:opacity-70"
                  style={{
                    width: 160,
                    height: 160,
                    borderRadius: "50%",
                    background: "radial-gradient(circle, rgba(33,196,93,0.12) 0%, transparent 70%)",
                  }}
                />
              )}

              <div className={`relative flex items-center gap-3`}>
                <div
                  className={`flex items-center justify-center rounded-xl ${
                    feature.accent
                      ? isCompact ? "size-12" : "size-14"
                      : isCompact ? "size-10" : "size-11"
                  }`}
                  style={{
                    background: feature.accent
                      ? "var(--stitch-primary)"
                      : "rgba(33,196,93,0.1)",
                  }}
                >
                  <MaterialIcon
                    name={feature.icon}
                    size={feature.accent ? (isCompact ? 24 : 28) : (isCompact ? 18 : 20)}
                    className={feature.accent ? "text-white" : "text-[var(--stitch-primary)]"}
                  />
                </div>
              </div>

              <div className={`relative flex flex-col gap-2 ${feature.accent && !isCompact ? "mt-6" : "mt-4"}`}>
                <h3 className={`${feature.accent && !isCompact ? "text-2xl" : isCompact ? "text-base" : "text-lg"} font-bold tracking-tight text-[var(--stitch-text)]`}>
                  {t(feature.titleKey)}
                </h3>
                <p className={`leading-relaxed text-[var(--stitch-text-muted)] ${
                  feature.accent && !isCompact ? "text-base" : isCompact ? "text-xs" : "text-sm"
                }`}>
                  {t(feature.descriptionKey)}
                </p>
              </div>

              {/* Accent 卡底部追加视觉锚点：信任指标行 */}
              {feature.accent && !isCompact && (
                <div className="relative mt-auto flex items-center gap-5 pt-8">
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xl font-bold tracking-tight text-[var(--stitch-text)]">100%</span>
                    <span className="text-[10px] uppercase tracking-wider text-[var(--stitch-text-muted)]">官方凭证</span>
                  </div>
                  <div className="h-8 w-px bg-[var(--stitch-border)]" />
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xl font-bold tracking-tight text-[var(--stitch-text)]">0</span>
                    <span className="text-[10px] uppercase tracking-wider text-[var(--stitch-text-muted)]">中间代理</span>
                  </div>
                  <div className="h-8 w-px bg-[var(--stitch-border)]" />
                  <div className="flex flex-col gap-0.5">
                    <span className="font-mono text-xl font-bold tracking-tight text-[var(--stitch-primary)]">T+0</span>
                    <span className="text-[10px] uppercase tracking-wider text-[var(--stitch-text-muted)]">实时转发</span>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

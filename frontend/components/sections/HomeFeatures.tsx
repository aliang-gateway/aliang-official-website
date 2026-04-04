"use client";

import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeFeaturesProps {
  variant?: HomeVariant;
}

const features = [
  { icon: "verified_user", titleKey: "feature1Title", descriptionKey: "feature1Description" },
  { icon: "shield_with_heart", titleKey: "feature2Title", descriptionKey: "feature2Description" },
  { icon: "analytics", titleKey: "feature3Title", descriptionKey: "feature3Description" },
] as const;

export function HomeFeatures({ variant = "full" }: HomeFeaturesProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeFeatures");

  return (
    <section id="features" className={`bg-[var(--stitch-bg-elevated)] ${isCompact ? "py-12" : "py-20"}`}>
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className={`mb-12 text-center ${isCompact ? "space-y-2" : "space-y-4"}`}>
          <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl"} font-bold tracking-tight text-[var(--stitch-text)]`}>
            {t("title")}
          </h2>
          <p className={`mx-auto max-w-2xl text-[var(--stitch-text-muted)] ${isCompact ? "text-sm" : "text-lg"}`}>
            {isCompact ? t("subtitleCompact") : t("subtitleFull")}
          </p>
        </div>

        <div className={`grid grid-cols-1 gap-8 md:grid-cols-3 ${isCompact ? "md:gap-6" : "md:gap-8"}`}>
          {features.map((feature) => (
            <div
              key={feature.titleKey}
              className={`flex flex-col rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] transition-transform hover:-translate-y-1 ${isCompact ? "gap-3 p-6" : "gap-4 p-8"}`}
            >
              <div className={`flex items-center justify-center rounded-lg bg-[var(--stitch-primary)]/10 ${isCompact ? "size-10" : "size-12"}`}>
                <MaterialIcon name={feature.icon} size={isCompact ? 20 : 24} className="text-[var(--stitch-primary)]" />
              </div>
              <h3 className={`${isCompact ? "text-base" : "text-lg"} font-bold text-[var(--stitch-text)]`}>
                {t(feature.titleKey)}
              </h3>
              <p className={`leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "text-sm" : "text-base"}`}>
                {t(feature.descriptionKey)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

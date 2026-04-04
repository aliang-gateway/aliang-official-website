"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeIntegrationsProps {
  variant?: HomeVariant;
}

interface Integration {
  icon: string;
  name: string;
  compactName: string;
  subtitle: string;
}

export function HomeIntegrations({ variant = "full" }: HomeIntegrationsProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeIntegrations");

  const integrations: Integration[] = [
    { icon: "terminal", name: t("cursor"), compactName: t("cursor"), subtitle: t("cursorSubtitle") },
    { icon: "chat", name: t("claude"), compactName: t("claude"), subtitle: t("claudeSubtitle") },
    { icon: "neurology", name: t("openai"), compactName: t("openai"), subtitle: t("openaiSubtitle") },
    { icon: "code_blocks", name: t("copilot"), compactName: t("copilotShort"), subtitle: t("copilotSubtitle") },
  ];

  return (
    <section id="integrations" className={`bg-[var(--stitch-bg)] ${isCompact ? "py-12" : "py-20"}`}>
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className={`flex flex-col gap-4 md:flex-row md:items-end md:justify-between ${isCompact ? "mb-8" : "mb-12"}`}>
          <div className={isCompact ? "space-y-1" : "space-y-4"}>
            <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl"} font-bold tracking-tight text-[var(--stitch-text)]`}>
              {t("title")}
            </h2>
            <p className={`max-w-xl text-[var(--stitch-text-muted)] ${isCompact ? "text-base" : "text-lg"}`}>
              {isCompact
                ? t("subtitleCompact")
                : t("subtitleFull")}
            </p>
          </div>
          <Link
            href="/services"
            className={`flex items-center gap-2 font-bold text-[var(--stitch-primary)] hover:underline ${isCompact ? "text-sm" : ""}`}
          >
            {t("exploreAll")}
            <MaterialIcon name="arrow_forward" size={isCompact ? 12 : 16} />
          </Link>
        </div>
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {integrations.map((integration) => (
            <div
              key={integration.name}
              className={`flex flex-col items-center rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] transition-transform hover:-translate-y-1 ${isCompact ? "gap-3 p-6" : "gap-4 p-8"}`}
            >
              <div className={`flex items-center justify-center text-[var(--stitch-text)] ${isCompact ? "size-12" : "size-16"}`}>
                <MaterialIcon name={integration.icon} size={isCompact ? 40 : 48} />
              </div>
              <div className="text-center">
                <h4 className={`${isCompact ? "text-base" : "text-lg"} font-bold text-[var(--stitch-text)]`}>
                  {isCompact ? integration.compactName : integration.name}
                </h4>
                {!isCompact && <p className="text-sm text-slate-500">{integration.subtitle}</p>}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

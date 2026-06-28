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
    <section
      id="integrations"
      data-od-id="home-integrations"
      className={`bg-[var(--stitch-bg-elevated)] ${isCompact ? "py-12" : "py-20 md:py-24"}`}
    >
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className={`flex flex-col gap-4 md:flex-row md:items-end md:justify-between ${isCompact ? "mb-8" : "mb-12"}`}>
          <div className={`flex flex-col gap-3 ${isCompact ? "" : "max-w-xl"}`}>
            <span className={`inline-flex w-fit items-center gap-1.5 rounded-full bg-[var(--stitch-primary)]/10 px-2.5 py-1 text-[9px] font-bold uppercase tracking-[0.1em] text-[var(--stitch-primary)]`}>
              <MaterialIcon name="extension" size={10} />
              Integrations
            </span>
            <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl lg:text-5xl"} font-black tracking-[-0.025em] text-[var(--stitch-text)]`}>
              {t("title")}
            </h2>
            <p className={`leading-relaxed text-[var(--stitch-text-muted)] ${isCompact ? "text-sm" : "text-lg"}`}>
              {isCompact ? t("subtitleCompact") : t("subtitleFull")}
            </p>
          </div>
          <Link
            href="/services"
            className={`group flex items-center gap-1.5 font-bold text-[var(--stitch-primary)] hover:underline ${isCompact ? "text-sm" : "text-base"}`}
          >
            {t("exploreAll")}
            <MaterialIcon
              name="arrow_forward"
              size={isCompact ? 14 : 16}
              className="transition-transform group-hover:translate-x-0.5"
            />
          </Link>
        </div>

        {/* 集成网格：横版紧凑卡片，hover 时边框转为品牌绿 */}
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
          {integrations.map((integration) => (
            <div
              key={integration.name}
              className={`group relative flex cursor-pointer flex-col rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] transition-all duration-300 hover:-translate-y-1 hover:border-[var(--stitch-primary)]/40 ${isCompact ? "gap-2 p-5" : "gap-3 p-6"}`}
            >
              {/* hover 时的微妙背景 */}
              <div
                aria-hidden="true"
                className="pointer-events-none absolute inset-0 rounded-xl opacity-0 transition-opacity duration-300 group-hover:opacity-100"
                style={{
                  background: "radial-gradient(circle at 50% 0%, rgba(33,196,93,0.06) 0%, transparent 70%)",
                }}
              />

              <div className={`relative flex items-center justify-center rounded-xl bg-[var(--stitch-bg-elevated)] ${isCompact ? "size-12" : "size-14"}`}>
                <MaterialIcon
                  name={integration.icon}
                  size={isCompact ? 24 : 28}
                  className="text-[var(--stitch-text)] transition-colors group-hover:text-[var(--stitch-primary)]"
                />
              </div>
              <div className="relative flex flex-col gap-0.5">
                <h4 className={`${isCompact ? "text-sm" : "text-base"} font-bold tracking-tight text-[var(--stitch-text)]`}>
                  {isCompact ? integration.compactName : integration.name}
                </h4>
                {!isCompact && (
                  <p className="text-xs leading-relaxed text-[var(--stitch-text-muted)]">
                    {integration.subtitle}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

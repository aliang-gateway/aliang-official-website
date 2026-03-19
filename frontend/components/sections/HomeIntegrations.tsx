import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeIntegrationsProps {
  variant?: HomeVariant;
}

const integrations = [
  { icon: "terminal", name: "Cursor", subtitle: "Instant IDE setup" },
  { icon: "chat", name: "Claude", subtitle: "Official model hub" },
  { icon: "neurology", name: "OpenAI", subtitle: "Global gateway" },
  { icon: "code_blocks", name: "GitHub Copilot", subtitle: "AI-powered coding" },
];

export function HomeIntegrations({ variant = "full" }: HomeIntegrationsProps) {
  const isCompact = variant === "compact";

  return (
    <section id="integrations" className={`bg-[var(--stitch-bg)] ${isCompact ? "py-12" : "py-20"}`}>
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className={`flex flex-col gap-4 md:flex-row md:items-end md:justify-between ${isCompact ? "mb-8" : "mb-12"}`}>
          <div className={isCompact ? "space-y-1" : "space-y-4"}>
            <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl"} font-bold tracking-tight text-[var(--stitch-text)]`}>
              One-Click Integrations
            </h2>
            <p className={`max-w-xl text-[var(--stitch-text-muted)] ${isCompact ? "text-base" : "text-lg"}`}>
              {isCompact
                ? "Connect your tools in seconds with our pre-built adapters."
                : "Connect your favorite development tools in seconds with our pre-built gateway adapters."}
            </p>
          </div>
          <Link
            href="/services"
            className={`flex items-center gap-2 font-bold text-[var(--stitch-primary)] hover:underline ${isCompact ? "text-sm" : ""}`}
          >
            Explore all 50+ integrations
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
                  {isCompact && integration.name === "GitHub Copilot" ? "GitHub" : integration.name}
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

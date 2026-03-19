import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeFeaturesProps {
  variant?: HomeVariant;
}

const features = [
  {
    icon: "verified_user",
    title: "Official API Support",
    description: "Direct access to Claude, OpenAI, and more with official credentials. No middleman proxies, just pure performance.",
  },
  {
    icon: "shield_with_heart",
    title: "Enterprise Stability",
    description: "Built-in redundancy and multi-region load balancing for zero-downtime performance when your users need it most.",
  },
  {
    icon: "analytics",
    title: "Rich Feature Set",
    description: "Advanced prompt management, real-time rate limiting, and granular cost tracking out of the box.",
  },
];

export function HomeFeatures({ variant = "full" }: HomeFeaturesProps) {
  const isCompact = variant === "compact";

  return (
    <section id="features" className={`bg-[var(--stitch-bg-elevated)] ${isCompact ? "py-12" : "py-20"}`}>
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className={`mx-auto max-w-3xl text-center ${isCompact ? "mb-10 space-y-2" : "mb-16 space-y-4"}`}>
          <h2 className={`${isCompact ? "text-2xl md:text-3xl" : "text-3xl md:text-4xl"} font-bold tracking-tight text-[var(--stitch-text)]`}>
            Engineered for Excellence
          </h2>
          <p className={`${isCompact ? "text-base" : "text-lg"} text-[var(--stitch-text-muted)]`}>
            {isCompact
              ? "Military-grade security and official API integrations."
              : "Built to handle enterprise-level workloads with military-grade security and official API integrations."}
          </p>
        </div>
        <div className={`grid grid-cols-1 md:grid-cols-3 ${isCompact ? "gap-6" : "gap-8"}`}>
          {features.map((feature) => (
            <div
              key={feature.title}
              className={`group rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] shadow-sm transition-all hover:border-[var(--stitch-primary)]/50 ${isCompact ? "p-6" : "p-8 hover:shadow-md"}`}
            >
              <div className={`flex items-center justify-center rounded-lg bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)] transition-transform group-hover:scale-110 ${isCompact ? "mb-4 size-10" : "mb-6 size-12"}`}>
                <MaterialIcon name={feature.icon} size={isCompact ? 20 : 24} />
              </div>
              <h3 className={`${isCompact ? "mb-2 text-lg" : "mb-3 text-xl"} font-bold text-[var(--stitch-text)]`}>
                {feature.title}
              </h3>
              <p className={`${isCompact ? "text-sm" : ""} leading-relaxed text-[var(--stitch-text-muted)]`}>
                {isCompact
                  ? feature.description
                      .replace(" No middleman proxies, just pure performance.", "")
                      .replace(" performance when your users need it most.", ".")
                      .replace(" real-time ", " ")
                      .replace(" and granular", "")
                      .replace(" out of the box", "")
                  : feature.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

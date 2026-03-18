import { MaterialIcon } from "@/components/ui/MaterialIcon";

const features = [
  {
    icon: "verified_user",
    title: "Official API Support",
    description: "Direct access to Claude, OpenAI, and more with official credentials.",
  },
  {
    icon: "shield_with_heart",
    title: "Enterprise Stability",
    description: "Built-in redundancy and multi-region load balancing for zero-downtime.",
  },
  {
    icon: "analytics",
    title: "Rich Feature Set",
    description: "Advanced prompt management, rate limiting, and cost tracking.",
  },
];

export function HomeFeatures() {
  return (
    <section className="bg-[var(--stitch-bg-elevated)] py-12">
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className="mx-auto mb-10 max-w-3xl space-y-2 text-center">
          <h2 className="text-2xl font-bold tracking-tight text-[var(--stitch-text)] md:text-3xl">
            Engineered for Excellence
          </h2>
          <p className="text-base text-[var(--stitch-text-muted)]">
            Military-grade security and official API integrations.
          </p>
        </div>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          {features.map((feature) => (
            <div
              key={feature.title}
              className="group rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-6 shadow-sm transition-all hover:border-[var(--stitch-primary)]/50"
            >
              <div className="mb-4 flex size-10 items-center justify-center rounded-lg bg-[var(--stitch-primary)]/10 text-[var(--stitch-primary)] transition-transform group-hover:scale-110">
                <MaterialIcon name={feature.icon} size={20} />
              </div>
              <h3 className="mb-2 text-lg font-bold text-[var(--stitch-text)]">
                {feature.title}
              </h3>
              <p className="text-sm leading-relaxed text-[var(--stitch-text-muted)]">
                {feature.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

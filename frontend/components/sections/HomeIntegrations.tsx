import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const integrations = [
  { icon: "terminal", name: "Cursor" },
  { icon: "chat", name: "Claude" },
  { icon: "neurology", name: "OpenAI" },
  { icon: "code_blocks", name: "GitHub" },
];

export function HomeIntegrations() {
  return (
    <section className="bg-[var(--stitch-bg)] py-12">
      <div className="mx-auto max-w-7xl px-6 md:px-20">
        <div className="mb-8 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="space-y-1">
            <h2 className="text-2xl font-bold tracking-tight text-[var(--stitch-text)] md:text-3xl">
              One-Click Integrations
            </h2>
            <p className="max-w-xl text-base text-[var(--stitch-text-muted)]">
              Connect your tools in seconds with our pre-built adapters.
            </p>
          </div>
          <Link
            href="/services"
            className="flex items-center gap-2 text-sm font-bold text-[var(--stitch-primary)] hover:underline"
          >
            Explore all 50+ integrations
            <MaterialIcon name="arrow_forward" size={14} />
          </Link>
        </div>
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {integrations.map((integration) => (
            <div
              key={integration.name}
              className="flex flex-col items-center gap-3 rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-6 transition-transform hover:-translate-y-1"
            >
              <div className="flex size-12 items-center justify-center text-[var(--stitch-text)]">
                <MaterialIcon name={integration.icon} size={40} />
              </div>
              <div className="text-center">
                <h4 className="text-base font-bold text-[var(--stitch-text)]">
                  {integration.name}
                </h4>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

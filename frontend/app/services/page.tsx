import Link from "next/link";
import Image from "next/image";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const platforms = [
  {
    name: "macOS",
    icon: "laptop_mac",
    description: "Compatible with Apple Silicon (M1/M2/M3) and Intel processors.",
    downloadExt: ".dmg",
    version: "v2.4.0",
  },
  {
    name: "Windows",
    icon: "window",
    description: "Support for Windows 10 & 11. Available in EXE and MSI installers.",
    downloadExt: ".exe",
    version: "v2.4.0",
  },
  {
    name: "Linux",
    icon: "terminal",
    description: "Universal support via DEB, RPM, and portable AppImage formats.",
    downloadExt: ".deb",
    version: "v2.4.0",
  },
];

function PlatformIcon({ name }: { name: string }) {
  if (name === "laptop_mac") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="4" y="5" width="16" height="11" rx="1.5" />
        <path d="M2.5 19h19" />
      </svg>
    );
  }

  if (name === "window") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="16" rx="1.5" />
        <path d="M3 10h18" />
        <path d="M12 10v10" />
      </svg>
    );
  }

  if (name === "terminal") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="16" rx="1.5" />
        <path d="M7 9l3 3-3 3" />
        <path d="M12.5 15H17" />
      </svg>
    );
  }

  return <MaterialIcon name={name} size={40} className="text-[var(--stitch-text)] transition-colors group-hover:text-white" />;
}

const pricingPlans = [
  {
    name: "Free",
    price: "$0",
    period: "/mo",
    description: "Perfect for exploring and personal testing.",
    features: [
      { text: "2 Global Nodes", included: true },
      { text: "50GB Monthly Traffic", included: true },
      { text: "Community Support", included: true },
      { text: "Advanced Analytics", included: false },
    ],
    cta: "Current Plan",
    ctaVariant: "outline",
  },
  {
    name: "Pro",
    price: "$12",
    period: "/mo",
    description: "Enhanced speed for dedicated developers.",
    features: [
      { text: "10 Global Nodes", included: true },
      { text: "500GB Monthly Traffic", included: true },
      { text: "Priority Email Support", included: true },
      { text: "Basic Analytics", included: true },
    ],
    cta: "Upgrade to Pro",
    ctaVariant: "secondary",
  },
  {
    name: "Plus",
    price: "$29",
    period: "/mo",
    description: "Full power for small to medium AI teams.",
    features: [
      { text: "50+ Global Edge Nodes", included: true },
      { text: "Unlimited Traffic", included: true },
      { text: "24/7 Priority Support", included: true },
      { text: "Custom API Integration", included: true },
    ],
    cta: "Start Plus Trial",
    ctaVariant: "primary",
    recommended: true,
  },
  {
    name: "Ultra",
    price: "$99",
    period: "/mo",
    description: "Enterprise grade reliability and scale.",
    features: [
      { text: "Dedicated AI Ingress", included: true },
      { text: "SLA Guarantees (99.99%)", included: true },
      { text: "Technical Account Manager", included: true },
      { text: "On-prem Deployment", included: true },
    ],
    cta: "Contact Enterprise",
    ctaVariant: "secondary",
  },
];

export default function ServicesPage() {
  return (
    <>
      <section className="relative overflow-hidden py-20 px-6">
        <div className="mx-auto max-w-7xl grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div className="space-y-8">
            <div className="inline-flex items-center gap-2 rounded-full border border-[var(--stitch-primary)]/20 bg-[var(--stitch-primary)]/10 px-3 py-1 text-xs font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75"></span>
                <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]"></span>
              </span>
              Now v2.4.0 Available
            </div>
            <h1 className="text-5xl font-black leading-[1.1] tracking-tight text-[var(--stitch-text)] md:text-6xl">
              Powering Your <span className="text-[var(--stitch-primary)]">AI Workflow</span> Everywhere
            </h1>
            <p className="max-w-xl text-lg leading-relaxed text-[var(--stitch-text-muted)]">
              Experience seamless multi-platform availability with ALiang Gateway. High-performance connectivity for your AI models, wherever you are. Unified, secure, and blazingly fast.
            </p>
            <div className="flex flex-wrap gap-4">
              <Link
                href="/register"
                className="rounded-xl bg-[var(--stitch-primary)] px-8 py-4 text-lg font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:-translate-y-0.5"
              >
                Get Started Free
              </Link>
              <Link
                href="/docs"
                className="rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-8 py-4 text-lg font-bold text-[var(--stitch-text)] transition-all hover:bg-[var(--stitch-bg)]/80"
              >
                View Documentation
              </Link>
            </div>
          </div>
          <div className="relative">
            <div className="aspect-video overflow-hidden rounded-2xl border-4 border-white bg-slate-200 shadow-2xl dark:border-slate-700 dark:bg-slate-800">
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBdbfe62AqCJSKa5V7u1se0IJGHIFUWK-fOmLPZ7MMaQwIyWYRTfpjRcDAxXxQoJypZFckiH1wbkf9e0P_UnsH-S1aNF65HAJX77TbNHSYo1hqtEpBgpeKai3qqu6V98jhIvmYZg-uEQ93BsCudtfwvmyYY9jxRYEz0H9HRnj4_jyBfHBIIJcM_2CJrPEDYRjFORR64yGaJNyaPdBEdXLZ-0LPUkAE4o7-ZVKeOOFJvmJnPJd6F3lVt90b2xYE8IZxbTdXtULknYrE"
                alt="Futuristic AI neural network visualization with green accents"
                fill
                className="object-cover"
                unoptimized
              />
            </div>
            <div className="absolute -bottom-6 -left-6 hidden rounded-xl border border-slate-100 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-slate-800 md:block">
              <div className="flex items-center gap-4">
                <div className="rounded-lg bg-[var(--stitch-primary)]/20 p-3 text-[var(--stitch-primary)]">
                  <MaterialIcon name="speed" size={24} />
                </div>
                <div>
                  <p className="text-xs font-bold uppercase text-[var(--stitch-text-muted)]">Average Latency</p>
                  <p className="text-2xl font-black text-[var(--stitch-text)]">&lt; 15ms</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-[var(--stitch-bg-elevated)] py-20">
        <div className="mx-auto max-w-7xl px-6">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-3xl font-black text-[var(--stitch-text)]">Choose Your Platform</h2>
            <p className="text-[var(--stitch-text-muted)]">Download the native client for your operating system</p>
          </div>
          <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
            {platforms.map((platform) => (
              <div
                key={platform.name}
                className="group rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8 text-center transition-all hover:border-[var(--stitch-primary)]"
              >
                <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-2xl bg-white shadow-sm transition-all group-hover:bg-[var(--stitch-primary)] group-hover:text-white dark:bg-slate-700">
                  <PlatformIcon name={platform.icon} />
                </div>
                <h3 className="mb-2 text-xl font-bold text-[var(--stitch-text)]">{platform.name}</h3>
                <p className="mb-6 text-sm text-[var(--stitch-text-muted)]">{platform.description}</p>
                <div className="space-y-2">
                  <button type="button" className="w-full rounded-lg bg-slate-900 py-2 font-medium text-white transition-colors hover:bg-slate-800 dark:bg-slate-700 dark:hover:bg-slate-600">
                    Download {platform.downloadExt}
                  </button>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text-muted)]">
                    Latest: {platform.version}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Pricing Plans */}
      <section className="bg-[var(--stitch-bg)] py-24 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-4xl font-black text-[var(--stitch-text)]">Flexible Service Plans</h2>
            <p className="text-[var(--stitch-text-muted)]">Scalable solutions for developers, researchers, and enterprises</p>
          </div>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
            {pricingPlans.map((plan) => (
              <div
                key={plan.name}
                className={`flex flex-col rounded-2xl border p-8 shadow-sm transition-all hover:shadow-md ${
                  plan.recommended
                    ? "relative z-10 scale-105 border-2 border-[var(--stitch-primary)] ring-4 ring-[var(--stitch-primary)]/10"
                    : "border-[var(--stitch-border)] bg-[var(--stitch-bg)]"
                }`}
              >
                {plan.recommended && (
                  <div className="absolute -top-4 left-1/2 -translate-x-1/2 rounded-full bg-[var(--stitch-primary)] px-4 py-1 text-[10px] font-black uppercase tracking-widest text-white">
                    Recommended
                  </div>
                )}
                <div className="mb-8">
                  <h3 className={`mb-2 text-lg font-bold uppercase tracking-tight ${plan.recommended ? "text-[var(--stitch-primary)]" : "text-[var(--stitch-text-muted)]"}`}>
                    {plan.name}
                  </h3>
                  <div className="flex items-baseline gap-1">
                    <span className="text-4xl font-black text-[var(--stitch-text)]">{plan.price}</span>
                    <span className="text-sm text-[var(--stitch-text-muted)]">{plan.period}</span>
                  </div>
                  <p className="mt-4 text-sm text-[var(--stitch-text-muted)]">{plan.description}</p>
                </div>
                <ul className="mb-8 flex-grow space-y-4">
                  {plan.features.map((feature) => (
                    <li
                      key={feature.text}
                      className={`flex items-center gap-3 text-sm ${!feature.included ? "opacity-50" : ""}`}
                    >
                      <MaterialIcon
                        name={feature.included ? "check_circle" : "cancel"}
                        size={18}
                        className={feature.included ? "text-[var(--stitch-primary)]" : "text-[var(--stitch-text-muted)]"}
                      />
                      {feature.text}
                    </li>
                  ))}
                </ul>
                <button
                  type="button"
                  className={`w-full rounded-lg py-3 font-bold transition-colors ${
                    plan.ctaVariant === "primary"
                      ? "bg-[var(--stitch-primary)] text-white shadow-lg shadow-[var(--stitch-primary)]/30 hover:shadow-xl"
                      : plan.ctaVariant === "outline"
                      ? "border border-[var(--stitch-primary)] text-[var(--stitch-primary)] hover:bg-[var(--stitch-primary)]/5"
                      : "bg-slate-900 text-white hover:bg-slate-800 dark:bg-slate-700 dark:hover:bg-slate-600"
                  }`}
                >
                  {plan.cta}
                </button>
              </div>
            ))}
          </div>
        </div>
      </section>
    </>
  );
}

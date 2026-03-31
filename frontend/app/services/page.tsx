"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
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

type DynamicPackage = {
  code: string;
  name: string;
  price_micros: number;
  value_type: string;
  value_amount: number;
  description: string;
  features: string[];
};

function formatPrice(priceMicros: number): string {
  if (priceMicros <= 0) return "0";
  return (priceMicros / 1000000).toFixed(priceMicros % 1000000 === 0 ? 0 : 2);
}

export default function ServicesPage() {
  const router = useRouter();
  const [packages, setPackages] = useState<DynamicPackage[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [sessionToken, setSessionToken] = useState("");
  const [checkoutPendingCode, setCheckoutPendingCode] = useState<string | null>(null);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);

  useEffect(() => {
    setSessionToken(localStorage.getItem("session_token") ?? "");
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await fetch("/api/packages");
        if (!res.ok) return;
        const data = await res.json();
        if (!cancelled && Array.isArray(data.packages)) {
          setPackages(data.packages);
        }
      } catch {
        // silent — falls back to empty state
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, []);

  const handlePackageCheckout = async (pkg: DynamicPackage) => {
    setCheckoutError(null);

    if (pkg.price_micros <= 0) {
      router.push("/dashboard");
      return;
    }

    if (!sessionToken) {
      router.push(`/login?next=${encodeURIComponent(`/services?package=${pkg.code}`)}`);
      return;
    }

    setCheckoutPendingCode(pkg.code);

    try {
      const response = await fetch("/api/checkout/package", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ tier_code: pkg.code }),
      });

      const payload = (await response.json()) as { error?: string; checkout_url?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "Failed to start package checkout");
      }

      const checkoutURL = String(payload.checkout_url ?? "").trim();
      if (!checkoutURL) {
        throw new Error("Checkout session did not return a redirect URL");
      }

      window.location.assign(checkoutURL);
    } catch (error) {
      setCheckoutError(error instanceof Error ? error.message : "Failed to start package checkout");
      setCheckoutPendingCode(null);
    }
  };

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
            <div className="aspect-video overflow-hidden rounded-2xl border-4 border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] shadow-2xl">
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBdbfe62AqCJSKa5V7u1se0IJGHIFUWK-fOmLPZ7MMaQwIyWYRTfpjRcDAxXxQoJypZFckiH1wbkf9e0P_UnsH-S1aNF65HAJX77TbNHSYo1hqtEpBgpeKai3qqu6V98jhIvmYZg-uEQ93BsCudtfwvmyYY9jxRYEz0H9HRnj4_jyBfHBIIJcM_2CJrPEDYRjFORR64yGaJNyaPdBEdXLZ-0LPUkAE4o7-ZVKeOOFJvmJnPJd6F3lVt90b2xYE8IZxbTdXtULknYrE"
                alt="Futuristic AI neural network visualization with green accents"
                fill
                className="object-cover"
                unoptimized
              />
            </div>
            <div className="absolute -bottom-6 -left-6 hidden rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-6 shadow-xl md:block">
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
                <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-2xl bg-[var(--stitch-bg-elevated)] shadow-sm transition-all group-hover:bg-[var(--stitch-primary)] group-hover:text-white">
                  <PlatformIcon name={platform.icon} />
                </div>
                <h3 className="mb-2 text-xl font-bold text-[var(--stitch-text)]">{platform.name}</h3>
                <p className="mb-6 text-sm text-[var(--stitch-text-muted)]">{platform.description}</p>
                <div className="space-y-2">
                  <button type="button" className="w-full rounded-lg bg-[var(--stitch-text)] py-2 font-medium text-[var(--stitch-bg)] transition-colors hover:bg-[var(--stitch-text)]/80">
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

      {/* Pricing Plans — Dynamic */}
      <section className="bg-[var(--stitch-bg)] py-24 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-4xl font-black text-[var(--stitch-text)]">Flexible Service Plans</h2>
            <p className="text-[var(--stitch-text-muted)]">Scalable solutions for developers, researchers, and enterprises</p>
            {checkoutError ? (
              <p className="mx-auto mt-4 max-w-2xl rounded-xl border border-red-400/35 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:border-red-400/45 dark:text-red-300">
                {checkoutError}
              </p>
            ) : null}
          </div>
          {isLoading ? (
            <p className="text-center text-[var(--stitch-text-muted)]">Loading plans...</p>
          ) : packages.length === 0 ? (
            <p className="text-center text-[var(--stitch-text-muted)]">No plans available at this time.</p>
          ) : (
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
              {packages.map((pkg) => (
                <div
                  key={pkg.code}
                  className="flex flex-col rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8 shadow-sm transition-all hover:shadow-md"
                >
                  <div className="mb-8">
                    <h3 className="mb-2 text-lg font-bold uppercase tracking-tight text-[var(--stitch-text-muted)]">
                      {pkg.name}
                    </h3>
                    <div className="flex items-baseline gap-1">
                      <span className="text-4xl font-black text-[var(--stitch-text)]">¥{formatPrice(pkg.price_micros)}</span>
                      {pkg.value_type === "days" ? <span className="text-sm text-[var(--stitch-text-muted)]">/ {pkg.value_amount}d</span> : null}
                    </div>
                    {pkg.description ? <p className="mt-4 text-sm text-[var(--stitch-text-muted)]">{pkg.description}</p> : null}
                  </div>
                  <ul className="mb-8 flex-grow space-y-4">
                    {pkg.features.map((feature) => (
                      <li key={feature} className="flex items-center gap-3 text-sm">
                        <MaterialIcon name="check_circle" size={18} className="text-[var(--stitch-primary)]" />
                        {feature}
                      </li>
                    ))}
                  </ul>
                  <button
                    type="button"
                    className="w-full rounded-lg py-3 font-bold transition-colors bg-[var(--stitch-text)] text-[var(--stitch-bg)] hover:bg-[var(--stitch-text)]/80"
                    onClick={() => void handlePackageCheckout(pkg)}
                    disabled={checkoutPendingCode === pkg.code}
                  >
                    {checkoutPendingCode === pkg.code
                      ? "Redirecting..."
                      : pkg.price_micros > 0
                        ? "Buy with Stripe"
                        : "Open Dashboard"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    </>
  );
}

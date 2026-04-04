"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useTranslations } from "next-intl";

type DownloadItem = {
  id: number;
  software_name: string;
  platform: string;
  file_type: string;
  download_url: string;
  version: string;
  force_update: boolean;
  changelog: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
};

const platformMeta: Record<string, { name: string; icon: string; description: string }> = {
  darwin: {
    name: "macOS",
    icon: "laptop_mac",
    description: "Compatible with Apple Silicon (M1/M2/M3) and Intel processors.",
  },
  windows: {
    name: "Windows",
    icon: "window",
    description: "Support for Windows 10 & 11. Available in EXE and MSI installers.",
  },
  linux: {
    name: "Linux",
    icon: "terminal",
    description: "Universal support via DEB, RPM, and portable AppImage formats.",
  },
};

const platformOrder = ["darwin", "windows", "linux"];

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

function DownloadButton({ items, t }: { items: DownloadItem[]; t: ReturnType<typeof useTranslations> }) {
  const [open, setOpen] = useState(false);

  if (items.length === 0) return null;

  const primary = items.find((d) => d.is_default) ?? items[0];
  const rest = items.filter((d) => d.id !== primary.id);

  const handleDownload = (url: string) => {
    window.open(url, "_blank", "noopener");
    setOpen(false);
  };

  if (rest.length === 0) {
    return (
      <div className="space-y-2">
        <button
          type="button"
          onClick={() => handleDownload(primary.download_url)}
          className="w-full rounded-lg bg-[var(--stitch-text)] py-2 font-medium text-[var(--stitch-bg)] transition-colors hover:bg-[var(--stitch-text)]/80"
        >
          Download .{primary.file_type}
        </button>
        <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text-muted)]">
          {t("latest", { version: primary.version })}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex gap-0.5">
        <button
          type="button"
          onClick={() => handleDownload(primary.download_url)}
          className="flex-1 rounded-l-lg bg-[var(--stitch-text)] py-2 font-medium text-[var(--stitch-bg)] transition-colors hover:bg-[var(--stitch-text)]/80"
        >
          .{primary.file_type} &middot; {primary.version}
        </button>
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="rounded-r-lg border-l border-[var(--stitch-bg)]/30 bg-[var(--stitch-text)] px-3 py-2 text-[var(--stitch-bg)] transition-colors hover:bg-[var(--stitch-text)]/80"
          aria-expanded={open}
          aria-haspopup="true"
        >
          <MaterialIcon name="arrow_drop_down" size={18} />
        </button>
      </div>

      {open && (
        <div className="rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] shadow-lg">
          {rest.map((dl) => (
            <button
              key={dl.id}
              type="button"
              onClick={() => handleDownload(dl.download_url)}
              className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-[var(--stitch-bg)] last:rounded-b-lg first:rounded-t-lg"
            >
              <span className="flex items-center gap-2">
                <span className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold uppercase" style={{ backgroundColor: "var(--stitch-primary)", color: "white" }}>
                  {dl.file_type}
                </span>
                <span className="font-mono text-xs text-[var(--stitch-text-muted)]">{dl.version}</span>
              </span>
              <MaterialIcon name="download" size={14} className="text-[var(--stitch-text-muted)]" />
            </button>
          ))}
        </div>
      )}

      <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text-muted)]">
        {t("latest", { version: primary.version })}
      </p>
    </div>
  );
}

export default function ServicesPage() {
  const router = useRouter();
  const t = useTranslations("services");
  const [packages, setPackages] = useState<DynamicPackage[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [sessionToken, setSessionToken] = useState("");
  const [checkoutPendingCode, setCheckoutPendingCode] = useState<string | null>(null);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);

  const [downloads, setDownloads] = useState<DownloadItem[]>([]);
  const [downloadsLoading, setDownloadsLoading] = useState(true);

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
        // silent
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function loadDownloads() {
      try {
        const res = await fetch("/api/public/downloads");
        if (!res.ok) return;
        const data = await res.json();
        if (!cancelled && Array.isArray(data.downloads)) {
          setDownloads(data.downloads);
        }
      } catch {
        // silent
      } finally {
        if (!cancelled) setDownloadsLoading(false);
      }
    }
    void loadDownloads();
    return () => { cancelled = true; };
  }, []);

  const handlePackageCheckout = async (pkg: DynamicPackage) => {
    setCheckoutError(null);

    if (pkg.price_micros <= 0) {
      router.push("/dashboard");
      return;
    }

    if (!sessionToken) {
      router.push("/login?next=" + encodeURIComponent("/services?package=" + pkg.code));
      return;
    }

    setCheckoutPendingCode(pkg.code);

    try {
      const response = await fetch("/api/checkout/package", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + sessionToken,
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

  // Group downloads by platform
  const grouped = new Map<string, DownloadItem[]>();
  for (const dl of downloads) {
    const existing = grouped.get(dl.platform) ?? [];
    existing.push(dl);
    grouped.set(dl.platform, existing);
  }

  // Use platformOrder to render in consistent order
  const platformsToShow = platformOrder.filter((p) => grouped.has(p));

  // Find latest version across all downloads for hero badge
  const latestVersion = downloads.length > 0 ? downloads[0].version : null;

  return (
    <>
      <section className="relative overflow-hidden py-20 px-6">
        <div className="mx-auto max-w-7xl grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div className="space-y-8">
            {latestVersion && (
              <div className="inline-flex items-center gap-2 rounded-full border border-[var(--stitch-primary)]/20 bg-[var(--stitch-primary)]/10 px-3 py-1 text-xs font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
                <span className="relative flex h-2 w-2">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75"></span>
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]"></span>
                </span>
                {t("nowAvailable", { version: latestVersion })}
              </div>
            )}
            <h1 className="text-5xl font-black leading-[1.1] tracking-tight text-[var(--stitch-text)] md:text-6xl">
              Powering Your <span className="text-[var(--stitch-primary)]">AI Workflow</span> Everywhere
            </h1>
            <p className="max-w-xl text-lg leading-relaxed text-[var(--stitch-text-muted)]">
              {t("description")}
            </p>
            <div className="flex flex-wrap gap-4">
              <Link
                href="/register"
                className="rounded-xl bg-[var(--stitch-primary)] px-8 py-4 text-lg font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:-translate-y-0.5"
              >
                {t("getStartedFree")}
              </Link>
              <Link
                href="/docs"
                className="rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-8 py-4 text-lg font-bold text-[var(--stitch-text)] transition-all hover:bg-[var(--stitch-bg)]/80"
              >
                {t("viewDocumentation")}
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
                  <p className="text-xs font-bold uppercase text-[var(--stitch-text-muted)]">{t("averageLatency")}</p>
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
            <h2 className="mb-4 text-3xl font-black text-[var(--stitch-text)]">{t("choosePlatform")}</h2>
            <p className="text-[var(--stitch-text-muted)]">{t("choosePlatformSubtitle")}</p>
          </div>

          {downloadsLoading ? (
            <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
              {[1, 2, 3].map((s) => (
                <div key={s} className="h-64 animate-pulse rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8" />
              ))}
            </div>
          ) : platformsToShow.length === 0 ? (
            <p className="text-center text-[var(--stitch-text-muted)]">{t("noDownloads")}</p>
          ) : (
            <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
              {platformsToShow.map((platformKey) => {
                const meta = platformMeta[platformKey];
                const items = grouped.get(platformKey) ?? [];
                return (
                  <div
                    key={platformKey}
                    className="group rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8 text-center transition-all hover:border-[var(--stitch-primary)]"
                  >
                    <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-2xl bg-[var(--stitch-bg-elevated)] shadow-sm transition-all group-hover:bg-[var(--stitch-primary)] group-hover:text-white">
                      <PlatformIcon name={meta.icon} />
                    </div>
                    <h3 className="mb-2 text-xl font-bold text-[var(--stitch-text)]">{meta.name}</h3>
                    <p className="mb-6 text-sm text-[var(--stitch-text-muted)]">{meta.description}</p>
                    <DownloadButton items={items} t={t} />
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </section>

      {/* Pricing Plans */}
      <section className="bg-[var(--stitch-bg)] py-24 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-4xl font-black text-[var(--stitch-text)]">{t("flexiblePlans")}</h2>
            <p className="text-[var(--stitch-text-muted)]">{t("flexiblePlansSubtitle")}</p>
            {checkoutError ? (
              <p className="mx-auto mt-4 max-w-2xl rounded-xl border border-red-400/35 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:border-red-400/45 dark:text-red-300">
                {checkoutError}
              </p>
            ) : null}
          </div>
          {isLoading ? (
            <p className="text-center text-[var(--stitch-text-muted)]">{t("loadingPlans")}</p>
          ) : packages.length === 0 ? (
            <p className="text-center text-[var(--stitch-text-muted)]">{t("noPlans")}</p>
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
                      <span className="text-4xl font-black text-[var(--stitch-text)]">{"\u00A5"}{formatPrice(pkg.price_micros)}</span>
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
                      ? t("redirecting")
                      : pkg.price_micros > 0
                        ? t("buyWithStripe")
                        : t("openDashboard")}
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

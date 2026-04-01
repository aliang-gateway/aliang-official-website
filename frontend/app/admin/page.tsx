"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { extractApiError, unwrapData, asRecord } from "@/lib/api-response";

type AuthMeResponse = {
  id: number;
  email: string;
  name: string;
  role: "user" | "admin";
  created_at: string;
  updated_at: string;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

const quickLinks = [
  { href: "/admin/packages", icon: "inventory_2", label: "Packages", desc: "Manage subscription packages & tiers" },
  { href: "/admin/payments", icon: "receipt_long", label: "Payments", desc: "View payment records & transactions" },
  { href: "/admin/articles", icon: "article", label: "Articles", desc: "Create & publish MDX articles" },
  { href: "/admin/config-center", icon: "settings", label: "Config Center", desc: "Software configs, templates & global vars" },
];

export default function AdminPage() {
  const router = useRouter();
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [authError, setAuthError] = useState<string | null>(null);
  const [adminProfile, setAdminProfile] = useState<AuthMeResponse | null>(null);

  useEffect(() => {
    const run = async () => {
      const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
      if (!sessionToken) {
        router.replace("/login");
        return;
      }

      try {
        const meResponse = await fetch("/api/auth/me", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
        });

        const mePayload = (await meResponse.json()) as unknown;
        if (!meResponse.ok) {
          throw new Error(extractApiError(mePayload, "failed to verify session"));
        }

        const profile = unwrapData<AuthMeResponse>(mePayload) ?? (asRecord(mePayload) as AuthMeResponse | null);
        if (!profile) {
          throw new Error("failed to verify session");
        }
        if (profile.role !== "admin") {
          router.replace("/account");
          return;
        }

        setAdminProfile(profile);
      } catch (error) {
        const message = error instanceof Error ? error.message : "failed to verify session";
        setAuthError(message);
        localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
        router.replace("/login");
        return;
      } finally {
        setIsCheckingAuth(false);
      }
    };

    void run();
  }, [router]);

  if (isCheckingAuth) {
    return (
      <p className="text-sm text-[var(--stitch-text-muted)]">Checking admin session...</p>
    );
  }

  if (authError) {
    return (
      <div
        className="rounded-xl border border-red-400/40 bg-red-500/10 p-4 text-sm text-red-700"
        role="alert"
      >
        {authError}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Welcome */}
      <div className="block-card p-5">
        <div className="flex items-center gap-3 mb-1">
          <span className="rounded-lg bg-[var(--stitch-primary)]/10 p-2 text-[var(--stitch-primary)]">
            <MaterialIcon name="admin_panel_settings" size={20} />
          </span>
          <div>
            <h2 className="text-lg font-bold text-[var(--portal-ink)]">
              Welcome back{adminProfile ? `, ${adminProfile.name}` : ""}
            </h2>
            <p className="text-sm text-[var(--portal-muted)]">
              {adminProfile ? adminProfile.email : ""}
            </p>
          </div>
        </div>
      </div>

      {/* Quick links grid */}
      <div className="grid gap-4 sm:grid-cols-2">
        {quickLinks.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            className="block-card p-5 flex items-start gap-4 transition-transform hover:translate-y-[-2px] hover:shadow-lg"
          >
            <span className="rounded-xl bg-[var(--portal-accent)]/10 p-3 text-[var(--portal-accent)]">
              <MaterialIcon name={link.icon} size={24} />
            </span>
            <div>
              <h3 className="font-semibold text-[var(--portal-ink)]">{link.label}</h3>
              <p className="text-sm text-[var(--portal-muted)] mt-0.5">{link.desc}</p>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { LanguageSwitcher } from "@/components/ui/LanguageSwitcher";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

const adminNavItems = [
  { href: "/admin", labelKey: "overview", icon: "dashboard" },
  { href: "/admin/users", labelKey: "users", icon: "people" },
  { href: "/admin/packages", labelKey: "packages", icon: "inventory_2" },
  { href: "/admin/payments", labelKey: "payments", icon: "receipt_long" },
  { href: "/admin/articles", labelKey: "articles", icon: "article" },
  { href: "/admin/docs", labelKey: "docs", icon: "description" },
  { href: "/admin/config-center", labelKey: "configCenter", icon: "settings" },
  { href: "/admin/download-center", labelKey: "downloads", icon: "cloud_download" },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const t = useTranslations("adminLayout");
  const pathname = usePathname();
  const router = useRouter();
  const [authed, setAuthed] = useState(false);
  const [role, setRole] = useState<"admin" | null>(null);

  useEffect(() => {
    const check = async () => {
      const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY);
      if (!token) {
        router.replace("/login");
        return;
      }

      try {
        const res = await fetch("/api/auth/me", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${token}`,
          },
          cache: "no-store",
        });
        if (!res.ok) {
          localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
          router.replace("/login");
          return;
        }
        const payload = await res.json();
        const role = payload?.data?.role ?? payload?.role;
        if (role === "distributor") {
          router.replace("/distributor");
          return;
        }
        if (role !== "admin") {
          router.replace("/account");
          return;
        }
        setRole(role);
        setAuthed(true);
      } catch {
        router.replace("/login");
      }
    };
    void check();
  }, [router]);

  if (!authed) {
    return (
      <section className="portal-shell py-8">
        <p className="text-sm text-[var(--stitch-text-muted)]">{t("checking")}</p>
      </section>
    );
  }

  return (
    <section className="portal-shell py-8">
      {/* Admin header */}
      <div className="clay-panel space-y-3 p-5 mb-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-1">
            <h1 className="section-title">
              <span className="gradient-text">{t("title")}</span>
            </h1>
          </div>
          <LanguageSwitcher />
        </div>
        <nav className="flex flex-wrap gap-2">
          {adminNavItems.map((item) => {
            const isActive =
              item.href === "/admin"
                ? pathname === "/admin"
                : pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`nav-pill ${isActive ? "!bg-[var(--portal-accent)] !text-white" : ""}`}
              >
                <MaterialIcon name={item.icon} size={16} className="mr-1" />
                {t(item.labelKey)}
              </Link>
            );
          })}
        </nav>
      </div>

      {/* Page content */}
      <div>{children}</div>
    </section>
  );
}

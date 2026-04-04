"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const adminNavItems = [
  { href: "/admin", label: "Overview", icon: "dashboard" },
  { href: "/admin/packages", label: "Packages", icon: "inventory_2" },
  { href: "/admin/payments", label: "Payments", icon: "receipt_long" },
  { href: "/admin/articles", label: "Articles", icon: "article" },
  { href: "/admin/docs", label: "Docs", icon: "description" },
  { href: "/admin/config-center", label: "Config Center", icon: "settings" },
  { href: "/admin/download-center", label: "Downloads", icon: "cloud_download" },
];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <section className="portal-shell py-8">
      {/* Admin header */}
      <div className="clay-panel space-y-3 p-5 mb-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-1">
            <h1 className="section-title">
              <span className="gradient-text">Admin Console</span>
            </h1>
          </div>
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
                {item.label}
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

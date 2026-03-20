import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

const footerLinks = {
  product: [
    { href: "/services", label: "Features" },
    { href: "/pricing", label: "Pricing" },
    { href: "/download", label: "Download" },
    { href: "#", label: "Changelog" },
  ],
  resources: [
    { href: "/docs", label: "Documentation" },
    { href: "/blog", label: "Blog" },
    { href: "#", label: "API Reference" },
    { href: "#", label: "Status" },
  ],
  company: [
    { href: "#", label: "About" },
    { href: "#", label: "Careers" },
    { href: "#", label: "Contact" },
    { href: "#", label: "Partners" },
  ],
  social: [
    { href: "#", label: "GitHub", icon: "code" },
    { href: "#", label: "Twitter", icon: "share" },
    { href: "#", label: "Discord", icon: "forum" },
  ],
};

export function SiteFooter() {
  return (
    <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-6 py-12 md:px-20">
      <div className="grid grid-cols-2 gap-8 md:grid-cols-4">
        <div>
          <h3 className="mb-4 font-bold text-[var(--stitch-text)]">Product</h3>
          <ul className="space-y-2 text-sm text-[var(--stitch-text-muted)]">
            {footerLinks.product.map((link) => (
              <li key={link.href + link.label}>
                <Link
                  href={link.href}
                  className="transition-colors hover:text-[var(--stitch-primary)]"
                >
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h3 className="mb-4 font-bold text-[var(--stitch-text)]">Resources</h3>
          <ul className="space-y-2 text-sm text-[var(--stitch-text-muted)]">
            {footerLinks.resources.map((link) => (
              <li key={link.href + link.label}>
                <Link
                  href={link.href}
                  className="transition-colors hover:text-[var(--stitch-primary)]"
                >
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h3 className="mb-4 font-bold text-[var(--stitch-text)]">Company</h3>
          <ul className="space-y-2 text-sm text-[var(--stitch-text-muted)]">
            {footerLinks.company.map((link) => (
              <li key={link.href + link.label}>
                <Link
                  href={link.href}
                  className="transition-colors hover:text-[var(--stitch-primary)]"
                >
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h3 className="mb-4 font-bold text-[var(--stitch-text)]">Social</h3>
          <ul className="space-y-2 text-sm text-[var(--stitch-text-muted)]">
            {footerLinks.social.map((link) => (
              <li key={link.href + link.label}>
                <Link
                  href={link.href}
                  className="flex items-center gap-2 transition-colors hover:text-[var(--stitch-primary)]"
                >
                  <MaterialIcon name={link.icon} size={18} />
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="mt-12 flex flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] pt-8 md:flex-row">
        <div className="flex items-center gap-3">
          <div className="flex size-7 items-center justify-center rounded bg-[var(--stitch-primary)] text-white dark:text-white">
            <MaterialIcon name="hub" size={18} />
          </div>
          <span className="text-sm font-medium text-[var(--stitch-text)]">
            ALiang Gateway
          </span>
        </div>
        <p className="text-sm text-[var(--stitch-text-muted)]">
          © {new Date().getFullYear()} ALiang Gateway. All rights reserved.
        </p>
      </div>
    </footer>
  );
}

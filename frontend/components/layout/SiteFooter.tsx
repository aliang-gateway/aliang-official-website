"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type FooterGroup = {
  title: string;
  links: Array<{ href: string; label: string }>;
};

function FooterBrand({ title, description }: { title: string; description: string }) {
  return (
    <div className="col-span-1 space-y-4 md:col-span-1">
      <div className="flex items-center gap-3">
        <div className="flex size-6 items-center justify-center rounded bg-[var(--stitch-primary)] text-white">
          <MaterialIcon name="hub" size={14} />
        </div>
        <h2 className="text-lg font-bold text-[var(--stitch-text)]">{title}</h2>
      </div>
      <p className="text-sm text-[var(--stitch-text-muted)]">{description}</p>
    </div>
  );
}

function FooterColumns({ groups }: { groups: FooterGroup[] }) {
  return (
    <>
      {groups.map((group) => (
        <div key={group.title}>
          <h4 className="mb-6 text-xs font-bold uppercase tracking-widest text-[var(--stitch-text)]">
            {group.title}
          </h4>
          <ul className="space-y-4 text-sm text-[var(--stitch-text-muted)]">
            {group.links.map((link) => (
              <li key={link.label}>
                <Link href={link.href} className="transition-colors hover:text-[var(--stitch-primary)]">
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </>
  );
}

export function SiteFooter() {
  const pathname = usePathname();
  const isBlog = pathname.startsWith("/blog");
  const isServices = pathname === "/services";
  const isCompactHome = pathname === "/compact";

  if (isBlog) {
    const groups: FooterGroup[] = [
      {
        title: "Resources",
        links: [
          { href: "/docs", label: "Documentation" },
          { href: "#", label: "API Reference" },
          { href: "#", label: "Community Forum" },
          { href: "#", label: "Open Source" },
        ],
      },
      {
        title: "Platform",
        links: [
          { href: "#", label: "AI Routing Engine" },
          { href: "#", label: "TUN Proxy Service" },
          { href: "#", label: "Edge Locations" },
          { href: "#", label: "Status Page" },
        ],
      },
      {
        title: "Company",
        links: [
          { href: "#", label: "About Us" },
          { href: "#", label: "Careers" },
          { href: "#", label: "Security Policy" },
          { href: "#", label: "Contact" },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-6 py-12 md:px-20">
        <div className="mx-auto grid max-w-7xl grid-cols-1 gap-12 md:grid-cols-4">
          <FooterBrand
            title="ALiang Gateway"
            description="Redefining edge intelligence through advanced networking protocols and LLM-centric gateway architecture."
          />
          <FooterColumns groups={groups} />
        </div>

        <div className="mx-auto mt-12 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] pt-8 md:flex-row">
          <p className="text-xs text-[var(--stitch-text-muted)]">© 2024 ALiang Tech Gateway. All rights reserved.</p>
          <div className="flex gap-6 text-[var(--stitch-text-muted)]">
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="share" size={20} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="terminal" size={20} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
          </div>
        </div>
      </footer>
    );
  }

  if (isServices) {
    const groups: FooterGroup[] = [
      {
        title: "Resources",
        links: [
          { href: "/docs", label: "Documentation" },
          { href: "#", label: "API Reference" },
          { href: "#", label: "Community Forum" },
          { href: "#", label: "Status Page" },
        ],
      },
      {
        title: "Platform",
        links: [
          { href: "#", label: "macOS Client" },
          { href: "#", label: "Windows Client" },
          { href: "#", label: "Linux Client" },
          { href: "#", label: "Browser Extension" },
        ],
      },
      {
        title: "Company",
        links: [
          { href: "#", label: "About Us" },
          { href: "#", label: "Careers" },
          { href: "#", label: "Privacy Policy" },
          { href: "#", label: "Terms of Service" },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-4 py-16">
        <div className="mx-auto grid max-w-7xl grid-cols-2 gap-12 md:grid-cols-4 lg:grid-cols-5">
          <div className="col-span-2">
            <div className="mb-6 flex items-center gap-2">
              <MaterialIcon name="hub" size={28} className="text-[var(--stitch-primary)]" />
              <span className="text-xl font-bold tracking-tight text-[var(--stitch-text)]">ALiang Gateway</span>
            </div>
            <p className="mb-6 max-w-xs text-sm leading-relaxed text-[var(--stitch-text-muted)]">
              Accelerating the world&apos;s AI development with secure, low-latency connectivity solutions for every platform.
            </p>
            <div className="flex gap-4 text-[var(--stitch-text-muted)]">
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="language" size={20} /></Link>
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="share" size={20} /></Link>
              <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
            </div>
          </div>
          <FooterColumns groups={groups} />
        </div>
        <div className="mx-auto mt-16 max-w-7xl border-t border-[var(--stitch-border)] pt-8 text-center text-xs text-[var(--stitch-text-muted)]">
          © 2024 ALiang Gateway. All rights reserved. Built with precision for the AI era.
        </div>
      </footer>
    );
  }

  if (isCompactHome) {
    const groups: FooterGroup[] = [
      {
        title: "Product",
        links: [
          { href: "/#features", label: "Features" },
          { href: "/services", label: "Pricing" },
        ],
      },
      {
        title: "Resources",
        links: [
          { href: "/docs", label: "Documentation" },
          { href: "/blog", label: "Blog" },
        ],
      },
      {
        title: "Company",
        links: [
          { href: "#", label: "Security" },
          { href: "#", label: "Privacy" },
        ],
      },
    ];

    return (
      <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-8">
        <div className="mx-auto grid max-w-7xl grid-cols-1 gap-8 px-6 md:grid-cols-4 md:px-20">
          <div className="col-span-1 space-y-3">
            <div className="flex items-center gap-3">
              <div className="flex size-5 items-center justify-center rounded bg-[var(--stitch-primary)] text-white">
                <MaterialIcon name="hub" size={10} />
              </div>
              <h2 className="text-base font-bold text-[var(--stitch-text)]">ALiang Gateway</h2>
            </div>
            <p className="text-xs text-[var(--stitch-text-muted)]">The reliable bridge to professional AI services.</p>
          </div>
          {groups.map((group) => (
            <div key={group.title}>
              <h4 className="mb-4 text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text)]">{group.title}</h4>
              <ul className="space-y-2 text-xs text-[var(--stitch-text-muted)]">
                {group.links.map((link) => (
                  <li key={link.label}>
                    <Link href={link.href} className="transition-colors hover:text-[var(--stitch-primary)]">
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mx-auto mt-8 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] px-6 pt-6 md:flex-row md:px-20">
          <p className="text-[10px] text-[var(--stitch-text-muted)]">© 2024 ALiang Gateway Inc. All rights reserved.</p>
          <div className="flex gap-4 text-[var(--stitch-text-muted)]">
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={16} /></Link>
            <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="alternate_email" size={16} /></Link>
          </div>
        </div>
      </footer>
    );
  }

  const groups: FooterGroup[] = [
    {
      title: "Product",
      links: [
        { href: "/#features", label: "Features" },
        { href: "/#integrations", label: "Integrations" },
        { href: "/services", label: "Pricing" },
        { href: "#", label: "API Reference" },
      ],
    },
    {
      title: "Resources",
      links: [
        { href: "/docs", label: "Documentation" },
        { href: "/blog", label: "Blog" },
        { href: "#", label: "Support" },
        { href: "#", label: "Status" },
      ],
    },
    {
      title: "Company",
      links: [
        { href: "#", label: "About" },
        { href: "#", label: "Security" },
        { href: "#", label: "Privacy" },
        { href: "#", label: "Terms" },
      ],
    },
  ];

  return (
    <footer className="border-t border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] py-12">
      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-12 px-6 md:grid-cols-4 md:px-20">
        <FooterBrand
          title="ALiang AI"
          description="The world's most reliable bridge to professional AI services."
        />
        <FooterColumns groups={groups} />
      </div>

      <div className="mx-auto mt-12 flex max-w-7xl flex-col items-center justify-between gap-4 border-t border-[var(--stitch-border)] px-6 pt-8 md:flex-row md:px-20">
        <p className="text-xs text-[var(--stitch-text-muted)]">© 2024 ALiang AI Services Inc. All rights reserved.</p>
        <div className="flex gap-6 text-[var(--stitch-text-muted)]">
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="public" size={20} /></Link>
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="alternate_email" size={20} /></Link>
          <Link href="#" className="transition-colors hover:text-[var(--stitch-primary)]"><MaterialIcon name="rss_feed" size={20} /></Link>
        </div>
      </div>
    </footer>
  );
}

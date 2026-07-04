"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { EditorialHeader } from "./EditorialHeader";
import { EditorialFooter } from "./EditorialFooter";

/**
 * Editorial page chrome: warm-paper wrapper, side rail, topbar/nav header,
 * footer, plus the scroll behaviors (nav dock/hide + section-dock scroll-spy)
 * and reveal-on-scroll. Rendered once by (marketing)/layout.tsx.
 */
export function EditorialShell({ children }: { children: React.ReactNode }) {
  const t = useTranslations("editorial");
  const pathname = usePathname();
  const shellRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const shell = shellRef.current;
    if (!shell) return;

    const nav = shell.querySelector<HTMLElement>(".nav");
    const dockLinks = Array.from(
      shell.querySelectorAll<HTMLAnchorElement>(".section-dock a[href^='#']")
    );
    const targets = dockLinks
      .map((link) => {
        const id = link.getAttribute("href");
        return id ? shell.querySelector(id) : null;
      })
      .filter((el): el is Element => el !== null);
    const revealItems = Array.from(shell.querySelectorAll<HTMLElement>("[data-reveal]"));
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    let lastY = window.scrollY;
    let ticking = false;

    const update = () => {
      const y = window.scrollY;
      if (nav) {
        nav.classList.toggle("is-docked", y > 18);
        nav.classList.toggle("is-hidden", y > 420 && y > lastY);
      }
      let active = -1;
      targets.forEach((section, index) => {
        const rect = section.getBoundingClientRect();
        if (rect.top <= 150 && rect.bottom > 150) active = index;
      });
      dockLinks.forEach((link, index) => {
        if (index === active) link.setAttribute("aria-current", "true");
        else link.removeAttribute("aria-current");
      });
      lastY = y;
      ticking = false;
    };

    const onScroll = () => {
      if (!ticking) {
        window.requestAnimationFrame(update);
        ticking = true;
      }
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    update();

    let observer: IntersectionObserver | null = null;
    if (reduceMotion || !("IntersectionObserver" in window)) {
      revealItems.forEach((el) => el.classList.add("is-visible"));
    } else {
      const obs = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            if (entry.isIntersecting) {
              entry.target.classList.add("is-visible");
              obs.unobserve(entry.target);
            }
          });
        },
        { threshold: 0.16, rootMargin: "0px 0px -8% 0px" }
      );
      observer = obs;
      revealItems.forEach((el) => obs.observe(el));
    }

    return () => {
      window.removeEventListener("scroll", onScroll);
      observer?.disconnect();
    };
  }, [pathname]);

  return (
    <div className="editorial">
      <div className="side-rail right" aria-hidden="true">
        {t("brandName")} · {t("edition")}
      </div>
      <div className="shell" id="top" ref={shellRef}>
        <EditorialHeader />
        <main>{children}</main>
        <EditorialFooter />
      </div>
    </div>
  );
}

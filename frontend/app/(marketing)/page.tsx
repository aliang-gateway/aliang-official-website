"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

type Stat = { value: string; label: string };
type DockItem = { name: string; note: string };
type NextItem = { b: string; small: string; href: string };
type CapCard = { num: string; tag: string; title: string; desc: string };
type Filter = { key: string; label: string };
type LabCard = { kicker: string; title: string; desc: string; category: string; img: string };
type MethodStep = { title: string; desc: string };
type WorkCard = { chip: string; title: string; desc: string; img: string };

function useEditableBrief(ed: ReturnType<typeof useTranslations>) {
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [fresh, setFresh] = useState(false);
  const briefText = ed.raw("home.briefText") as string;

  const copy = async () => {
    const flash = () => {
      setFresh(false);
      requestAnimationFrame(() => setFresh(true));
    };
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(briefText);
        setStatus("success");
      } else {
        setStatus("error");
      }
    } catch {
      setStatus("error");
    }
    flash();
  };

  return { status, fresh, copy, briefText };
}

export default function HomePage() {
  const ed = useTranslations("editorial");

  const stats = ed.raw("home.stats") as Stat[];
  const heroIndex = ed.raw("home.heroIndex") as string[];
  const dockItems = ed.raw("home.dockItems") as DockItem[];
  const nextItems = ed.raw("home.nextItems") as NextItem[];
  const capCards = ed.raw("home.capCards") as CapCard[];
  const filters = ed.raw("home.filters") as Filter[];
  const labCards = ed.raw("home.labCards") as LabCard[];
  const methodSteps = ed.raw("home.methodSteps") as MethodStep[];
  const workCards = ed.raw("home.workCards") as WorkCard[];
  const partners = ed.raw("home.partners") as string[];

  const dockAnchors = ["#about", "#capabilities", "#labs", "#method", "#work", "#testimonial", "#cta"];
  const [filter, setFilter] = useState("all");
  const visibleLabs = filter === "all" ? labCards : labCards.filter((l) => l.category === filter);
  const brief = useEditableBrief(ed);

  return (
    <>
      {/* ---------------- HERO ---------------- */}
      <header className="hero" aria-labelledby="hero-title">
        <div className="container wide hero-grid">
          <div className="hero-copy" data-reveal>
            <div className="label">{ed("home.heroLabel")}</div>
            <h1 className="display" id="hero-title">
              {ed("home.heroTitlePre")} <em>{ed("home.heroTitleEm")}</em> {ed("home.heroTitlePost")}
              <span className="dot">.</span>
            </h1>
            <p className="lead">{ed("home.heroLead")}</p>
            <div className="button-row">
              <a className="btn primary" href="#cta">
                {ed("home.heroPrimary")}
              </a>
              <Link className="btn" href="/services">
                {ed("home.heroSecondary")}
              </Link>
            </div>
            <div className="hero-stats" aria-label={ed("home.statsAria")}>
              {stats.map((s) => (
                <div className="stat" key={s.label}>
                  <b>{s.value}</b>
                  <span>{s.label}</span>
                </div>
              ))}
            </div>
          </div>
          <figure className="hero-art" data-reveal data-reveal-delay="1">
            <img src="/editorial/hero.svg" alt="" width={1024} height={1024} />
            <figcaption className="hero-index">
              {heroIndex.map((item, i) => (
                <span key={item}>
                  <b>{String(i + 1).padStart(2, "0")}</b>
                  {item}
                </span>
              ))}
            </figcaption>
          </figure>
        </div>

        <nav className="section-dock container wide" aria-label={ed("home.dockAria")}>
          <div className="section-dock-title">
            <span className="section-dock-kicker">{ed("home.dockKicker")}</span>
            <strong>{ed("home.dockTitle")}</strong>
            <span>{ed("home.dockNote")}</span>
          </div>
          <div className="section-dock-links">
            {dockItems.map((item, i) => (
              <a key={item.name} href={dockAnchors[i]}>
                <span className="num">{String(i + 1).padStart(2, "0")}</span>
                <span className="chapter-name">{item.name}</span>
                <span className="chapter-note">{item.note}</span>
              </a>
            ))}
          </div>
        </nav>

        <div className="container wide next-strip" aria-label={ed("home.nextAria")}>
          <div className="next-strip-track">
            <div className="next-strip-title">
              <span>{ed("home.nextTitle")}</span>
              <strong>{ed("home.nextLabel")}</strong>
            </div>
            {nextItems.map((item) =>
              item.href.startsWith("#") ? (
                <a key={item.b} href={item.href}>
                  <b>{item.b}</b>
                  <small>{item.small}</small>
                </a>
              ) : (
                <Link key={item.b} href={item.href}>
                  <b>{item.b}</b>
                  <small>{item.small}</small>
                </Link>
              )
            )}
          </div>
        </div>
      </header>

      {/* ---------------- ABOUT ---------------- */}
      <section id="about" aria-labelledby="about-title">
        <div className="container about-grid">
          <figure className="plate" data-reveal>
            <img src="/editorial/about.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
          <div className="manifesto" data-reveal data-reveal-delay="1">
            <span className="stamp">{ed("home.aboutStamp")}</span>
            <div>
              <div className="label">{ed("home.aboutLabel")}</div>
              <h2 className="display" id="about-title">
                {ed("home.aboutTitle")}
                <span className="dot">.</span>
              </h2>
            </div>
            <p>{ed("home.aboutLead1")}</p>
            <p>{ed("home.aboutLead2")}</p>
          </div>
        </div>
      </section>

      {/* ---------------- CAPABILITIES ---------------- */}
      <section id="capabilities" aria-labelledby="capabilities-title">
        <div className="container wide capability-layout">
          <figure className="plate capability-art" data-reveal>
            <img src="/editorial/capabilities.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
          <div>
            <div className="section-header" data-reveal>
              <div>
                <div className="label">{ed("home.capLabel")}</div>
                <h2 className="display" id="capabilities-title">
                  {ed("home.capTitle")}
                  <span className="dot">.</span>
                </h2>
              </div>
              <p className="lead">{ed("home.capLead")}</p>
            </div>
            <div className="capability-list">
              {capCards.map((c, i) => (
                <article
                  className="cap-card"
                  key={c.title}
                  data-reveal
                  data-reveal-delay={i === 0 ? undefined : String(i)}
                >
                  <span className="num">{c.num}</span>
                  <span className="tag">{c.tag}</span>
                  <div className="cap-body">
                    <h3>{c.title}</h3>
                    <p>{c.desc}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ---------------- LABS ---------------- */}
      <section id="labs" aria-labelledby="labs-title">
        <div className="container wide">
          <div className="labs-head" data-reveal>
            <div>
              <div className="label">{ed("home.labsLabel")}</div>
              <h2 className="display" id="labs-title">
                {ed("home.labsTitle")}
                <span className="dot">.</span>
              </h2>
            </div>
            <div className="filter-pills" aria-label={ed("home.filterAria")}>
              {filters.map((f) => (
                <button
                  key={f.key}
                  type="button"
                  onClick={() => setFilter(f.key)}
                  aria-pressed={filter === f.key}
                >
                  {f.label}
                </button>
              ))}
            </div>
          </div>
          <p className="filter-status" aria-live="polite">
            {filter === "all"
              ? ed("home.filterStatusAll", { count: visibleLabs.length })
              : ed("home.filterStatusFiltered", {
                  filter: filters.find((f) => f.key === filter)?.label ?? "",
                  count: visibleLabs.length,
                })}
          </p>
          <div className="labs-progress" aria-hidden="true">
            <span style={{ transform: `scaleX(${Math.max(visibleLabs.length / labCards.length, 0.08)})` }} />
          </div>
          <div className="labs-grid">
            {labCards.map((lab) => {
              const hidden = filter !== "all" && lab.category !== filter;
              return (
                <article
                  key={lab.title}
                  className={`lab-card${hidden ? " is-hidden" : ""}`}
                  hidden={hidden}
                  data-reveal
                >
                  <img src={lab.img} alt="" width={768} height={1024} loading="lazy" />
                  <span className="kicker">{lab.kicker}</span>
                  <h3>{lab.title}</h3>
                  <p>{lab.desc}</p>
                </article>
              );
            })}
          </div>
        </div>
      </section>

      {/* ---------------- METHOD ---------------- */}
      <section id="method" aria-labelledby="method-title">
        <div className="container">
          <div className="section-header" data-reveal>
            <div>
              <div className="label">{ed("home.methodLabel")}</div>
              <h2 className="display" id="method-title">
                {ed("home.methodTitle")}
                <span className="dot">.</span>
              </h2>
            </div>
            <p className="lead">{ed("home.methodLead")}</p>
          </div>
          <div className="method-grid">
            {methodSteps.map((step, i) => (
              <article
                className="method-step"
                key={step.title}
                data-reveal
                data-reveal-delay={i === 0 ? undefined : String(i)}
              >
                <img src={`/editorial/method-${i + 1}.svg`} alt="" width={816} height={816} loading="lazy" />
                <span className="num">{String(i + 1).padStart(2, "0")}</span>
                <h3>{step.title}</h3>
                <p>{step.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* ---------------- WORK ---------------- */}
      <section className="work" id="work" aria-labelledby="work-title">
        <div className="container wide work-grid">
          <div data-reveal>
            <div className="label">{ed("home.workLabel")}</div>
            <h2 className="display" id="work-title">
              {ed("home.workTitle")}
              <span className="dot">.</span>
            </h2>
            <p className="lead">{ed("home.workLead")}</p>
          </div>
          <div className="work-cards">
            {workCards.map((w, i) => (
              <article
                className="work-card"
                key={w.title}
                data-reveal
                data-reveal-delay={i === 0 ? undefined : String(i + 1)}
              >
                <img src={w.img} alt="" width={768} height={1024} loading="lazy" />
                <span className="work-chip">{w.chip}</span>
                <h3>{w.title}</h3>
                <p>{w.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* ---------------- TESTIMONIAL ---------------- */}
      <section id="testimonial" aria-labelledby="testimonial-title">
        <div className="container testimonial-grid">
          <div data-reveal>
            <div className="label">{ed("home.quoteLabel")}</div>
            <h2 className="quote" id="testimonial-title">
              {ed("home.quoteText")}
            </h2>
            <div className="quote-author">{ed("home.quoteAuthor")}</div>
            <div className="partner-row" aria-label={ed("home.partnersAria")}>
              {partners.map((p) => (
                <span key={p}>{p}</span>
              ))}
            </div>
          </div>
          <figure className="plate" data-reveal data-reveal-delay="1">
            <img src="/editorial/testimonial.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
        </div>
      </section>

      {/* ---------------- CTA ---------------- */}
      <section className="cta" id="cta" aria-labelledby="cta-title">
        <div className="container cta-grid">
          <div data-reveal>
            <div className="label">{ed("home.ctaLabel")}</div>
            <h2 className="display" id="cta-title">
              {ed("home.ctaTitle")}
              <span className="dot">.</span>
            </h2>
            <p className="lead">{ed("home.ctaLead")}</p>
            <div className="button-row">
              <button className="btn primary" type="button" onClick={brief.copy}>
                {ed("home.ctaCopyBtn")}
              </button>
              <Link className="btn" href="/services">
                {ed("home.ctaSecondary")}
              </Link>
            </div>
            <p
              className={`copy-status${brief.fresh ? " is-fresh" : ""}${
                brief.status === "success" ? " is-success" : brief.status === "error" ? " is-error" : ""
              }`}
              aria-live="polite"
            >
              {brief.status === "success"
                ? ed("home.copySuccess")
                : brief.status === "error"
                  ? ed("home.copyError")
                  : ""}
            </p>
            {brief.status === "error" && (
              <div className="fallback-brief">
                <b>{ed("home.ctaCopyBtn")}</b>
                <pre>{brief.briefText}</pre>
              </div>
            )}
            <span className="ribbon">{ed("home.ctaRibbon")}</span>
          </div>
          <figure className="plate" data-reveal data-reveal-delay="1">
            <img src="/editorial/cta.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
        </div>
      </section>
    </>
  );
}

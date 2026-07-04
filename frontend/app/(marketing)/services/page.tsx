"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

type Stat = { value: string; label: string };
type Filter = { key: string; label: string };
type TimelineItem = { phase: string; status: "research" | "done"; title: string; desc: string };

export default function ServicesPage() {
  const s = useTranslations("editorial.services");

  const stats = s.raw("stats") as Stat[];
  const filters = s.raw("filters") as Filter[];
  const items = s.raw("items") as TimelineItem[];

  const [filter, setFilter] = useState("all");
  const visible = filter === "all" ? items : items.filter((it) => it.status === filter);
  const feedbackKey = filter as "all" | "done" | "research";
  const feedbackMap: Record<string, string> = {
    all: s("filterAll"),
    done: s("filterDone"),
    research: s("filterResearch"),
  };
  const empty = visible.length === 0;

  return (
    <div className="page-services">
      <header className="hero" aria-labelledby="hero-title">
        <div className="container wide hero-grid">
          <div data-reveal>
            <div className="label">{s("heroLabel")}</div>
            <h1 className="display" id="hero-title">
              {s("heroTitlePre")} <em>{s("heroTitleEm")}</em>
              <span className="dot">.</span>
            </h1>
            <p className="lead">{s("heroLead")}</p>
            <div className="hero-note" aria-label={s("statsAria")}>
              {stats.map((n) => (
                <div className="note" key={n.label}>
                  <b>{n.value}</b>
                  <span>{n.label}</span>
                </div>
              ))}
            </div>
          </div>
          <figure className="plate" data-reveal>
            <img src="/editorial/capabilities.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
        </div>
      </header>

      <main>
        <section className="timeline-section" aria-labelledby="timeline-title">
          <div className="container">
            <div className="timeline-head" data-reveal>
              <div>
                <div className="label">{s("timelineLabel")}</div>
                <h2 className="display" id="timeline-title">
                  {s("timelineTitle")}
                  <span className="dot">.</span>
                </h2>
              </div>
              <p className="lead">{s("timelineLead")}</p>
            </div>

            <div className="filters" aria-label={s("filterAria")}>
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
            <p className="filter-feedback" aria-live="polite">
              {empty ? s("emptyState") : feedbackMap[feedbackKey] ?? feedbackMap.all}
            </p>

            <div className="timeline">
              {items.map((it, i) => {
                const hidden = filter !== "all" && it.status !== filter;
                const isFirstVisible = !hidden && visible.indexOf(it) === 0;
                return (
                  <article
                    key={it.title}
                    className={`timeline-item${hidden ? " is-hidden" : ""}${
                      isFirstVisible ? " is-current" : ""
                    }`}
                    data-status={it.status}
                    hidden={hidden}
                    data-reveal
                    data-reveal-delay={i === 0 ? undefined : String(Math.min(i, 3))}
                  >
                    <div className="phase">{it.phase}</div>
                    <div>
                      <h3>{it.title}</h3>
                      <p>{it.desc}</p>
                    </div>
                    <span className={`status${it.status === "research" ? " research" : ""}`}>
                      {it.status === "research" ? s("statusResearch") : s("statusDone")}
                    </span>
                  </article>
                );
              })}
            </div>
          </div>
        </section>
      </main>

      <section className="closing" aria-labelledby="closing-title">
        <div className="container closing-grid">
          <div>
            <div className="label">{s("closingLabel")}</div>
            <h2 className="display" id="closing-title">
              {s("closingTitle")}
              <span className="dot">.</span>
            </h2>
            <p>{s("closingLead")}</p>
          </div>
          <Link className="btn" href="/download">
            {s("closingBtn")}
          </Link>
        </div>
      </section>
    </div>
  );
}

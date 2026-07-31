"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

type Stat = { value: string; label: string };
type Filter = { key: string; label: string };
type ServiceItem = {
  id: number;
  status: "research" | "done";
  phase: string;
  title: string;
  desc: string;
  repo_url?: string;
};

function isGithubUrl(url: string): boolean {
  return /github\.com/i.test(url);
}

function GithubMarkIcon() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="currentColor" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
      <polyline points="15 3 21 3 21 9" />
      <line x1="10" y1="14" x2="21" y2="3" />
    </svg>
  );
}

export default function ServicesPage() {
  const s = useTranslations("editorial.services");
  const locale = useLocale();
  const lang = locale === "en" ? "en" : "zh";

  const rawStats = s.raw("stats") as Stat[];
  const filters = s.raw("filters") as Filter[];

  const [items, setItems] = useState<ServiceItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetch(`/api/public/services?lang=${encodeURIComponent(lang)}`, { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (cancelled) return;
        setItems(Array.isArray(data?.services) ? (data.services as ServiceItem[]) : []);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [lang]);

  const [filter, setFilter] = useState("all");
  const visible = filter === "all" ? items : items.filter((it) => it.status === filter);
  const feedbackKey = filter as "all" | "done" | "research";
  const feedbackMap: Record<string, string> = {
    all: s("filterAll", { count: visible.length }),
    done: s("filterDone", { count: visible.length }),
    research: s("filterResearch", { count: visible.length }),
  };
  const empty = visible.length === 0;

  // stats: derive counts from live items so they always match the DB.
  const doneCount = items.filter((it) => it.status === "done").length;
  const researchCount = items.filter((it) => it.status === "research").length;
  const stats = [
    { value: String(doneCount), label: rawStats[0]?.label ?? "" },
    { value: String(researchCount), label: rawStats[1]?.label ?? "" },
    { value: rawStats[2]?.value ?? "1", label: rawStats[2]?.label ?? "" },
  ];

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
            <Link href="/price" className="price-stamp" aria-label={s("stampAria")}>
              <span className="stamp-ring" aria-hidden="true">
                <svg viewBox="0 0 120 120">
                  <defs>
                    <path id="stamp-circle-path" d="M60,60 m-46,0 a46,46 0 1,1 92,0 a46,46 0 1,1 -92,0" fill="none" />
                  </defs>
                  <text>
                    <textPath href="#stamp-circle-path" startOffset="0">
                      {s("stampText")}
                    </textPath>
                  </text>
                </svg>
              </span>
              <span className="stamp-center" aria-hidden="true">¥</span>
              <span className="stamp-sub">{s("stampCenter")}</span>
            </Link>
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
              {loading ? s("loading") : empty ? s("emptyState") : feedbackMap[feedbackKey] ?? feedbackMap.all}
            </p>

            <div className="timeline">
              {visible.map((it, i) => (
                <article
                  key={it.id}
                  className={`timeline-item${i === 0 ? " is-current" : ""}`}
                  data-status={it.status}
                >
                  <div className="phase">{it.phase}</div>
                  <div>
                    <h3>{it.title}</h3>
                    <p>{it.desc}</p>
                    {it.repo_url && (
                      <a
                        className="service-repo-link"
                        href={it.repo_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        aria-label={s("viewProject")}
                      >
                        {isGithubUrl(it.repo_url) ? <GithubMarkIcon /> : <ExternalLinkIcon />}
                        <span>{s("viewProject")}</span>
                      </a>
                    )}
                  </div>
                  <span className={`status${it.status === "research" ? " research" : ""}`}>
                    {it.status === "research" ? s("statusResearch") : s("statusDone")}
                  </span>
                </article>
              ))}
            </div>
          </div>
        </section>
      </main>

      <section className="container" style={{ padding: "48px 0" }} aria-labelledby="tool-card-title">
        <div className="tool-card">
          <div className="tool-card-body">
            <div className="label">{s("toolCardLabel")}</div>
            <h3 id="tool-card-title">{s("toolCardTitle")}</h3>
            <p>{s("toolCardDesc")}</p>
          </div>
          <Link className="btn primary" href="/tools/sse-parser">{s("toolCardBtn")}</Link>
        </div>
      </section>

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

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
};

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

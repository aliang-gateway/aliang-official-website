"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import TechRadar3D from "./TechRadar3D";

type Article = {
  slug: string;
  title: string;
  tag: string;
  publishedAt: string;
  excerpt: string;
  readTime: string;
  image: string;
  author: {
    name: string;
    avatar?: string;
    icon?: string;
  };
};

type PublicArticle = {
  slug: string;
  title: string;
  excerpt: string;
  cover_image_url: string;
  tag: string;
  read_time: string;
  author_name: string;
  author_avatar_url?: string;
  published_at: string;
};

type PublicArticlesResponse = {
  articles?: PublicArticle[];
  error?: string;
};

type TechBlip = {
  id: string;
  name: string;
  ring: "adopt" | "trial" | "assess" | "hold";
  x: number;
  y: number;
  relatedTags: string[];
  relatedKeywords: string[];
};

const techBlips: TechBlip[] = [
  { id: "gpt", name: "GPT-4o", ring: "adopt", x: 34, y: 27, relatedTags: ["AI评测"], relatedKeywords: ["GPT", "Claude"] },
  { id: "claude", name: "Claude", ring: "trial", x: 46, y: 19, relatedTags: ["AI评测"], relatedKeywords: ["Claude", "GPT"] },
  { id: "cursor", name: "Cursor", ring: "trial", x: 67, y: 33, relatedTags: ["工具技巧"], relatedKeywords: ["Cursor", "写作", "效率"] },
  { id: "deepseek", name: "DeepSeek", ring: "assess", x: 60, y: 69, relatedTags: ["AI评测"], relatedKeywords: ["DeepSeek", "Qwen"] },
  { id: "qwen", name: "Qwen", ring: "hold", x: 73, y: 74, relatedTags: ["AI评测"], relatedKeywords: ["Qwen", "DeepSeek"] },
  { id: "api-gateway", name: "API网关", ring: "adopt", x: 30, y: 60, relatedTags: ["教程"], relatedKeywords: ["API", "网关"] },
  { id: "api-key", name: "API Key", ring: "assess", x: 22, y: 73, relatedTags: ["安全"], relatedKeywords: ["API Key", "安全"] },
  { id: "prompt", name: "Prompt工程", ring: "trial", x: 42, y: 58, relatedTags: ["工具技巧"], relatedKeywords: ["写作", "效率", "AI"] },
  { id: "model-eval", name: "模型评测", ring: "adopt", x: 56, y: 43, relatedTags: ["AI评测"], relatedKeywords: ["评测", "模型", "对比"] },
];

function formatPublishedDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  }).format(date);
}

function normalizeArticle(article: PublicArticle): Article {
  return {
    slug: article.slug,
    title: article.title,
    tag: article.tag,
    publishedAt: formatPublishedDate(article.published_at),
    excerpt: article.excerpt,
    readTime: article.read_time,
    image: article.cover_image_url,
    author: {
      name: article.author_name,
      avatar: article.author_avatar_url,
      icon: "person",
    },
  };
}

export default function BlogPage() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [isLoadingArticles, setIsLoadingArticles] = useState(true);
  const [articlesError, setArticlesError] = useState<string | null>(null);
  const [selectedBlip, setSelectedBlip] = useState<TechBlip | null>(null);
  const modalRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const lastTriggerRef = useRef<HTMLButtonElement | null>(null);
  const hadModalOpenRef = useRef(false);

  const closeModal = useCallback(() => setSelectedBlip(null), []);

  const relatedArticles = useMemo(() => {
    if (!selectedBlip) return [];

    return articles.filter((article) => {
      const tagMatched = selectedBlip.relatedTags.includes(article.tag);
      const keywordMatched = selectedBlip.relatedKeywords.some((keyword) =>
        `${article.title} ${article.excerpt}`.toLowerCase().includes(keyword.toLowerCase())
      );
      return tagMatched || keywordMatched;
    });
  }, [articles, selectedBlip]);

  useEffect(() => {
    let isMounted = true;

    const loadArticles = async () => {
      setIsLoadingArticles(true);
      setArticlesError(null);

      try {
        const response = await fetch("/api/public/articles", {
          method: "GET",
          headers: { "content-type": "application/json", accept: "application/json" },
          cache: "no-store",
        });

        const payload = (await response.json()) as PublicArticlesResponse;
        if (!response.ok) {
          throw new Error(payload.error ?? "Failed to load articles");
        }

        if (!isMounted) {
          return;
        }

        setArticles((payload.articles ?? []).map(normalizeArticle));
      } catch (error) {
        if (!isMounted) {
          return;
        }
        setArticles([]);
        setArticlesError(error instanceof Error ? error.message : "Failed to load articles");
      } finally {
        if (isMounted) {
          setIsLoadingArticles(false);
        }
      }
    };

    void loadArticles();

    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    if (!selectedBlip) {
      if (hadModalOpenRef.current) {
        lastTriggerRef.current?.focus();
        hadModalOpenRef.current = false;
      }
      return;
    }

    hadModalOpenRef.current = true;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeModal();
        return;
      }

      if (event.key !== "Tab") return;

      const modal = modalRef.current;
      if (!modal) return;
      const focusable = modal.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey && activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [closeModal, selectedBlip]);

  return (
    <>
      <section className="relative overflow-hidden bg-slate-900 py-16 px-6 text-slate-100 md:px-20">
        <div
          className="pointer-events-none absolute inset-0 opacity-20"
          style={{ backgroundImage: "radial-gradient(circle at 2px 2px, #21c45d 1px, transparent 0)", backgroundSize: "40px 40px" }}
        />
        <div className="max-w-7xl mx-auto flex flex-col lg:flex-row items-center gap-12 relative z-10">
          <div className="flex flex-col gap-6 lg:w-1/2 text-left">
            <div
              className="inline-flex w-max items-center rounded-full px-3 py-1 text-xs font-bold uppercase tracking-widest"
              style={{ backgroundColor: "rgba(33, 196, 93, 0.2)", color: "#21c45d" }}
            >
              Edition 2024.Q3
            </div>
            <h1 className="text-4xl font-black leading-tight tracking-tighter text-white md:text-6xl">
              The ALiang <br /><span className="text-[var(--stitch-primary)]">Tech Radar</span>
            </h1>
            <p className="max-w-xl text-lg font-normal leading-relaxed text-slate-400 md:text-xl">
              A data-driven visualization of our strategic technology landscape: from high-performance TUN/HTTP proxies to advanced LLM Routing strategies.
            </p>
            <div className="flex flex-wrap gap-4 pt-4">
              <button
                type="button"
                className="flex h-12 items-center gap-2 rounded-lg bg-[var(--stitch-primary)] px-8 font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:-translate-y-0.5"
              >
                <span>Explore the Radar</span>
                <MaterialIcon name="explore" size={20} />
              </button>
              <button
                type="button"
                className="flex h-12 items-center gap-2 rounded-lg border border-white/10 bg-white/10 px-8 font-bold text-white backdrop-blur-sm transition-all hover:bg-white/20"
              >
                <span>Methodology</span>
              </button>
            </div>
          </div>
          
          <div className="lg:w-1/2 relative flex justify-center items-center">
            <div className="relative size-[320px] md:size-[500px] border rounded-full flex items-center justify-center" style={{ borderColor: "rgba(33, 196, 93, 0.3)" }}>
              <div className="absolute inset-0 border rounded-full scale-75" style={{ borderColor: "rgba(33, 196, 93, 0.2)" }} />
              <div className="absolute inset-0 border rounded-full scale-50" style={{ borderColor: "rgba(33, 196, 93, 0.1)" }} />
              <div className="absolute inset-0 border rounded-full scale-[0.25]" style={{ borderColor: "rgba(33, 196, 93, 0.05)" }} />
              
              <div className="absolute inset-0 w-full h-full flex items-center justify-center pointer-events-auto z-10">
                <TechRadar3D 
                  blips={techBlips} 
                  onBlipClick={(blip, buttonEl) => {
                    lastTriggerRef.current = buttonEl;
                    setSelectedBlip(blip);
                  }} 
                />
              </div>

              <div className="absolute h-px w-full top-1/2" style={{ backgroundColor: "rgba(33, 196, 93, 0.1)" }} />
              <div className="absolute w-px h-full left-1/2" style={{ backgroundColor: "rgba(33, 196, 93, 0.1)" }} />
              
              <div className="absolute bottom-0 right-0 z-10 rounded-lg border border-[var(--stitch-primary)]/50 bg-slate-900 p-4 font-mono text-sm uppercase tracking-widest text-[var(--stitch-primary)]">
                Active Matrix
              </div>
            </div>
          </div>
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-6 md:px-20 py-12">
        <div className="flex items-center justify-between border-b mb-10" style={{ borderColor: 'var(--stitch-border)' }}>
          <div className="flex gap-8 overflow-x-auto no-scrollbar">
            <button type="button" className="flex flex-col items-center py-4 border-b-2 font-bold text-sm tracking-wide" style={{ borderColor: 'var(--stitch-primary)', color: 'var(--stitch-text)' }}>
              All Publications
            </button>
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent dark:border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              AI Gateways
            </button>
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent dark:border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              Networking
            </button>
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent dark:border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              Security
            </button>
          </div>
          <div className="hidden md:flex items-center gap-2 text-sm" style={{ color: 'var(--stitch-text-muted)' }}>
            <MaterialIcon name="sort" size={16} />
            <span>Latest First</span>
          </div>
        </div>

        {isLoadingArticles ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8" aria-live="polite" aria-busy="true">
            {["article-skeleton-1", "article-skeleton-2", "article-skeleton-3"].map((skeletonKey) => (
              <div
                key={skeletonKey}
                className="flex flex-col rounded-xl overflow-hidden border animate-pulse"
                style={{ backgroundColor: 'var(--stitch-bg)', borderColor: 'var(--stitch-border)' }}
              >
                <div className="aspect-video" style={{ backgroundColor: 'var(--stitch-bg-elevated)' }} />
                <div className="p-6 flex flex-col gap-3">
                  <div className="h-3 w-2/3 rounded" style={{ backgroundColor: 'var(--stitch-bg-elevated)' }} />
                  <div className="h-6 w-full rounded" style={{ backgroundColor: 'var(--stitch-bg-elevated)' }} />
                  <div className="h-4 w-full rounded" style={{ backgroundColor: 'var(--stitch-bg-elevated)' }} />
                  <div className="h-4 w-4/5 rounded" style={{ backgroundColor: 'var(--stitch-bg-elevated)' }} />
                </div>
              </div>
            ))}
          </div>
        ) : articlesError ? (
          <div className="py-16 text-center border rounded-xl" style={{ borderColor: 'var(--stitch-border)', backgroundColor: 'var(--stitch-bg)' }}>
            <p className="text-lg font-semibold mb-2" style={{ color: 'var(--stitch-text)' }}>Unable to load publications.</p>
            <p className="text-sm" style={{ color: 'var(--stitch-text-muted)' }}>{articlesError}</p>
          </div>
        ) : articles.length === 0 ? (
          <div className="py-16 text-center border rounded-xl" style={{ borderColor: 'var(--stitch-border)', backgroundColor: 'var(--stitch-bg)' }}>
            <p className="text-lg font-semibold mb-2" style={{ color: 'var(--stitch-text)' }}>No publications yet.</p>
            <p className="text-sm" style={{ color: 'var(--stitch-text-muted)' }}>Please check back soon for new articles.</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
            {articles.map((article) => (
            <Link
              key={article.slug}
              href={`/blog/${article.slug}`}
              className="group flex flex-col rounded-xl overflow-hidden border hover:shadow-xl transition-all duration-300"
              style={{ backgroundColor: 'var(--stitch-bg)', borderColor: 'var(--stitch-border)' }}
            >
              <div className="aspect-video relative overflow-hidden">
                <Image 
                  src={article.image} 
                  alt={article.title} 
                  width={400}
                  height={300}
                  className="object-cover w-full h-full group-hover:scale-105 transition-transform duration-500"
                  unoptimized
                />
                <div className="absolute top-4 left-4">
                  <span className="text-white px-3 py-1 text-[10px] font-bold uppercase tracking-wider rounded" style={{ backgroundColor: 'color-mix(in srgb, var(--stitch-primary) 90%, transparent)' }}>
                    {article.tag}
                  </span>
                </div>
              </div>
              <div className="p-6 flex flex-col grow">
                <div className="flex items-center gap-2 text-xs mb-3 font-medium" style={{ color: 'var(--stitch-text-muted)' }}>
                  <MaterialIcon name="calendar_today" size={14} />
                  <span>{article.publishedAt}</span>
                  <span className="mx-1">•</span>
                  <MaterialIcon name="schedule" size={14} />
                  <span>{article.readTime}</span>
                </div>
                <h3 className="text-xl font-bold mb-3 transition-colors leading-snug group-hover:text-primary" style={{ color: 'var(--stitch-text)' }}>
                  {article.title}
                </h3>
                <p className="text-sm leading-relaxed mb-6 line-clamp-3" style={{ color: 'var(--stitch-text-muted)' }}>
                  {article.excerpt}
                </p>
                <div className="mt-auto pt-4 border-t flex items-center justify-between" style={{ borderColor: 'var(--stitch-border)' }}>
                  <div className="flex items-center gap-2">
                    <div className="size-6 rounded-full flex items-center justify-center overflow-hidden" style={{ backgroundColor: 'var(--stitch-border)' }}>
                      {article.author.avatar ? (
                        <Image 
                          src={article.author.avatar} 
                          alt={article.author.name}
                          width={24}
                          height={24}
                          className="object-cover size-full"
                          unoptimized
                        />
                      ) : (
                        <MaterialIcon name={article.author.icon || "person"} size={14} />
                      )}
                    </div>
                    <span className="text-xs font-bold" style={{ color: 'var(--stitch-text)' }}>{article.author.name}</span>
                  </div>
                  <div className="transition-all group-hover:translate-x-1" style={{ color: 'var(--stitch-text-muted)' }}>
                    <MaterialIcon name="arrow_forward" size={20} />
                  </div>
                </div>
              </div>
            </Link>
            ))}
          </div>
        )}

        <div className="flex items-center justify-center mt-16 gap-2">
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border transition-colors hover:bg-[var(--stitch-bg-elevated)]" style={{ borderColor: 'var(--stitch-border)', color: 'var(--stitch-text-muted)' }}>
            <span aria-hidden="true" className="text-lg leading-none">‹</span>
          </button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg text-white font-bold text-sm" style={{ backgroundColor: 'var(--stitch-primary)' }}>1</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent dark:border-transparent transition-colors font-semibold text-sm hover:bg-[var(--stitch-bg-elevated)]" style={{ color: 'var(--stitch-text-muted)' }}>2</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent dark:border-transparent transition-colors font-semibold text-sm hover:bg-[var(--stitch-bg-elevated)]" style={{ color: 'var(--stitch-text-muted)' }}>3</button>
          <span className="px-2" style={{ color: 'var(--stitch-text-muted)' }}>...</span>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent dark:border-transparent transition-colors font-semibold text-sm hover:bg-[var(--stitch-bg-elevated)]" style={{ color: 'var(--stitch-text-muted)' }}>12</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border transition-colors hover:bg-[var(--stitch-bg-elevated)]" style={{ borderColor: 'var(--stitch-border)', color: 'var(--stitch-text-muted)' }}>
            <span aria-hidden="true" className="text-lg leading-none">›</span>
          </button>
        </div>
      </div>

      <section className="py-16 px-6 border-y" style={{ backgroundColor: 'color-mix(in srgb, var(--stitch-primary) 5%, transparent)', borderColor: 'color-mix(in srgb, var(--stitch-primary) 10%, transparent)' }}>
        <div className="max-w-4xl mx-auto text-center flex flex-col items-center gap-6">
          <div style={{ color: 'var(--stitch-primary)' }}>
            <MaterialIcon name="mail" size={48} />
          </div>
          <h2 className="text-3xl font-black tracking-tight" style={{ color: 'var(--stitch-text)' }}>Stay ahead of the network.</h2>
          <p className="text-lg" style={{ color: 'var(--stitch-text-muted)' }}>Join 5,000+ engineers receiving our monthly breakdown of gateway architecture and AI networking trends.</p>
          <form className="w-full max-w-md flex flex-col sm:flex-row gap-3 mt-4">
            <input className="flex-1 rounded-lg border px-4 py-3 focus:ring-primary focus:border-primary outline-none" style={{ backgroundColor: 'var(--stitch-bg)', borderColor: 'var(--stitch-border)', color: 'var(--stitch-text)' }} placeholder="engineer@enterprise.com" type="email" />
            <button type="submit" className="text-white font-bold px-8 py-3 rounded-lg hover:shadow-lg transition-all" style={{ backgroundColor: 'var(--stitch-primary)', boxShadow: '0 10px 15px -3px color-mix(in srgb, var(--stitch-primary) 20%, transparent)' }}>Subscribe</button>
          </form>
          <p className="text-[10px] uppercase tracking-widest font-bold" style={{ color: 'var(--stitch-text-muted)' }}>Academic release • No spam • Opt-out anytime</p>
        </div>
      </section>

      {selectedBlip && (
        <section
          className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-label={`${selectedBlip.name} 相关博客`}
        >
          <button
            type="button"
            className="absolute inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
            aria-label="关闭弹窗"
            onClick={closeModal}
          />

          <div className="relative w-full max-w-2xl rounded-2xl shadow-2xl flex flex-col max-h-[90vh] overflow-hidden" style={{ backgroundColor: 'var(--stitch-bg)', border: '1px solid var(--stitch-border)' }} ref={modalRef}>
            <div className="flex items-center justify-between p-6 border-b" style={{ borderColor: 'var(--stitch-border)' }}>
              <h3 className="text-xl font-bold" style={{ color: 'var(--stitch-text)' }}>{selectedBlip.name} 相关博客</h3>
              <button
                type="button"
                className="rounded-full p-2 transition-colors hover:bg-[var(--stitch-bg-elevated)]"
                style={{ color: 'var(--stitch-text-muted)' }}
                aria-label="关闭弹窗"
                ref={closeButtonRef}
                onClick={closeModal}
              >
                <MaterialIcon name="close" size={24} />
              </button>
            </div>

            <div className="overflow-y-auto p-6 flex flex-col gap-4">
              {relatedArticles.length > 0 ? (
                relatedArticles.map((article) => (
                  <Link key={article.slug} href={`/blog/${article.slug}`} className="block p-4 rounded-xl border transition-all hover:-translate-y-0.5 hover:shadow-md" style={{ borderColor: 'var(--stitch-border)', backgroundColor: 'var(--stitch-bg-elevated)' }}>
                    <p className="font-bold mb-2 transition-colors hover:text-primary" style={{ color: 'var(--stitch-text)' }}>{article.title}</p>
                    <p className="text-sm" style={{ color: 'var(--stitch-text-muted)' }}>{article.tag} · {article.publishedAt} · {article.readTime}</p>
                  </Link>
                ))
              ) : (
                <div className="py-12 text-center flex flex-col items-center gap-4">
                  <div style={{ color: 'var(--stitch-text-muted)' }}>
                    <MaterialIcon name="article" size={48} />
                  </div>
                  <p style={{ color: 'var(--stitch-text-muted)' }}>暂无相关博客，后续会持续补充。</p>
                </div>
              )}
            </div>
          </div>
        </section>
      )}
    </>
  );
}

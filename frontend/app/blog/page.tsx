"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import TechRadar3D from "./TechRadar3D";

type Article = {
  slug: string;
  title: string;
  tag: string;
  publishedAt: string;
  excerpt: string;
  readTime: string;
  image?: string;
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

// 分类配置：id 对应 i18n key，icon 为 Material Symbol 名称
type CategoryConfig = {
  id: string;
  icon: string;
  tagMatch: string[];
};

const categories: CategoryConfig[] = [
  { id: "allPublications", icon: "layers", tagMatch: [] },
  { id: "aiGateways", icon: "neurology", tagMatch: ["AI网关", "AI评测", "AI Gateway"] },
  { id: "networking", icon: "lan", tagMatch: ["网络", "教程", "Networking"] },
  { id: "security", icon: "shield", tagMatch: ["安全", "Security"] },
];

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

function normalizeOptionalAsset(value?: string | null) {
  const normalized = String(value ?? "").trim();
  return normalized || undefined;
}

function normalizeArticle(article: PublicArticle): Article {
  return {
    slug: article.slug,
    title: article.title,
    tag: article.tag,
    publishedAt: formatPublishedDate(article.published_at),
    excerpt: article.excerpt,
    readTime: article.read_time,
    image: normalizeOptionalAsset(article.cover_image_url),
    author: {
      name: article.author_name,
      avatar: normalizeOptionalAsset(article.author_avatar_url),
      icon: "person",
    },
  };
}

export default function BlogPage() {
  const t = useTranslations("blog");
  const [articles, setArticles] = useState<Article[]>([]);
  const [isLoadingArticles, setIsLoadingArticles] = useState(true);
  const [articlesError, setArticlesError] = useState<string | null>(null);
  const [selectedBlip, setSelectedBlip] = useState<TechBlip | null>(null);
  const [activeCategory, setActiveCategory] = useState(0);
  const modalRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const lastTriggerRef = useRef<HTMLButtonElement | null>(null);
  const hadModalOpenRef = useRef(false);

  const closeModal = useCallback(() => setSelectedBlip(null), []);

  const filteredArticles = useMemo(() => {
    if (activeCategory === 0) return articles;
    const cat = categories[activeCategory];
    if (!cat || cat.tagMatch.length === 0) return articles;
    return articles.filter((article) =>
      cat.tagMatch.some(
        (tag) =>
          article.tag === tag ||
          article.tag.includes(tag) ||
          tag.includes(article.tag)
      )
    );
  }, [articles, activeCategory]);

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

  // 每个分类的文章计数
  const categoryCounts = useMemo(() => {
    return categories.map((cat) => {
      if (cat.tagMatch.length === 0) return articles.length;
      return articles.filter((article) =>
        cat.tagMatch.some(
          (tag) =>
            article.tag === tag ||
            article.tag.includes(tag) ||
            tag.includes(article.tag)
        )
      ).length;
    });
  }, [articles]);

  return (
    <>
      {/* ===== Hero 区域：编辑式排版 + 雷达 + 分类卡片 ===== */}
      <section
        data-od-id="blog-hero"
        className="relative overflow-hidden bg-[var(--stitch-bg-elevated)] px-6 pt-16 pb-0 md:px-20 md:pt-24"
      >
        {/* 背景纹理：克制的圆点网格 */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage: "radial-gradient(circle at 2px 2px, var(--stitch-text) 1px, transparent 0)",
            backgroundSize: "32px 32px",
          }}
        />
        {/* 背景辉光 */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-40 top-0 hidden lg:block"
          style={{
            width: 600,
            height: 600,
            borderRadius: "50%",
            background: "radial-gradient(circle, rgba(33,196,93,0.06) 0%, transparent 70%)",
          }}
        />

        <div className="relative z-10 mx-auto max-w-7xl">
          {/* 顶部：badge + 标题 + 雷达 并列 */}
          <div className="grid grid-cols-1 items-center gap-10 lg:grid-cols-[1fr_auto] lg:gap-16">
            {/* 左侧文案 */}
            <div className="flex flex-col gap-5">
              <span className="inline-flex w-fit items-center gap-1.5 rounded-full bg-[var(--stitch-primary)]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--stitch-primary)]">
                <MaterialIcon name="radar" size={11} />
                {t("edition")}
              </span>

              <h1
                className="text-4xl font-black leading-[1.05] tracking-[-0.03em] text-[var(--stitch-text)] md:text-5xl lg:text-6xl"
                dangerouslySetInnerHTML={{ __html: t("title") }}
              />

              <p className="max-w-xl text-base leading-relaxed text-[var(--stitch-text-muted)] md:text-lg">
                {t("subtitle")}
              </p>

              <div className="flex flex-wrap gap-3 pt-2">
                <button
                  type="button"
                  className="group flex h-11 items-center gap-2 rounded-lg bg-[var(--stitch-primary)] px-6 text-sm font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:shadow-xl hover:shadow-[var(--stitch-primary)]/30 active:scale-[0.98]"
                >
                  {t("exploreRadar")}
                  <MaterialIcon
                    name="explore"
                    size={16}
                    className="transition-transform group-hover:rotate-45"
                  />
                </button>
                <button
                  type="button"
                  className="flex h-11 items-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-6 text-sm font-bold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg-elevated)]"
                >
                  <MaterialIcon name="science" size={16} />
                  {t("methodology")}
                </button>
              </div>
            </div>

            {/* 右侧雷达 — 桌面端可见，移动端隐藏 */}
            <div className="relative hidden justify-center lg:flex">
              <div
                className="relative flex items-center justify-center rounded-full border"
                style={{
                  width: 360,
                  height: 360,
                  borderColor: "rgba(33, 196, 93, 0.2)",
                }}
              >
                {/* 同心圆环 */}
                <div className="absolute inset-0 rounded-full border" style={{ borderColor: "rgba(33, 196, 93, 0.15)" }} />
                <div className="absolute inset-[12%] rounded-full border" style={{ borderColor: "rgba(33, 196, 93, 0.12)" }} />
                <div className="absolute inset-[28%] rounded-full border" style={{ borderColor: "rgba(33, 196, 93, 0.08)" }} />
                <div className="absolute inset-[44%] rounded-full border" style={{ borderColor: "rgba(33, 196, 93, 0.05)" }} />

                {/* 十字线 */}
                <div className="absolute top-1/2 h-px w-full" style={{ background: "rgba(33, 196, 93, 0.08)" }} />
                <div className="absolute left-1/2 h-full w-px" style={{ background: "rgba(33, 196, 93, 0.08)" }} />

                {/* 3D 雷达 */}
                <div className="absolute inset-0 z-10 flex items-center justify-center">
                  <TechRadar3D
                    blips={techBlips}
                    onBlipClick={(blip, buttonEl) => {
                      lastTriggerRef.current = buttonEl;
                      setSelectedBlip(blip);
                    }}
                  />
                </div>

                {/* Active Matrix 标签 */}
                <div className="absolute -bottom-3 right-4 z-20 rounded-md border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-3 py-1.5 font-mono text-[9px] font-bold uppercase tracking-[0.15em] text-[var(--stitch-primary)]">
                  {t("activeMatrix")}
                </div>
              </div>
            </div>
          </div>

          {/* 底部：4 分类卡片 — 取代旧的 tab bar */}
          <div className="mt-14 grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
            {categories.map((cat, idx) => {
              const isActive = activeCategory === idx;
              const count = categoryCounts[idx];
              return (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => setActiveCategory(idx)}
                  className={[
                    "group relative flex flex-col gap-3 overflow-hidden rounded-xl border p-4 text-left transition-all duration-300 md:p-5",
                    isActive
                      ? "border-[var(--stitch-primary)] bg-[var(--stitch-bg)] shadow-md"
                      : "border-[var(--stitch-border)] bg-[var(--stitch-bg)] hover:-translate-y-0.5 hover:border-[var(--stitch-primary)]/30",
                  ].join(" ")}
                >
                  {/* 选中态背景辉光 */}
                  {isActive && (
                    <div
                      aria-hidden="true"
                      className="pointer-events-none absolute inset-0"
                      style={{
                        background: "radial-gradient(circle at 0% 0%, rgba(33,196,93,0.06) 0%, transparent 60%)",
                      }}
                    />
                  )}

                  <div className="relative flex items-center justify-between">
                    <div
                      className={[
                        "flex items-center justify-center rounded-lg transition-colors",
                        "size-9 md:size-10",
                      ].join(" ")}
                      style={{
                        background: isActive
                          ? "var(--stitch-primary)"
                          : "rgba(33,196,93,0.1)",
                      }}
                    >
                      <MaterialIcon
                        name={cat.icon}
                        size={18}
                        className={isActive ? "text-white" : "text-[var(--stitch-primary)]"}
                      />
                    </div>
                    {/* 文章计数 */}
                    <span
                      className={[
                        "rounded-full px-2 py-0.5 font-mono text-[10px] font-bold tabular-nums",
                        isActive
                          ? "bg-[var(--stitch-primary)]/15 text-[var(--stitch-primary)]"
                          : "bg-[var(--stitch-border)] text-[var(--stitch-text-muted)]",
                      ].join(" ")}
                    >
                      {count}
                    </span>
                  </div>

                  <div className="relative">
                    <h3
                      className={[
                        "text-sm font-bold tracking-tight md:text-base",
                        isActive ? "text-[var(--stitch-text)]" : "text-[var(--stitch-text)]",
                      ].join(" ")}
                    >
                      {t(cat.id)}
                    </h3>
                  </div>

                  {/* 选中指示条 */}
                  {isActive && (
                    <div
                      aria-hidden="true"
                      className="absolute bottom-0 left-0 h-0.5 w-full"
                      style={{ background: "var(--stitch-primary)" }}
                    />
                  )}
                </button>
              );
            })}
          </div>
        </div>
      </section>

      {/* ===== 文章列表 ===== */}
      <div className="mx-auto max-w-7xl px-6 py-12 md:px-20 md:py-16">
        {/* 排序指示器 */}
        <div className="mb-8 flex items-center justify-between">
          <div className="flex items-baseline gap-2">
            <span className="text-sm font-bold text-[var(--stitch-text)]">
              {t(categories[activeCategory].id)}
            </span>
            <span className="font-mono text-xs text-[var(--stitch-text-muted)]">
              ({filteredArticles.length})
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-xs text-[var(--stitch-text-muted)]">
            <MaterialIcon name="sort" size={14} />
            {t("latestFirst")}
          </div>
        </div>

        {isLoadingArticles ? (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3" aria-live="polite" aria-busy="true">
            {["article-skeleton-1", "article-skeleton-2", "article-skeleton-3"].map((skeletonKey) => (
              <div
                key={skeletonKey}
                className="flex animate-pulse flex-col overflow-hidden rounded-xl border"
                style={{ backgroundColor: "var(--stitch-bg)", borderColor: "var(--stitch-border)" }}
              >
                <div className="aspect-video" style={{ backgroundColor: "var(--stitch-bg-elevated)" }} />
                <div className="flex flex-col gap-3 p-6">
                  <div className="h-3 w-2/3 rounded" style={{ backgroundColor: "var(--stitch-bg-elevated)" }} />
                  <div className="h-6 w-full rounded" style={{ backgroundColor: "var(--stitch-bg-elevated)" }} />
                  <div className="h-4 w-full rounded" style={{ backgroundColor: "var(--stitch-bg-elevated)" }} />
                  <div className="h-4 w-4/5 rounded" style={{ backgroundColor: "var(--stitch-bg-elevated)" }} />
                </div>
              </div>
            ))}
          </div>
        ) : articlesError ? (
          <div className="rounded-xl border py-16 text-center" style={{ borderColor: "var(--stitch-border)", backgroundColor: "var(--stitch-bg)" }}>
            <p className="mb-2 text-lg font-semibold" style={{ color: "var(--stitch-text)" }}>{t("unableToLoad")}</p>
            <p className="text-sm" style={{ color: "var(--stitch-text-muted)" }}>{articlesError}</p>
          </div>
        ) : filteredArticles.length === 0 ? (
          <div className="rounded-xl border py-16 text-center" style={{ borderColor: "var(--stitch-border)", backgroundColor: "var(--stitch-bg)" }}>
            <div className="mb-3 flex justify-center" style={{ color: "var(--stitch-text-muted)" }}>
              <MaterialIcon name="article" size={40} />
            </div>
            <p className="mb-2 text-lg font-semibold" style={{ color: "var(--stitch-text)" }}>{t("noPublications")}</p>
            <p className="text-sm" style={{ color: "var(--stitch-text-muted)" }}>{t("checkBackSoon")}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {filteredArticles.map((article) => (
              <Link
                key={article.slug}
                href={`/blog/${article.slug}`}
                className="group flex flex-col overflow-hidden rounded-xl border transition-all duration-300 hover:-translate-y-1 hover:shadow-xl"
                style={{ backgroundColor: "var(--stitch-bg)", borderColor: "var(--stitch-border)" }}
              >
                <div className="relative aspect-video overflow-hidden">
                  {article.image ? (
                    <Image
                      src={article.image}
                      alt={article.title}
                      width={400}
                      height={300}
                      className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
                      unoptimized
                    />
                  ) : (
                    <div
                      className="flex h-full w-full items-center justify-center bg-[var(--stitch-bg-elevated)] text-[var(--stitch-text-muted)]"
                      aria-hidden="true"
                    >
                      <MaterialIcon name="article" size={40} />
                    </div>
                  )}
                  <div className="absolute left-4 top-4">
                    <span
                      className="rounded px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-white"
                      style={{ backgroundColor: "color-mix(in srgb, var(--stitch-primary) 90%, transparent)" }}
                    >
                      {article.tag}
                    </span>
                  </div>
                </div>
                <div className="flex grow flex-col p-6">
                  <div className="mb-3 flex items-center gap-2 text-xs font-medium" style={{ color: "var(--stitch-text-muted)" }}>
                    <MaterialIcon name="calendar_today" size={14} />
                    <span>{article.publishedAt}</span>
                    <span className="mx-1">·</span>
                    <MaterialIcon name="schedule" size={14} />
                    <span>{article.readTime}</span>
                  </div>
                  <h3
                    className="mb-3 text-lg font-bold leading-snug transition-colors group-hover:text-[var(--stitch-primary)]"
                    style={{ color: "var(--stitch-text)" }}
                  >
                    {article.title}
                  </h3>
                  <p className="mb-6 line-clamp-3 text-sm leading-relaxed" style={{ color: "var(--stitch-text-muted)" }}>
                    {article.excerpt}
                  </p>
                  <div
                    className="mt-auto flex items-center justify-between border-t pt-4"
                    style={{ borderColor: "var(--stitch-border)" }}
                  >
                    <div className="flex items-center gap-2">
                      <div className="flex size-6 items-center justify-center overflow-hidden rounded-full" style={{ backgroundColor: "var(--stitch-border)" }}>
                        {article.author.avatar ? (
                          <Image
                            src={article.author.avatar}
                            alt={article.author.name}
                            width={24}
                            height={24}
                            className="size-full object-cover"
                            unoptimized
                          />
                        ) : (
                          <MaterialIcon name={article.author.icon || "person"} size={14} />
                        )}
                      </div>
                      <span className="text-xs font-bold" style={{ color: "var(--stitch-text)" }}>{article.author.name}</span>
                    </div>
                    <div className="transition-all group-hover:translate-x-1" style={{ color: "var(--stitch-text-muted)" }}>
                      <MaterialIcon name="arrow_forward" size={20} />
                    </div>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}

        {/* 分页 */}
        <div className="mt-14 flex items-center justify-center gap-2">
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg border transition-colors hover:bg-[var(--stitch-bg-elevated)]"
            style={{ borderColor: "var(--stitch-border)", color: "var(--stitch-text-muted)" }}
          >
            <span aria-hidden="true" className="text-lg leading-none">‹</span>
          </button>
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg text-sm font-bold text-white"
            style={{ backgroundColor: "var(--stitch-primary)" }}
          >
            1
          </button>
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg border border-transparent text-sm font-semibold transition-colors hover:bg-[var(--stitch-bg-elevated)]"
            style={{ color: "var(--stitch-text-muted)" }}
          >
            2
          </button>
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg border border-transparent text-sm font-semibold transition-colors hover:bg-[var(--stitch-bg-elevated)]"
            style={{ color: "var(--stitch-text-muted)" }}
          >
            3
          </button>
          <span className="px-2" style={{ color: "var(--stitch-text-muted)" }}>...</span>
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg border border-transparent text-sm font-semibold transition-colors hover:bg-[var(--stitch-bg-elevated)]"
            style={{ color: "var(--stitch-text-muted)" }}
          >
            12
          </button>
          <button
            type="button"
            className="flex size-10 items-center justify-center rounded-lg border transition-colors hover:bg-[var(--stitch-bg-elevated)]"
            style={{ borderColor: "var(--stitch-border)", color: "var(--stitch-text-muted)" }}
          >
            <span aria-hidden="true" className="text-lg leading-none">›</span>
          </button>
        </div>
      </div>

      {/* ===== Newsletter ===== */}
      <section
        data-od-id="blog-newsletter"
        className="border-y px-6 py-16 md:px-20"
        style={{
          backgroundColor: "color-mix(in srgb, var(--stitch-primary) 4%, transparent)",
          borderColor: "color-mix(in srgb, var(--stitch-primary) 10%, transparent)",
        }}
      >
        <div className="mx-auto flex max-w-4xl flex-col items-center gap-5 text-center">
          <div style={{ color: "var(--stitch-primary)" }}>
            <MaterialIcon name="mail" size={40} />
          </div>
          <h2 className="text-2xl font-black tracking-[-0.02em] md:text-3xl" style={{ color: "var(--stitch-text)" }}>
            {t("stayAhead")}
          </h2>
          <p className="max-w-xl text-base md:text-lg" style={{ color: "var(--stitch-text-muted)" }}>
            {t("newsletterDescription")}
          </p>
          <form className="mt-2 flex w-full max-w-md flex-col gap-3 sm:flex-row">
            <input
              className="flex-1 rounded-lg border px-4 py-3 outline-none focus:border-[var(--stitch-primary)]"
              style={{
                backgroundColor: "var(--stitch-bg)",
                borderColor: "var(--stitch-border)",
                color: "var(--stitch-text)",
              }}
              placeholder="engineer@gateway.ai"
              type="email"
            />
            <button
              type="submit"
              className="rounded-lg px-8 py-3 font-bold text-white transition-all hover:shadow-lg"
              style={{
                backgroundColor: "var(--stitch-primary)",
                boxShadow: "0 10px 15px -3px color-mix(in srgb, var(--stitch-primary) 20%, transparent)",
              }}
            >
              {t("subscribe")}
            </button>
          </form>
          <p className="text-[10px] font-bold uppercase tracking-[0.08em]" style={{ color: "var(--stitch-text-muted)" }}>
            {t("newsletterDisclaimer")}
          </p>
        </div>
      </section>

      {/* ===== 雷达弹窗 ===== */}
      {selectedBlip && (
        <section
          className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-label={t("relatedBlogs", { name: selectedBlip.name })}
        >
          <button
            type="button"
            className="absolute inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
            aria-label={t("closeModal")}
            onClick={closeModal}
          />

          <div
            className="relative flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl shadow-2xl"
            style={{ backgroundColor: "var(--stitch-bg)", border: "1px solid var(--stitch-border)" }}
            ref={modalRef}
          >
            <div className="flex items-center justify-between border-b p-6" style={{ borderColor: "var(--stitch-border)" }}>
              <div className="flex items-center gap-3">
                <div
                  className="flex items-center justify-center rounded-lg size-10"
                  style={{ backgroundColor: "rgba(33,196,93,0.1)" }}
                >
                  <MaterialIcon name="hub" size={20} className="text-[var(--stitch-primary)]" />
                </div>
                <h3 className="text-lg font-bold" style={{ color: "var(--stitch-text)" }}>
                  {t("relatedBlogs", { name: selectedBlip.name })}
                </h3>
              </div>
              <button
                type="button"
                className="rounded-full p-2 transition-colors hover:bg-[var(--stitch-bg-elevated)]"
                style={{ color: "var(--stitch-text-muted)" }}
                aria-label={t("closeModal")}
                ref={closeButtonRef}
                onClick={closeModal}
              >
                <MaterialIcon name="close" size={22} />
              </button>
            </div>

            <div className="flex flex-col gap-3 overflow-y-auto p-6">
              {relatedArticles.length > 0 ? (
                relatedArticles.map((article) => (
                  <Link
                    key={article.slug}
                    href={`/blog/${article.slug}`}
                    className="block rounded-xl border p-4 transition-all hover:-translate-y-0.5 hover:shadow-md"
                    style={{ borderColor: "var(--stitch-border)", backgroundColor: "var(--stitch-bg-elevated)" }}
                  >
                    <p className="mb-2 font-bold transition-colors hover:text-[var(--stitch-primary)]" style={{ color: "var(--stitch-text)" }}>
                      {article.title}
                    </p>
                    <p className="text-sm" style={{ color: "var(--stitch-text-muted)" }}>
                      {article.tag} · {article.publishedAt} · {article.readTime}
                    </p>
                  </Link>
                ))
              ) : (
                <div className="flex flex-col items-center gap-4 py-12 text-center">
                  <div style={{ color: "var(--stitch-text-muted)" }}>
                    <MaterialIcon name="article" size={40} />
                  </div>
                  <p style={{ color: "var(--stitch-text-muted)" }}>{t("noRelatedBlogs")}</p>
                </div>
              )}
            </div>
          </div>
        </section>
      )}
    </>
  );
}

import Link from "next/link";
import Image from "next/image";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { useMDXComponents } from "@/mdx-components";

type PublicArticleDetail = {
  slug: string;
  title: string;
  excerpt: string;
  cover_image_url: string;
  tag: string;
  read_time: string;
  author_name: string;
  author_avatar_url?: string;
  author_icon?: string;
  mdx_body: string;
  published_at: string;
};

type PublicArticleDetailResponse = {
  article?: PublicArticleDetail;
  error?: string;
};

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

async function getArticleBySlug(slug: string) {
  const apiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (!apiBaseUrl) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }

  const response = await fetch(
    `${apiBaseUrl.replace(/\/$/, "")}/public/articles/${encodeURIComponent(slug)}`,
    {
      method: "GET",
      headers: { "content-type": "application/json", accept: "application/json" },
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    return null;
  }

  const payload = (await response.json()) as PublicArticleDetailResponse;

  if (!response.ok || !payload.article) {
    throw new Error(payload.error ?? "Failed to load article detail");
  }

  return payload.article;
}

export default async function BlogSlugDetailPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const article = await getArticleBySlug(slug);
  const mdxComponents = useMDXComponents({});

  if (!article) {
    notFound();
  }

  return (
    <article className="max-w-3xl mx-auto space-y-8">
      <Link
        href="/blog"
        className="inline-flex items-center text-[var(--portal-muted)] hover:text-[var(--portal-ink)] transition-colors"
      >
        <svg aria-hidden="true" className="w-5 h-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        返回博客
      </Link>

      <div className="relative w-full aspect-[2/1] rounded-2xl overflow-hidden">
        <Image src={article.cover_image_url} alt={article.title} fill className="object-cover" />
      </div>

      <header className="space-y-4">
        <div className="flex items-center gap-3">
          <span className="px-3 py-1 bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-sm font-medium rounded-full">
            {article.tag}
          </span>
          <span className="text-[var(--portal-muted)] text-sm">{formatPublishedDate(article.published_at)}</span>
          <span className="text-[var(--portal-muted)] text-sm">• {article.read_time}</span>
        </div>
        <h1 className="text-3xl md:text-4xl font-bold text-[var(--portal-ink)]">{article.title}</h1>
        <p className="text-xl text-[var(--portal-muted)]">{article.excerpt}</p>
      </header>

      <div className="space-y-3">
        <div className="text-[var(--portal-muted)] leading-relaxed space-y-4">
          <MDXRemote
            source={article.mdx_body}
            components={mdxComponents}
            options={{
              mdxOptions: {
                remarkPlugins: [remarkGfm],
                rehypePlugins: [rehypeHighlight],
              },
            }}
          />
        </div>
      </div>

      <footer className="border-t border-[var(--portal-line)] pt-8 mt-8">
        <div className="flex items-center justify-between">
          <Link
            href="/blog"
            className="inline-flex items-center text-emerald-600 dark:text-emerald-400 hover:text-emerald-500 dark:hover:text-emerald-300 transition-colors"
          >
            <svg aria-hidden="true" className="w-5 h-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            查看更多文章
          </Link>
        </div>
      </footer>
    </article>
  );
}

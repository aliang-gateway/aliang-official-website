import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { SseParserClient } from "./SseParserClient";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("editorial.tools.sseParser");
  const title = `${t("heroTitle")} | Anthropic / OpenAI 流式调试 - 阿良家的AI`;
  const description = t("seoIntro");
  const path = "/tools/sse-parser";
  const url = process.env.NEXT_PUBLIC_SITE_URL
    ? `${process.env.NEXT_PUBLIC_SITE_URL}${path}`
    : path;
  return {
    title,
    description,
    keywords: [
      "SSE 解析", "流式响应解析", "Anthropic 流式", "OpenAI 流式",
      "ChatGPT stream", "Claude stream", "Server-Sent Events",
      "token 统计", "工具调用解析", "function calling", "tool use",
    ],
    alternates: { canonical: path },
    openGraph: { title, description, url, type: "website" },
    twitter: { card: "summary_large_image", title, description },
  };
}

const jsonLd = {
  "@context": "https://schema.org",
  "@type": "WebApplication",
  name: "SSE 流式响应解析器",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Any",
  url: "/tools/sse-parser",
  description: "在线解析 Anthropic / OpenAI 流式 SSE 响应，提取文本、Token 用量、工具调用与事件时间线。",
  offers: { "@type": "Offer", price: "0", priceCurrency: "CNY" },
};

export default async function SseParserPage() {
  const t = await getTranslations("editorial.tools.sseParser");
  return (
    <div className="tool-page">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }} />
      <div className="container wide">
        <header style={{ marginBottom: 32 }}>
          <div className="label">{t("heroLabel")}</div>
          <h1 className="display">
            {t("heroTitle")}
            <span className="dot">.</span>
          </h1>
          <p className="lead" style={{ maxWidth: 760, marginTop: 12 }}>{t("heroLead")}</p>
          <p style={{ maxWidth: 820, marginTop: 12, color: "var(--ink-soft)" }}>{t("seoIntro")}</p>
        </header>
        <SseParserClient />
        <p style={{ marginTop: 24 }}>
          <Link href="/services" className="btn">← {t("backToServices")}</Link>
        </p>
      </div>
    </div>
  );
}

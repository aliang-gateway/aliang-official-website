import type { MetadataRoute } from "next";

const BASE = process.env.NEXT_PUBLIC_SITE_URL ?? "https://aliang.one";

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  const routes: { path: string; priority: number; changefreq: MetadataRoute.Sitemap[number]["changeFrequency"] }[] = [
    { path: "", priority: 1.0, changefreq: "weekly" },
    { path: "/services", priority: 0.9, changefreq: "weekly" },
    { path: "/tools/sse-parser", priority: 0.8, changefreq: "monthly" },
    { path: "/download", priority: 0.7, changefreq: "monthly" },
    { path: "/docs", priority: 0.7, changefreq: "weekly" },
    { path: "/price", priority: 0.7, changefreq: "monthly" },
    { path: "/about", priority: 0.5, changefreq: "monthly" },
    { path: "/blog", priority: 0.6, changefreq: "weekly" },
  ];
  return routes.map((r) => ({
    url: `${BASE}${r.path}`,
    lastModified: now,
    changeFrequency: r.changefreq,
    priority: r.priority,
  }));
}

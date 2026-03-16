"use client";

import Link from "next/link";
import Image from "next/image";

export default function BlogPage() {
  const articles = [
    {
      id: 1,
      title: "Welcome to AI API Portal",
      tag: "Announcement",
      date: "2025-01-15",
      excerpt: "We're excited to launch our new platform for AI API services with transparent pricing.",
      readTime: "3 min read",
      image: "https://picsum.photos/seed/blog1/400/300"
    },
    {
      id: 2,
      title: "Getting Started with our API",
      tag: "Tutorial",
      date: "2025-01-10",
      excerpt: "A comprehensive guide to integrating with our pricing API and managing subscriptions.",
      readTime: "5 min read",
      image: "https://picsum.photos/seed/blog2/400/300"
    },
    {
      id: 3,
      title: "Understanding Usage-Based Billing",
      tag: "Deep Dive",
      date: "2024-12-20",
      excerpt: "Deep dive into our usage-based billing model and how to optimize your costs.",
      readTime: "8 min read",
      image: "https://picsum.photos/seed/blog3/400/300"
    },
    {
      id: 4,
      title: "New Year, New Features",
      tag: "Preview",
      date: "2024-12-15",
      excerpt: "Get a sneak peek at all upcoming features planned for the new year.",
      readTime: "5 min read",
      image: "https://picsum.photos/seed/blog4/400/300"
    },
    {
      id: 5,
      title: "Security Best Practices",
      tag: "Security",
      date: "2024-12-01",
      excerpt: "Learn how to secure your API keys and protect your applications.",
      readTime: "4 min read",
      image: "https://picsum.photos/seed/blog5/400/300"
    },
    {
      id: 6,
      title: "API Rate Limits Update",
      tag: "Update",
      date: "2024-11-15",
      excerpt: "Important updates to our API rate limits for better service quality.",
      readTime: "2 min read",
      image: "https://picsum.photos/seed/blog6/400/300"
    },
  ];

  return (
    <div className="min-h-screen bg-black">
      {/* Hero Section */}
      <section className="clay-panel p-8 mb-12">
        <p className="text-sm font-semibold uppercase tracking-wide text-emerald-400">
          Blog
        </p>
        <h1 className="text-4xl md:text-5xl font-bold text-white mb-4">
          Latest <span className="gradient-text">Articles</span> & Updates
        </h1>
        <p className="text-lg text-gray-400 mb-6 max-w-2xl">
          Explore our latest articles about AI, APIs, and development. 
          Stay updated with our news and insights.
        </p>
      </section>

      {/* Articles Grid */}
      <section className="mt-8">
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {articles.map((article) => (
            <article key={article.id} className="block-card">
              <div className="block-card-image">
                <Image
                  src={article.image}
                  alt={article.title}
                  width={400}
                  height={300}
                  className="object-cover"
                />
              </div>
              <div className="p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="block-card-title">{article.title}</h3>
                  <span className="block-card-tag">{article.tag}</span>
                </div>
                <p className="block-card-excerpt">{article.excerpt}</p>
                <div className="block-card-meta">
                  <span>{article.date}</span>
                  <span>• {article.readTime}</span>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

import Link from "next/link";
import Image from "next/image";

// Article data
const cards = [
  {
    id: 1,
    title: "Welcome to AI API Portal",
    date: "2025-01-15",
    excerpt: "We are excited to launch our new AI API Portal...",
    slug: "2025-01-15-welcome-ai-api-portal",
    image: "https://picsum.photos/seed/1/400/200",
    tag: "News",
    readTime: "2 min read"
  },
  {
    id: 2,
    title: "Getting Started",
    date: "2025-01-15",
    excerpt: "Learn how to integrate with our pricing API...",
    slug: "2025-01-15-getting-started",
    image: "https://picsum.photos/seed/4/400/200",
    tag: "Tutorial",
    readTime: "5 min read"
  },
  {
    id: 3,
    title: "Announcing API v2",
    date: "2025-01-15",
    excerpt: "New endpoints for subscription management...",
    slug: "2025-01-15-announcing-api-v2",
    image: "https://picsum.photos/seed/3/400/200",
    tag: "Release",
    readTime: "3 min read"
  },
];

export default function Home() {
  return (
    <div className="min-h-screen">
      <section className="hero-section clay-panel p-8 md:p-12 lg:p-16 mb-12 relative overflow-hidden">
        <div className="hero-decoration" aria-hidden="true" />
        <div className="hero-glow" aria-hidden="true" />
        <div className="relative z-10">
          <p className="hero-badge">
            AI API Portal
          </p>
          <h1 className="hero-title">
            Build and Scale with <span className="gradient-text">Confidence</span>
          </h1>
          <p className="hero-subtitle">
            Explore our powerful AI APIs with transparent pricing, flexible subscription management, and world-class developer experience. 
            Start building the future today.
          </p>
          <div className="hero-actions">
            <Link href="/blog" className="btn-primary btn-large inline-flex items-center gap-2">
              <span>Get Started</span>
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
              </svg>
            </Link>
            <Link href="/pricing" className="btn-secondary btn-large inline-flex items-center gap-2">
              <span>View Pricing</span>
            </Link>
          </div>
          <div className="hero-stats">
            <div className="hero-stat">
              <span className="hero-stat-value">99.9%</span>
              <span className="hero-stat-label">Uptime SLA</span>
            </div>
            <div className="hero-stat-divider" />
            <div className="hero-stat">
              <span className="hero-stat-value">10M+</span>
              <span className="hero-stat-label">API Calls/Day</span>
            </div>
            <div className="hero-stat-divider" />
            <div className="hero-stat">
              <span className="hero-stat-value">50ms</span>
              <span className="hero-stat-label">Avg Response</span>
            </div>
          </div>
        </div>
      </section>

      <section className="mt-12">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-2xl font-bold">Latest Articles</h2>
            <p className="text-sm mt-1" style={{color: 'var(--portal-muted)'}}>
              Stay updated with our latest news and tutorials
            </p>
          </div>
          <Link href="/blog" className="nav-pill text-sm">
            View All
          </Link>
        </div>
        
        <div className="horizontal-scroll flex gap-6 pb-4" suppressHydrationWarning>
          {cards.map((card) => (
            <article key={card.id} className="block-card min-w-[320px] flex-shrink-0 snap-center cursor-pointer group">
              <div className="block-card-image">
                <Image 
                  src={card.image} 
                  alt={card.title} 
                  width={400}
                  height={200}
                  className="object-cover"
                />
              </div>
              <div className="p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="block-card-title">{card.title}</h3>
                  <span className="block-card-tag">{card.tag}</span>
                </div>
                <p className="block-card-excerpt">{card.excerpt}</p>
                <div className="block-card-meta">
                  <span>{card.date}</span>
                  <span>• {card.readTime}</span>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

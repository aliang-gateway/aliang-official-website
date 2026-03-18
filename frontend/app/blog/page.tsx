"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import TechRadar3D from "./TechRadar3D";

type Article = {
  id: number;
  title: string;
  tag: string;
  date: string;
  excerpt: string;
  readTime: string;
  image: string;
  author: {
    name: string;
    avatar?: string;
    icon?: string;
  };
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

const articles: Article[] = [
  {
    id: 1,
    title: "The Future of AI Gateways: Orchestrating LLM Workloads",
    tag: "Academic",
    date: "Oct 24, 2024",
    excerpt: "Exploring the shift from traditional API management to intelligent semantic routing and context-aware protocol mediation for generative AI.",
    readTime: "12 min read",
    image: "https://lh3.googleusercontent.com/aida-public/AB6AXuDZvW-KTF9iWaCFboqP1LY1wawzF4cT4iRinHzzA98gGMHoW3n7E872hDiRLRpFV9Lgk1eUkBp0C5tFk7ciV-h83F-T8rQ9aSL54mq52wn4s2190sPIpfYHaOBWSDoORyyW51t7TAIFmgaixOfVBA5xviZqiAJVLIF21IFo_PU3frHPgmbyRpEyP0JWbCQpCX3T1gwqPFHh_14oe4j9NayqlyvrzjAku3Q-tlX2vEgrbTVb7UjS5NdARfSxXWUkEPTwOCVtUY5B0iw",
    author: {
      name: "Dr. Liang Tech",
      avatar: "https://lh3.googleusercontent.com/aida-public/AB6AXuCX5whxJVxi2uv3nqHWP4Ln5YBc4S53zCLo05XxpKF_62FQN0wMPA4OX6bFDShEvnJiolfeWpJ-iWYfPOThH9vD-_5OLnlRDtQDS0wy7V91shfl-XV8fMK-pGV7vovaB5nN_NB4Ef3vh6z1shMZSnv2H5M2ch7V7IhoMM0nDFdNegLAdhDpPmISxU77vgwmn97pyrqU0tq0OItNgircp86m_f6lxHCJ3FtsSlWipeXm9J4GMeehImA69qC2Yu47xnpVFcGkmBnccCs"
    }
  },
  {
    id: 2,
    title: "Optimizing Latency in TUN Mode: Kernel-Level Performance",
    tag: "Technical",
    date: "Oct 18, 2024",
    excerpt: "A deep dive into user-space network stack optimizations and reducing context switching overhead for high-throughput tunneling.",
    readTime: "15 min read",
    image: "https://lh3.googleusercontent.com/aida-public/AB6AXuCo7RwdCNWbFTe0iEEfily7t7bdDPmuo5PMltoyf75XR34eizCxJd0rHP3cZgeLjBeeBFIZqh55kskluY1FJZq8_UhtchAx_M8P8s_ciy04OIN_Awx4IoLLM5aiusVhLkjcP0McEGN7oawpuefnBtFSCQ7CnvlJh-ZNxa35MHtBzpZPK9ZszEnZqutl49g3rjFfVGkcFSXReA3Dp2cQl4P1pXnwLNimN9Ntj95PowoyYRwcknrU462tXD0N9LEdfXsL1dD3ZRPu02k",
    author: {
      name: "NetOps Team",
      icon: "person"
    }
  },
  {
    id: 3,
    title: "Security Best Practices for LLM API Key Management",
    tag: "Security",
    date: "Oct 12, 2024",
    excerpt: "Ensuring robust protection for sensitive gateway endpoints and preventing token leakage in edge computing environments.",
    readTime: "8 min read",
    image: "https://lh3.googleusercontent.com/aida-public/AB6AXuBomKbfjMIYYFfkmA1VLrMwLicACUIX2K7XJ9uLNZStKCR_UB1U6iL42Wjf-h14EWKO-EpJvuY1UCo4R1KP4dJBJVrP7lsDzt3HF7r5Um7-7dZ2b09cS4Opyjdnkv0UDEwWH0A4lGnT2ShbtM5_vJlyRFGqbcgE5PJ-FP3zz2m3HlIbbTBy4Bh3YX2i8vd92n0N0x_ap2rtKQxYKQRo3NS8iMT4auAxa27qkPbupw_4gdI-d-sk8t3uoT_IWLe8nCz6lareHZufdjk",
    author: {
      name: "SecOps Intel",
      icon: "shield"
    }
  },
];

export default function BlogPage() {
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
  }, [selectedBlip]);

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
      <section className="relative overflow-hidden py-16 px-6 md:px-20" style={{ backgroundColor: 'var(--stitch-bg-elevated)', color: 'var(--stitch-text)' }}>
        <div className="absolute inset-0 opacity-20 pointer-events-none" style={{ backgroundImage: 'radial-gradient(circle at 2px 2px, var(--stitch-primary) 1px, transparent 0)', backgroundSize: '40px 40px' }} />
        <div className="max-w-7xl mx-auto flex flex-col lg:flex-row items-center gap-12 relative z-10">
          <div className="flex flex-col gap-6 lg:w-1/2 text-left">
            <div className="inline-flex items-center px-3 py-1 rounded-full w-max text-xs font-bold tracking-widest uppercase" style={{ backgroundColor: 'color-mix(in srgb, var(--stitch-primary) 20%, transparent)', color: 'var(--stitch-primary)' }}>
              Edition 2024.Q3
            </div>
            <h1 className="text-4xl md:text-6xl font-black leading-tight tracking-tighter" style={{ color: 'var(--stitch-text)' }}>
              The ALiang <br /><span style={{ color: 'var(--stitch-primary)' }}>Tech Radar</span>
            </h1>
            <p className="text-lg md:text-xl font-normal max-w-xl leading-relaxed" style={{ color: 'var(--stitch-text-muted)' }}>
              A data-driven visualization of our strategic technology landscape: from high-performance TUN/HTTP proxies to advanced LLM Routing strategies.
            </p>
            <div className="flex flex-wrap gap-4 pt-4">
              <button type="button" className="flex items-center gap-2 rounded-lg h-12 px-8 font-bold hover:-translate-y-0.5 transition-all shadow-lg" style={{ backgroundColor: 'var(--stitch-primary)', color: 'white', boxShadow: '0 10px 15px -3px color-mix(in srgb, var(--stitch-primary) 20%, transparent)' }}>
                <span>Explore the Radar</span>
                <MaterialIcon name="explore" size={20} />
              </button>
              <button type="button" className="flex items-center gap-2 rounded-lg h-12 px-8 font-bold hover:bg-white/10 transition-all border backdrop-blur-sm" style={{ backgroundColor: 'rgba(255, 255, 255, 0.1)', color: 'var(--stitch-text)', borderColor: 'rgba(255, 255, 255, 0.1)' }}>
                <span>Methodology</span>
              </button>
            </div>
          </div>
          
          <div className="lg:w-1/2 relative flex justify-center items-center">
            <div className="relative size-[320px] md:size-[500px] border rounded-full flex items-center justify-center" style={{ borderColor: 'color-mix(in srgb, var(--stitch-primary) 30%, transparent)' }}>
              <div className="absolute inset-0 border rounded-full scale-75" style={{ borderColor: 'color-mix(in srgb, var(--stitch-primary) 20%, transparent)' }} />
              <div className="absolute inset-0 border rounded-full scale-50" style={{ borderColor: 'color-mix(in srgb, var(--stitch-primary) 10%, transparent)' }} />
              <div className="absolute inset-0 border rounded-full scale-[0.25]" style={{ borderColor: 'color-mix(in srgb, var(--stitch-primary) 5%, transparent)' }} />
              
              <div className="absolute inset-0 w-full h-full flex items-center justify-center pointer-events-auto z-10">
                <TechRadar3D 
                  blips={techBlips} 
                  onBlipClick={(blip, buttonEl) => {
                    lastTriggerRef.current = buttonEl;
                    setSelectedBlip(blip);
                  }} 
                />
              </div>

              <div className="absolute h-px w-full top-1/2" style={{ backgroundColor: 'color-mix(in srgb, var(--stitch-primary) 10%, transparent)' }} />
              <div className="absolute w-px h-full left-1/2" style={{ backgroundColor: 'color-mix(in srgb, var(--stitch-primary) 10%, transparent)' }} />
              
              <div className="z-10 p-4 border rounded-lg font-mono text-sm tracking-widest uppercase absolute bottom-0 right-0" style={{ backgroundColor: 'var(--stitch-bg-elevated)', borderColor: 'color-mix(in srgb, var(--stitch-primary) 50%, transparent)', color: 'var(--stitch-primary)' }}>
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
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              AI Gateways
            </button>
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              Networking
            </button>
            <button type="button" className="flex flex-col items-center py-4 border-b-2 border-transparent transition-colors font-semibold text-sm tracking-wide hover:text-primary" style={{ color: 'var(--stitch-text-muted)' }}>
              Security
            </button>
          </div>
          <div className="hidden md:flex items-center gap-2 text-sm" style={{ color: 'var(--stitch-text-muted)' }}>
            <MaterialIcon name="sort" size={16} />
            <span>Latest First</span>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {articles.map((article) => (
            <Link
              key={article.id}
              href={`/blog/${article.id}`}
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
                  <span>{article.date}</span>
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

        <div className="flex items-center justify-center mt-16 gap-2">
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border transition-colors hover:bg-slate-50 dark:hover:bg-slate-800" style={{ borderColor: 'var(--stitch-border)', color: 'var(--stitch-text-muted)' }}>
            <MaterialIcon name="chevron_left" size={18} />
          </button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg text-white font-bold text-sm" style={{ backgroundColor: 'var(--stitch-primary)' }}>1</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent transition-colors font-semibold text-sm hover:bg-slate-50 dark:hover:bg-slate-800" style={{ color: 'var(--stitch-text-muted)' }}>2</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent transition-colors font-semibold text-sm hover:bg-slate-50 dark:hover:bg-slate-800" style={{ color: 'var(--stitch-text-muted)' }}>3</button>
          <span className="px-2" style={{ color: 'var(--stitch-text-muted)' }}>...</span>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border border-transparent transition-colors font-semibold text-sm hover:bg-slate-50 dark:hover:bg-slate-800" style={{ color: 'var(--stitch-text-muted)' }}>12</button>
          <button type="button" className="size-10 flex items-center justify-center rounded-lg border transition-colors hover:bg-slate-50 dark:hover:bg-slate-800" style={{ borderColor: 'var(--stitch-border)', color: 'var(--stitch-text-muted)' }}>
            <MaterialIcon name="chevron_right" size={18} />
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
                className="rounded-full p-2 transition-colors hover:bg-slate-100 dark:hover:bg-slate-800"
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
                  <Link key={article.id} href={`/blog/${article.id}`} className="block p-4 rounded-xl border transition-all hover:-translate-y-0.5 hover:shadow-md" style={{ borderColor: 'var(--stitch-border)', backgroundColor: 'var(--stitch-bg-elevated)' }}>
                    <p className="font-bold mb-2 transition-colors hover:text-primary" style={{ color: 'var(--stitch-text)' }}>{article.title}</p>
                    <p className="text-sm" style={{ color: 'var(--stitch-text-muted)' }}>{article.tag} · {article.date} · {article.readTime}</p>
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

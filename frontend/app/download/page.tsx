"use client";

import Link from "next/link";
import { useState } from "react";

export default function DownloadPage() {
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCopy = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      console.error("Failed to copy text: ", err);
    }
  };

  const sdks = [
    {
      id: "nodejs",
      name: "Node.js SDK",
      command: "npm install @ai-api/node",
      icon: (
        <svg className="w-8 h-8" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
          <title>Node.js</title>
          <path d="M12 2L2 7.8V16.2L12 22L22 16.2V7.8L12 2ZM10.5 17.5L6 14.9V9.7L10.5 12.3V17.5ZM12 10.6L7.5 8L12 5.4L16.5 8L12 10.6ZM18 14.9L13.5 17.5V12.3L18 9.7V14.9Z" fill="currentColor" />
        </svg>
      )
    },
    {
      id: "python",
      name: "Python SDK",
      command: "pip install ai-api-python",
      icon: (
        <svg className="w-8 h-8" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
          <title>Python</title>
          <path d="M12 2C6.48 2 2 6.48 2 12C2 17.52 6.48 22 12 22C17.52 22 22 17.52 22 12C22 6.48 17.52 2 12 2ZM12 20C7.59 20 4 16.41 4 12C4 7.59 7.59 4 12 4C16.41 4 20 7.59 20 12C20 16.41 16.41 20 12 20ZM11 7H13V13H11V7ZM11 15H13V17H11V15Z" fill="currentColor" />
        </svg>
      )
    },
    {
      id: "go",
      name: "Go SDK",
      command: "go get github.com/ai-api/go-sdk",
      icon: (
        <svg className="w-8 h-8" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
          <title>Go</title>
          <path d="M12 2L2 7.8V16.2L12 22L22 16.2V7.8L12 2ZM12 19.4L4.5 15V9L12 4.6L19.5 9V15L12 19.4ZM12 10.5C11.2 10.5 10.5 11.2 10.5 12C10.5 12.8 11.2 13.5 12 13.5C12.8 13.5 13.5 12.8 13.5 12C13.5 11.2 12.8 10.5 12 10.5Z" fill="currentColor" />
        </svg>
      )
    }
  ];

  return (
    <div className="min-h-screen">
      <section className="hero-section clay-panel p-8 md:p-12 lg:p-16 mb-12 relative overflow-hidden">
        <div className="hero-decoration" aria-hidden="true" />
        <div className="hero-glow" aria-hidden="true" />
        <div className="relative z-10 text-center max-w-3xl mx-auto">
          <p className="hero-badge">
            开发者资源
          </p>
          <h1 className="hero-title">
            <span className="gradient-text">下载中心</span>
          </h1>
          <p className="hero-subtitle mx-auto">
            获取适用于各个平台的最新 SDK 和接入工具，快速将强大的 AI 能力集成到您的应用中。
          </p>
        </div>
      </section>

      <section className="mb-12">
        <div className="mb-6">
          <h2 className="text-2xl font-bold" style={{ color: 'var(--portal-ink)' }}>官方 SDK</h2>
          <p className="text-sm mt-1" style={{ color: 'var(--portal-muted)' }}>
            提供主流编程语言的官方支持，开箱即用
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          {sdks.map((sdk) => (
            <div key={sdk.id} className="block-card flex flex-col h-full group transition-all duration-300 hover:-translate-y-1 hover:shadow-lg">
              <div className="flex items-center gap-4 mb-6">
                <div className="p-3 rounded-xl" style={{ background: 'var(--portal-bg-subtle)', color: 'var(--portal-accent)' }}>
                  {sdk.icon}
                </div>
                <h3 className="text-xl font-bold" style={{ color: 'var(--portal-ink)' }}>{sdk.name}</h3>
              </div>
              
              <div className="mt-auto">
                <div className="flex items-center justify-between p-3 rounded-lg" style={{ background: 'var(--portal-bg-subtle)', border: '1px solid var(--portal-line)' }}>
                  <code className="text-sm font-mono truncate mr-4" style={{ color: 'var(--portal-muted)' }}>
                    {sdk.command}
                  </code>
                  <button 
                    type="button"
                    onClick={() => handleCopy(sdk.command, sdk.id)}
                    className="btn-ghost text-xs whitespace-nowrap px-3 py-1.5"
                    aria-label={`Copy ${sdk.name} install command`}
                  >
                    {copiedId === sdk.id ? '已复制' : '复制'}
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="mb-12">
        <div className="mb-6">
          <h2 className="text-2xl font-bold" style={{ color: 'var(--portal-ink)' }}>命令行工具</h2>
          <p className="text-sm mt-1" style={{ color: 'var(--portal-muted)' }}>
            直接通过终端调用 API，适合快速测试和脚本集成
          </p>
        </div>

        <div className="block-card">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
            <div>
              <h3 className="text-lg font-bold mb-2" style={{ color: 'var(--portal-ink)' }}>cURL 示例</h3>
              <p className="text-sm mb-4" style={{ color: 'var(--portal-muted)' }}>
                使用标准的 HTTP 请求即可轻松接入我们的服务。
              </p>
            </div>
          </div>
          
          <div className="relative rounded-xl overflow-hidden" style={{ background: 'var(--portal-bg-subtle)', border: '1px solid var(--portal-line)' }}>
            <div className="flex items-center justify-between px-4 py-2 border-b" style={{ borderColor: 'var(--portal-line)', background: 'var(--portal-clay)' }}>
              <span className="text-xs font-mono" style={{ color: 'var(--portal-muted)' }}>bash</span>
              <button 
                type="button"
                onClick={() => handleCopy('curl -X POST https://api.aliang.one/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer $YOUR_API_KEY" \\\n  -d \'{\n    "model": "gpt-4o",\n    "messages": [{"role": "user", "content": "Hello!"}]\n  }\'', 'curl')}
                className="text-xs hover:opacity-80 transition-opacity"
                style={{ color: 'var(--portal-accent)' }}
              >
                {copiedId === 'curl' ? '已复制' : '复制代码'}
              </button>
            </div>
            <pre className="p-4 overflow-x-auto text-sm font-mono leading-relaxed" style={{ color: 'var(--portal-ink)' }}>
              <code>
<span style={{ color: 'var(--portal-accent)' }}>curl</span> -X POST https://api.aliang.one/v1/chat/completions \
  -H <span style={{ color: 'var(--portal-muted)' }}>"Content-Type: application/json"</span> \
  -H <span style={{ color: 'var(--portal-muted)' }}>"Authorization: Bearer $YOUR_API_KEY"</span> \
  -d <span style={{ color: 'var(--portal-muted)' }}>'{'{'}
    "model": "gpt-4o",
    "messages": [{'{'}"role": "user", "content": "Hello!"{'}'}]
  {'}'}'</span>
              </code>
            </pre>
          </div>
        </div>
      </section>

      <section className="mb-12">
        <div className="block-card flex flex-col md:flex-row items-center justify-between gap-6 p-8" style={{ background: 'var(--portal-gradient)', color: 'white', border: 'none' }}>
          <div>
            <h2 className="text-2xl font-bold mb-2 text-white">需要更多帮助？</h2>
            <p className="text-white/80 max-w-xl">
              查阅我们的完整开发文档，了解所有 API 端点、参数说明、错误码以及最佳实践指南。
            </p>
          </div>
          <Link href="/docs" className="btn-secondary whitespace-nowrap" style={{ background: 'white', color: 'var(--portal-accent-dark)', border: 'none' }}>
            查看开发文档
          </Link>
        </div>
      </section>

      <div className="mt-12 text-center">
        <Link href="/" className="btn-ghost inline-flex items-center gap-2">
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
            <title>Back</title>
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
          </svg>
          返回首页
        </Link>
      </div>
    </div>
  );
}

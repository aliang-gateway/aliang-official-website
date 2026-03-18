"use client";

import { Fragment, type ReactNode } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import Image from "next/image";

const articles = [
  {
    id: 1,
    title: "GPT-4o 与 Claude 3.5 深度对比评测",
    tag: "AI评测",
    date: "2025-01-15",
    excerpt: "从代码能力、推理能力、创意写作等多个维度对比两大主流AI模型的实际表现。",
    readTime: "10 min read",
    image: "https://picsum.photos/seed/blog1/800/400",
    content: `
## 前言

在AI模型快速迭代的今天，GPT-4o 和 Claude 3.5 Sonnet 无疑是当前最受关注的两大主流模型。本文将从多个维度对它们进行深度对比评测。

## 测试环境

- 测试时间：2025年1月
- 测试场景：代码生成、逻辑推理、创意写作、多语言处理
- 评测标准：准确性、响应速度、成本效益

## 代码能力对比

### GPT-4o 表现
GPT-4o 在代码补全和调试方面表现出色，特别是在处理复杂算法时能够给出清晰的解释。

### Claude 3.5 Sonnet 表现
Claude 3.5 在代码审查和重构建议方面更为细致，能够发现潜在的边界情况问题。

## 推理能力

两款模型在复杂推理任务上都有不错的表现，但 Claude 3.5 在需要多步骤推理的任务中表现略胜一筹。

## 成本分析

从API调用成本来看，两者的定价策略各有优势，需要根据具体使用场景选择。

## 总结

选择哪个模型取决于你的具体需求。如果你需要强大的代码能力，GPT-4o 是不错的选择；如果你更看重推理和分析能力，Claude 3.5 可能更适合。
    `,
  },
  {
    id: 2,
    title: "Cursor 编辑器高效使用技巧",
    tag: "工具技巧",
    date: "2024-12-20",
    excerpt: "分享我在使用 Cursor 进行AI辅助编程时总结的一些实用技巧和最佳实践。",
    readTime: "8 min read",
    image: "https://picsum.photos/seed/blog2/800/400",
    content: `
## 为什么选择 Cursor

Cursor 是一款基于 VSCode 的 AI 辅助编程工具，它集成了强大的 AI 能力，能够显著提升开发效率。

## 核心功能介绍

### 1. AI 对话
使用 Cmd+L 快速唤起 AI 对话窗口，可以直接询问代码相关问题。

### 2. 代码补全
Tab 键接受 AI 建议，支持多行代码补全。

### 3. 代码重构
选中代码后使用 Cmd+K 进行智能重构。

## 最佳实践

1. **善用上下文**：让 AI 了解你的项目结构
2. **渐进式开发**：先写核心逻辑，再让 AI 补充细节
3. **代码审查**：让 AI 帮你检查代码质量

## 快捷键总结

- Cmd+L：打开 AI 对话
- Cmd+K：内联编辑
- Tab：接受建议
- Cmd+Shift+L：解释代码
    `,
  },
  {
    id: 3,
    title: "国产大模型横评：DeepSeek vs Qwen",
    tag: "AI评测",
    date: "2024-12-15",
    excerpt: "对比评测 DeepSeek 和通义千问在中文场景下的表现，帮你选择合适的模型。",
    readTime: "12 min read",
    image: "https://picsum.photos/seed/blog3/800/400",
    content: `
## 评测背景

国产大模型发展迅速，DeepSeek 和通义千问（Qwen）是目前最受关注的两款产品。本文将从中文处理能力、代码能力、性价比等维度进行对比。

## 中文理解能力

### DeepSeek
- 古文理解能力较强
- 成语运用准确
- 中文创意写作流畅

### 通义千问
- 中文常识知识丰富
- 对中国文化背景理解深入
- 商务文案生成专业

## 代码能力

两款模型都支持主流编程语言，但在细节表现上各有千秋。

## API 接入

两款产品都提供了完善的 API 接口，接入文档清晰。

## 价格对比

DeepSeek 的定价更具竞争力，适合大规模调用的场景。

## 选择建议

- 注重性价比：选择 DeepSeek
- 需要丰富中文知识：选择通义千问
- 两者都是优秀的国产模型
    `,
  },
  {
    id: 4,
    title: "国产大模型横评：DeepSeek vs Qwen",
    tag: "AI评测",
    date: "2024-12-15",
    excerpt: "对比评测 DeepSeek 和通义千问在中文场景下的表现，帮你选择合适的模型。",
    readTime: "12 min read",
    image: "https://picsum.photos/seed/blog4/800/400",
    content: `
## 评测背景

国产大模型发展迅速，DeepSeek 和通义千问（Qwen）是目前最受关注的两款产品。本文将从中文处理能力、代码能力、性价比等维度进行对比。

## 中文理解能力

### DeepSeek
- 古文理解能力较强
- 成语运用准确
- 中文创意写作流畅

### 通义千问
- 中文常识知识丰富
- 对中国文化背景理解深入
- 商务文案生成专业

## 代码能力

两款模型都支持主流编程语言，但在细节表现上各有千秋。

## API 接入

两款产品都提供了完善的 API 接口，接入文档清晰。

## 价格对比

DeepSeek 的定价更具竞争力，适合大规模调用的场景。

## 选择建议

- 注重性价比：选择 DeepSeek
- 需要丰富中文知识：选择通义千问
- 两者都是优秀的国产模型
    `,
  },
  {
    id: 5,
    title: "API Key 安全管理最佳实践",
    tag: "安全",
    date: "2024-12-01",
    excerpt: "如何安全地管理和存储你的 API 密钥，避免泄露和滥用。",
    readTime: "4 min read",
    image: "https://picsum.photos/seed/blog5/800/400",
    content: `
## 为什么 API Key 安全很重要

API Key 泄露可能导致：
- 账户被滥用产生高额费用
- 敏感数据泄露
- 服务被恶意调用

## 安全管理原则

### 1. 永远不要硬编码
不要将 API Key 直接写在代码中，应该使用环境变量。

### 2. 使用 .env 文件
创建 .env 文件存储密钥，并将其加入 .gitignore。

### 3. 定期轮换密钥
建议每隔 3-6 个月更换一次 API Key。

### 4. 设置使用限制
为不同的应用创建不同的 Key，并设置合理的使用限额。

## 代码示例

\`\`\`bash
# .env 文件
API_KEY=your_secret_key_here
\`\`\`

\`\`\`javascript
// 读取环境变量
const apiKey = process.env.API_KEY;
\`\`\`

## 泄露后的应对措施

1. 立即撤销泄露的 Key
2. 检查账户使用记录
3. 创建新的 Key
4. 更新所有使用该 Key 的应用
    `,
  },
];

export default function BlogDetailPage() {
  const renderInline = (text: string, keyPrefix: string) => {
    const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g).filter(Boolean);
    const seen = new Map<string, number>();
    const nodes: ReactNode[] = [];

    for (const part of parts) {
      const count = (seen.get(part) ?? 0) + 1;
      seen.set(part, count);
      const key = `${keyPrefix}-${part}-${count}`;

        if (/^`[^`]+`$/.test(part)) {
          nodes.push(
            <code
              key={key}
              className="bg-black/10 dark:bg-black/40 px-1.5 py-0.5 rounded text-emerald-600 dark:text-emerald-400"
            >
              {part.slice(1, -1)}
            </code>
          );
          continue;
        }
        if (/^\*\*[^*]+\*\*$/.test(part)) {
          nodes.push(
            <strong key={key} className="text-[var(--portal-ink)] font-semibold">
              {part.slice(2, -2)}
            </strong>
          );
          continue;
        }
        nodes.push(<Fragment key={key}>{part}</Fragment>);
      }

    return nodes;
  };

  const renderArticleContent = (content: string): ReactNode[] => {
    const lines = content.trim().split("\n");
    const nodes: ReactNode[] = [];
    let unorderedItems: string[] = [];
    let orderedItems: string[] = [];
    let inCodeBlock = false;
    let codeBuffer: string[] = [];
    let nodeCounter = 0;

    const nextNodeKey = (prefix: string) => {
      nodeCounter += 1;
      return `${prefix}-${nodeCounter}`;
    };

    const flushUnorderedList = () => {
      if (unorderedItems.length === 0) {
        return;
      }
      const key = nextNodeKey("ul");
      const itemSeen = new Map<string, number>();
      nodes.push(
        <ul key={key} className="list-disc pl-6 space-y-2 text-[var(--portal-muted)]">
          {unorderedItems.map((item) => {
            const count = (itemSeen.get(item) ?? 0) + 1;
            itemSeen.set(item, count);
            const itemKey = `${key}-${item}-${count}`;
            return <li key={itemKey}>{renderInline(item, itemKey)}</li>;
          })}
        </ul>,
      );
      unorderedItems = [];
    };

    const flushOrderedList = () => {
      if (orderedItems.length === 0) {
        return;
      }
      const key = nextNodeKey("ol");
      const itemSeen = new Map<string, number>();
      nodes.push(
        <ol key={key} className="list-decimal pl-6 space-y-2 text-[var(--portal-muted)]">
          {orderedItems.map((item) => {
            const count = (itemSeen.get(item) ?? 0) + 1;
            itemSeen.set(item, count);
            const itemKey = `${key}-${item}-${count}`;
            return <li key={itemKey}>{renderInline(item, itemKey)}</li>;
          })}
        </ol>,
      );
      orderedItems = [];
    };

    const flushCodeBlock = () => {
      if (codeBuffer.length === 0) {
        return;
      }
      const key = nextNodeKey("code");
      nodes.push(
        <pre
          key={key}
          className="bg-black/10 dark:bg-black/40 border border-[var(--portal-line)] rounded-xl p-4 overflow-x-auto text-sm"
        >
          <code className="text-emerald-600 dark:text-emerald-400">{codeBuffer.join("\n")}</code>
        </pre>,
      );
      codeBuffer = [];
    };

    lines.forEach((rawLine) => {
      const line = rawLine.trim();

      if (line === "```") {
        flushUnorderedList();
        flushOrderedList();
        if (inCodeBlock) {
          flushCodeBlock();
        }
        inCodeBlock = !inCodeBlock;
        return;
      }

      if (inCodeBlock) {
        codeBuffer.push(rawLine);
        return;
      }

      if (!line) {
        flushUnorderedList();
        flushOrderedList();
        return;
      }

      const unorderedMatch = line.match(/^[-*]\s+(.+)/);
      if (unorderedMatch) {
        flushOrderedList();
        unorderedItems.push(unorderedMatch[1]);
        return;
      }

      const orderedMatch = line.match(/^\d+\.\s+(.+)/);
      if (orderedMatch) {
        flushUnorderedList();
        orderedItems.push(orderedMatch[1]);
        return;
      }

      flushUnorderedList();
      flushOrderedList();

      if (line.startsWith("### ")) {
        const text = line.slice(4);
        const key = nextNodeKey("h3");
        nodes.push(
          <h3 key={key} className="text-xl font-semibold text-[var(--portal-ink)] mt-6 mb-2">
            {renderInline(text, key)}
          </h3>,
        );
        return;
      }

      if (line.startsWith("## ")) {
        const text = line.slice(3);
        const key = nextNodeKey("h2");
        nodes.push(
          <h2 key={key} className="text-2xl font-bold text-[var(--portal-ink)] mt-8 mb-3">
            {renderInline(text, key)}
          </h2>,
        );
        return;
      }

      const paragraphKey = nextNodeKey("p");
      nodes.push(
        <p key={paragraphKey} className="text-[var(--portal-muted)] leading-7">
          {renderInline(line, paragraphKey)}
        </p>,
      );
    });

    flushUnorderedList();
    flushOrderedList();
    if (inCodeBlock) {
      flushCodeBlock();
    }

    return nodes;
  };

  const params = useParams();
  const article = articles.find((a) => a.id === parseInt(params.id as string));

  if (!article) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] space-y-4">
        <h1 className="text-2xl font-bold text-[var(--portal-ink)]">文章未找到</h1>
        <p className="text-[var(--portal-muted)]">该文章可能已被删除或移动。</p>
        <Link href="/blog" className="btn-primary">
          返回博客列表
        </Link>
      </div>
    );
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
        <Image
          src={article.image}
          alt={article.title}
          fill
          className="object-cover"
        />
      </div>

      {/* Header */}
      <header className="space-y-4">
        <div className="flex items-center gap-3">
          <span className="px-3 py-1 bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-sm font-medium rounded-full">
            {article.tag}
          </span>
          <span className="text-[var(--portal-muted)] text-sm">{article.date}</span>
          <span className="text-[var(--portal-muted)] text-sm">• {article.readTime}</span>
        </div>
        <h1 className="text-3xl md:text-4xl font-bold text-[var(--portal-ink)]">
          {article.title}
        </h1>
        <p className="text-xl text-[var(--portal-muted)]">
          {article.excerpt}
        </p>
      </header>

      {/* Content */}
      <div className="space-y-3">
        <div className="text-[var(--portal-muted)] leading-relaxed space-y-4">
          {renderArticleContent(article.content)}
        </div>
      </div>

      {/* Footer */}
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

# SSE 流式协议解析工具（SSE Stream Parser Tool）

## Overview

在营销站点的「服务」页（`/services`）新增一个**开发者小工具**入口，点击后进入一个独立工具页 `/tools/sse-parser`。该工具让用户把 **Anthropic Messages API** 或 **OpenAI Chat Completions API** 的**流式响应原始报文（SSE, Server-Sent Events）**粘贴进来，纯前端解析，提取出对调试有用的四类信息：**完整文本、Token 用量、工具调用（Tool Use）、事件时间线**。

工具**完全在浏览器端运行**，不向任何后端发送粘贴内容，零隐私风险。同时为该工具页配置完整的 SEO（页面级 metadata + JSON-LD 结构化数据 + 语义化 HTML），并补齐项目当前缺失的全站 SEO 基础设施（`sitemap.ts` / `robots.ts` / 全局 metadata 增强），使页面能被搜索引擎发现和收录。

## Background

- 前端：Next.js 16.1.6（App Router + TypeScript）+ Tailwind v4 + `next-intl`（中/英，默认 `zh`，cookie 切换）+ MDX。`output: "standalone"`，`assetPrefix` 指向 CDN。
- 营销页面位于 `app/(marketing)` 路由组，统一「editorial」设计风格（`app/(marketing)/editorial.css`、`_editorial/EditorialShell.tsx` 提供页头/页脚/滚动行为）。
- 实际导航由 `_editorial/EditorialHeader.tsx` 渲染（主菜单：首页 01 / 服务 02 / 下载 03 / 文档 04）；`components/layout/SiteHeader.tsx` 属另一套（app 组），与营销页无关。
- `/services` 现为「研究路线 + 已交付能力」时间线页，数据来自 `/api/public/services`。
- i18n 文案集中在 `messages/zh.json` / `en.json`，按命名空间组织（如 `editorial.services`）。
- **SEO 现状**：仅根 `app/layout.tsx` 有一组全局 `title`/`description`；**无** `sitemap.ts`、**无** `robots.ts`，几乎无页面级 metadata。i18n 采用 cookie 切换（无 `[locale]` 路由段），中英文共享同一 URL。

## Goals

1. `/services` 页新增「小工具」入口卡，引导至 `/tools/sse-parser`。
2. `/tools/sse-parser` 能粘贴原始 SSE 文本，**自动识别** Anthropic / OpenAI 协议（含手动覆盖），输出四类结果。
3. 解析逻辑为**纯函数**、框架无关、可单测。
4. 工具页具备完整 SEO（metadata、JSON-LD、语义化、OG），并补齐全站 `sitemap.ts` / `robots.ts` / 全局 metadata。
5. 全程纯前端，零网络上报；UI 文案中英双语，视觉沿用 editorial 风格。

## Non-Goals（本次明确不做）

- 实时 API 抓包 / 在线请求上游模型（需要 API Key，公开页不宜收集）。
- 文件上传（可后续迭代，本次仅支持粘贴）。
- `[locale]` 路由改造与 `hreflang` 多语言 SEO（独立议题，超出本次范围）。
- 后端改动：本工具无后端依赖。

## Decisions（已与用户确认）

| 维度 | 决策 |
|---|---|
| 提取内容 | 完整文本拼接 + Token 用量统计 + Tool Use 解析 + 事件时间线（四类全做） |
| 入口形态 | `/services` 加入口卡 → 独立工具页（独立 URL + 独立 metadata，利于 SEO） |
| 输入方式 | 粘贴原始 SSE 文本；纯前端；自动识别协议 + 手动覆盖 |
| 路由结构 | 新建 `/tools` 命名空间，工具页为 `/tools/sse-parser`（未来可扩展更多工具） |
| SEO 范围 | 工具页专属 SEO + 补齐全站 `sitemap.ts` / `robots.ts` / 全局 metadata 基础设施 |
| 解析实现 | 手写解析器（SSE 格式简单；协议特定归约是核心逻辑，不引第三方库） |

## Architecture

工具页 `page.tsx` 为 **server component**（承载 `generateMetadata`、JSON-LD、静态语义化文案），内嵌一个 **client component** `SseParserClient.tsx` 负责交互。解析逻辑抽到 `frontend/lib/sse/`，为纯函数、无 React 依赖，便于单测。

### 新增文件

```
frontend/app/(marketing)/tools/sse-parser/
  ├─ page.tsx              # server component: generateMetadata + JSON-LD + 页面骨架（hero + 静态说明 + <SseParserClient/>）
  └─ SseParserClient.tsx   # "use client": 文本框、协议选择、Tab 结果、复制、示例
frontend/lib/sse/
  ├─ types.ts              # 统一结果类型 UnifiedResult、SseEvent、协议枚举
  ├─ parseSse.ts           # raw 文本 → SseEvent[]
  ├─ detect.ts             # SseEvent[] → "anthropic" | "openai"
  ├─ anthropic.ts          # SseEvent[] → UnifiedResult
  └─ openai.ts             # SseEvent[] → UnifiedResult
frontend/app/sitemap.ts    # 站点地图（新）
frontend/app/robots.ts     # robots.txt（新）
```

### 修改文件

- `app/(marketing)/services/page.tsx` — timeline 之后新增「开发者小工具」入口卡区块。
- `app/layout.tsx` — 全局 metadata 增强（`metadataBase`、默认 openGraph、keywords）。
- `messages/zh.json` / `en.json` — 新增 `editorial.tools.sseParser` 命名空间；`editorial.services` 增入口卡文案。

## Detailed Design

### 1. SSE 通用解析（`parseSse.ts`）

输入：原始文本（含 `data: ...` 行，可能含 `event: ...` 行，块间以空行分隔）。

算法：
1. 以 `\n\n`（或 `\r\n\r\n`）切分为 chunk。
2. 每个 chunk 按行解析：`event:` 行记录 `event` 名；以 `data:` 开头的行剥去前缀与一个可选空格，拼接（多行 data 用 `\n` 连接）。
3. `data: [DONE]` → 标记终止，不计入事件数据。
4. 其余 `data` 尝试 `JSON.parse`：成功 → `{ event, raw, json, ok: true }`；失败 → `{ event, raw, json: null, ok: false, error }`（保留原文本，不中断）。
5. 输出 `SseEvent[]`，保留原始顺序，供「时间线」Tab 与归约器使用。

类型（`types.ts`）：
```ts
type Protocol = "anthropic" | "openai";
interface SseEvent {
  index: number;        // 序号（1-based），用于时间线
  event?: string;       // Anthropic 的 event: 名；OpenAI 通常无
  raw: string;          // data 原文本
  json: unknown | null; // 解析后的对象；解析失败为 null
  ok: boolean;          // JSON.parse 是否成功
  error?: string;       // 失败原因
}
interface ToolCall {
  index: number;
  id: string;
  name: string;
  arguments: object | string; // 成功 parse 为 object，失败保留拼接字符串
  argumentsParseError?: string;
}
interface UsageStats {
  // 通用归一化字段
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  reasoningTokens?: number;
  model?: string;
  stopReason?: string;  // Anthropic stop_reason / OpenAI finish_reason
  raw?: unknown;        // 原始 usage 对象，供高级用户查看
}
interface UnifiedResult {
  protocol: Protocol;
  text: string;                 // 拼接后的完整文本
  toolCalls: ToolCall[];
  usage: UsageStats;
  events: SseEvent[];           // 原始事件，供时间线
  errors: { index: number; error: string }[]; // 解析失败的事件
}
```

### 2. 协议识别（`detect.ts`）

- 命中任一即判为 **Anthropic**：存在 `event:` 行；或某事件 `json.type` ∈ {`message_start`, `content_block_delta`, `message_delta`, `message_stop`}。
- 命中任一即判为 **OpenAI**：某事件 `json.object === "chat.completion.chunk"`；或存在 `json.choices` 数组。
- 两者都命中或都不命中 → 返回 `null`（UI 提示用户手动选择；不强行猜测）。
- 用户手动选择时，以用户选择为准。

### 3. Anthropic 归约（`anthropic.ts`）

按事件 `type` 归约：

| 事件 / 字段 | 提取内容 |
|---|---|
| `message_start.message.model` | `usage.model` |
| `message_start.message.usage.{input_tokens, cache_creation_input_tokens, cache_read_input_tokens}` | usage 对应字段（初始 input 侧） |
| `content_block_delta.delta.type === "text_delta"` → `.delta.text` | 追加到 `text` |
| `content_block_start.content_block.type === "tool_use"` → `{id, name}`（按 `.index`） | 建立该 index 的 ToolCall（id/name） |
| `content_block_delta.delta.type === "input_json_delta"` → `.delta.partial_json`（按 `.index`） | 累加到该 ToolCall 的 arguments 字符串 |
| `content_block_stop`（按 index） | 对该 ToolCall 的 arguments 字符串 `JSON.parse`，失败则 `argumentsParseError` |
| `message_delta.delta.stop_reason` | `usage.stopReason` |
| `message_delta.usage.output_tokens` | `usage.outputTokens`（累计值，直接取最新） |
| `error` 事件 | 记录到 errors |

> tool_use 的 `input` 是**流式分片**的 JSON（`input_json_delta.partial_json`），必须按 `index` 累加，到 `content_block_stop` 再整体 parse。

### 4. OpenAI 归约（`openai.ts`）

逐 chunk 处理 `choices[0]`：

| 字段 | 提取内容 |
|---|---|
| `choices[0].delta.content` | 追加到 `text` |
| `choices[0].delta.tool_calls[]`（按 `.index`） | 首块含 `id`/`function.name` 建立该 index 的 ToolCall；后续 `function.arguments` 增量累加 |
| `choices[0].finish_reason` | `usage.stopReason`（`stop` / `tool_calls` / `length` / ...） |
| `choices[0].delta.role` | （仅识别用，不展示） |
| 末块 `usage` | `usage` 各字段：`prompt_tokens`→input、`completion_tokens`→output、`total_tokens`→total、`prompt_tokens_details.cached_tokens`→cacheRead、`completion_tokens_details.reasoning_tokens`→reasoning |
| `model`（任意块，通常首个或末个） | `usage.model` |

> tool_calls 的 `function.arguments` 是**增量字符串**，必须按 `index` 累加；流结束后整体 `JSON.parse`，失败则 `argumentsParseError`。usage 仅在调用方开启 `stream_options.include_usage` 时出现，缺失时对应字段留空。

### 5. UI（`SseParserClient.tsx`，editorial 风格）

- **输入区**：
  - 大 `<textarea>`（粘贴 SSE；`aria-label`）。
  - 协议选择：默认「自动识别」，可手动切 Anthropic / OpenAI（`<select>`）。
  - 「解析」主按钮、「填入示例」（下拉：Anthropic 文本 / Anthropic tool_use / OpenAI 文本 / OpenAI tool_calls 四份内置示例）、「清空」。
  - 解析触发：点击「解析」或粘贴后自动 debounce（500ms）。
- **结果区 4 个 Tab**（解析成功后展示）：
  1. **完整文本**：`<pre>` 展示拼接文本 + 「复制」按钮。
  2. **Token 用量**：表格展示 input/output/cache read/cache creation/reasoning/total + model + stopReason；缺字段标注「—」。
  3. **工具调用**：每个 ToolCall 一张卡（name、id、格式化高亮的 arguments JSON + 复制）；无则提示「无工具调用」。
  4. **事件时间线**：列表，每行 `#index | event/type | 增量摘要`（文本增量截断显示、tool 增量标记）；解析失败的事件红色标注。
- **错误处理**：整体无法识别协议或无有效事件 → 友好提示 + 手动选择引导；单事件 JSON 损坏 → 不中断，时间线标注。
- 全部状态在组件内，**不发送任何网络请求**。

### 6. SEO

- **工具页 `generateMetadata`**（`page.tsx`，server 端，读 messages 取中英文案）：
  - `title`：含关键词，如「SSE 流式解析工具 | Anthropic / OpenAI 调试 - 阿良家的AI」。
  - `description`：20–40 字中文，覆盖「解析流式响应、SSE、Anthropic、OpenAI、token、工具调用」。
  - `keywords`、`openGraph`（title/desc/url/type=website）、`twitter`（card=summary_large_image）、`alternates.canonical`。
- **JSON-LD**：`page.tsx` 内嵌 `<script type="application/ld+json">`，类型 `WebApplication`（name/applicationCategory/url/operatingSystem=Any/offerings free）。
- **语义化**：整页单个 `<h1>`（工具名 + 核心关键词），`<h2>` 分区（简介 / 使用步骤 / 工具 / FAQ 关键词段落），所有交互元素带 `aria-label`。落地页含一段含关键词的说明文字（兼作 SEO 文案与新手引导）。
- **`sitemap.ts`**（新）：导出主要公开路由：`/`、`/services`、`/tools/sse-parser`、`/download`、`/docs`、`/price`、`/about`、`/blog`（含 `lastModified`、`changeFrequency`、`priority`，工具页 priority 提至 0.8）。
- **`robots.ts`**（新）：允许全部抓取，`Host`、`Sitemap` 指向站点域名（从 `NEXT_PUBLIC_SITE_URL` 或既有 `metadataBase` 推导；无则留可配置项并在文档注明）。
- **全局 `app/layout.tsx` metadata 增强**：新增 `metadataBase`（基于 `NEXT_PUBLIC_SITE_URL`，缺省回退请求 host）、默认 `openGraph`、`keywords`；保留现有全局 title/description。
- **约束**：因 i18n 走 cookie（无 `[locale]` 路由），中英文共享 URL，本次不做 hreflang；默认 `zh` 版本可被索引。

### 7. i18n

- `editorial.tools.sseParser`（zh/en）：hero 标题/副标题、输入区 label 与按钮、四个 Tab 标题与空态、错误提示、SEO 文案段落、示例标签。
- `editorial.services`：新增入口卡标题/描述/按钮（「开发者小工具 / 在线解析 AI 流式响应」）。
- 翻译通过 `useTranslations` / `getTranslations` 消费，与现有一致。

## Verification

- **单元测试**（解析器为纯函数）：项目 `package.json` 现无测试框架，采用 Node 内置 `node --test` + `tsx`（已在依赖树内）执行，零新增运行时依赖。测试目录 `frontend/lib/sse/__tests__/`，覆盖：
  1. OpenAI 纯文本流 → text 正确、finish_reason、usage。
  2. OpenAI tool_calls 流 → arguments 增量拼接 + parse。
  3. OpenAI 含 `[DONE]` 与含损坏 JSON 的事件（不中断）。
  4. Anthropic 纯文本流 → text 正确、stop_reason、output_tokens。
  5. Anthropic tool_use 流 → partial_json 拼接 + parse。
  6. `detect` 对两协议样本与混淆/空样本的判定。
  7. 空输入 / 仅空白 → 合理空结果，不抛异常。
- `npm run build` 通过（含类型检查）。
- 本地 `npm run dev` 手动验证：四份内置示例分别解析，四个 Tab 输出符合预期，复制可用，移动端布局正常。
- `curl /sitemap.xml`、`/robots.txt` 可访问且含 `/tools/sse-parser`。

## Risks & Notes

- **协议字段时效**：Anthropic/OpenAI 可能新增字段（如新的 usage detail）。设计上 `UsageStats.raw` 保留原始对象，归约只提取已知字段，未知字段不致解析失败；实现阶段以官方文档最终核对字段名。
- **示例报文**：内置示例须为脱敏的真实结构，避免泄露真实 token；同时承担 SEO 关键词载体作用，措辞需斟酌。
- **CDN / assetPrefix**：`next.config.ts` 的 `assetPrefix` 仅影响静态资源，不影响路由与 SEO；sitemap/robots 用相对站点域名即可。
- **测试框架**：若团队已有偏好的测试框架（vitest 等），可在实现时替换 `node --test`，spec 不强制。

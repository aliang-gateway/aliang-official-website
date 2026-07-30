# SSE 流式协议解析工具 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/services` 增加入口卡，跳转独立工具页 `/tools/sse-parser`，纯前端解析 Anthropic/OpenAI 流式 SSE 报文，输出完整文本/Token用量/工具调用/事件时间线，并配齐页面级 SEO 与全站 sitemap/robots。

**Architecture:** 工具页 `page.tsx` 为 server component（承载 `generateMetadata` + JSON-LD + 静态语义化文案），内嵌 client component `SseParserClient.tsx` 做交互。SSE 解析逻辑抽成纯函数（`frontend/lib/sse/`），框架无关、可单测。所有解析在浏览器端运行，零网络上报。

**Tech Stack:** Next.js 16 App Router · TypeScript · Tailwind v4（仅靠 `editorial.css` 复用）· next-intl（中/英）· vitest（新增 devDependency，纯函数单测）。

**Spec:** `docs/superpowers/specs/2026-07-31-sse-parser-tool-design.md`

**分支:** 已在 `feat/sse-parser-tool`（spec 已提交 `cb329aa`）。后续每个 Task 末尾 commit 到该分支。

---

## File Structure

### 新增

| 文件 | 职责 |
|---|---|
| `frontend/lib/sse/types.ts` | 统一类型：`Protocol`、`SseEvent`、`ToolCall`、`UsageStats`、`UnifiedResult` |
| `frontend/lib/sse/parseSse.ts` | 纯函数：raw 文本 → `SseEvent[]` |
| `frontend/lib/sse/detect.ts` | 纯函数：`SseEvent[]` → `Protocol \| null` |
| `frontend/lib/sse/openai.ts` | 纯函数：`SseEvent[]` → `UnifiedResult`（OpenAI 归约） |
| `frontend/lib/sse/anthropic.ts` | 纯函数：`SseEvent[]` → `UnifiedResult`（Anthropic 归约） |
| `frontend/lib/sse/index.ts` | barrel：`parseAndReduce(raw, protocol?)` 一站式入口 |
| `frontend/lib/sse/__tests__/*.test.ts` | 纯函数单测 |
| `frontend/app/(marketing)/tools/sse-parser/page.tsx` | server component：metadata + JSON-LD + hero + `<SseParserClient/>` |
| `frontend/app/(marketing)/tools/sse-parser/SseParserClient.tsx` | client component：输入/协议选择/Tab/复制/示例 |
| `frontend/app/sitemap.ts` | 站点地图 |
| `frontend/app/robots.ts` | robots.txt |

### 修改

| 文件 | 改动 |
|---|---|
| `frontend/app/(marketing)/editorial.css` | 追加 `.tool-*` 类（textarea / tabs / 结果卡 / 时间线 / token 表） |
| `frontend/app/(marketing)/services/page.tsx` | timeline 之后追加「开发者小工具」入口卡区块 |
| `frontend/app/layout.tsx` | 全局 metadata 增强（`metadataBase` + 默认 OG + keywords） |
| `frontend/messages/zh.json` / `en.json` | 新增 `editorial.tools.sseParser`；`editorial.services` 增入口卡文案 |
| `frontend/package.json` | 新增 devDependency `vitest`；新增 `test` / `test:watch` 脚本 |

---

## Task 1: 测试基建 + `types.ts` + `parseSse.ts`（TDD）

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/lib/sse/types.ts`
- Create: `frontend/lib/sse/parseSse.ts`
- Create: `frontend/lib/sse/__tests__/parseSse.test.ts`

- [ ] **Step 1: 安装 vitest 并加脚本**

Run（在 `frontend/` 目录下）:
```bash
npm install -D vitest
```
在 `package.json` 的 `scripts` 加：
```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 2: 写 `types.ts`**

Create `frontend/lib/sse/types.ts`:
```ts
export type Protocol = "anthropic" | "openai";

export interface SseEvent {
  index: number;        // 1-based 序号，用于时间线
  event?: string;       // Anthropic 的 `event:` 名；OpenAI 通常无
  raw: string;          // `data:` 原文本（多行以 \n 拼接）
  json: unknown | null; // JSON.parse 结果；失败为 null
  ok: boolean;          // 是否成功 JSON.parse（[DONE] 视为 ok:true）
  isDone?: boolean;     // 是否为 `data: [DONE]`
  error?: string;       // 解析失败原因
}

export interface ToolCall {
  index: number;
  id: string;
  name: string;
  arguments: object | string; // 成功 parse 为 object；失败保留拼接字符串
  argumentsParseError?: string;
}

export interface UsageStats {
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  reasoningTokens?: number;
  model?: string;
  stopReason?: string; // Anthropic stop_reason / OpenAI finish_reason
  raw?: unknown;       // 原始 usage 对象
}

export interface UnifiedResult {
  protocol: Protocol;
  text: string;
  toolCalls: ToolCall[];
  usage: UsageStats;
  events: SseEvent[];
  errors: { index: number; error: string }[];
}
```

- [ ] **Step 3: 写失败测试**

Create `frontend/lib/sse/__tests__/parseSse.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { parseSse } from "../parseSse";

describe("parseSse", () => {
  it("解析 OpenAI 风格 data 块为有序事件", () => {
    const raw = [
      `data: {"choices":[{"delta":{"content":"Hi"}}]}`,
      ``,
      `data: {"choices":[{"delta":{"content":" there"}}]}`,
      ``,
      `data: [DONE]`,
      ``,
    ].join("\n");
    const ev = parseSse(raw);
    expect(ev).toHaveLength(3);
    expect(ev[0].index).toBe(1);
    expect(ev[0].ok).toBe(true);
    expect((ev[0].json as any).choices[0].delta.content).toBe("Hi");
    expect(ev[2].isDone).toBe(true);
  });

  it("保留 Anthropic 的 event 名", () => {
    const raw = `event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"X"}}\n\n`;
    const ev = parseSse(raw);
    expect(ev[0].event).toBe("content_block_delta");
  });

  it("损坏的 JSON 不中断，记录 error", () => {
    const raw = `data: {not json}\n\ndata: {"ok":true}\n\n`;
    const ev = parseSse(raw);
    expect(ev).toHaveLength(2);
    expect(ev[0].ok).toBe(false);
    expect(ev[0].error).toBeTruthy();
    expect(ev[1].ok).toBe(true);
  });

  it("空输入返回空数组", () => {
    expect(parseSse("")).toEqual([]);
    expect(parseSse("   \n\n  ")).toEqual([]);
  });
});
```

- [ ] **Step 4: 运行测试，确认失败**

Run: `npm test -- lib/sse/__tests__/parseSse.test.ts`
Expected: FAIL（`parseSse` 未实现 / 模块找不到）

- [ ] **Step 5: 实现 `parseSse.ts`**

Create `frontend/lib/sse/parseSse.ts`:
```ts
import type { SseEvent } from "./types";

export function parseSse(raw: string): SseEvent[] {
  const events: SseEvent[] = [];
  if (!raw || !raw.trim()) return events;

  const normalized = raw.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const chunks = normalized.split(/\n\n+/);

  let index = 0;
  for (const chunk of chunks) {
    if (!chunk.trim()) continue;
    let event: string | undefined;
    const dataLines: string[] = [];
    for (const line of chunk.split("\n")) {
      if (line.startsWith("event:")) {
        event = line.slice("event:".length).trim();
      } else if (line.startsWith("data:")) {
        dataLines.push(line.slice("data:".length).replace(/^ /, ""));
      }
    }
    if (dataLines.length === 0) continue;
    const dataStr = dataLines.join("\n");
    index += 1;
    if (dataStr.trim() === "[DONE]") {
      events.push({ index, event, raw: dataStr, json: null, ok: true, isDone: true });
      continue;
    }
    try {
      const json = JSON.parse(dataStr);
      events.push({ index, event, raw: dataStr, json, ok: true });
    } catch (e) {
      events.push({
        index,
        event,
        raw: dataStr,
        json: null,
        ok: false,
        error: e instanceof Error ? e.message : "parse error",
      });
    }
  }
  return events;
}
```

- [ ] **Step 6: 运行测试，确认通过**

Run: `npm test -- lib/sse/__tests__/parseSse.test.ts`
Expected: PASS（4 passed）

- [ ] **Step 7: Commit**
```bash
git add frontend/package.json frontend/package-lock.json frontend/lib/sse/types.ts frontend/lib/sse/parseSse.ts frontend/lib/sse/__tests__/parseSse.test.ts
git commit -m "feat(sse): 新增 SSE 通用解析器与测试基建"
```

---

## Task 2: `detect.ts`（TDD）

**Files:**
- Create: `frontend/lib/sse/detect.ts`
- Test: `frontend/lib/sse/__tests__/detect.test.ts`

- [ ] **Step 1: 写失败测试**

Create `frontend/lib/sse/__tests__/detect.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { parseSse } from "../parseSse";
import { detectProtocol } from "../detect";

describe("detectProtocol", () => {
  it("识别 Anthropic（有 event 行）", () => {
    const raw = `event: message_start\ndata: {"type":"message_start","message":{}}\n\n`;
    expect(detectProtocol(parseSse(raw))).toBe("anthropic");
  });
  it("识别 OpenAI（chat.completion.chunk）", () => {
    const raw = `data: {"object":"chat.completion.chunk","choices":[{"delta":{"content":"Hi"}}]}\n\n`;
    expect(detectProtocol(parseSse(raw))).toBe("openai");
  });
  it("无法识别返回 null", () => {
    const raw = `data: {"hello":"world"}\n\n`;
    expect(detectProtocol(parseSse(raw))).toBeNull();
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npm test -- lib/sse/__tests__/detect.test.ts` → Expected: FAIL

- [ ] **Step 3: 实现 `detect.ts`**

Create `frontend/lib/sse/detect.ts`:
```ts
import type { Protocol, SseEvent } from "./types";

const ANTHROPIC_TYPES = new Set([
  "message_start",
  "content_block_delta",
  "message_delta",
  "message_stop",
]);

export function detectProtocol(events: SseEvent[]): Protocol | null {
  let anthropic = false;
  let openai = false;
  for (const ev of events) {
    if (ev.event) anthropic = true;
    const j = (ev.ok && ev.json && typeof ev.json === "object" ? ev.json : null) as
      | Record<string, unknown>
      | null;
    if (j) {
      if (typeof j.type === "string" && ANTHROPIC_TYPES.has(j.type)) anthropic = true;
      if (j.object === "chat.completion.chunk" || Array.isArray(j.choices)) openai = true;
    }
  }
  if (anthropic && !openai) return "anthropic";
  if (openai && !anthropic) return "openai";
  return null;
}
```

- [ ] **Step 4: 运行确认通过**

Run: `npm test -- lib/sse/__tests__/detect.test.ts` → Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add frontend/lib/sse/detect.ts frontend/lib/sse/__tests__/detect.test.ts
git commit -m "feat(sse): 新增协议自动识别"
```

---

## Task 3: `openai.ts`（TDD，含 tool_calls + usage）

**Files:**
- Create: `frontend/lib/sse/openai.ts`
- Create: `frontend/lib/sse/__tests__/openai.test.ts`
- Create: `frontend/lib/sse/internal.ts`（共享 `finalizeToolCalls`）

- [ ] **Step 1: 写共享 helper**

Create `frontend/lib/sse/internal.ts`:
```ts
import type { ToolCall } from "./types";

/** 把按 index 累加好的 arguments 字符串整体 JSON.parse；失败保留字符串并记 error。 */
export function finalizeToolCalls(map: Map<number, ToolCall>): ToolCall[] {
  return Array.from(map.values())
    .sort((a, b) => a.index - b.index)
    .map((tc) => {
      const argsStr = typeof tc.arguments === "string" ? tc.arguments : "";
      if (argsStr === "") return { ...tc, arguments: {} };
      try {
        return { ...tc, arguments: JSON.parse(argsStr) as object };
      } catch (e) {
        return {
          ...tc,
          arguments: argsStr,
          argumentsParseError: e instanceof Error ? e.message : "parse error",
        };
      }
    });
}
```

- [ ] **Step 2: 写失败测试**

Create `frontend/lib/sse/__tests__/openai.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { parseSse } from "../parseSse";
import { reduceOpenai } from "../openai";

describe("reduceOpenai", () => {
  it("拼接文本并取 finish_reason", () => {
    const raw = [
      `data: {"model":"gpt-4o","choices":[{"delta":{"content":"Hello"}}]}`,
      `data: {"choices":[{"delta":{"content":" world"}}]}`,
      `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
      `data: [DONE]`,
    ].join("\n\n");
    const r = reduceOpenai(parseSse(raw));
    expect(r.protocol).toBe("openai");
    expect(r.text).toBe("Hello world");
    expect(r.usage.model).toBe("gpt-4o");
    expect(r.usage.stopReason).toBe("stop");
  });

  it("增量拼接 tool_calls 参数并 parse", () => {
    const raw = [
      `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
      `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\\"city\\""}}]}}]}`,
      `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\\"BJ\\"}"}}]}}]}`,
      `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
    ].join("\n\n");
    const r = reduceOpenai(parseSse(raw));
    expect(r.toolCalls).toHaveLength(1);
    expect(r.toolCalls[0].name).toBe("get_weather");
    expect(r.toolCalls[0].id).toBe("call_1");
    expect(r.toolCalls[0].arguments).toEqual({ city: "BJ" });
    expect(r.usage.stopReason).toBe("tool_calls");
  });

  it("解析末块 usage", () => {
    const raw = [
      `data: {"choices":[{"delta":{"content":"hi"}}]}`,
      `data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":1}}}`,
    ].join("\n\n");
    const r = reduceOpenai(parseSse(raw));
    expect(r.usage.inputTokens).toBe(10);
    expect(r.usage.outputTokens).toBe(5);
    expect(r.usage.totalTokens).toBe(15);
    expect(r.usage.cacheReadTokens).toBe(3);
    expect(r.usage.reasoningTokens).toBe(1);
  });
});
```

- [ ] **Step 3: 运行确认失败** → Run: `npm test -- lib/sse/__tests__/openai.test.ts` → FAIL

- [ ] **Step 4: 实现 `openai.ts`**

Create `frontend/lib/sse/openai.ts`:
```ts
import type { SseEvent, ToolCall, UnifiedResult, UsageStats } from "./types";
import { finalizeToolCalls } from "./internal";

export function reduceOpenai(events: SseEvent[]): UnifiedResult {
  let text = "";
  const toolMap = new Map<number, ToolCall>();
  const usage: UsageStats = {};
  const errors: { index: number; error: string }[] = [];

  for (const ev of events) {
    if (!ev.ok) {
      if (ev.error) errors.push({ index: ev.index, error: ev.error });
      continue;
    }
    const j = ev.json as Record<string, unknown> | null;
    if (!j || typeof j !== "object") continue;

    if (typeof j.model === "string" && !usage.model) usage.model = j.model;

    const choices = Array.isArray(j.choices) ? j.choices : [];
    for (const choice of choices) {
      const c = choice as Record<string, any>;
      const delta = c?.delta;
      if (delta && typeof delta === "object") {
        if (typeof delta.content === "string") text += delta.content;
        if (Array.isArray(delta.tool_calls)) {
          for (const tc of delta.tool_calls) {
            const idx = typeof tc.index === "number" ? tc.index : 0;
            let entry = toolMap.get(idx);
            if (!entry) {
              entry = { index: idx, id: "", name: "", arguments: "" };
              toolMap.set(idx, entry);
            }
            if (typeof tc.id === "string") entry.id = tc.id;
            const fn = tc.function;
            if (fn) {
              if (typeof fn.name === "string" && !entry.name) entry.name = fn.name;
              if (typeof fn.arguments === "string") entry.arguments += fn.arguments;
            }
          }
        }
      }
      if (typeof c?.finish_reason === "string" && c.finish_reason) {
        usage.stopReason = c.finish_reason;
      }
    }

    if (j.usage && typeof j.usage === "object") {
      const u = j.usage as Record<string, any>;
      if (typeof u.prompt_tokens === "number") usage.inputTokens = u.prompt_tokens;
      if (typeof u.completion_tokens === "number") usage.outputTokens = u.completion_tokens;
      if (typeof u.total_tokens === "number") usage.totalTokens = u.total_tokens;
      const pd = u.prompt_tokens_details;
      if (pd && typeof pd.cached_tokens === "number") usage.cacheReadTokens = pd.cached_tokens;
      const cd = u.completion_tokens_details;
      if (cd && typeof cd.reasoning_tokens === "number") usage.reasoningTokens = cd.reasoning_tokens;
      usage.raw = u;
    }
  }

  return { protocol: "openai", text, toolCalls: finalizeToolCalls(toolMap), usage, events, errors };
}
```

- [ ] **Step 5: 运行确认通过** → Run: `npm test -- lib/sse/__tests__/openai.test.ts` → PASS

- [ ] **Step 6: Commit**
```bash
git add frontend/lib/sse/openai.ts frontend/lib/sse/internal.ts frontend/lib/sse/__tests__/openai.test.ts
git commit -m "feat(sse): 新增 OpenAI 流式归约"
```

---

## Task 4: `anthropic.ts`（TDD，含 tool_use + usage）

**Files:**
- Create: `frontend/lib/sse/anthropic.ts`
- Test: `frontend/lib/sse/__tests__/anthropic.test.ts`

> 字段以 spec「Anthropic 归约」表为准；实现后用官方文档核对字段名（见 spec Risks）。

- [ ] **Step 1: 写失败测试**

Create `frontend/lib/sse/__tests__/anthropic.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { parseSse } from "../parseSse";
import { reduceAnthropic } from "../anthropic";

describe("reduceAnthropic", () => {
  it("拼接 text_delta 并取 stop_reason + output_tokens", () => {
    const raw = [
      `event: message_start\ndata: {"type":"message_start","message":{"model":"claude-sonnet","usage":{"input_tokens":12,"cache_read_input_tokens":2}}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}`,
      `event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
      `event: message_stop\ndata: {"type":"message_stop"}`,
    ].join("\n\n");
    const r = reduceAnthropic(parseSse(raw));
    expect(r.protocol).toBe("anthropic");
    expect(r.text).toBe("Hi!");
    expect(r.usage.model).toBe("claude-sonnet");
    expect(r.usage.inputTokens).toBe(12);
    expect(r.usage.cacheReadTokens).toBe(2);
    expect(r.usage.outputTokens).toBe(7);
    expect(r.usage.stopReason).toBe("end_turn");
  });

  it("按 index 拼接 tool_use 的 input_json_delta 并 parse", () => {
    const raw = [
      `event: content_block_start\ndata: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\\"city\":"}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"BJ\"}"}}`,
      `event: content_block_stop\ndata: {"type":"content_block_stop","index":1}`,
    ].join("\n\n");
    const r = reduceAnthropic(parseSse(raw));
    expect(r.toolCalls).toHaveLength(1);
    expect(r.toolCalls[0].name).toBe("get_weather");
    expect(r.toolCalls[0].id).toBe("toolu_1");
    expect(r.toolCalls[0].arguments).toEqual({ city: "BJ" });
  });
});
```

- [ ] **Step 2: 运行确认失败** → Run: `npm test -- lib/sse/__tests__/anthropic.test.ts` → FAIL

- [ ] **Step 3: 实现 `anthropic.ts`**

Create `frontend/lib/sse/anthropic.ts`:
```ts
import type { SseEvent, ToolCall, UnifiedResult, UsageStats } from "./types";

interface ToolAgg {
  index: number;
  id: string;
  name: string;
  jsonBuf: string;
  stopped: boolean;
}

export function reduceAnthropic(events: SseEvent[]): UnifiedResult {
  let text = "";
  const agg = new Map<number, ToolAgg>();
  const usage: UsageStats = {};
  const errors: { index: number; error: string }[] = [];

  const ensure = (index: number): ToolAgg => {
    let e = agg.get(index);
    if (!e) {
      e = { index, id: "", name: "", jsonBuf: "", stopped: false };
      agg.set(index, e);
    }
    return e;
  };

  for (const ev of events) {
    if (!ev.ok) {
      if (ev.error) errors.push({ index: ev.index, error: ev.error });
      continue;
    }
    const j = ev.json as Record<string, any> | null;
    if (!j || typeof j !== "object") continue;
    const type = j.type;

    if (type === "message_start" && j.message) {
      const m = j.message;
      if (typeof m.model === "string") usage.model = m.model;
      const u = m.usage;
      if (u && typeof u === "object") {
        if (typeof u.input_tokens === "number") usage.inputTokens = u.input_tokens;
        if (typeof u.cache_read_input_tokens === "number") usage.cacheReadTokens = u.cache_read_input_tokens;
        if (typeof u.cache_creation_input_tokens === "number") usage.cacheCreationTokens = u.cache_creation_input_tokens;
        usage.raw = u;
      }
    } else if (type === "content_block_start" && j.content_block?.type === "tool_use") {
      const a = ensure(j.index ?? 0);
      a.id = j.content_block.id ?? a.id;
      a.name = j.content_block.name ?? a.name;
    } else if (type === "content_block_delta") {
      const d = j.delta;
      if (d?.type === "text_delta" && typeof d.text === "string") {
        text += d.text;
      } else if (d?.type === "input_json_delta" && typeof d.partial_json === "string") {
        ensure(j.index ?? 0).jsonBuf += d.partial_json;
      }
    } else if (type === "content_block_stop") {
      const a = agg.get(j.index ?? 0);
      if (a) a.stopped = true;
    } else if (type === "message_delta") {
      if (j.delta && typeof j.delta.stop_reason === "string") usage.stopReason = j.delta.stop_reason;
      const u = j.usage;
      if (u && typeof u.output_tokens === "number") usage.outputTokens = u.output_tokens;
    } else if (type === "error") {
      errors.push({ index: ev.index, error: j.error?.message ?? "stream error" });
    }
  }

  const toolCalls: ToolCall[] = Array.from(agg.values())
    .filter((a) => a.name || a.id || a.jsonBuf)
    .sort((a, b) => a.index - b.index)
    .map((a) => {
      if (a.jsonBuf === "") return { index: a.index, id: a.id, name: a.name, arguments: {} };
      try {
        return { index: a.index, id: a.id, name: a.name, arguments: JSON.parse(a.jsonBuf) as object };
      } catch (e) {
        return {
          index: a.index,
          id: a.id,
          name: a.name,
          arguments: a.jsonBuf,
          argumentsParseError: e instanceof Error ? e.message : "parse error",
        };
      }
    });

  return { protocol: "anthropic", text, toolCalls, usage, events, errors };
}
```

- [ ] **Step 4: 运行确认通过** → Run: `npm test -- lib/sse/__tests__/anthropic.test.ts` → PASS

- [ ] **Step 5: 全量跑一次 lib/sse 测试** → Run: `npm test -- lib/sse` → 全部 PASS

- [ ] **Step 6: Commit**
```bash
git add frontend/lib/sse/anthropic.ts frontend/lib/sse/__tests__/anthropic.test.ts
git commit -m "feat(sse): 新增 Anthropic 流式归约"
```

---

## Task 5: `index.ts` 一站式入口 + 示例报文

**Files:**
- Create: `frontend/lib/sse/index.ts`
- Create: `frontend/lib/sse/samples.ts`

- [ ] **Step 1: 写 `index.ts`**

Create `frontend/lib/sse/index.ts`:
```ts
import type { Protocol, UnifiedResult } from "./types";
import { parseSse } from "./parseSse";
import { detectProtocol } from "./detect";
import { reduceOpenai } from "./openai";
import { reduceAnthropic } from "./anthropic";

export function parseAndReduce(
  raw: string,
  forced?: Protocol | null,
): { protocol: Protocol | null; result: UnifiedResult | null; events: ReturnType<typeof parseSse> } {
  const events = parseSse(raw);
  if (events.length === 0) return { protocol: null, result: null, events };
  const protocol = forced ?? detectProtocol(events);
  if (!protocol) return { protocol: null, result: null, events };
  const result = protocol === "openai" ? reduceOpenai(events) : reduceAnthropic(events);
  return { protocol, result, events };
}

export * from "./types";
```

- [ ] **Step 2: 写示例报文（脱敏，兼作 SEO 关键词载体）**

Create `frontend/lib/sse/samples.ts`:
```ts
export type SampleKey = "openai-text" | "openai-tool" | "anthropic-text" | "anthropic-tool";

export const SAMPLES: Record<SampleKey, { labelZh: string; labelEn: string; raw: string }> = {
  "openai-text": {
    labelZh: "OpenAI 文本流",
    labelEn: "OpenAI text stream",
    raw: [
      `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
      `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
      `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
      `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":2,"total_tokens":11}}`,
      `data: [DONE]`,
    ].join("\n\n"),
  },
  "openai-tool": {
    labelZh: "OpenAI 工具调用",
    labelEn: "OpenAI tool calls",
    raw: [
      `data: {"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
      `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\\"city\\":"}}]}}]}`,
      `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\\"Beijing\\"}"}}]}}]}`,
      `data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
      `data: [DONE]`,
    ].join("\n\n"),
  },
  "anthropic-text": {
    labelZh: "Anthropic 文本流",
    labelEn: "Anthropic text stream",
    raw: [
      `event: message_start\ndata: {"type":"message_start","message":{"id":"msg_x","model":"claude-sonnet","role":"assistant","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`,
      `event: content_block_start\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}`,
      `event: content_block_stop\ndata: {"type":"content_block_stop","index":0}`,
      `event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":4}}`,
      `event: message_stop\ndata: {"type":"message_stop"}`,
    ].join("\n\n"),
  },
  "anthropic-tool": {
    labelZh: "Anthropic 工具调用",
    labelEn: "Anthropic tool use",
    raw: [
      `event: content_block_start\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\\"city\\":"}}`,
      `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\\"Beijing\\"}"}}`,
      `event: content_block_stop\ndata: {"type":"content_block_stop","index":0}`,
      `event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
      `event: message_stop\ndata: {"type":"message_stop"}`,
    ].join("\n\n"),
  },
};
```

- [ ] **Step 3: Commit**
```bash
git add frontend/lib/sse/index.ts frontend/lib/sse/samples.ts
git commit -m "feat(sse): 新增一站式入口与示例报文"
```

---

## Task 6: i18n 文案

**Files:**
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

> 遵循现有结构：`editorial.tools.sseParser`（工具页）+ `editorial.services`（入口卡）。中英文都加。

- [ ] **Step 1: 在 `zh.json` 的 `editorial` 对象内新增 `tools.sseParser`，并在 `editorial.services` 内追加入口卡字段**

在 `editorial.services` 内追加（与现有字段同级）：
```json
"toolCardLabel": "开发者小工具",
"toolCardTitle": "SSE 流式响应解析器",
"toolCardDesc": "在线解析 Anthropic / OpenAI 的流式响应报文，提取完整文本、Token 用量、工具调用与事件时间线。纯浏览器端运行，不上传任何数据。",
"toolCardBtn": "打开工具",
```

在 `editorial.tools` 下新增（若 `tools` 不存在则创建）：
```json
"tools": {
  "sseParser": {
    "heroLabel": "开发者工具",
    "heroTitle": "SSE 流式响应解析器",
    "heroLead": "把 Anthropic Messages 或 OpenAI Chat Completions 的流式响应（SSE）原始报文粘贴进来，自动识别协议并提取完整文本、Token 用量、工具调用与事件时间线。所有解析在浏览器本地完成，不上传、不存储。",
    "seoIntro": "这是一个在线的 AI 流式响应调试工具，支持解析 Anthropic Claude 与 OpenAI ChatGPT 的 Server-Sent Events（SSE）流式协议，常用于排查接口对接、token 计费、工具调用（function calling / tool use）等问题。",
    "inputLabel": "粘贴原始 SSE 报文",
    "inputPlaceholder": "data: {\"choices\":[...]}\n\ndata: {\"choices\":[...]}\n\ndata: [DONE]",
    "protocolAuto": "自动识别",
    "protocolAnthropic": "Anthropic",
    "protocolOpenai": "OpenAI",
    "protocolLabel": "协议",
    "parseBtn": "解析",
    "clearBtn": "清空",
    "sampleLabel": "填入示例",
    "tabText": "完整文本",
    "tabUsage": "Token 用量",
    "tabTools": "工具调用",
    "tabTimeline": "事件时间线",
    "copyBtn": "复制",
    "copied": "已复制",
    "noText": "无文本内容",
    "noTools": "无工具调用",
    "noUsage": "未包含用量信息（OpenAI 需开启 stream_options.include_usage）",
    "usageModel": "模型",
    "usageInput": "输入 tokens",
    "usageOutput": "输出 tokens",
    "usageTotal": "合计",
    "usageCacheRead": "缓存命中",
    "usageCacheCreate": "缓存写入",
    "usageReasoning": "推理 tokens",
    "usageStopReason": "结束原因",
    "timelineIndex": "#",
    "timelineType": "类型",
    "timelineDelta": "增量",
    "unrecognized": "未能自动识别协议，请手动选择 Anthropic 或 OpenAI。",
    "empty": "粘贴报文后点击「解析」查看结果。",
    "argsHint": "参数解析失败，已保留原始字符串",
    "backToServices": "返回服务页"
  }
}
```

- [ ] **Step 2: 在 `en.json` 增加同样结构的英文文案**（key 完全一致，value 为英文）。`editorial.services` 追加：
```json
"toolCardLabel": "Dev tool",
"toolCardTitle": "SSE Stream Parser",
"toolCardDesc": "Parse raw streaming responses (SSE) from Anthropic / OpenAI in your browser. Extracts full text, token usage, tool calls and the event timeline. Nothing is uploaded.",
"toolCardBtn": "Open tool",
```
`editorial.tools.sseParser` 英文版（同结构，value 译为英文，如 `heroTitle: "SSE Stream Parser"`, `parseBtn: "Parse"`, `empty: "Paste a payload and hit Parse."` 等）。

- [ ] **Step 3: 校验 JSON 合法**

Run:
```bash
node -e "JSON.parse(require('fs').readFileSync('frontend/messages/zh.json','utf8')); JSON.parse(require('fs').readFileSync('frontend/messages/en.json','utf8')); console.log('OK')"
```
Expected: `OK`

- [ ] **Step 4: Commit**
```bash
git add frontend/messages/zh.json frontend/messages/en.json
git commit -m "i18n(sse-parser): 新增工具页与入口卡中英文案"
```

---

## Task 7: `editorial.css` 追加 `.tool-*` 样式

**Files:**
- Modify: `frontend/app/(marketing)/editorial.css`（文件末尾追加）

> 复用现有变量 `--ink / --ink-soft / --ink-muted / --paper / --paper-warm / --line / --line-soft / --accent / --accent-soft / --mono / --shadow / --focus-ring`；复用现有类 `.container / .btn / .btn.primary / .label / .display / .lead / .dot`。仅在末尾追加工具页专属类。

- [ ] **Step 1: 在 `editorial.css` 末尾追加**

```css
/* ===== SSE Parser tool ===== */
.tool-page { padding: 64px 0 96px; }
.tool-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr); gap: 32px; }
@media (max-width: 960px) { .tool-layout { grid-template-columns: 1fr; } }

.tool-input { display: flex; flex-direction: column; gap: 14px; }
.tool-input textarea {
  width: 100%; min-height: 320px; resize: vertical;
  font-family: var(--mono); font-size: 13px; line-height: 1.6;
  padding: 16px; border: 1px solid var(--line); border-radius: 12px;
  background: var(--paper-warm); color: var(--ink);
}
.tool-input textarea:focus-visible { outline: 3px solid var(--focus-ring); outline-offset: 2px; }
.tool-controls { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; }
.tool-controls select {
  font: inherit; padding: 10px 12px; border: 1px solid var(--line); border-radius: 10px;
  background: var(--paper); color: var(--ink);
}

.tool-result { min-width: 0; }
.tool-tabs { display: flex; gap: 8px; flex-wrap: wrap; border-bottom: 1px solid var(--line); margin-bottom: 20px; }
.tool-tabs button {
  padding: 10px 14px; border: 0; background: transparent; cursor: pointer;
  font-weight: 600; color: var(--ink-muted); border-bottom: 3px solid transparent;
}
.tool-tabs button[aria-selected="true"] { color: var(--accent); border-bottom-color: var(--accent); }

.tool-panel { background: var(--paper-warm); border: 1px solid var(--line-soft); border-radius: 14px; padding: 20px; }
.tool-panel pre { white-space: pre-wrap; word-break: break-word; font-family: var(--mono); font-size: 13.5px; line-height: 1.6; color: var(--ink); }
.tool-copy { margin-top: 12px; }

.tool-usage { width: 100%; border-collapse: collapse; font-size: 14px; }
.tool-usage th, .tool-usage td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--line-soft); }
.tool-usage th { color: var(--ink-muted); font-weight: 600; }
.tool-usage td.num { font-family: var(--mono); text-align: right; }

.tool-call { border: 1px solid var(--line); border-radius: 12px; padding: 14px 16px; margin-bottom: 12px; background: var(--paper); }
.tool-call-head { display: flex; justify-content: space-between; gap: 12px; align-items: baseline; margin-bottom: 8px; }
.tool-call-name { font-family: var(--mono); font-weight: 700; color: var(--accent-ink); }
.tool-call-id { font-family: var(--mono); font-size: 12px; color: var(--ink-muted); }
.tool-call pre { background: var(--paper-warm); border-radius: 8px; padding: 10px 12px; font-size: 12.5px; }

.tool-timeline { font-family: var(--mono); font-size: 12.5px; }
.tool-timeline-row { display: grid; grid-template-columns: 40px 150px 1fr; gap: 10px; padding: 6px 0; border-bottom: 1px dashed var(--line-soft); }
.tool-timeline-row.is-error { color: var(--warn); }
.tool-timeline-row .idx { color: var(--ink-muted); }
.tool-timeline-row .delta { color: var(--ink-soft); word-break: break-word; white-space: pre-wrap; }

.tool-empty { color: var(--ink-muted); padding: 24px; text-align: center; }
.tool-note { font-size: 12.5px; color: var(--warn); margin-top: 8px; }
.tool-error-banner { background: rgba(164,111,22,0.12); color: var(--warn); border-radius: 10px; padding: 12px 14px; margin-bottom: 16px; }

/* services 入口卡 */
.tool-card { display: flex; align-items: center; justify-content: space-between; gap: 24px; flex-wrap: wrap; padding: 28px 32px; border: 1px solid var(--line); border-radius: 18px; background: var(--paper-warm); }
.tool-card-body { max-width: 640px; }
.tool-card-body h3 { font-size: 22px; margin-bottom: 6px; }
.tool-card-body p { color: var(--ink-soft); }
```

- [ ] **Step 2: Commit**
```bash
git add "frontend/app/(marketing)/editorial.css"
git commit -m "style(sse-parser): 追加工具页与入口卡样式"
```

---

## Task 8: `SseParserClient.tsx`（client component）

**Files:**
- Create: `frontend/app/(marketing)/tools/sse-parser/SseParserClient.tsx`

- [ ] **Step 1: 写组件**

Create `frontend/app/(marketing)/tools/sse-parser/SseParserClient.tsx`:
```tsx
"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { parseAndReduce } from "@/lib/sse";
import { SAMPLES, type SampleKey } from "@/lib/sse/samples";
import type { Protocol } from "@/lib/sse";

type Tab = "text" | "usage" | "tools" | "timeline";

export function SseParserClient() {
  const t = useTranslations("editorial.tools.sseParser");
  const [raw, setRaw] = useState("");
  const [forced, setForced] = useState<"auto" | Protocol>("auto");
  const [tab, setTab] = useState<Tab>("text");
  const [copied, setCopied] = useState(false);

  const parsed = useMemo(() => {
    if (!raw.trim()) return null;
    return parseAndReduce(raw, forced === "auto" ? null : forced);
  }, [raw, forced]);

  const result = parsed?.result ?? null;
  const recognized = parsed?.protocol ?? null;

  const onCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  };

  const tabs: { key: Tab; label: string }[] = [
    { key: "text", label: t("tabText") },
    { key: "usage", label: t("tabUsage") },
    { key: "tools", label: t("tabTools") },
    { key: "timeline", label: t("tabTimeline") },
  ];

  const usageRows = result
    ? [
        { k: "usageModel", v: result.usage.model },
        { k: "usageInput", v: result.usage.inputTokens },
        { k: "usageOutput", v: result.usage.outputTokens },
        { k: "usageTotal", v: result.usage.totalTokens },
        { k: "usageCacheRead", v: result.usage.cacheReadTokens },
        { k: "usageCacheCreate", v: result.usage.cacheCreationTokens },
        { k: "usageReasoning", v: result.usage.reasoningTokens },
        { k: "usageStopReason", v: result.usage.stopReason },
      ]
    : [];

  return (
    <div className="tool-layout">
      <div className="tool-input">
        <label className="label" htmlFor="sse-raw">{t("inputLabel")}</label>
        <textarea
          id="sse-raw"
          className="font-mono"
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          placeholder={t("inputPlaceholder")}
          spellCheck={false}
        />
        <div className="tool-controls">
          <label className="label" htmlFor="sse-proto">{t("protocolLabel')}</label>
          <select
            id="sse-proto"
            value={forced}
            onChange={(e) => setForced(e.target.value as "auto" | Protocol)}
          >
            <option value="auto">{t("protocolAuto")}</option>
            <option value="anthropic">{t("protocolAnthropic")}</option>
            <option value="openai">{t("protocolOpenai")}</option>
          </select>
          <select
            aria-label={t("sampleLabel")}
            value=""
            onChange={(e) => {
              const key = e.target.value as SampleKey;
              if (key) setRaw(SAMPLES[key].raw);
            }}
          >
            <option value="">{t("sampleLabel")}</option>
            {(Object.keys(SAMPLES) as SampleKey[]).map((k) => (
              <option key={k} value={k}>{SAMPLES[k].labelZh}</option>
            ))}
          </select>
          <button type="button" className="btn" onClick={() => onCopy(raw)}>{t("copyBtn")}</button>
          <button type="button" className="btn" onClick={() => { setRaw(""); }}>{t("clearBtn")}</button>
        </div>
        <p className="tool-note" aria-hidden={copied ? undefined : true}>{copied ? t("copied") : ""}</p>
      </div>

      <div className="tool-result">
        {raw.trim() && !recognized && (
          <div className="tool-error-banner">{t("unrecognized")}</div>
        )}

        {!result ? (
          <div className="tool-panel tool-empty">{raw.trim() ? "" : t("empty")}</div>
        ) : (
          <>
            <div className="tool-tabs" role="tablist">
              {tabs.map((tb) => (
                <button
                  key={tb.key}
                  role="tab"
                  aria-selected={tab === tb.key}
                  onClick={() => setTab(tb.key)}
                >
                  {tb.label}
                </button>
              ))}
            </div>

            {tab === "text" && (
              <div className="tool-panel">
                {result.text ? <pre>{result.text}</pre> : <p className="tool-empty">{t("noText")}</p>}
                <div className="tool-copy">
                  <button type="button" className="btn" onClick={() => onCopy(result.text)}>{t("copyBtn")}</button>
                </div>
              </div>
            )}

            {tab === "usage" && (
              <div className="tool-panel">
                {result.usage.raw || usageRows.some((r) => r.v !== undefined && r.v !== "") ? (
                  <table className="tool-usage">
                    <tbody>
                      {usageRows.map((r) => (
                        <tr key={r.k}>
                          <th>{t(r.k as any)}</th>
                          <td className={typeof r.v === "number" ? "num" : ""}>
                            {r.v === undefined || r.v === "" ? "—" : String(r.v)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <p className="tool-empty">{t("noUsage")}</p>
                )}
              </div>
            )}

            {tab === "tools" && (
              <div className="tool-panel">
                {result.toolCalls.length === 0 ? (
                  <p className="tool-empty">{t("noTools")}</p>
                ) : (
                  result.toolCalls.map((tc) => (
                    <div className="tool-call" key={tc.index}>
                      <div className="tool-call-head">
                        <span className="tool-call-name">{tc.name || "—"}</span>
                        <span className="tool-call-id">{tc.id}</span>
                      </div>
                      <pre>{JSON.stringify(tc.arguments, null, 2)}</pre>
                      {tc.argumentsParseError && <p className="tool-note">{t("argsHint")}</p>}
                    </div>
                  ))
                )}
              </div>
            )}

            {tab === "timeline" && (
              <div className="tool-panel tool-timeline">
                {parsed!.events.map((ev) => (
                  <div className={`tool-timeline-row${ev.ok ? "" : " is-error"}`} key={ev.index}>
                    <span className="idx">#{ev.index}</span>
                    <span>{ev.event || (ev.json as any)?.type || (ev.isDone ? "[DONE]" : "—")}</span>
                    <span className="delta">{ev.isDone ? "" : ev.error ?? ev.raw.slice(0, 120)}</span>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 类型检查**

Run: `npx tsc --noEmit -p frontend/tsconfig.json`（或直接 build 时一并校验）→ 无新增类型错误

- [ ] **Step 3: Commit**
```bash
git add "frontend/app/(marketing)/tools/sse-parser/SseParserClient.tsx"
git commit -m "feat(sse-parser): 新增解析器客户端交互组件"
```

---

## Task 9: 工具页 `page.tsx`（server component: metadata + JSON-LD + SEO 文案）

**Files:**
- Create: `frontend/app/(marketing)/tools/sse-parser/page.tsx`

- [ ] **Step 1: 写页面**

Create `frontend/app/(marketing)/tools/sse-parser/page.tsx`:
```tsx
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
```

- [ ] **Step 2: Commit**
```bash
git add "frontend/app/(marketing)/tools/sse-parser/page.tsx"
git commit -m "feat(sse-parser): 新增工具页(SEO metadata + JSON-LD)"
```

---

## Task 10: `/services` 入口卡

**Files:**
- Modify: `frontend/app/(marketing)/services/page.tsx`

> 在现有 `<section className="closing">` **之前**插入一个新 section。复用页面已有的 `s`（`editorial.services`）翻译。

- [ ] **Step 1: 在 `services/page.tsx` 的 `</main>` 之后、`<section className="closing">` 之前插入**

```tsx
<section className="container" style={{ padding: "48px 0" }} aria-labelledby="tool-card-title">
  <div className="tool-card">
    <div className="tool-card-body">
      <div className="label">{s("toolCardLabel")}</div>
      <h3 id="tool-card-title">{s("toolCardTitle")}</h3>
      <p>{s("toolCardDesc")}</p>
    </div>
    <Link className="btn primary" href="/tools/sse-parser">{s("toolCardBtn")}</Link>
  </div>
</section>
```
（`Link` 已在文件顶部 import；若未 import 则补 `import Link from "next/link";`。）

- [ ] **Step 2: 本地验证**

Run: `npm run dev`（在 `frontend/`），访问 `http://localhost:3000/services`，确认入口卡渲染、点击跳转 `/tools/sse-parser`。`Ctrl+C` 停止。

- [ ] **Step 3: Commit**
```bash
git add "frontend/app/(marketing)/services/page.tsx"
git commit -m "feat(services): 新增 SSE 解析工具入口卡"
```

---

## Task 11: 全站 SEO 基础设施（`sitemap.ts` + `robots.ts` + 全局 metadata）

**Files:**
- Create: `frontend/app/sitemap.ts`
- Create: `frontend/app/robots.ts`
- Modify: `frontend/app/layout.tsx`

- [ ] **Step 1: 写 `sitemap.ts`**

Create `frontend/app/sitemap.ts`:
```ts
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
```

> ⚠️ `new Date()` 在 `sitemap.ts` / `robots.ts`（Next 路由模块，非 workflow 脚本）中可用，不违反 workflow 限制。

- [ ] **Step 2: 写 `robots.ts`**

Create `frontend/app/robots.ts`:
```ts
import type { MetadataRoute } from "next";

const BASE = process.env.NEXT_PUBLIC_SITE_URL ?? "https://aliang.one";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: { userAgent: "*", allow: "/" },
    sitemap: `${BASE}/sitemap.xml`,
    host: BASE,
  };
}
```

- [ ] **Step 3: 增强 `layout.tsx` 全局 metadata**

在 `app/layout.tsx` 把现有 `metadata` 升级（保留原 title/description，新增 `metadataBase` 等）：
```ts
const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "https://aliang.one";

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: "aliang.one - 阿良家的AI",
  description: "阿良家的AI API网关 - 提供稳定可靠的AI接口服务",
  keywords: ["AI API", "AI 网关", "Anthropic", "OpenAI", "Claude", "ChatGPT", "阿良家的AI"],
  openGraph: {
    type: "website",
    siteName: "阿良家的AI",
    title: "aliang.one - 阿良家的AI",
    description: "阿良家的AI API网关 - 提供稳定可靠的AI接口服务",
    url: SITE_URL,
  },
};
```

- [ ] **Step 4: 在 `.env.example` 与 `.env.local` 加一行（可选但建议）**

```
NEXT_PUBLIC_SITE_URL=https://aliang.one
```
（若用户有真实域名则替换；缺失时 sitemap/robots 回退默认值，不报错。）

- [ ] **Step 5: Commit**
```bash
git add frontend/app/sitemap.ts frontend/app/robots.ts frontend/app/layout.tsx frontend/.env.example
git commit -m "feat(seo): 新增 sitemap/robots 与全局 metadata 增强"
```

---

## Task 12: 整体验证

**Files:** 无（仅校验）

- [ ] **Step 1: 全量单测**

Run（`frontend/`）: `npm test`
Expected: 全部 PASS（parseSse / detect / openai / anthropic）

- [ ] **Step 2: 生产构建**

Run: `npm run build`
Expected: 成功，无类型错误；路由表含 `/tools/sse-parser`、`/sitemap.xml`、`/robots.txt`

- [ ] **Step 3: 手动验证四份示例**

Run: `npm run start`（或 `npm run dev`），访问 `/tools/sse-parser`：
1. 「填入示例」→ OpenAI 文本流：完整文本 = `Hello world`，Token 表显示 input=9/output=2/total=11、stopReason=stop。
2. OpenAI 工具调用：工具调用 Tab 显示 `get_weather({"city":"Beijing"})`。
3. Anthropic 文本流：完整文本 = `Hello!`，output=4、stopReason=end_turn。
4. Anthropic 工具调用：工具调用 Tab 显示 `get_weather({"city":"Beijing"})`。
5. 粘贴无法识别报文（如 `data: {"x":1}`）→ 顶部出现「未能自动识别」提示，手动切协议后能解析。
6. 事件时间线 Tab 显示有序事件，损坏事件高亮。
7. `/services` 入口卡可见且可跳转。

- [ ] **Step 4: SEO 产物校验**

Run:
```bash
curl -s http://localhost:3000/sitemap.xml | grep -o "/tools/sse-parser" | head -1
curl -s http://localhost:3000/robots.txt
curl -s http://localhost:3000/tools/sse-parser | grep -o "application/ld+json" | head -1
```
Expected: 三条均有输出（sitemap 含工具页、robots 含 Sitemap 行、页面含 JSON-LD）。

- [ ] **Step 5: 最终 commit（如有残留改动）**
```bash
git add -A
git commit -m "chore(sse-parser): 整体验证通过" || echo "nothing to commit"
```

---

## Notes for the implementer

- **路径别名 `@/`**：解析器测试用相对 import（`../parseSse`），组件用 `@/lib/sse`。`tsconfig.json` 已配 `@/*` → `./lib/*`？如未配，组件改用相对 import。
- **字段核对**：Task 4 实现后，对照 Anthropic 官方 streaming 文档与 OpenAI chat streaming 文档快速复核字段名（spec Risks 已声明）。
- **i18n 默认 locale**：cookie `NEXT_LOCALE` 缺失时默认 `zh`；中英文共享 URL，本次不做 hreflang。
- **CDN / assetPrefix**：不影响路由与 SEO；`assetPrefix` 仅作用于静态资源。
- **不引入后端**：本工具完全前端；`/api/public/services` 仅服务于既有 `/services` 列表，与本工具无关。

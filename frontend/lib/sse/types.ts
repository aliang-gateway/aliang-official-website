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

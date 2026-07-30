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

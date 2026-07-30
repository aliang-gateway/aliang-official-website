import type { SseEvent, ToolCall, UnifiedResult, UsageStats } from "./types";
import { finalizeToolCalls } from "./internal";

export function reduceAnthropic(events: SseEvent[]): UnifiedResult {
  let text = "";
  const toolMap = new Map<number, ToolCall>();
  const usage: UsageStats = {};
  const errors: { index: number; error: string }[] = [];

  const ensure = (index: number): ToolCall => {
    let e = toolMap.get(index);
    if (!e) {
      e = { index, id: "", name: "", arguments: "" };
      toolMap.set(index, e);
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
      const idx = typeof j.index === "number" ? j.index : 0;
      const entry = ensure(idx);
      const cb = j.content_block;
      if (typeof cb.id === "string") entry.id = cb.id;
      if (typeof cb.name === "string") entry.name = cb.name;
    } else if (type === "content_block_delta") {
      const d = j.delta;
      if (d && typeof d === "object") {
        if (d.type === "text_delta" && typeof d.text === "string") {
          text += d.text;
        } else if (d.type === "input_json_delta" && typeof d.partial_json === "string") {
          ensure(typeof j.index === "number" ? j.index : 0).arguments += d.partial_json;
        }
      }
    } else if (type === "message_delta") {
      if (j.delta && typeof j.delta.stop_reason === "string") usage.stopReason = j.delta.stop_reason;
      const u = j.usage;
      if (u && typeof u.output_tokens === "number") usage.outputTokens = u.output_tokens;
    } else if (type === "error") {
      errors.push({ index: ev.index, error: j.error?.message ?? "stream error" });
    }
  }

  // 只保留真正出现过 tool_use 的 index（有 name 或 id）；其余 finalize 出空工具会被过滤
  const toolCalls = finalizeToolCalls(toolMap).filter((tc) => tc.name || tc.id);

  return { protocol: "anthropic", text, toolCalls, usage, events, errors };
}

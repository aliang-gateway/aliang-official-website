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

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

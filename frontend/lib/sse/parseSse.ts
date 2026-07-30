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

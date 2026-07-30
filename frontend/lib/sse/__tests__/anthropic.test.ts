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
    // 用 JSON.stringify 构造 data: 负载，确保内层转义正确（partial_json 两段拼成 {"city":"BJ"}）。
    const frame = (eventName: string, obj: unknown) =>
      `event: ${eventName}\ndata: ${JSON.stringify(obj)}`;
    const raw = [
      frame("content_block_start", { type: "content_block_start", index: 1, content_block: { type: "tool_use", id: "toolu_1", name: "get_weather", input: {} } }),
      frame("content_block_delta", { type: "content_block_delta", index: 1, delta: { type: "input_json_delta", partial_json: `{"city":` } }),
      frame("content_block_delta", { type: "content_block_delta", index: 1, delta: { type: "input_json_delta", partial_json: `"BJ"}` } }),
      frame("content_block_stop", { type: "content_block_stop", index: 1 }),
    ].join("\n\n");
    const r = reduceAnthropic(parseSse(raw));
    expect(r.toolCalls).toHaveLength(1);
    expect(r.toolCalls[0].name).toBe("get_weather");
    expect(r.toolCalls[0].id).toBe("toolu_1");
    expect(r.toolCalls[0].arguments).toEqual({ city: "BJ" });
  });
});

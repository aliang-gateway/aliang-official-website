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

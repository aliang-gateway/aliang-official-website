import { describe, it, expect } from "vitest";
import { parseAndReduce } from "../index";
import { SAMPLES } from "../samples";

describe("parseAndReduce on samples", () => {
  it("openai-text: 识别协议 + 文本 + usage", () => {
    const { protocol, result } = parseAndReduce(SAMPLES["openai-text"].raw);
    expect(protocol).toBe("openai");
    expect(result?.text).toBe("Hello world");
    expect(result?.usage.stopReason).toBe("stop");
    expect(result?.usage.totalTokens).toBe(11);
  });
  it("openai-tool: 识别 + 工具调用", () => {
    const { protocol, result } = parseAndReduce(SAMPLES["openai-tool"].raw);
    expect(protocol).toBe("openai");
    expect(result?.toolCalls[0].name).toBe("get_weather");
    expect(result?.toolCalls[0].arguments).toEqual({ city: "Beijing" });
  });
  it("anthropic-text: 识别 + 文本 + stop_reason", () => {
    const { protocol, result } = parseAndReduce(SAMPLES["anthropic-text"].raw);
    expect(protocol).toBe("anthropic");
    expect(result?.text).toBe("Hello!");
    expect(result?.usage.stopReason).toBe("end_turn");
  });
  it("anthropic-tool: 识别 + 工具调用", () => {
    const { protocol, result } = parseAndReduce(SAMPLES["anthropic-tool"].raw);
    expect(protocol).toBe("anthropic");
    expect(result?.toolCalls[0].name).toBe("get_weather");
    expect(result?.toolCalls[0].arguments).toEqual({ city: "Beijing" });
  });
  it("空输入返回 null", () => {
    const { protocol, result } = parseAndReduce("");
    expect(protocol).toBeNull();
    expect(result).toBeNull();
  });
  it("手动强制协议", () => {
    const { protocol } = parseAndReduce(SAMPLES["openai-text"].raw, "openai");
    expect(protocol).toBe("openai");
  });
});

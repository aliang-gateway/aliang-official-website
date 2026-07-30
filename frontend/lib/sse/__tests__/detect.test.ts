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

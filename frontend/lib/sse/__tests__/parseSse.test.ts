import { describe, it, expect } from "vitest";
import { parseSse } from "../parseSse";

describe("parseSse", () => {
  it("解析 OpenAI 风格 data 块为有序事件", () => {
    const raw = [
      `data: {"choices":[{"delta":{"content":"Hi"}}]}`,
      ``,
      `data: {"choices":[{"delta":{"content":" there"}}]}`,
      ``,
      `data: [DONE]`,
      ``,
    ].join("\n");
    const ev = parseSse(raw);
    expect(ev).toHaveLength(3);
    expect(ev[0].index).toBe(1);
    expect(ev[0].ok).toBe(true);
    expect((ev[0].json as any).choices[0].delta.content).toBe("Hi");
    expect(ev[2].isDone).toBe(true);
  });

  it("保留 Anthropic 的 event 名", () => {
    const raw = `event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"X"}}\n\n`;
    const ev = parseSse(raw);
    expect(ev[0].event).toBe("content_block_delta");
  });

  it("损坏的 JSON 不中断，记录 error", () => {
    const raw = `data: {not json}\n\ndata: {"ok":true}\n\n`;
    const ev = parseSse(raw);
    expect(ev).toHaveLength(2);
    expect(ev[0].ok).toBe(false);
    expect(ev[0].error).toBeTruthy();
    expect(ev[1].ok).toBe(true);
  });

  it("空输入返回空数组", () => {
    expect(parseSse("")).toEqual([]);
    expect(parseSse("   \n\n  ")).toEqual([]);
  });
});

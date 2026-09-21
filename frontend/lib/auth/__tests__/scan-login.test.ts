import { describe, it, expect } from "vitest";
import { scanPollNeedsNewSession } from "../scan-login";

describe("scanPollNeedsNewSession", () => {
  it("404（扫码行已被 TTL+宽限清理）必须废弃当前会话重新 init——原实现会拿着死码永远轮询", () => {
    expect(scanPollNeedsNewSession(404)).toBe(true);
  });

  it("瞬时/服务端错误继续轮询同一个 device_code", () => {
    expect(scanPollNeedsNewSession(500)).toBe(false);
    expect(scanPollNeedsNewSession(502)).toBe(false);
    expect(scanPollNeedsNewSession(401)).toBe(false);
    expect(scanPollNeedsNewSession(429)).toBe(false);
  });
});

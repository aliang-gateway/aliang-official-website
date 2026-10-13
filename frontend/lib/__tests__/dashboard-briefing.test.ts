import { describe, it, expect } from "vitest";

import { parseTrendUsageSummary } from "../dashboard-home";

const trendPayload = {
  data: {
    granularity: "day",
    start_date: "2026-10-07",
    end_date: "2026-10-13",
    trend: [
      {
        date: "2026-10-07",
        requests: 10,
        input_tokens: 100,
        output_tokens: 50,
        cache_read_tokens: 200,
        cache_creation_tokens: 10,
        cost: 0.1,
        actual_cost: 0.2,
      },
      {
        date: "2026-10-08",
        requests: 5,
        input_tokens: 40,
        output_tokens: 30,
        cache_read_tokens: 0,
        cache_creation_tokens: 0,
        cost: 0.05,
      },
    ],
  },
};

describe("parseTrendUsageSummary 按范围聚合 trend 行", () => {
  it("汇总 requests/input/output/cache/total/cost,total_tokens 无字段时回退 input+output", () => {
    const summary = parseTrendUsageSummary(trendPayload);
    expect(summary.requests).toBe(15);
    expect(summary.input_tokens).toBe(140);
    expect(summary.output_tokens).toBe(80);
    expect(summary.cache_read_tokens).toBe(200);
    expect(summary.cache_creation_tokens).toBe(10);
    // 第二行缺 actual_cost 回退 cost=0.05 → 0.2 + 0.05
    expect(summary.actual_cost).toBeCloseTo(0.25, 6);
    // 两行都无 total_tokens → (100+50)+(40+30)=220
    expect(summary.total_tokens).toBe(220);
    expect(summary.start_date).toBe("2026-10-07");
    expect(summary.end_date).toBe("2026-10-13");
  });

  it("空 trend 返回全 0 并保留日期", () => {
    const summary = parseTrendUsageSummary({
      data: { granularity: "day", start_date: "2026-10-07", end_date: "2026-10-13", trend: [] },
    });
    expect(summary.requests).toBe(0);
    expect(summary.total_tokens).toBe(0);
    expect(summary.actual_cost).toBe(0);
    expect(summary.start_date).toBe("2026-10-07");
  });

  it("null/畸形 payload 不抛错,返回全 0", () => {
    const summary = parseTrendUsageSummary(null);
    expect(summary.total_tokens).toBe(0);
    expect(summary.start_date).toBe("");
  });
});

import { describe, it, expect } from "vitest";

import { parseTrendUsageSummary, parseWalletTransactions } from "../dashboard-home";

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

  it("行内显式 total_tokens 优先于 input+output 回退", () => {
    const summary = parseTrendUsageSummary({
      data: {
        granularity: "day",
        start_date: "2026-10-07",
        end_date: "2026-10-07",
        trend: [{ date: "2026-10-07", requests: 1, input_tokens: 100, output_tokens: 50, total_tokens: 200 }],
      },
    });
    // 上游已算好的 total_tokens(200)不被 input+output(150)覆盖
    expect(summary.total_tokens).toBe(200);
  });
});

describe("parseWalletTransactions 宽容解析三种形态", () => {
  const row = {
    id: 7,
    type: "recharge",
    amount_micros: 50_000_000,
    currency: "CNY",
    balance_after_micros: 120_500_000,
    description: "Recharge via card code",
    created_at: "2026-10-12T14:30:00Z",
  };

  it("标准形态 {transactions:[...]},金额 micros→主体单位", () => {
    const list = parseWalletTransactions({ transactions: [row] });
    expect(list).toHaveLength(1);
    expect(list[0]).toEqual({
      id: 7,
      type: "recharge",
      amount: 50,
      currency: "CNY",
      balance_after: 120.5,
      description: "Recharge via card code",
      created_at: "2026-10-12T14:30:00Z",
    });
  });

  it("data 包裹形态 {data:[...]} 与裸数组", () => {
    expect(parseWalletTransactions({ data: [row] })).toHaveLength(1);
    expect(parseWalletTransactions([row])).toHaveLength(1);
  });

  it("双层包裹形态 {data:{transactions:[...]}} 同样可解析", () => {
    expect(parseWalletTransactions({ data: { transactions: [row] } })).toHaveLength(1);
  });

  it("负数 amount_micros 表示扣减,amount 为负主体单位", () => {
    const list = parseWalletTransactions({ transactions: [{ ...row, amount_micros: -5_000_000 }] });
    expect(list[0].amount).toBe(-5);
  });

  it("字段缺失回退:type→unknown,currency→CNY,金额→0", () => {
    const list = parseWalletTransactions({ transactions: [{ id: 1 }] });
    expect(list[0].type).toBe("unknown");
    expect(list[0].currency).toBe("CNY");
    expect(list[0].amount).toBe(0);
    expect(list[0].balance_after).toBe(0);
  });

  it("非列表 payload 返回空数组", () => {
    expect(parseWalletTransactions(null)).toEqual([]);
    expect(parseWalletTransactions({})).toEqual([]);
  });
});

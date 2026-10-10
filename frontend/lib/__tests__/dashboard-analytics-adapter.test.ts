import { describe, it, expect } from "vitest";
import {
  cacheHitRate,
  formatCacheHitRate,
  aggregateCacheRate,
  parseDashboardUsageEnvelope,
} from "../dashboard-analytics-adapter";

describe("cacheHitRate 口径：cache_read/(input+cache_read+cache_creation)", () => {
  it("正常计算", () => {
    // 2886080 / (58731 + 2886080 + 0) ≈ 0.9801（线上实测样本）
    expect(cacheHitRate(58731, 2886080, 0)).toBeCloseTo(0.9801, 3);
  });

  it("分母为 0 时返回 null（无请求，展示 '—'）", () => {
    expect(cacheHitRate(0, 0, 0)).toBeNull();
  });

  it("全量 cache_creation（冷启动）命中率为 0", () => {
    expect(cacheHitRate(1000, 0, 500)).toBe(0);
  });
});

describe("aggregateCacheRate 按行聚合", () => {
  it("先汇总 token 再算比率，而不是逐行平均", () => {
    const rate = aggregateCacheRate([
      { input_tokens: 1000, cache_read_tokens: 0, cache_creation_tokens: 500 },
      { input_tokens: 1000, cache_read_tokens: 9000, cache_creation_tokens: 0 },
    ]);
    // 9000 / (2000 + 9000 + 500)
    expect(rate).toBeCloseTo(9000 / 11500, 5);
  });

  it("空列表返回 null", () => {
    expect(aggregateCacheRate([])).toBeNull();
  });
});

describe("parseDashboardUsageEnvelope 解析倍率与缓存字段", () => {
  it("行数据补齐 rate_multiplier / group_name / 缓存 token", () => {
    const envelope = parseDashboardUsageEnvelope({
      data: [
        {
          id: 421299,
          request_id: "req_1",
          model: "glm-5.3-flash",
          input_tokens: 1525,
          output_tokens: 900,
          cache_read_tokens: 2886080,
          cache_creation_tokens: 0,
          rate_multiplier: 1.5,
          total_cost: 0.02,
          actual_cost: 0.03,
          group: { id: 22, name: "anthropic-duration", rate_multiplier: 1.5 },
        },
      ],
      pagination: { page: 1, per_page: 20, total: 1, pages: 1 },
    });
    const row = envelope.data[0];
    expect(row.rate_multiplier).toBe(1.5);
    expect(row.group_name).toBe("anthropic-duration");
    expect(row.cache_read_tokens).toBe(2886080);
    expect(row.cache_creation_tokens).toBe(0);
    expect(cacheHitRate(row.input_tokens, row.cache_read_tokens, row.cache_creation_tokens)).toBeGreaterThan(0.9);
  });

  it("缺失字段时倍率回退 1、缓存字段回退 0", () => {
    const envelope = parseDashboardUsageEnvelope({
      data: [{ id: 1, model: "m", input_tokens: 10, output_tokens: 5 }],
    });
    expect(envelope.data[0].rate_multiplier).toBe(1);
    expect(envelope.data[0].cache_read_tokens).toBe(0);
    expect(envelope.data[0].cache_creation_tokens).toBe(0);
    expect(envelope.data[0].group_name).toBe("");
  });
});

describe("formatCacheHitRate", () => {
  it("null（无请求）渲染为占位符", () => {
    expect(formatCacheHitRate(null)).toBe("—");
  });
  it("小数转百分比，保留一位", () => {
    expect(formatCacheHitRate(0.9801)).toBe("98.0%");
    expect(formatCacheHitRate(1)).toBe("100.0%");
    expect(formatCacheHitRate(0)).toBe("0.0%");
  });
});

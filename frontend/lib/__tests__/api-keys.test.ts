import { describe, it, expect } from "vitest";
import { formatRateMultiplier, parseAvailableGroups, parseApiKeysList } from "../api-keys";

describe("parseAvailableGroups 的倍率解析", () => {
  it("保留上游的 rate_multiplier 字段", () => {
    const groups = parseAvailableGroups({
      data: [
        { id: 11, name: "openai-duration", platform: "openai", rate_multiplier: 1.5 },
        { id: 22, name: "anthropic-duration", platform: "anthropic", rate_multiplier: 1 },
      ],
    });
    expect(groups).toHaveLength(2);
    expect(groups[0].rate_multiplier).toBe(1.5);
    expect(groups[1].rate_multiplier).toBe(1);
  });

  it("上游缺失倍率字段时回退为 1（组默认无倍率语义）", () => {
    const groups = parseAvailableGroups({
      data: [{ id: 1, name: "default", platform: "openai" }],
    });
    expect(groups[0].rate_multiplier).toBe(1);
  });
});

describe("parseApiKeysList 的分组倍率解析", () => {
  it("从内嵌 group 对象取 rate_multiplier", () => {
    const keys = parseApiKeysList({
      data: [
        {
          id: 7,
          name: "my-key",
          key: "sk-xxx",
          group_id: 22,
          group: { name: "anthropic-duration", platform: "anthropic", rate_multiplier: 2 },
        },
      ],
    });
    expect(keys[0].group_name).toBe("anthropic-duration");
    expect(keys[0].group_rate_multiplier).toBe(2);
  });

  it("key 自带 rate_multiplier 或缺失时宽容处理", () => {
    const keys = parseApiKeysList({
      data: [
        { id: 1, name: "a", group_id: 1, rate_multiplier: 3 },
        { id: 2, name: "b", group_id: 2 },
      ],
    });
    expect(keys[0].group_rate_multiplier).toBe(3);
    expect(keys[1].group_rate_multiplier).toBe(1);
  });
});

describe("formatRateMultiplier", () => {
  it("整数倍率不带小数点", () => {
    expect(formatRateMultiplier(1)).toBe("×1");
    expect(formatRateMultiplier(2)).toBe("×2");
  });
  it("小数倍率保留有效位", () => {
    expect(formatRateMultiplier(1.5)).toBe("×1.5");
    expect(formatRateMultiplier(0.8)).toBe("×0.8");
  });
  it("异常值回退 ×1", () => {
    expect(formatRateMultiplier(0)).toBe("×1");
    expect(formatRateMultiplier(Number.NaN)).toBe("×1");
  });
});

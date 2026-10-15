import { describe, it, expect } from "vitest";
import { buildKeySelectOption, formatRateMultiplier, maskApiKey, parseAvailableGroups, parseApiKeysList, platformRowClasses } from "../api-keys";

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

describe("buildKeySelectOption 下拉选项组装", () => {
  const fullKey = {
    id: 7,
    name: "my-key",
    key: "sk-abcdefghij1234567890",
    group_id: 22,
    group_name: "anthropic-duration",
    group_platform: "anthropic",
    group_rate_multiplier: 1.5,
    status: "active",
    quota: 0,
    quota_used: 0,
    expires_at: "",
    created_at: "2026-10-15T00:00:00Z",
  };

  it("label=名称·略写,hint=类型 倍率", () => {
    const option = buildKeySelectOption(fullKey);
    expect(option.value).toBe("7");
    expect(option.label).toBe(`my-key · ${maskApiKey(fullKey.key)}`);
    expect(option.hint).toBe("Anthropic ×1.5");
  });

  it("platform 缺失时 hint 仅倍率", () => {
    expect(buildKeySelectOption({ ...fullKey, group_platform: "" }).hint).toBe("×1.5");
    expect(buildKeySelectOption({ ...fullKey, group_platform: "  " }).hint).toBe("×1.5");
  });

  it("name 缺失时回退 Key #id", () => {
    expect(buildKeySelectOption({ ...fullKey, name: "" }).label).toBe(
      `Key #7 · ${maskApiKey(fullKey.key)}`,
    );
  });
});

describe("platformRowClasses 平台行背景", () => {
  it("anthropic → key-row-anthropic 类", () => {
    expect(platformRowClasses("anthropic")).toBe("key-row-anthropic");
  });

  it("openai → key-row-openai 类", () => {
    expect(platformRowClasses("openai")).toBe("key-row-openai");
  });

  it("大小写归一(OpenAI 同 openai)", () => {
    expect(platformRowClasses("OpenAI")).toBe("key-row-openai");
  });

  it("未知/空平台返回空串(默认样式)", () => {
    expect(platformRowClasses("")).toBe("");
    expect(platformRowClasses("unknown")).toBe("");
  });
});

describe("maskApiKey 直接覆盖(安全相邻)", () => {
  it("常规 key 掐头去尾", () => {
    expect(maskApiKey("sk-abcdefghij1234567890")).toBe("sk-abcde***7890");
  });

  it("空串返回 ***", () => {
    expect(maskApiKey("")).toBe("***");
  });

  it("短 key 仅前 3 字符+***", () => {
    expect(maskApiKey("abc")).toBe("abc***");
  });
});

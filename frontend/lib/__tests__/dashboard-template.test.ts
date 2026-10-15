import { describe, it, expect } from "vitest";

import {
  GATEWAY_BASE_URL,
  buildTemplateContent,
  escapeSingleQuotedShell,
} from "../dashboard-template";

describe("claude shell 模板(默认,行为不变)", () => {
  it("生成两行 export 与启动命令,BASE_URL 不含 /v1", () => {
    const out = buildTemplateContent("claude", "sk-test");
    expect(out).toBe(
      [
        `export ANTHROPIC_BASE_URL='${GATEWAY_BASE_URL}'`,
        "export ANTHROPIC_AUTH_TOKEN='sk-test'",
        "claude",
      ].join("\n"),
    );
  });

  it("显式 format='shell' 与缺省输出一致", () => {
    expect(buildTemplateContent("claude", "sk-test", undefined, "shell")).toBe(
      buildTemplateContent("claude", "sk-test"),
    );
  });

  it("密钥含单引号时经 POSIX 转义", () => {
    const out = buildTemplateContent("claude", "it's", undefined, "shell");
    expect(out).toContain("export ANTHROPIC_AUTH_TOKEN='it'\\''s'");
  });
});

describe("claude settings.json 模板(format='json')", () => {
  it("输出合法 JSON,env 注入 BASE_URL 与 AUTH_TOKEN", () => {
    const out = buildTemplateContent("claude", "sk-live-1", undefined, "json");
    const parsed = JSON.parse(out) as { env: Record<string, string> };
    expect(parsed.env.ANTHROPIC_BASE_URL).toBe(GATEWAY_BASE_URL);
    expect(parsed.env.ANTHROPIC_AUTH_TOKEN).toBe("sk-live-1");
    expect(Object.keys(parsed)).toEqual(["env"]);
  });

  it("密钥含引号/换行时 JSON 仍合法且值无损", () => {
    const tricky = 'he said "hi"\nline2';
    const parsed = JSON.parse(buildTemplateContent("claude", tricky, undefined, "json")) as {
      env: Record<string, string>;
    };
    expect(parsed.env.ANTHROPIC_AUTH_TOKEN).toBe(tricky);
  });

  it("models 参数对 claude 输出无影响", () => {
    expect(buildTemplateContent("claude", "k", ["m1"], "json")).toBe(
      buildTemplateContent("claude", "k", undefined, "json"),
    );
  });
});

describe("opencode/codex 忽略 format 参数(回归保障)", () => {
  it("opencode 传 format 输出不变", () => {
    expect(buildTemplateContent("opencode", "k", ["a"], "toml")).toBe(
      buildTemplateContent("opencode", "k", ["a"]),
    );
  });

  it("codex 传 format 输出不变", () => {
    expect(buildTemplateContent("codex", "k", ["m"], "json")).toBe(
      buildTemplateContent("codex", "k", ["m"]),
    );
  });
});

describe("escapeSingleQuotedShell", () => {
  it("空串原样", () => {
    expect(escapeSingleQuotedShell("")).toBe("");
  });

  it("多引号逐个转义", () => {
    expect(escapeSingleQuotedShell("a'b'c")).toBe("a'\\''b'\\''c");
  });
});

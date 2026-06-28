// Dashboard 客户端配置模板(opencode/claude/codex)定义与生成。
// 从 app/dashboard/page.tsx 提取,保持行为逐字一致。

import type { ClientTemplateId, TemplateFormat } from "./dashboard-types";

export type TemplateDefinition = {
  id: ClientTemplateId;
  labelKey: string;
  helperKey: string;
  supportedFormats: TemplateFormat[];
};

export const TEMPLATE_DEFINITIONS: TemplateDefinition[] = [
  {
    id: "opencode",
    labelKey: "templateOpencodeLabel",
    helperKey: "templateOpencodeHelper",
    supportedFormats: ["json"],
  },
  {
    id: "claude",
    labelKey: "templateClaudeLabel",
    helperKey: "templateClaudeHelper",
    supportedFormats: ["shell"],
  },
  {
    id: "codex",
    labelKey: "templateCodexLabel",
    helperKey: "templateCodexHelper",
    supportedFormats: ["json", "yaml"],
  },
];

export function escapeJsonString(value: string) {
  return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
}

export function escapeSingleQuotedShell(value: string) {
  return value.replaceAll("'", "'\\''");
}

export function formatYamlScalar(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
}

export function buildTemplateContent(templateId: ClientTemplateId, format: TemplateFormat, userKey: string) {
  const baseUrl = "https://api.aliang.one";
  const escapedKey = escapeJsonString(userKey);
  const escapedBaseUrl = escapeJsonString(baseUrl);
  const yamlKey = formatYamlScalar(userKey);
  const yamlBaseUrl = formatYamlScalar(baseUrl);

  if (templateId === "opencode") {
    return [
      "{",
      '  "provider": "custom",',
      `  "base_url": "${escapedBaseUrl}/v1",`,
      `  "api_key": "${escapedKey}",`,
      '  "model": "claude-sonnet-4-20250514"',
      "}",
    ].join("\n");
  }

  if (templateId === "claude") {
    return [
      `export ANTHROPIC_BASE_URL='${escapeSingleQuotedShell(baseUrl)}'`,
      `export ANTHROPIC_AUTH_TOKEN='${escapeSingleQuotedShell(userKey)}'`,
      "claude",
    ].join("\n");
  }

  if (templateId === "codex") {
    if (format === "yaml") {
      return [
        "provider: openai",
        `base_url: ${yamlBaseUrl}`,
        `api_key: ${yamlKey}`,
        "model: gpt-4.1",
      ].join("\n");
    }

    return [
      "{",
      '  "provider": "openai",',
      `  "base_url": "${escapedBaseUrl}",`,
      `  "api_key": "${escapedKey}",`,
      '  "model": "gpt-4.1"',
      "}",
    ].join("\n");
  }

  return "";
}

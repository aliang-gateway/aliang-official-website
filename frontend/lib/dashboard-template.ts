// Dashboard 客户端配置模板(opencode/claude/codex)定义与生成。
// 生成结果应直接可用:opencode → ~/.opencode.json;codex → ~/.codex/config.toml;
// claude → 终端环境变量。网关根地址统一取自 GATEWAY_BASE_URL,改一处即全改。

import type { ClientTemplateId, TemplateFormat } from "./dashboard-types";

/**
 * 网关根地址(不含 /v1)。构建期可用 NEXT_PUBLIC_GATEWAY_BASE_URL 覆盖,
 * 与 app/api/models/route.ts 的 GATEWAY_BASE_URL 保持同值。
 */
export const GATEWAY_BASE_URL = (process.env.NEXT_PUBLIC_GATEWAY_BASE_URL ?? "https://api.aliang.one").replace(/\/+$/, "");

/** 密钥未提供或 /v1/models 拉取失败时,opencode 配置回退到该模型。 */
const OPENCODE_DEFAULT_MODEL = "claude-sonnet-4-20250514";
/** 密钥未提供或 /v1/models 拉取失败时,codex 配置回退到该模型。 */
const CODEX_DEFAULT_MODEL = "gpt-4.1";

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
    supportedFormats: ["toml"],
  },
];

/** 单引号 POSIX shell 转义:' → '\'' */
export function escapeSingleQuotedShell(value: string) {
  return value.replaceAll("'", "'\\''");
}

/** TOML 基本字符串(双引号)转义:反斜杠、引号与控制字符。 */
export function escapeTomlBasicString(value: string) {
  return value
    .replaceAll("\\", "\\\\")
    .replaceAll('"', '\\"')
    .replaceAll("\n", "\\n")
    .replaceAll("\r", "\\r")
    .replaceAll("\t", "\\t");
}

function buildOpencodeConfig(userKey: string, models?: string[]): string {
  // opencode 要求 provider 为嵌套对象,顶层模型形如 "<providerId>/<modelId>"。
  // 走 @ai-sdk/openai-compatible 打 /v1/chat/completions。用 JSON.stringify
  // 自动完成转义,避免手拼 JSON 漏掉控制字符。
  // 模型优先用所选密钥实际可用的(/v1/models),并把它们全部列入 models 映射,
  // 便于在 opencode 里切换;为空则回退到默认。
  const list = models && models.length > 0 ? models : [OPENCODE_DEFAULT_MODEL];
  const defaultModel = list[0];
  const config = {
    $schema: "https://opencode.ai/config.json",
    provider: {
      aliang: {
        npm: "@ai-sdk/openai-compatible",
        name: "Aliang Gateway",
        options: {
          baseURL: `${GATEWAY_BASE_URL}/v1`,
          apiKey: userKey,
        },
        models: Object.fromEntries(list.map((modelId) => [modelId, { name: modelId }])),
      },
    },
    model: `aliang/${defaultModel}`,
  };
  return JSON.stringify(config, null, 2);
}

function buildClaudeConfig(userKey: string): string {
  // Claude Code 走 Anthropic Messages 协议(网关已暴露 /v1/messages)。
  // AUTH_TOKEN 以 Bearer 发送,适配中转网关的自定义令牌。
  return [
    `export ANTHROPIC_BASE_URL='${escapeSingleQuotedShell(GATEWAY_BASE_URL)}'`,
    `export ANTHROPIC_AUTH_TOKEN='${escapeSingleQuotedShell(userKey)}'`,
    "claude",
  ].join("\n");
}

function buildCodexConfig(userKey: string, models?: string[]): string {
  // Codex CLI 只读 ~/.codex/config.toml(不读 JSON/YAML),且密钥只能经 env_key 注入。
  // 网关已暴露 /v1/responses,故 wire_api 取 responses(当前唯一合法值)。
  // base_url 须含 /v1,codex 会在此基础上拼 /responses。
  // 模型优先用所选密钥实际可用的;为空则回退到默认。
  const model = models && models.length > 0 ? models[0] : CODEX_DEFAULT_MODEL;
  const exportLine = `export ALIANG_API_KEY='${escapeSingleQuotedShell(userKey)}'`;
  return [
    `# 1) 在 shell 中导出密钥(建议加到 ~/.zshrc 或 ~/.bashrc):`,
    `#      ${exportLine}`,
    `# 2) 再把下方内容写入 ~/.codex/config.toml(已有文件请合并 [model_providers.aliang] 段)。`,
    ``,
    `model = "${escapeTomlBasicString(model)}"`,
    `model_provider = "aliang"`,
    ``,
    `[model_providers.aliang]`,
    `name = "Aliang Gateway"`,
    `base_url = "${escapeTomlBasicString(GATEWAY_BASE_URL)}/v1"`,
    `wire_api = "responses"`,
    `env_key = "ALIANG_API_KEY"`,
  ].join("\n");
}

export function buildTemplateContent(templateId: ClientTemplateId, userKey: string, models?: string[]) {
  if (templateId === "opencode") {
    return buildOpencodeConfig(userKey, models);
  }

  if (templateId === "claude") {
    return buildClaudeConfig(userKey);
  }

  if (templateId === "codex") {
    return buildCodexConfig(userKey, models);
  }

  return "";
}

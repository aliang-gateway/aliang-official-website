# 配置生成优化:模板置顶选择 + Claude settings.json 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 配置面板(opencode/claude/codex 三客户端模板生成)两处优化——模板选择从左栏底部抽成顶部全宽横排的 TemplateSelector;claude 模板在 shell(env)之外新增 settings.json(`{"env":{...}}`)输出格式,走现有格式 pills 机制。

**Architecture:** 零后端改动。模板层(`dashboard-template.ts`)为 claude 增加 json 分支并让 `buildTemplateContent` 接受可选 format;hook(`use-config-modal`)把既有 format 状态传入生成函数;UI 层抽出 `TemplateSelector` 子组件横排置顶,`ConfigPanel` 删除左栏模板块。格式切换完全复用既有 pills 与回退状态机。

**Tech Stack:** Next.js App Router、next-intl(zh/en)、TypeScript、vitest。

**Spec:** `docs/superpowers/specs/2026-10-15-config-template-top-selector-claude-settings-design.md`

---

## 背景速览(给零上下文的实施者)

- 功能入口:dashboard 的「配置与 API 密钥」弹窗(`ConfigModal`)与 `/keys` 页「配置」tab,**共用** `frontend/components/dashboard/ConfigPanel.tsx`(257 行)与 hook `frontend/lib/hooks/use-config-modal.ts`。
- 模板层 `frontend/lib/dashboard-template.ts`:`TEMPLATE_DEFINITIONS`(opencode→`["json"]`、claude→`["shell"]`、codex→`["toml"]`);`buildTemplateContent(templateId, userKey, models?)` 按 id 分发;`GATEWAY_BASE_URL`(不含 `/v1`);`escapeSingleQuotedShell` / `escapeTomlBasicString` 转义工具。
- hook 状态:`template`(默认 opencode)、`format`(默认 json);既有 effect 保证 format 总在当前模板 `supportedFormats` 内(不支持则回退首个);`renderedConfig` memo(第 106 行)是 `buildTemplateContent` 的**唯一**调用方。
- 右栏格式 pills 渲染 `templateDefinition.supportedFormats`,选中格式驱动 `format` 状态——claude 加 json 后 pills 自动出现,零新 UI。

## 环境须知(必须遵守)

1. **所有工作在 git worktree 中进行**(sibling 目录惯例,如 `../aliang-official-website-configtpl`,分支 `feat/config-template-selector`,基于最新 main)。
2. **装依赖用 `npm install`,严禁 pnpm**:本仓 frontend 跟踪 `package-lock.json`;pnpm 的严格布局缺提升的传递依赖(`@types/mdx`),会让 tsc/build 假红。运行测试用 `pnpm test`(脚本走 vitest,node_modules 为 npm 布局时同样可用)。
3. 门禁口径:仓库有约 70 个**既有** lint 噪音,门禁 = 变更文件级 `npx eslint <files>` 干净 + `pnpm test` 全过 + `pnpm build` exit 0。
4. `frontend/pnpm-lock.yaml` 若在 worktree 出现为未跟踪文件(pnpm 残留),**不要提交**。
5. 每个 commit 消息体末尾加 `Co-Authored-By: Claude Code <noreply@anthropic.com>`。

## 文件结构(改动全景)

| 动作 | 路径 | 职责 |
| --- | --- | --- |
| Modify | `frontend/lib/dashboard-template.ts` | claude 加 json 分支;`buildTemplateContent` 加可选 format 参数 |
| Create | `frontend/lib/__tests__/dashboard-template.test.ts` | 模板层单测 |
| Modify | `frontend/lib/hooks/use-config-modal.ts` | renderedConfig memo 传 format(含依赖数组) |
| Create | `frontend/components/dashboard/TemplateSelector.tsx` | 顶部模板选择条 |
| Modify | `frontend/components/dashboard/ConfigPanel.tsx` | 删左栏模板块;顶部插入 TemplateSelector;清理不再使用的 import |
| Modify | `frontend/components/dashboard/index.ts` | barrel 导出 |
| Modify | `frontend/messages/zh.json`、`en.json` | 仅改 `templateClaudeHelper` 的值 |

---

### Task 1: claude settings.json 模板层(TDD)

**Files:**
- Modify: `frontend/lib/dashboard-template.ts`
- Test: `frontend/lib/__tests__/dashboard-template.test.ts`(新建)

- [ ] **Step 1: 写失败测试 `frontend/lib/__tests__/dashboard-template.test.ts`**

```ts
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd <worktree>/frontend && pnpm vitest run lib/__tests__/dashboard-template.test.ts`
Expected: FAIL — `buildTemplateContent("claude", …, undefined, "json")` 等用例不满足(claude 尚无 format 分支,输出仍为 shell;TS 可能报第四参不存在)

- [ ] **Step 3: 实现(全部在 `dashboard-template.ts`)**

3a. claude 定义的 `supportedFormats` 改为:

```ts
    supportedFormats: ["shell", "json"],
```

3b. `buildClaudeConfig` 替换为(签名加 format,json 分支新增,shell 分支逐字保留):

```ts
function buildClaudeConfig(userKey: string, format: TemplateFormat = "shell"): string {
  // Claude Code 走 Anthropic Messages 协议(网关已暴露 /v1/messages)。
  // shell 形态:AUTH_TOKEN 以 Bearer 发送,适配中转网关的自定义令牌。
  // json 形态:Claude Code 官方 settings.json 的 env 注入——写入或合并
  // ~/.claude/settings.json 即可;转义全权交给 JSON.stringify。
  if (format === "json") {
    return JSON.stringify(
      {
        env: {
          ANTHROPIC_BASE_URL: GATEWAY_BASE_URL,
          ANTHROPIC_AUTH_TOKEN: userKey,
        },
      },
      null,
      2,
    );
  }

  return [
    `export ANTHROPIC_BASE_URL='${escapeSingleQuotedShell(GATEWAY_BASE_URL)}'`,
    `export ANTHROPIC_AUTH_TOKEN='${escapeSingleQuotedShell(userKey)}'`,
    "claude",
  ].join("\n");
}
```

3c. `buildTemplateContent` 签名加第 4 个可选参数并透传:

```ts
export function buildTemplateContent(
  templateId: ClientTemplateId,
  userKey: string,
  models?: string[],
  format?: TemplateFormat,
) {
  if (templateId === "opencode") {
    return buildOpencodeConfig(userKey, models);
  }

  if (templateId === "claude") {
    return buildClaudeConfig(userKey, format);
  }

  if (templateId === "codex") {
    return buildCodexConfig(userKey, models);
  }

  return "";
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm vitest run lib/__tests__/dashboard-template.test.ts`
Expected: PASS(11 tests)

- [ ] **Step 5: 全量回归**

Run: `pnpm test`
Expected: 全部 PASS(基线 62 + 新增 11 = 73)

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/dashboard-template.ts frontend/lib/__tests__/dashboard-template.test.ts
git commit -m "feat: claude 模板支持 settings.json(env 注入)格式"
```

---

### Task 2: hook 接线 format

**Files:**
- Modify: `frontend/lib/hooks/use-config-modal.ts:105-107`

- [ ] **Step 1: `renderedConfig` memo 传入 format,并把它加入依赖数组(spec 明确要求,勿漏)**

```ts
  const renderedConfig = useMemo(() => {
    return buildTemplateContent(template, userKey.trim(), availableModels, format);
  }, [template, userKey, availableModels, format]);
```

- [ ] **Step 2: 验证**

Run: `npx eslint lib/hooks/use-config-modal.ts && pnpm test`
Expected: eslint 干净;73/73 通过

- [ ] **Step 3: Commit**

```bash
git add frontend/lib/hooks/use-config-modal.ts
git commit -m "feat: 配置生成按所选格式渲染(claude shell/json)"
```

---

### Task 3: i18n helper 文案

**Files:**
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: 仅替换 `templateClaudeHelper` 的值(key 名、位置均不变)**

zh.json:

```json
"templateClaudeHelper": "Claude Code 客户端配置：shell 在终端导出环境变量后直接启动；settings.json 写入或合并到 ~/.claude/settings.json。",
```

en.json:

```json
"templateClaudeHelper": "Claude Code client config: shell exports env vars in your terminal; settings.json goes into (or merges with) ~/.claude/settings.json.",
```

- [ ] **Step 2: 校验 JSON 合法且两侧 key 集合一致**

Run: `node -e "const zh=require('./messages/zh.json'), en=require('./messages/en.json'); const a=Object.keys(zh.dashboard), b=Object.keys(en.dashboard); console.log(a.length, b.length, 'zh-only:'+a.filter(k=>!b.includes(k)).length, 'en-only:'+b.filter(k=>!a.includes(k)).length)"`
Expected: 两侧数量相同、差集 0

- [ ] **Step 3: Commit**

```bash
git add frontend/messages/zh.json frontend/messages/en.json
git commit -m "feat: claude 模板 helper 文案覆盖 shell/settings.json 两条路径"
```

---

### Task 4: TemplateSelector 组件 + ConfigPanel 重排 + barrel

**Files:**
- Create: `frontend/components/dashboard/TemplateSelector.tsx`
- Modify: `frontend/components/dashboard/ConfigPanel.tsx`
- Modify: `frontend/components/dashboard/index.ts`

- [ ] **Step 1: 新建 `TemplateSelector.tsx`**

```tsx
"use client";

import { useTranslations } from "next-intl";

import { TEMPLATE_DEFINITIONS } from "@/lib/dashboard-template";
import type { ClientTemplateId } from "@/lib/dashboard-types";

type TemplateSelectorProps = {
  template: ClientTemplateId;
  onTemplateChange: (value: ClientTemplateId) => void;
};

/** 面板顶部全宽模板选择条:先选客户端(opencode/claude/codex)再配密钥。
 *  ConfigPanel 的两个消费方(dashboard 弹窗、/keys 配置 tab)共用。
 *  卡片样式沿用原左栏模板块,仅由纵列改为横排。 */
export function TemplateSelector({ template, onTemplateChange }: TemplateSelectorProps) {
  const t = useTranslations("dashboard");

  return (
    <div className="space-y-2">
      <p className="text-sm font-semibold text-[var(--portal-ink)]">{t("template")}</p>
      <div className="grid gap-2 sm:grid-cols-3">
        {TEMPLATE_DEFINITIONS.map((templateDef) => {
          const isActive = templateDef.id === template;
          return (
            <button
              key={templateDef.id}
              type="button"
              className={`rounded-[1rem] border px-4 py-3 text-left transition-all duration-200 ${
                isActive
                  ? "border-[var(--accent)]/40 bg-[var(--accent-wash)]"
                  : "border-[var(--portal-line)] bg-[var(--portal-clay-strong)] hover:-translate-y-[1px]"
              }`}
              onClick={() => onTemplateChange(templateDef.id)}
            >
              <p className="text-sm font-semibold text-[var(--portal-ink)]">{t(templateDef.labelKey)}</p>
              <p className="mt-1 text-xs leading-5 text-[var(--portal-muted)]">{t(templateDef.helperKey)}</p>
            </button>
          );
        })}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: `ConfigPanel.tsx` 重排**

2a. import 区:`import { TEMPLATE_DEFINITIONS } from "@/lib/dashboard-template"` 若因此不再使用则移除该具名导入(`GATEWAY_BASE_URL` 仍在右栏使用,保留;以 eslint 结果为准)。新增:

```tsx
import { TemplateSelector } from "./TemplateSelector";
```

2b. 返回结构:在最外层包一层,顶部插入 TemplateSelector,原两栏 grid 原样内移(左栏删除整个模板选择块,即 `{t("template")}` 起到该 `</div>` 收尾的 `<div className="space-y-2">` 区块,约 170-192 行):

```tsx
  return (
    <div className="min-h-0 space-y-4 overflow-y-auto">
      <div className="border-b border-[var(--portal-line)] px-5 pb-4 pt-5 sm:px-6">
        <TemplateSelector template={template} onTemplateChange={onTemplateChange} />
      </div>
      <div className="grid min-h-0 gap-0 lg:grid-cols-[280px_minmax(0,1fr)]">
        {/* 左栏:原内容减去模板选择块,逐字保留其余 */}
        {/* 右栏:逐字保留 */}
      </div>
    </div>
  );
```

注意:原外层 grid 上的 `overflow-y-auto` 移到新外层;左栏 `border-b lg:border-b-0` 等类名不变。

- [ ] **Step 3: `index.ts` barrel 追加**

```ts
export { TemplateSelector } from "./TemplateSelector";
```

- [ ] **Step 4: 验证**

Run: `npx eslint components/dashboard/TemplateSelector.tsx components/dashboard/ConfigPanel.tsx components/dashboard/index.ts && pnpm test`
Expected: eslint 干净;73/73 通过

- [ ] **Step 5: Commit**

```bash
git add frontend/components/dashboard/TemplateSelector.tsx frontend/components/dashboard/ConfigPanel.tsx frontend/components/dashboard/index.ts
git commit -m "feat: 模板选择抽为顶部全宽 TemplateSelector"
```

---

### Task 5: 终验

- [ ] **Step 1: 全量测试** — `pnpm test` → 73/73 PASS
- [ ] **Step 2: lint + build** — `npx eslint` 于全部变更文件干净;`pnpm build` exit 0(在 worktree 内首次运行会生成 next-env.d.ts/.next,属正常)
- [ ] **Step 3: 汇报** — `git log --oneline <base>..HEAD`、门禁输出、git status(应只剩未跟踪安装产物)

目验(controller 与用户做):dashboard 弹窗与 /keys tab 顶部横排三卡、claude pills 出现 shell|json、切换输出联动、复制内容随格式、opencode→claude 停 json / codex→claude 落 shell、窄屏三卡纵列堆叠。

---

## 风险与回滚

- 全部改动在前端,逐 task 独立 commit,任一步出问题 `git revert` 对应提交即可
- `buildTemplateContent` 第 4 参可选,唯一调用方同步更新,无其他消费方(grep 已核实)
- claude shell 输出被测试逐字锁定,行为不变有回归保障

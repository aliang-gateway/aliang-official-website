# 密钥下拉信息增强 + /keys 平台背景色 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 配置面板「选择已有密钥」下拉展示 `名称·略写 + 类型 ×倍率` 组合信息(选中后触发按钮也回显);/keys 页密钥行按平台类型渲染左边框色条 + 淡背景。

**Architecture:** 零后端改动。lib 层(`api-keys.ts`)新增两个纯函数——`buildKeySelectOption`(TDD)与 `platformRowClasses`(TDD);UI 层只做接线:SelectMenu 触发按钮补 hint 回显、ConfigPanel 换用组装函数、KeyRow 附加平台类。信息源与 /keys 既有徽章同源(`platformBadgeLabel`/`formatRateMultiplier`)。

**Tech Stack:** Next.js、next-intl、TypeScript、vitest。

**Spec:** `docs/superpowers/specs/2026-10-15-key-selector-info-platform-colors-design.md`

---

## 背景速览(给零上下文的实施者)

- 配置面板 `frontend/components/dashboard/ConfigPanel.tsx`(dashboard 弹窗与 /keys 配置 tab 共用)的「选择已有密钥」用自绘下拉 `frontend/components/ui/SelectMenu.tsx`:选项类型 `{value, label, hint?}`,下拉项渲染 label(左,truncate)+ hint(右,小字灰);**触发按钮当前只回显 `selected.label`**。
- `frontend/lib/api-keys.ts` 已有:`ApiKeyItem`(含 `group_platform`、`group_rate_multiplier`)、`normalizeGroupPlatform`(大小写归一)、`platformBadgeLabel`(→ OpenAI/Anthropic/其他)、`formatRateMultiplier`(→ ×1/×1.5,异常回退 ×1)、`maskApiKey`(→ 略写,空回退 `***`)。
- /keys 页 `frontend/app/(app)/keys/page.tsx` 的 `KeyRow`(~line 496)行容器是 `<li className="clay-panel p-4">`;文件已从 `@/lib/api-keys` 导入多个具名符号(第 13-25 行 import 块)。
- 色条先例:ConfigPanel 敏感警告用 `border-l-[3px] border-l-[var(--mustard)]`;emerald 半透明 tint 在库内多处使用(含 dark 变体)。

## 环境须知(必须遵守)

1. **所有工作在 git worktree**(sibling 惯例,如 `../aliang-official-website-keyinfo`,分支 `feat/key-select-info`,基于最新 main)。
2. **装依赖用 `npm install`,严禁 pnpm**(pnpm 布局缺提升传递依赖会让 tsc/build 假红);跑测试用 `pnpm test`。
3. 门禁 = 变更文件级 `npx eslint <files>` 干净 + `pnpm test` 全过 + `pnpm build` exit 0(仓库约 70 个既有 lint 噪音不是门禁)。
4. commit 消息体末尾 `Co-Authored-By: Claude Code <noreply@anthropic.com>`(下文各 commit 命令为简洁省略该行,提交时一律补上)。

## 文件结构(改动全景)

| 动作 | 路径 | 职责 |
| --- | --- | --- |
| Modify | `frontend/lib/api-keys.ts` | 新增 `buildKeySelectOption` + `platformRowClasses`(纯函数) |
| Modify | `frontend/lib/__tests__/api-keys.test.ts` | 两函数单测 |
| Modify | `frontend/components/ui/SelectMenu.tsx` | 触发按钮回显 hint |
| Modify | `frontend/components/dashboard/ConfigPanel.tsx` | options 换 `buildKeySelectOption` |
| Modify | `frontend/app/globals.css` | 新增 `.key-row-anthropic`/`.key-row-openai` 未分层类 |
| Modify | `frontend/app/(app)/keys/page.tsx` | KeyRow 附加平台类 |

---

### Task 1: lib 层两函数(TDD)

**Files:**
- Modify: `frontend/lib/api-keys.ts`
- Test: `frontend/lib/__tests__/api-keys.test.ts`

- [ ] **Step 1: 追加失败测试(文件顶部 import 改为 `import { buildKeySelectOption, formatRateMultiplier, maskApiKey, parseAvailableGroups, parseApiKeysList, platformRowClasses } from "../api-keys";`,文件末尾追加两个 describe)**

```ts
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
    expect(platformRowClasses("OpenAI")).toBe(platformRowClasses("openai"));
  });

  it("未知/空平台返回空串(默认样式)", () => {
    expect(platformRowClasses("")).toBe("");
    expect(platformRowClasses("unknown")).toBe("");
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd <worktree>/frontend && pnpm vitest run lib/__tests__/api-keys.test.ts`
Expected: FAIL — `buildKeySelectOption`/`platformRowClasses` 未导出

- [ ] **Step 3: 在 `api-keys.ts` 末尾实现(`normalizeGroupPlatform`/`platformBadgeLabel`/`formatRateMultiplier`/`maskApiKey` 均为同文件既有函数,无需新 import)**

```ts
export type KeySelectOption = {
  value: string;
  label: string;
  hint: string;
};

/** 配置面板「选择已有密钥」下拉选项:label=名称·略写,hint=类型 倍率(缺失走各自回退)。 */
export function buildKeySelectOption(item: ApiKeyItem): KeySelectOption {
  const badge = item.group_platform ? platformBadgeLabel(item.group_platform) : "";
  const rate = formatRateMultiplier(item.group_rate_multiplier);
  return {
    value: String(item.id),
    label: `${item.name || `Key #${item.id}`} · ${maskApiKey(item.key)}`,
    hint: badge ? `${badge} ${rate}` : rate,
  };
}

/** /keys 密钥行按平台区分的附加类(globals.css 中 .clay-panel 之后定义的未分层类,
 *  能覆盖 .clay-panel 的 border/background);未知/缺失平台返回空串(默认样式)。 */
export function platformRowClasses(platform: string): string {
  const normalized = normalizeGroupPlatform(platform);
  if (normalized === "anthropic") {
    return "key-row-anthropic";
  }
  if (normalized === "openai") {
    return "key-row-openai";
  }
  return "";
}
```

> ⚠️ 为什么返回类名而不是 Tailwind 工具类:`.clay-panel` 是 globals.css 里的**未分层**手写 CSS(unlayered),按 CSS 级联层规则压过 `@layer utilities` 的 Tailwind v4 工具类——`border-l-*`/`bg-*` 工具类会被 `.clay-panel` 的 `border`/`background` 静默覆盖。必须在 `.clay-panel` 之后定义未分层类(Task 3)才能生效。

- [ ] **Step 4: 跑测试确认通过**

Run: `pnpm vitest run lib/__tests__/api-keys.test.ts`
Expected: PASS(该文件既有 7 例 + 新增 7 例 = 14 全过)

- [ ] **Step 5: Commit**

```bash
git add frontend/lib/api-keys.ts frontend/lib/__tests__/api-keys.test.ts
git commit -m "feat: buildKeySelectOption 与 platformRowClasses 纯函数"
```

---

### Task 2: SelectMenu 触发回显 hint + ConfigPanel 接线

**Files:**
- Modify: `frontend/components/ui/SelectMenu.tsx`
- Modify: `frontend/components/dashboard/ConfigPanel.tsx`

- [ ] **Step 1: SelectMenu 触发按钮(label 加 `min-w-0 flex-1`,hint 插在 label 与图标之间)**

```tsx
        <span className={`min-w-0 flex-1 truncate ${selected ? "" : "text-[var(--ink-muted)]"}`}>{selected ? selected.label : placeholder}</span>
        {selected?.hint ? <span className="shrink-0 text-xs text-[var(--ink-muted)]">{selected.hint}</span> : null}
        <MaterialIcon name={rect ? "expand_less" : "expand_more"} size={18} className="shrink-0 text-[var(--ink-muted)]" />
```

(其余逐字不动——弹出定位、Esc、aria-haspopup/expanded 均保持。)

- [ ] **Step 2: ConfigPanel 接线**

import 块 `@/lib/api-keys` 加入 `buildKeySelectOption`(`maskApiKey` 保留——既有密钥列表还在用)。原内联组装替换:

```tsx
                options={existingKeys.map(buildKeySelectOption)}
```

- [ ] **Step 3: 验证**

Run: `npx eslint components/ui/SelectMenu.tsx components/dashboard/ConfigPanel.tsx && pnpm test`
Expected: eslint 干净;全部 PASS(73 + Task 1 的 7 = 80)

- [ ] **Step 4: Commit**

```bash
git add frontend/components/ui/SelectMenu.tsx frontend/components/dashboard/ConfigPanel.tsx
git commit -m "feat: 密钥下拉展示类型与倍率,触发按钮回显 hint"
```

---

### Task 3: /keys KeyRow 平台背景色(含 globals.css 平台类)

**Files:**
- Modify: `frontend/app/globals.css`
- Modify: `frontend/app/(app)/keys/page.tsx`

- [ ] **Step 1: `frontend/app/globals.css` 在 `.clay-panel` 规则块(第 299 行起)之后追加两个未分层类(必须在其后定义,同特异性后者胜出;`color-mix` 组合底色,亮暗主题随 CSS 变量自动适配;实色左边框与 `.clay-panel` 的 mustard 色条先例同款手法)**

```css
.key-row-anthropic {
  border-left: 3px solid var(--accent);
  background: color-mix(in srgb, var(--paper-warm) 60%, var(--accent-wash) 40%);
}

.key-row-openai {
  border-left: 3px solid #10b981;
  background: color-mix(in srgb, var(--paper-warm) 96%, #10b981 4%);
}
```

- [ ] **Step 2: keys/page.tsx 的 import 块(`@/lib/api-keys`,第 13-25 行)加入 `platformRowClasses`**

- [ ] **Step 3: KeyRow 行容器(原 `<li className="clay-panel p-4">`,~line 503)改为**

```tsx
    <li className={`clay-panel p-4 ${platformRowClasses(apiKey.group_platform)}`}>
```

- [ ] **Step 4: 验证**

Run: `npx eslint "app/(app)/keys/page.tsx" && pnpm test && pnpm build`
Expected: eslint 干净;全部 PASS;build exit 0

- [ ] **Step 5: Commit**

```bash
git add frontend/app/globals.css "frontend/app/(app)/keys/page.tsx"
git commit -m "feat: /keys 密钥行按平台渲染左边框色条与淡背景"
```

---

### Task 4: 终验

- [ ] **Step 1: 全量** — `pnpm test` 全 PASS(基线 73 + 新增 7 = 80)
- [ ] **Step 2: build** — `pnpm build` exit 0
- [ ] **Step 3: 汇报** — `git log --oneline <base>..HEAD`、门禁输出、`git status --short`

目验(controller 与用户):配置下拉选项右端 `Anthropic ×1.5`、选中后触发按钮回显;/keys 三种平台行背景(anthropic 橙条 / openai 绿条 / 其他默认);暗色主题观感。

---

## 风险与回滚

- 全部前端、逐 task 独立 commit,可单独 revert
- SelectMenu 仅触发按钮渲染变化,弹出/关闭/键盘逻辑不动
- 两纯函数被测试锁定,平台未知/字段缺失全走回退

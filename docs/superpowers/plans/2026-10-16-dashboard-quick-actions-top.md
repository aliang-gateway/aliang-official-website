# Dashboard 快捷入口置顶(紧凑 pill 条)实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** dashboard 三个入口(配置密钥 / 工单反馈 / 历史记录)从页面底部移到 Header 正下方,改为紧凑 pill 条(图标底座 + 短标题 + 箭头),底部旧三格删除。

**Architecture:** 新建 `QuickActionsBar` 组件(纯布局,Props 仅 `onOpenTicket` + `ticketTriggerRef`),page.tsx 顶部插入、底部旧块删除;三入口行为逐字不变(2 个 Link + 工单 modal 触发 + 焦点管理)。零新增 i18n、零后端。

**Tech Stack:** Next.js、next-intl、TypeScript。

**Spec:** `docs/superpowers/specs/2026-10-16-dashboard-quick-actions-top-design.md`

---

## 背景速览(给零上下文的实施者)

- 页面 `frontend/app/(app)/dashboard/page.tsx`:当前顺序 = DashboardHeader(刷新/退出)→ StatusCard → 简报/流水两列 grid → CacheRateCard → TokenTrendCard → ModelShareCard → 底部三格 tile(~81-111 行,`{/* 入口 tile:… */}` 注释起的 grid)→ modals。
- 底部三格 = `/keys` Link(icon `key`)、工单 button(`support_agent`,`ticketTriggerRef` + `setShowTicket(true)`,Modal 关闭后焦点经 triggerRef 还原)、`/dashboard/details` Link(icon `insights`)。
- i18n key:`configApiKey` / `ticketFeedback` / `details`(dashboard namespace,**复用不新增**)。
- 工单 modal 在 page.tsx 底部,`isOpen={showTicket}`,依赖 `ticketTriggerRef` 做焦点还原——新组件必须接同一个 ref。

## 环境须知(必须遵守)

1. **所有工作在 git worktree**(sibling 惯例,如 `../aliang-official-website-quickact`,分支 `feat/dashboard-quick-actions`,基于最新 main)。
2. 装依赖用 **npm**(严禁 pnpm);测试 `pnpm test`。
3. 门禁 = 变更文件级 `npx eslint` 干净 + `pnpm test` 全过(83 基线)+ `pnpm build` exit 0。**所有命令均在 worktree 的 `frontend/` 目录下执行。**
4. commit 消息体末尾 `Co-Authored-By: Claude Code <noreply@anthropic.com>`(下文命令省略该行,提交时补上)。

## 文件结构(改动全景)

| 动作 | 路径 | 职责 |
| --- | --- | --- |
| Create | `frontend/components/dashboard/QuickActionsBar.tsx` | 顶部快捷入口条 |
| Modify | `frontend/components/dashboard/index.ts` | barrel 导出 |
| Modify | `frontend/app/(app)/dashboard/page.tsx` | 顶部插入 + 底部旧三格删除 |

---

### Task 1: QuickActionsBar 组件 + barrel

**Files:**
- Create: `frontend/components/dashboard/QuickActionsBar.tsx`
- Modify: `frontend/components/dashboard/index.ts`

- [ ] **Step 1: 新建组件(逐字)**

```tsx
"use client";

import type { RefObject } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { MaterialIcon } from "@/components/ui/MaterialIcon";

type QuickActionsBarProps = {
  onOpenTicket: () => void;
  /** 工单格 ref:modal 关闭后焦点还原(沿用 page 既有机制)。 */
  ticketTriggerRef: RefObject<HTMLButtonElement | null>;
};

const CELL_CLASS =
  "group flex min-w-0 items-center gap-3 rounded-xl px-4 py-3 text-left transition-colors hover:bg-[var(--accent-wash)]";

const ARROW_CLASS =
  "shrink-0 text-[var(--ink-faint)] transition-all duration-200 group-hover:translate-x-0.5 group-hover:text-[var(--accent)]";

/** Header 正下方的快捷入口条:配置密钥 / 工单反馈 / 历史记录。
 *  行为与原页面底部三格逐字一致(两个跳转 + 工单 modal 触发),仅位置上移、形态改紧凑 pill。 */
export function QuickActionsBar({ onOpenTicket, ticketTriggerRef }: QuickActionsBarProps) {
  const t = useTranslations("dashboard");

  return (
    <nav className="clay-panel p-2">
      <div className="grid divide-y divide-[var(--line)] sm:grid-cols-3 sm:divide-x sm:divide-y-0">
        <Link href="/keys" className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="key" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("configApiKey")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </Link>

        <button ref={ticketTriggerRef} type="button" onClick={onOpenTicket} className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="support_agent" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("ticketFeedback")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </button>

        <Link href="/dashboard/details" className={CELL_CLASS}>
          <span className="shrink-0 rounded-lg bg-[var(--accent-wash)] p-2">
            <MaterialIcon name="insights" size={18} className="text-[var(--accent)]" />
          </span>
          <span className="min-w-0 flex-1 truncate text-sm font-bold text-[var(--portal-ink)]">{t("details")}</span>
          <MaterialIcon name="arrow_forward" size={16} className={ARROW_CLASS} />
        </Link>
      </div>
    </nav>
  );
}
```

(nav 不加 aria-label——三个子项各自自述,且保持零新增 i18n key。三格显式写死而非 spec 草稿里的数组驱动——spec 该处是实现细节建议,显式三格更直白,属有意取舍。)

- [ ] **Step 2: `index.ts` barrel 追加**

```ts
export { QuickActionsBar } from "./QuickActionsBar";
```

- [ ] **Step 3: 验证**

Run: `npx eslint components/dashboard/QuickActionsBar.tsx components/dashboard/index.ts`
Expected: 干净

- [ ] **Step 4: Commit**

```bash
git add frontend/components/dashboard/QuickActionsBar.tsx frontend/components/dashboard/index.ts
git commit -m "feat: QuickActionsBar 顶部快捷入口条组件"
```

---

### Task 2: page.tsx 集成(顶部插入 + 底部删除)

**Files:**
- Modify: `frontend/app/(app)/dashboard/page.tsx`

- [ ] **Step 1: barrel 具名导入列表加入 `QuickActionsBar`**

- [ ] **Step 2: `<DashboardHeader …/>` 之后插入**

```tsx
      <QuickActionsBar onOpenTicket={() => setShowTicket(true)} ticketTriggerRef={ticketTriggerRef} />
```

- [ ] **Step 3: 删除底部旧三格**——`{/* 入口 tile:密钥与配置(跳转页) / 工单(modal) / 详情(跳转,最右) */}` 注释起、含三格的整个 `<div className="grid gap-4 sm:grid-cols-3">…</div>` 区块整体删除。删除后确认:page 内不再有指向 `/keys`、`/dashboard/details` 的 Link(唯一入口已上移);`ticketTriggerRef` 仅由 QuickActionsBar 持有;**`MaterialIcon` import 若因此不再被 page 使用则一并移除**(旧三格是它在本页的唯一用点),保持 eslint 字面干净。

- [ ] **Step 4: 验证**

Run: `npx eslint "app/(app)/dashboard/page.tsx" && pnpm test && pnpm build`
Expected: eslint 干净;83/83;build exit 0

- [ ] **Step 5: Commit**

```bash
git add "frontend/app/(app)/dashboard/page.tsx"
git commit -m "feat: 快捷入口条置顶,移除底部旧三格"
```

---

### 终验(整体终审后,controller 与用户做)

目验:三入口(2 跳转 + 1 modal)行为不变、工单 modal 关闭后焦点还原到工单 pill、hover(底色 + 箭头右移变色)、窄屏三格纵排、暗色主题、页尾不再有旧三格。

---

## 风险与回滚

- 纯前端布局,2 commit 可单独 revert
- 工单焦点管理机制未动(同一 ref 传递),modal 既有逻辑零改动

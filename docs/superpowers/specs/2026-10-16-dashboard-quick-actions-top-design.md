# Dashboard 快捷入口置顶(紧凑 pill 条)设计

- 日期:2026-10-16
- 状态:设计方向已与需求方确认(紧凑 pill 条,Header 正下方)
- 范围:仅前端 `frontend/`,零后端改动

## 背景

dashboard(`frontend/app/(app)/dashboard/page.tsx`)的三个入口——配置与 API 密钥(→ `/keys`)、工单与反馈(→ modal)、历史记录(→ `/dashboard/details`)——目前是页面**最底部**的三格扁条(纯「图标+文字+箭头」),位置差(要滚到底才能看到)且样式单薄。

需求方确认:**入口移至顶部(DashboardHeader 正下方、StatusCard 之前),形态取紧凑 pill 条**(一行三格,图标+短标题;非图文卡)。

## 需求决策(已确认)

| 决策点 | 结论 |
| --- | --- |
| 位置 | DashboardHeader 正下方、StatusCard 之前,全宽 |
| 形态 | 紧凑 pill 条:一张 clay-panel 容器内三等分,每格 = 图标底座 + 短标题 + 箭头 |
| 底部原三格 | **删除**(入口唯一化,不留重复) |
| 行为 | 三个入口行为逐字不变(两个 Link 跳转 + 工单 modal 触发,保留 focus 管理) |
| 文案 | 全部复用现有 i18n key(`configApiKey`/`ticketFeedback`/`details`),零新增 |

被否掉的备选:图文快速操作卡(信息足但用户选了紧凑型);融入 Header 右侧(空间挤);底部保留入口(重复)。

## 设计

### 1. QuickActionsBar 组件

新建 `frontend/components/dashboard/QuickActionsBar.tsx`(barrel 导出):

- Props:`onOpenTicket: () => void`、`ticketTriggerRef: RefObject<HTMLButtonElement | null>`(沿用 page.tsx 既有工单 modal 触发与焦点管理;两个 Link 内置 href)
- 结构:外层 `<nav className="clay-panel p-2">`(语义:页面快捷导航),内部 `<div className="grid sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-[var(--line)]">` 三格;每格为整格可点元素(Link 或 button),内容横排:
  - **图标底座**:小圆角方块(`rounded-lg bg-[var(--accent-wash)] p-2` 内嵌 `MaterialIcon`,accent 色)——精致感来源
  - **短标题**:`text-sm font-bold`,复用现有 key
  - **箭头**:`MaterialIcon arrow_forward`,默认 `text-[var(--ink-faint)]`,格 hover 时 `text-[var(--accent)] translate-x-0.5`(transition)
  - 整格 hover:底色淡染 `hover:bg-[var(--accent-wash)]` + 圆角(容器 p-2 内格自然圆角)
- 三格定义用局部数组驱动(`{icon, labelKey, render}`):配置密钥=Link `/keys`(icon `key`)、工单反馈=button(icon `support_agent`)、历史记录=Link `/dashboard/details`(icon `insights`)——与被删除的旧三格逐字同源,行为零变化
- focus/焦点管理:工单格沿用 `ticketTriggerRef`(modal 关闭后焦点还原,既有机制)

### 2. page.tsx 布局

- `<DashboardHeader>` 之后插入 `<QuickActionsBar onOpenTicket={() => setShowTicket(true)} ticketTriggerRef={ticketTriggerRef} />`
- 删除底部原「入口 tile」区块(注释 ` {/* 入口 tile:… */}` 起的整个 grid)
- 其余卡片顺序不变:Header → **QuickActionsBar** → StatusCard → CacheRateCard → TokenTrendCard → ModelShareCard → modals

### 3. 样式与一致性

- 全部用既有设计系统:`clay-panel`、`--accent*`/`--portal-*` CSS 变量、`MaterialIcon`;不引入新色值
- 亮暗主题自适应(变量驱动);移动端三格纵排(divide-y 转横线)

## 错误处理

无新增失败面:纯布局组件;Link/button 行为与被删三格逐字一致;无数据请求。

## 验证

- 变更文件级 `npx eslint` 干净;`pnpm test` 全过(无新增测试——纯布局组件,库内组件无测试惯例);`pnpm build` exit 0
- 手测:三入口(2 跳转 + 1 modal)正常、工单 modal 关闭后焦点还原、hover 效果(底色/箭头)、窄屏纵排、暗色主题

## 非目标(YAGNI)

- 不做图文大卡(用户明确选紧凑型)
- 不改三个入口的目标页面内容
- 不新增 i18n key、不改 Header(刷新/退出按钮保持原位)
- 不改后端与 BFF

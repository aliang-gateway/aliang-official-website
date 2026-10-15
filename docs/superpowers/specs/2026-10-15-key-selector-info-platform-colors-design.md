# 密钥下拉信息增强 + /keys 平台背景色 设计

- 日期:2026-10-15
- 状态:设计方向已与需求方确认(双槽 hint 方案 / 行背景色表)
- 范围:仅前端 `frontend/`,零后端改动

## 背景

配置面板(ConfigPanel,dashboard 弹窗与 /keys 配置 tab 共用)的「选择已有密钥」自绘下拉(SelectMenu)目前选项只显示 `名称 · 略写`,信息太薄:用户无法在下拉里区分密钥的平台类型与计费倍率,而这两项直接影响该密钥生成的配置是否合用。

数据现成:`ApiKeyItem`(frontend/lib/api-keys.ts)已含 `group_platform`(openai/anthropic/其他)与 `group_rate_multiplier`;工具 `platformBadgeLabel()`(→ OpenAI/Anthropic/其他)与 `formatRateMultiplier()`(→ ×1/×1.5)均已存在并用于 /keys 页徽章。

/keys 页的密钥列表(KeyRow,frontend/app/(app)/keys/page.tsx)所有行统一 `clay-panel` 背景,平台只是行内小徽章,扫视时无法按类型区分。

## 需求决策(已确认)

| 决策点 | 结论 |
| --- | --- |
| 下拉信息承载 | 用 SelectMenu 既有双槽:label = `名称 · 略写`(不变),hint = `类型 ×倍率` 右对齐小字;不改 SelectMenu 选项结构 |
| 选中回显 | SelectMenu 触发按钮增强:选中后 hint 也回显(右侧小字),否则选完类型信息丢失 |
| /keys 背景色 | KeyRow 行容器加「左边框色条 + 淡背景」,按 group_platform 区分;沿用 ConfigPanel 敏感警告的 `border-l-[3px]` 既有模式 |

被否掉的备选:改造 SelectMenu 支持富渲染(render prop)——双槽已够,避免组件复杂化;仅给平台徽章换色——视觉权重不够,用户明确要行背景色。

## 设计

### 1. 选项组装函数(lib 层,便于单测)

`frontend/lib/api-keys.ts` 新增:

```ts
export type KeySelectOption = { value: string; label: string; hint: string };
export function buildKeySelectOption(item: ApiKeyItem): KeySelectOption {
  const label = `${item.name || `Key #${item.id}`} · ${maskApiKey(item.key)}`;
  const badge = item.group_platform ? platformBadgeLabel(item.group_platform) : "";
  const rate = formatRateMultiplier(item.group_rate_multiplier);
  return { value: String(item.id), label, hint: badge ? `${badge} ${rate}` : rate };
}
```

(platform 缺失时 hint 只显示倍率;name/key/platform/倍率任意缺失均有既有回退:`Key #id`、`***`、`其他`、`×1`。)

### 2. SelectMenu 触发按钮回显 hint

`frontend/components/ui/SelectMenu.tsx` 触发按钮:当前只渲染 `selected.label`。改为 label + hint 双段(与下拉项一致的两端布局):label 左(`truncate`),`selected.hint` 存在时右侧渲染小字 hint(`shrink-0 text-xs text-[var(--ink-muted)]`,现有下拉项 hint 同款样式),再右侧 expand 图标。placeholder/未选中态行为不变。其余(弹出定位、Esc、aria)逐字不动。

### 3. ConfigPanel 接线

`frontend/components/dashboard/ConfigPanel.tsx` 既有密钥下拉的 options 组装(现内联 `existingKeys.map(...)` 拼 label)改为 `existingKeys.map(buildKeySelectOption)`。显示行为:下拉项 = 左 `名称 · 略写`、右 `Anthropic ×1.5`。

### 4. /keys 页 KeyRow 平台背景色

`frontend/app/(app)/keys/page.tsx` KeyRow 行容器 `<li className="clay-panel p-4">` 改为按平台附加左边框色条与淡背景(色条模式沿用 ConfigPanel 敏感警告 `border-l-[3px] border-l-[var(--mustard)]` 的既有先例):

| group_platform(normalizeGroupPlatform 归一后小写比较) | 附加类 |
| --- | --- |
| `anthropic` | `.key-row-anthropic` |
| `openai` | `.key-row-openai` |
| 其他/缺失 | 无附加(默认 clay-panel) |

⚠️ 实现约束:`.clay-panel` 是 globals.css 里的**未分层**手写 CSS,按 CSS 级联层规则会压过 `@layer utilities` 的 Tailwind v4 工具类——因此两个平台类必须以未分层类形式定义在 globals.css 中 `.clay-panel` 规则之后(同特异性,后者胜出),用 `color-mix` 组合底色(随 CSS 变量自动适配亮暗主题),实色 3px 左边框沿用 mustard 色条先例。`platformRowClasses(platform)`(放 api-keys.ts)返回上述类名,未知平台返回空串。行内既有平台徽章、倍率徽章保持不变(信息双通道,不删旧展示)。

### 5. 风格一致性

- 不引入新色值体系:anthropic 用品牌 `var(--accent)`/`var(--accent-wash)`(行内徽章同源),openai 用 emerald(代码库既有语义色,dark 变体已有先例),半透明 tint 亮暗主题均可用
- 下拉 hint 的排版与 /keys 徽章文案同源同词(platformBadgeLabel/formatRateMultiplier)

## 错误处理

无新增失败面:纯展示分支;platform/倍率/name/key 缺失全部走既有回退值;SelectMenu 改动仅触发按钮渲染,弹出/关闭逻辑不动。

## 验证

- `pnpm test`(frontend):`frontend/lib/__tests__/api-keys.test.ts` 追加 `buildKeySelectOption` 用例——正常项(label/hint 组装)、platform 缺失(hint 仅倍率)、name 缺失(`Key #id`)、倍率小数(×1.5)
- 变更文件级 `npx eslint` 干净;`pnpm test` 全过;`pnpm build` 通过
- 手测:dashboard 弹窗与 /keys 配置 tab 的下拉(选项带类型/倍率、选中后触发按钮回显)、/keys 行背景三态(anthropic/openai/其他)、暗色主题观感

## 非目标(YAGNI)

- 不改 SelectMenu 的弹出/定位/键盘逻辑
- 不给 ConfigPanel 左栏「已有密钥列表」加平台背景色(用户仅要求 /keys 页;列表已有平台徽章)
- 不引入第四种平台色(未知平台保持默认)
- 不改后端与 BFF

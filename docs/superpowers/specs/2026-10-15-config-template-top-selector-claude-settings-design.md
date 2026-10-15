# 配置生成优化:模板置顶选择 + Claude settings.json 格式 设计

- 日期:2026-10-15
- 状态:设计方向已与需求方确认(顶部全宽横排 / 两个子需求均取方案 A)
- 范围:仅前端 `frontend/`,零后端改动

## 背景

自动生成客户端配置的功能(`ConfigPanel`,被 dashboard 的 `ConfigModal` 与 `/keys` 页「配置」tab 共用)现状:

1. **模板选择位置差**:opencode/claude/codex 三张模板卡放在左栏最底部,用户要先滚过密钥区才能看到,使用顺序颠倒(应先选模板再配密钥)。
2. **Claude 只有 env 一种形态**:`buildClaudeConfig` 仅生成终端 `export ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN` 的 shell 片段。Claude Code 官方支持在 `~/.claude/settings.json` 的 `env` 字段注入环境变量,是更持久的配置方式,应一并支持。

相关现状事实(已核实):

- `frontend/components/dashboard/ConfigPanel.tsx`(257 行):左栏(280px)自上而下 = 选已有密钥下拉 → 底层密钥 textarea → 清除按钮 → 已有密钥列表 → 敏感警告 → **模板选择块(底部)**;右栏 = 当前模板 eyebrow + 渲染结果标题 + 格式 pills(`supportedFormats`)+ 暗色代码块 + 网关地址卡 + 复制按钮。
- `frontend/lib/dashboard-template.ts`:`TEMPLATE_DEFINITIONS`(opencode→`["json"]`、claude→`["shell"]`、codex→`["toml"]`);`buildTemplateContent(templateId, userKey, models?)` 按 id 分发到 `buildOpencodeConfig` / `buildClaudeConfig` / `buildCodexConfig`;`GATEWAY_BASE_URL` 不含 `/v1`。
- `frontend/lib/hooks/use-config-modal.ts`:状态 `template`(默认 opencode)+ `format`(默认 json);既有联动 effect——切模板时若当前 format 不在新模板 `supportedFormats` 内则回退到首个支持格式;`renderedConfig` memo 调 `buildTemplateContent(template, userKey.trim(), availableModels)`;密钥经 `/api/models` 去抖拉取可用模型。

## 需求决策(已确认)

| 决策点 | 结论 |
| --- | --- |
| 模板区形态 | **顶部全宽横排**:三张模板卡横排在面板最顶部(密钥/渲染两栏之上) |
| 组件化 | 抽独立 `TemplateSelector` 子组件(ConfigPanel 已 257 行,选择器职责单一) |
| claude settings.json | 走**现有格式切换机制**:`supportedFormats` 扩为 `["shell", "json"]`,`buildTemplateContent` 加 format 参数;不新增模板卡、不新增独立开关 |
| 格式默认 | shell 在前(env 仍是主路径);格式联动沿用既有状态机,不改 |

被否掉的备选:模板块仅左栏内上移(不够醒目);拆第四张模板卡 "claude-settings"(语义重复、类型扩散);独立"输出方式"开关(格式 pills 已是切换器)。

## 设计

### 1. TemplateSelector 组件与布局重排

新建 `frontend/components/dashboard/TemplateSelector.tsx`:

- Props:`template: ClientTemplateId`、`onTemplateChange: (value: ClientTemplateId) => void`
- 渲染:标签(`t("template")`)+ 三张模板卡横排(`grid gap-2 sm:grid-cols-3`),卡内容与样式沿用现左栏版(标题 `t(labelKey)` + 说明 `t(helperKey)`;选中 `border-[var(--accent)]/40 bg-[var(--accent-wash)]`,未选中 hover 浮起)
- 加入 `frontend/components/dashboard/index.ts` barrel(供 /keys 页等复用)

`ConfigPanel.tsx` 改动:

- 删除左栏底部模板选择块
- 在两栏 `grid lg:grid-cols-[280px_minmax(0,1fr)]` 之上插入全宽 `<TemplateSelector>`(外层留出与既有 spacing 一致的间距)
- 其余(左栏密钥区、右栏渲染区/格式 pills/复制)不动;右栏头部的当前模板 eyebrow 继续反映选中模板

两个消费方(dashboard `ConfigModal`、`/keys` 配置 tab)经 ConfigPanel 自动获得新布局,无需各自改动。

### 2. claude settings.json 格式(dashboard-template.ts)

- claude 定义:`supportedFormats: ["shell", "json"]`(顺序即 pills 顺序,shell 默认)
- `buildTemplateContent` 签名扩为 `(templateId, userKey, models?, format?: TemplateFormat)`;仅 claude 消费 format,opencode/codex 忽略(单格式模板,行为逐字不变)
- `buildClaudeConfig(userKey, format)`:
  - `"shell"`:输出逐字不变(`export ANTHROPIC_BASE_URL='…'` + `export ANTHROPIC_AUTH_TOKEN='…'` + `claude`,沿用 `escapeSingleQuotedShell`)
  - `"json"`:`JSON.stringify({ env: { ANTHROPIC_BASE_URL: GATEWAY_BASE_URL, ANTHROPIC_AUTH_TOKEN: userKey } }, null, 2)`——Claude Code settings.json 的官方 env 注入形态;转义全权交给 JSON.stringify,密钥含引号/控制字符亦安全
- JSON 不可内嵌注释,与 codex toml 的内嵌步骤注释不同:落盘指引由 helper 文案承担(见第 4 节)

### 3. 状态与数据流

- `use-config-modal.ts` 的 `renderedConfig` memo 改为 `buildTemplateContent(template, userKey.trim(), availableModels, format)`,**format 必须同时加入 memo 依赖数组**
- 格式联动沿用既有 effect:切模板时 format 不受支持才回退。推论:opencode(json)→claude 停留 json(settings.json 视图),codex(toml)→claude 回退 shell——已上线的可预期行为,不改
- `/keys` 页与 dashboard 共用同一 hook 与 ConfigPanel,自动一致

### 4. i18n

- 更新 `templateClaudeHelper`(zh/en):覆盖两条落盘路径——shell → 终端 export 后运行 `claude`;json → 写入或合并 `~/.claude/settings.json`
- 顶部选择区标签复用既有 `t("template")`,不新增 key(helper 两个 key 名不变,只改值);`copyIdle` 文案两侧语言本就写明「当前模板与格式」,确认无需改动

### 5. 错误处理

无新增失败面:纯前端渲染分支;format 参数缺失时 claude 默认 shell(向后兼容任何旧调用);JSON.stringify 对任意字符串安全。

## 验证

- `pnpm test`(frontend):新增 `frontend/lib/__tests__/dashboard-template.test.ts`
  - claude shell:输出含两行 export(单引号转义正确,密钥含 `'` 时断言 `'\''` 序列)与 `claude` 行
  - claude json:`JSON.parse` 通过、`env.ANTHROPIC_BASE_URL === GATEWAY_BASE_URL`、`env.ANTHROPIC_AUTH_TOKEN` 等于输入密钥;密钥含 `"` 与换行时 parse 仍成功且值无损
  - `escapeSingleQuotedShell` 边界(空串、多引号)
  - opencode/codex:传不传 format 输出一致(回归保障)
- 变更文件级 `npx eslint` 干净;`pnpm build` 通过
- 手测:dashboard 弹窗与 /keys tab 顶部横排渲染、claude pills 切 shell/json 输出联动、复制内容随格式变化、格式联动(opencode→claude 停 json;codex→claude 落 shell)

## 非目标(YAGNI)

- 不新增第四张模板卡、不新增独立"输出方式"开关
- 不改 opencode/codex 模板内容与行为
- 不做 claude json 的模型注入(settings.json env 形态不含模型选择;Claude Code 自行协商模型)
- 不改后端与 BFF

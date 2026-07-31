# service 列表项「项目预览」链接（GitHub / 代码仓库）

## Overview

在 `/services` 页的研究方向 + 已落地能力时间线（`als_service_directions`）每条记录上，新增一个可选的**代码仓库链接**入口：管理员可在 `/admin/services` 编辑表单里为每条记录填写 `repo_url`；当该字段非空时，前端在对应列表项的描述下方渲染一个「查看项目」链接（新标签打开）；为空则不渲染。链接图标根据 URL 是否为 GitHub 自动切换（GitHub 地址显示 GitHub mark，其他地址显示通用外链图标）。

## Background

- `/services` 时间线由后端 `als_service_directions` 表驱动（迁移 `0025`），字段：`id, status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, sort_order, is_published, created_at, updated_at`。
- 链路：表 → `model.ServiceDirection`（`backend/internal/model/entities.go:168`）→ `servicedirection.Service`（`backend/internal/servicedirection/service.go`，含 `columns` 常量 / `scanServiceDirection` / `normalizeAndValidate` / CRUD）→ `httpapi/service_direction_handlers.go`（`serviceDirectionRequest` / `serviceDirectionResponse` / `publicServiceDirectionResponse` / `toModel` / `*ToResponse`）→ 路由 `/admin/services*`（CRUD）与 `/public/services`（公开列表）。
- 前端 admin：`(app)/admin/services/page.tsx`，每字段一个 `useState`，`handleEdit` 回填、`handleCreateOrUpdate` 提交 JSON body。
- 前端展示：`(marketing)/services/page.tsx`（`"use client"`），从 `/api/public/services?lang=` 取列表渲染 `timeline-item`（phase / title / desc / status）。
- 透传路由 `app/api/admin/services*` 与 `app/api/public/services/route.ts` 原样转发 body / response，新增字段无需改动它们。
- 双数据库驱动（sqlite / postgres），迁移成对存在；最新迁移编号 `0028`，下一个 `0029`。
- i18n：`messages/zh.json`、`messages/en.json`，`editorial.services` 命名空间。

## Goals

1. 每条 service 项可携带一个可选代码仓库 URL（`repo_url`）。
2. `/admin/services` 表单可填写该字段；空值合法（不强制）。
3. `/services` 列表项在 `repo_url` 非空时渲染「查看项目」外链（新标签、`noopener noreferrer`）；为空不渲染。
4. 链接图标自适应：URL 命中 GitHub → GitHub mark 图标；否则 → 通用外链图标。
5. 全栈贯通、双驱动迁移、含后端校验与测试。

## Non-Goals

- 多链接（demo / 文档 / 多仓库）：本次单字段 `repo_url`，YAGNI。
- 链接预览卡 / OG 抓取：仅外链跳转，不内嵌预览。
- `hreflang` 等无关改造。

## Decisions（已与用户确认）

| 维度 | 决策 |
|---|---|
| 字段名 | `repo_url`（通用代码仓库；当前用于 GitHub，未来可填 GitLab/Gitee 等） |
| 默认值 | `''`（空串）；空 → 前端不渲染入口 |
| 后端校验 | `trim`；非空时必须 `http(s)://` 合法 URL；**不强制** `github.com` |
| 入口形态 | GitHub mark（或通用图标）+ 「查看项目 / View project」文字小链接，位于描述下方 |
| 图标检测 | 前端 `isGithubUrl(url)` = URL 含 `github.com`（大小写不敏感）→ GitHub mark SVG；否则通用外链 SVG |
| 打开方式 | `<a target="_blank" rel="noopener noreferrer">` |
| 文案 | 中英双语，走 `editorial.services.viewProject` |

## Database Migration

新迁移 `0029_add_service_direction_repo_url.sql`，sqlite 与 postgres 各一份。

sqlite:
```sql
ALTER TABLE als_service_directions ADD COLUMN repo_url TEXT NOT NULL DEFAULT '';
```
postgres:
```sql
ALTER TABLE als_service_directions ADD COLUMN repo_url TEXT NOT NULL DEFAULT '';
```

> 不 seed 任何值（现有 6 条记录 `repo_url` 默认 `''`，即默认不显示入口，符合「空则不展示」）。

## Backend Changes (Go)

1. **`model/entities.go`** `ServiceDirection` 增加 `RepoURL string`（对应 db 列 `repo_url`）。
2. **`servicedirection/service.go`**：
   - `columns` 常量追加 `repo_url`。
   - `scanServiceDirection` 增加扫描 `&sd.RepoURL`。
   - `normalizeAndValidate`：`sd.RepoURL = strings.TrimSpace(sd.RepoURL)`；若非空且不是合法 URL（没有 `http://`/`https://` 前缀）→ 返回 `errors.New("repo_url must be an http(s) URL")`。
   - `Create` 的 `INSERT` 列与 `VALUES` 占位符、参数列表追加 `repo_url` / `sd.RepoURL`。
   - `Update` 的 `SET` 与参数列表追加 `repo_url = ?` / `sd.RepoURL`。
   - `Get` / `ListAll` / `ListPublic` 因使用 `columns` 常量自动覆盖。
3. **`httpapi/service_direction_handlers.go`**：
   - `serviceDirectionRequest` 增 `RepoURL string `json:"repo_url"``。
   - `serviceDirectionResponse` 增 `RepoURL string `json:"repo_url"``。
   - `toModel` 设置 `RepoURL: req.RepoURL`。
   - `serviceDirectionToResponse` 设置 `RepoURL: sd.RepoURL`。
   - `publicServiceDirectionResponse` 增 `RepoURL string `json:"repo_url"``（公开接口需返回，前端展示用）。
   - `publicServiceDirectionToResponse` 设置 `RepoURL: sd.RepoURL`（与语言无关，直接透传）。

## Frontend Changes

### `app/(app)/admin/services/page.tsx`
- `ServiceDirection` 类型增 `repo_url: string`。
- 新增 `const [repoUrl, setRepoUrl] = useState("")`。
- `resetForm` 重置 `setRepoUrl("")`。
- `handleEdit` 回填 `setRepoUrl(it.repo_url ?? "")`。
- `handleCreateOrUpdate` 的提交 `body` 增 `repo_url: repoUrl.trim()`。
- 表单 JSX 增一个文本输入框（label「代码仓库地址（可选）」/ "Repo URL (optional)"，placeholder `https://github.com/...`），放在描述字段之后、排序/发布开关之前。

### `app/(marketing)/services/page.tsx`
- `ServiceItem` 类型增 `repo_url?: string`。
- 在 `timeline-item` 描述 `<p>{it.desc}</p>` 之后，条件渲染：
  ```tsx
  {it.repo_url && (
    <a
      className="tool-repo-link"  // 复用 editorial 风格，新增一个低调链接样式
      href={it.repo_url}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={s("viewProject")}
    >
      {isGithubUrl(it.repo_url) ? <GithubMarkIcon /> : <ExternalLinkIcon />}
      <span>{s("viewProject")}</span>
    </a>
  )}
  ```
- 新增辅助：
  - `function isGithubUrl(url: string): boolean { return /github\.com/i.test(url); }`
  - `GithubMarkIcon`：内联 GitHub mark SVG（标准官方 path），尺寸 ~16px。
  - `ExternalLinkIcon`：内联通用「外链」SVG（↗ 风格），尺寸 ~16px。
  - 三个 helper 可放在文件顶部或同目录小文件；与 editorial 内联 SVG 风格一致（不引入 MaterialIcon 依赖，marketing 页未使用）。

### `app/(marketing)/editorial.css`
- 追加 `.tool-repo-link`（或更贴切的类名 `.service-repo-link`）样式：inline-flex、align-items center、gap、小号字、accent 色、hover 下划线。复用现有变量（`--accent` / `--ink-muted`）。

### `messages/zh.json` / `en.json`
- `editorial.services` 增：
  - zh: `"viewProject": "查看项目"`
  - en: `"viewProject": "View project"`

## Verification

- **后端单测**（`backend/internal/servicedirection/`，沿用现有测试模式）：
  1. `Create` / `Update` 携带合法 `repo_url`（https github）→ 落库并可 `Get` 读回。
  2. `repo_url` 为空 → 合法（不报错）。
  3. `repo_url` 非空但非 http(s)（如 `ftp://x` 或 `github.com/x` 无协议）→ `normalizeAndValidate` 报错。
  4. `ListPublic` 返回项含 `repo_url` 字段。
  - 跑 `cd backend && go test ./internal/servicedirection/...`，并视情跑 `./internal/httpapi/` 相关 handler 测试。
- **迁移**：sqlite + postgres 各自执行 `0029` 成功，既有数据 `repo_url` 为 `''`。
- **前端**：`npm run build` 通过；手动验证：admin 填 `https://github.com/foo/bar` 保存 → `/services` 该项描述下出现 GitHub 图标 +「查看项目」→ 点击新标签打开 github；改为非 github URL（如 `https://gitlab.com/x`）→ 图标切换为通用外链；清空保存 → 入口消失。
- **双驱动**：迁移与 model 在 sqlite / postgres 均通过（项目 `DB_DRIVER` 双支持）。

## Risks & Notes

- **图标检测仅前端、基于 host 字符串**：`github.com` 子串匹配足以覆盖典型 GitHub URL；非 GitHub（GitLab/Gitee/自建）统一用通用图标。不做后端 host 校验（YAGNI，且 `repo_url` 通用）。
- **公开接口暴露 `repo_url`**：该字段本就面向访客展示，无敏感性问题。
- **空值语义**：`'' NOT NULL DEFAULT ''`——用空串而非 NULL，简化 Go `string` 映射与前端 falsy 判断（`it.repo_url` 为空串即不渲染）。
- **既有数据**：6 条 seed 记录迁移后 `repo_url=''`，默认无入口，符合「空则不展示」。

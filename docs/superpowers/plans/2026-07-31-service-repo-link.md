# service 列表项「项目预览」链接 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `/services` 时间线每条记录增加可选 `repo_url` 字段，admin 可填、空则不展示，前端按是否 GitHub 切换图标，全栈贯通。

**Architecture:** 后端 Go 加一列（双驱动迁移 0029）→ 贯穿 model / service / handler（含公开接口透传）→ 前端 admin 表单填写 + `/services` 条件渲染（GitHub mark / 通用外链图标）。透传路由无需改。

**Tech Stack:** Go（database/sql, sqlite + postgres 双驱动, embed 迁移）· Next.js 16 App Router · next-intl · editorial.css。

**Spec:** `docs/superpowers/specs/2026-07-31-service-repo-link-design.md`
**分支:** `feat/service-repo-link`（spec 已提交 `84396c7`）。

---

## File Structure

### 新增
| 文件 | 职责 |
|---|---|
| `backend/migrations/sqlite/0029_add_service_direction_repo_url.sql` | sqlite 加列 |
| `backend/migrations/postgres/0029_add_service_direction_repo_url.sql` | postgres 加列 |
| `backend/internal/servicedirection/service_test.go` | service 层单测（repo_url 校验 + CRUD） |

### 修改
| 文件 | 改动 |
|---|---|
| `backend/internal/model/entities.go` | `ServiceDirection` 加 `RepoURL string` |
| `backend/internal/servicedirection/service.go` | `columns`/`scan`/`validate`/`Create`/`Update` 带 `repo_url` |
| `backend/internal/httpapi/service_direction_handlers.go` | request/response/toModel/admin+public `*ToResponse` 加 `RepoURL` |
| `frontend/app/(app)/admin/services/page.tsx` | 类型/state/reset/handleEdit/body/表单输入 |
| `frontend/app/(marketing)/services/page.tsx` | `ServiceItem.repo_url`、`isGithubUrl`、两个图标组件、条件渲染 |
| `frontend/app/(marketing)/editorial.css` | `.service-repo-link` 样式 |
| `frontend/messages/zh.json` / `en.json` | `editorial.services.viewProject` |

---

## Task 1: 后端数据层（迁移 + model + service + 测试，TDD）

**Files:**
- Create: `backend/migrations/sqlite/0029_add_service_direction_repo_url.sql`
- Create: `backend/migrations/postgres/0029_add_service_direction_repo_url.sql`
- Modify: `backend/internal/model/entities.go:168-181`
- Modify: `backend/internal/servicedirection/service.go`
- Create: `backend/internal/servicedirection/service_test.go`

- [ ] **Step 1: 写两份迁移**

`backend/migrations/sqlite/0029_add_service_direction_repo_url.sql`:
```sql
-- Optional code repository URL for each service direction (empty = no link shown).
ALTER TABLE als_service_directions ADD COLUMN repo_url TEXT NOT NULL DEFAULT '';
```

`backend/migrations/postgres/0029_add_service_direction_repo_url.sql`:
```sql
-- Optional code repository URL for each service direction (empty = no link shown).
ALTER TABLE als_service_directions ADD COLUMN repo_url TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: model 加字段**

在 `backend/internal/model/entities.go` 的 `ServiceDirection` struct 里，`DescEn string` 之后加一行：
```go
	RepoURL     string
```
（放在 `DescEn` 与 `SortOrder` 之间。）

- [ ] **Step 3: 写失败测试** `backend/internal/servicedirection/service_test.go`:
```go
package servicedirection

import (
	"context"
	"path/filepath"
	"testing"

	db "ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

func newService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, "sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	return NewService(database, "sqlite"), ctx
}

func validDirection(repoURL string) *model.ServiceDirection {
	return &model.ServiceDirection{
		Status:      "done",
		PhaseZh:     "阶段", PhaseEn: "phase",
		TitleZh:     "标题", TitleEn: "title",
		DescZh:      "描述", DescEn: "desc",
		RepoURL:     repoURL,
		SortOrder:   1,
		IsPublished: true,
	}
}

func TestRepoURL_CreateAndGet(t *testing.T) {
	svc, ctx := newService(t)
	sd := validDirection("https://github.com/foo/bar")
	if err := svc.Create(ctx, sd); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := svc.Get(ctx, sd.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.RepoURL != "https://github.com/foo/bar" {
		t.Fatalf("RepoURL = %q, want github url", got.RepoURL)
	}
}

func TestRepoURL_EmptyAllowed(t *testing.T) {
	svc, ctx := newService(t)
	sd := validDirection("")
	if err := svc.Create(ctx, sd); err != nil {
		t.Fatalf("Create() with empty repo_url error = %v", err)
	}
	got, _ := svc.Get(ctx, sd.ID)
	if got.RepoURL != "" {
		t.Fatalf("RepoURL = %q, want empty", got.RepoURL)
	}
}

func TestRepoURL_RejectsNonHTTP(t *testing.T) {
	svc, ctx := newService(t)
	for _, bad := range []string{"github.com/foo", "ftp://x", "not a url"} {
		sd := validDirection(bad)
		if err := svc.Create(ctx, sd); err == nil {
			t.Fatalf("Create() with %q expected error, got nil", bad)
		}
	}
}

func TestRepoURL_Update(t *testing.T) {
	svc, ctx := newService(t)
	sd := validDirection("")
	if err := svc.Create(ctx, sd); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	sd.RepoURL = "https://github.com/foo/baz"
	if err := svc.Update(ctx, sd.ID, sd); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got, _ := svc.Get(ctx, sd.ID)
	if got.RepoURL != "https://github.com/foo/baz" {
		t.Fatalf("RepoURL after update = %q", got.RepoURL)
	}
}
```
（`newService` 用临时 sqlite 跑全量迁移，建表含新列 `repo_url`。）

- [ ] **Step 4: 跑测试，确认失败**

Run（从 `backend/`）: `go test ./internal/servicedirection/...`
Expected: 编译失败 / FAIL（`RepoURL` 字段不存在、columns/INSERT 未含 repo_url）

- [ ] **Step 5: 改 `service.go`**

在 `backend/internal/servicedirection/service.go`：

(a) `columns` 常量（第 33 行）改为（在 `desc_en,` 之后、`sort_order,` 之前插入 `repo_url,`）：
```go
const columns = `id, status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, repo_url, sort_order, is_published, created_at, updated_at`
```

(b) `scanServiceDirection`（第 35-40 行）在 `&sd.DescEn,` 之后加 `&sd.RepoURL,`：
```go
func scanServiceDirection(sd *model.ServiceDirection, scanner interface{ Scan(...any) error }) error {
	return scanner.Scan(
		&sd.ID, &sd.Status, &sd.PhaseZh, &sd.PhaseEn, &sd.TitleZh, &sd.TitleEn, &sd.DescZh, &sd.DescEn,
		&sd.RepoURL,
		&sd.SortOrder, &sd.IsPublished, &sd.CreatedAt, &sd.UpdatedAt,
	)
}
```

(c) `normalizeAndValidate`（第 42-63 行）在 trim `DescEn` 之后、校验 status 之前，加 repo_url 的 trim + URL 校验：
```go
	sd.RepoURL = strings.TrimSpace(sd.RepoURL)
	if sd.RepoURL != "" && !strings.HasPrefix(sd.RepoURL, "http://") && !strings.HasPrefix(sd.RepoURL, "https://") {
		return errors.New("repo_url must be an http(s) URL")
	}
```

(d) `Create` 的 INSERT（第 70-75 行）列、占位符、参数都加 repo_url：
```go
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_service_directions (status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, repo_url, sort_order, is_published, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"id",
		sd.Status, sd.PhaseZh, sd.PhaseEn, sd.TitleZh, sd.TitleEn, sd.DescZh, sd.DescEn, sd.RepoURL, sd.SortOrder, sd.IsPublished, now, now,
	)
```
（占位符数量 = 12 个 `?`，对应 12 个值列；`id` 由 InsertID 返回。）

(e) `Update` 的 SET（第 103-108 行）加 `repo_url = ?` 与参数：
```go
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_service_directions
		SET status = ?, phase_zh = ?, phase_en = ?, title_zh = ?, title_en = ?, desc_zh = ?, desc_en = ?, repo_url = ?, sort_order = ?, is_published = ?, updated_at = ?
		WHERE id = ?`),
		sd.Status, sd.PhaseZh, sd.PhaseEn, sd.TitleZh, sd.TitleEn, sd.DescZh, sd.DescEn, sd.RepoURL, sd.SortOrder, sd.IsPublished, now, id,
	)
```

- [ ] **Step 6: 跑测试，确认通过**

Run: `go test ./internal/servicedirection/...`
Expected: PASS（4 tests）

- [ ] **Step 7: 跑迁移成对校验 + 全量后端测试**

Run: `go test ./internal/db/...`（确认 0029 sqlite/postgres 成对，`migrate_test` 通过）
Run: `go test ./...`（确认无回归）
Expected: 全绿。

- [ ] **Step 8: Commit**
```bash
git add backend/migrations/sqlite/0029_add_service_direction_repo_url.sql backend/migrations/postgres/0029_add_service_direction_repo_url.sql backend/internal/model/entities.go backend/internal/servicedirection/service.go backend/internal/servicedirection/service_test.go
git commit -m "feat(services): 后端 service direction 增加 repo_url 字段与迁移"
```

---

## Task 2: 后端 handler 透传 `repo_url`

**Files:**
- Modify: `backend/internal/httpapi/service_direction_handlers.go`

> 5 处 struct/method 各加一个 `RepoURL` 字段（json `repo_url`）。无新逻辑（透传）。无 handler 单测（纯 struct 映射；业务逻辑由 Task 1 service 测试覆盖）。

- [ ] **Step 1: 改 handler 文件**

在 `backend/internal/httpapi/service_direction_handlers.go`：

(a) `serviceDirectionResponse`（第 16-29 行）在 `DescEn` 之后加：
```go
	RepoURL     string    `json:"repo_url"`
```

(b) `serviceDirectionToResponse`（第 31-46 行）在 `DescEn: sd.DescEn,` 之后加：
```go
		RepoURL:     sd.RepoURL,
```

(c) `publicServiceDirectionResponse`（第 49-55 行）在 `Desc string` 之后加：
```go
	RepoURL string `json:"repo_url"`
```

(d) `publicServiceDirectionToResponse`（第 57-69 行）：在 `resp` 初始化后、`if lang == "en"` 之前加一行（repo_url 与语言无关）：
```go
	resp.RepoURL = sd.RepoURL
```

(e) `serviceDirectionRequest`（第 71-81 行）在 `DescEn` 之后加：
```go
	RepoURL     string `json:"repo_url"`
```

(f) `(req *serviceDirectionRequest) toModel()`（第 83-95 行）在 `DescEn: req.DescEn,` 之后加：
```go
		RepoURL:     req.RepoURL,
```

- [ ] **Step 2: 编译 + 全量测试**

Run（从 `backend/`）: `go build ./... && go test ./...`
Expected: 编译通过，测试全绿。

- [ ] **Step 3: Commit**
```bash
git add backend/internal/httpapi/service_direction_handlers.go
git commit -m "feat(services): handler 透传 repo_url(含公开接口)"
```

---

## Task 3: 前端 admin 表单

**Files:**
- Modify: `frontend/app/(app)/admin/services/page.tsx`

- [ ] **Step 1: 类型加字段**

`ServiceDirection` type（第 7-20 行）在 `desc_en: string;` 之后加：
```ts
  repo_url: string;
```

- [ ] **Step 2: 加 state**

在 `const [isPublished, setIsPublished] = useState(true);`（约第 49 行）之后加：
```ts
  const [repoUrl, setRepoUrl] = useState("");
```

- [ ] **Step 3: resetForm 重置**

在 `resetForm`（约第 53-65 行）的 `setIsPublished(true);` 之后加：
```ts
    setRepoUrl("");
```

- [ ] **Step 4: handleEdit 回填**

在 `handleEdit`（约第 114-123 行）的 `setIsPublished(it.is_published);` 之后加：
```ts
      setRepoUrl(it.repo_url ?? "");
```

- [ ] **Step 5: 提交 body 带字段**

`handleCreateOrUpdate` 的 `body`（约第 147-157 行）在 `desc_en: descEn.trim(),` 之后加：
```ts
      repo_url: repoUrl.trim(),
```

- [ ] **Step 6: 表单 JSX 加输入框**

在编辑对话框的 JSX 里，找到描述字段（desc_en 的 textarea 或 input）之后、`sort_order`/`is_published` 之前，插入一个输入框（label 双语，placeholder 提示 GitHub）：
```tsx
<div>
  <label className="..." htmlFor="svc-repo-url">代码仓库地址（可选）/ Repo URL (optional)</label>
  <input
    id="svc-repo-url"
    type="url"
    value={repoUrl}
    onChange={(e) => setRepoUrl(e.target.value)}
    placeholder="https://github.com/your/repo"
    className="..." /* 与同表单其他 input 一致的 className */
  />
</div>
```
> 实现者：先读该文件找到现有输入框的 `className` 与外层 `<div>`/`<label>` 结构，**复用同样的样式**，保持一致；位置放在 desc_en 之后、sort_order 之前。

- [ ] **Step 7: 类型检查 + build**

Run（从 `frontend/`）: `npx tsc --noEmit`
Expected: 无新增错误。

- [ ] **Step 8: Commit**
```bash
git add "frontend/app/(app)/admin/services/page.tsx"
git commit -m "feat(admin): service 编辑表单增加 repo_url 字段"
```

---

## Task 4: 前端 `/services` 展示 + 图标 + 样式 + i18n

**Files:**
- Modify: `frontend/app/(marketing)/services/page.tsx`
- Modify: `frontend/app/(marketing)/editorial.css`
- Modify: `frontend/messages/zh.json` / `en.json`

- [ ] **Step 1: `ServiceItem` 加字段**

在 `services/page.tsx` 的 `type ServiceItem = {...}`（约第 9-15 行）加：
```ts
  repo_url?: string;
```

- [ ] **Step 2: 加图标组件 + 判断函数**

在 `services/page.tsx` 顶部（type 定义之前或 `export default function` 之前）加：
```tsx
function isGithubUrl(url: string): boolean {
  return /github\.com/i.test(url);
}

function GithubMarkIcon() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="currentColor" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
      <polyline points="15 3 21 3 21 9" />
      <line x1="10" y1="14" x2="21" y2="3" />
    </svg>
  );
}
```

- [ ] **Step 3: timeline-item 条件渲染链接**

在 `timeline` 的 `visible.map(...)` 里（约第 138-153 行），`<p>{it.desc}</p>` 之后、`</div>`（包裹 title/desc 的那个 div）之前插入：
```tsx
                    {it.repo_url && (
                      <a
                        className="service-repo-link"
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

- [ ] **Step 4: editorial.css 加样式**

在 `frontend/app/(marketing)/editorial.css` 末尾追加：
```css
/* service 列表项：代码仓库链接 */
.service-repo-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
  border-bottom: 1px solid transparent;
  transition: border-color 0.15s ease;
}
.service-repo-link:hover { border-bottom-color: var(--accent); }
.service-repo-link svg { flex: none; }
```

- [ ] **Step 5: i18n 文案**

`frontend/messages/zh.json` 的 `editorial.services` 内加：
```json
"viewProject": "查看项目",
```
`frontend/messages/en.json` 的 `editorial.services` 内加：
```json
"viewProject": "View project",
```

- [ ] **Step 6: 校验 + build**

Run（从 `frontend/`）:
```bash
node -e "JSON.parse(require('fs').readFileSync('messages/zh.json','utf8')); JSON.parse(require('fs').readFileSync('messages/en.json','utf8')); console.log('OK')"
npx tsc --noEmit
```
Expected: `OK` + 无新增 TS 错误。

- [ ] **Step 7: Commit**
```bash
git add "frontend/app/(marketing)/services/page.tsx" "frontend/app/(marketing)/editorial.css" frontend/messages/zh.json frontend/messages/en.json
git commit -m "feat(services): 列表项按 repo_url 渲染项目链接(GitHub 图标自适应)"
```

---

## Task 5: 整体验证

**Files:** 无（仅校验）

- [ ] **Step 1: 后端全量测试 + 双驱动迁移校验**

Run（从 `backend/`）:
```bash
go test ./...
```
Expected: 全绿（含 servicedirection 4 个新测、db migrate 成对校验）。

- [ ] **Step 2: 前端构建**

Run（从 `frontend/`）: `npm run build`
Expected: 成功，`/services` 路由仍在。

- [ ] **Step 3: 手动验证（后端 + 前端）**

启动后端（`go run ./backend`，sqlite）+ 前端（`npm run dev`）。用 admin 账号登录 `/admin/services`：
1. 编辑某条记录，填 `https://github.com/foo/bar` 保存 → 打开 `/services`，该条描述下方出现 GitHub 图标 +「查看项目」→ 点击新标签跳转 GitHub。
2. 编辑同条改为 `https://gitlab.com/foo/bar` 保存 → 刷新 `/services`，图标切换为通用外链。
3. 编辑同条清空 repo_url 保存 → 刷新 `/services`，该条无「查看项目」入口。
4. admin 尝试保存 `github.com/foo`（无协议）→ 后端报错 `repo_url must be an http(s) URL`。

- [ ] **Step 4: 最终 commit（如有残留）**
```bash
git add -A && git commit -m "chore: 整体验证通过" || echo "nothing to commit"
```

---

## Notes for the implementer

- **迁移必须双驱动**：`internal/db/migrate_test.go` 强制 sqlite 与 postgres 迁移一一对应，缺一个 CI 会失败。
- **`repo_url` 顺序**：DB 列、scan、INSERT、UPDATE 的参数顺序必须一致（计划里统一放在 `desc_en` 之后）。scan 顺序与 `columns` 顺序一致。
- **校验只在 service 层**：`normalizeAndValidate` 是 Create/Update 共用的唯一校验点；handler 不重复校验。
- **公开接口必须透传 `repo_url`**：`publicServiceDirectionToResponse` 要在 `lang` 分支外赋值（与语言无关）。
- **图标仅前端判断**：`isGithubUrl` 用正则匹配 `github.com`，简单够用；非 GitHub 一律通用外链图标。
- **空串而非 NULL**：`TEXT NOT NULL DEFAULT ''`，前端 `it.repo_url` 为空串即 falsy 不渲染。
- **不引新依赖**：图标用内联 SVG（marketing 页未用 MaterialIcon，保持一致）。

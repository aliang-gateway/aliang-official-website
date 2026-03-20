# Work Plan: MDX Blog Pages + Admin Article Editor

## TL;DR
> **Summary**: Migrate blog/article content from hardcoded in-page strings to a database-backed MDX pipeline, and add an admin article management interface (create/edit/publish/unpublish) secured by existing backend admin role checks.
> **Deliverables**:
> - Backend article schema + admin/public article APIs
> - Frontend blog list/detail switched to API + MDX rendering
> - Admin article editor page and API proxy routes
> - Legacy `/blog/[id]` compatibility redirect to slug URL
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: T1 → T3 → T5/T6 → T10 → T12 → T14 → T15

## Context
### Original Request
“我想让这个doc的页面，比如某个具体的文章，支持mdxjs，可以展示更多更丰富的内容；同时，也开发一个admin，让我可以编辑文章。”

### Interview Summary
- User explicitly requested creation of a detailed execution plan.
- No additional preference constraints were provided for storage/auth/editor, so defaults are applied and documented.

### Metis Review (gaps addressed)
- Addressed source-of-truth ambiguity by choosing DB-backed `mdx_body` as the only content source.
- Added guardrails for slug migration/SEO compatibility via id→slug redirect task.
- Added strict admin authorization requirements on backend (no frontend-only role trust).
- Added explicit draft/published visibility rules to prevent draft leakage.
- Added executable acceptance criteria and failure-path QA per task (no human-only checks).

## Work Objectives
### Core Objective
Enable rich MDX article rendering on blog detail pages and provide a secure admin workflow for creating and editing articles.

### Deliverables
- New article persistence model and migration in backend SQLite schema.
- Backend public endpoints for published article list/detail by slug.
- Backend admin endpoints for article CRUD + publish/unpublish (admin-only).
- Frontend proxy routes for public/admin article APIs.
- Frontend blog list and detail pages migrated to API + MDX pipeline.
- Frontend admin article editor page.
- Automated verification assets and final review wave checklist.

### Definition of Done (verifiable conditions with commands)
- `go test ./...` (from `backend/`) passes including new article route tests.
- `npm -C frontend run build` passes with new blog/admin routes.
- Admin API create/edit/publish/unpublish endpoints return expected status codes via `curl` (401/403/400/404/409/200/201 paths).
- Public blog detail route renders MDX content from persisted article record (not hardcoded array).
- `/blog/{legacy-id}` responds with redirect to canonical `/blog/{slug}`.

### Must Have
- MDX rendering supports headings, lists, code blocks, links, images, and controlled custom components.
- Article slug uniqueness enforced backend-side.
- Draft articles hidden from public APIs/pages.
- Admin actions require valid admin session bearer token.
- Frontend follows existing API proxy pattern under `frontend/app/api/**`.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No rich media upload pipeline.
- No tag/category/search/comment/revision-history systems.
- No replacement of existing auth model with new auth stack.
- No client-side-only authorization logic.
- No arbitrary remote MDX execution from untrusted domains.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: **tests-after** (backend Go tests existing; frontend has no unit-test framework in repo).
- QA policy: Every task includes agent-executed happy + failure scenarios.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5 tasks per wave for balanced parallel throughput.

Wave 1 (foundation)
- T1 Schema migration for articles table
- T2 Data backfill for current hardcoded articles
- T3 Backend article repository/service layer
- T9 Frontend public article proxy routes
- T10 Frontend admin article proxy routes

Wave 2 (backend APIs + robustness)
- T4 Public article endpoints (list/detail by slug)
- T5 Admin article CRUD endpoints (draft lifecycle)
- T6 Admin publish/unpublish endpoints
- T7 Backend public article tests
- T8 Backend admin/auth/validation tests

Wave 3 (frontend UX + compatibility + verification)
- T11 Blog list page switched to public article API
- T12 Blog detail page `/blog/[slug]` MDX rendering
- T13 Legacy `/blog/[id]` redirect compatibility
- T14 Admin article management UI
- T15 End-to-end verification scripts and QA artifact generation

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|---|---|---|
| T1 | - | T2,T3,T4,T5,T6,T7,T8 |
| T2 | T1 | T13 |
| T3 | T1 | T4,T5,T6 |
| T4 | T3 | T7,T9,T11,T12,T13 |
| T5 | T3 | T6,T8,T10,T14 |
| T6 | T5 | T8,T10,T14 |
| T7 | T4 | T15 |
| T8 | T5,T6 | T15 |
| T9 | T4 | T11,T12 |
| T10 | T5,T6 | T14 |
| T11 | T9 | T15 |
| T12 | T9 | T13,T15 |
| T13 | T2,T4,T12 | T15 |
| T14 | T10 | T15 |
| T15 | T7,T8,T11,T12,T13,T14 | Final Verification Wave |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 5 tasks → `unspecified-high`(3), `quick`(2)
- Wave 2 → 5 tasks → `unspecified-high`(3), `quick`(2)
- Wave 3 → 5 tasks → `visual-engineering`(2), `unspecified-high`(2), `quick`(1)

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

<!-- TASK DETAILS INSERTED BELOW -->

- [x] 1. Add article schema migration

  **What to do**: Add new migration file in `backend/migrations/` creating `articles` table with columns: `id`, `legacy_id`, `slug` (UNIQUE), `title`, `excerpt`, `cover_image_url`, `tag`, `read_time`, `author_name`, `author_avatar_url`, `author_icon`, `mdx_body`, `status` (`draft|published`), `published_at`, `created_by_user_id`, `updated_by_user_id`, `created_at`, `updated_at`; add indexes for `slug`, `status`, `published_at`, and optional unique index for non-null `legacy_id`.
  **Must NOT do**: Do not modify unrelated existing tables or auth/session schema.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: schema design must align with existing migration conventions and future API needs.
  - Skills: `[]` — no special skill required.
  - Omitted: `git-master` — no git history surgery needed.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 2,3,4,5,6,7,8 | Blocked By: none

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `backend/migrations/0001_initial_schema.sql:1` — table + index style and SQL dialect.
  - Pattern: `backend/migrations/0005_add_user_center_features.sql:1` — additive migration style.
  - API/Type: `backend/internal/model/entities.go:5` — existing entity naming and timestamp conventions.
  - Test: `backend/internal/db/migrate_test.go:1` — migration validation pattern.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Running `go test ./...` from `backend/` succeeds with new migration applied in test DB setup.
  - [ ] Querying schema shows `articles` table + expected indexes present.

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: migration applies cleanly
    Tool: Bash
    Steps: Run `go test ./internal/db -run Migrate -v` in backend.
    Expected: PASS with no SQL errors.
    Evidence: .sisyphus/evidence/task-1-article-schema-migration.txt

  Scenario: migration rollback/reapply safety in tests
    Tool: Bash
    Steps: Run full `go test ./...` in backend after migration file addition.
    Expected: PASS; no duplicate table/index failures in migration paths.
    Evidence: .sisyphus/evidence/task-1-article-schema-migration-full.txt
  ```

  **Commit**: YES | Message: `feat(backend): add articles schema migration` | Files: `backend/migrations/*`, optional `backend/internal/db/*_test.go`

- [x] 2. Seed/backfill existing blog entries

  **What to do**: Introduce deterministic backfill path that inserts current hardcoded articles into `articles` with stable `legacy_id` values matching existing `/blog/[id]` behavior and generated slugs.
  **Must NOT do**: Do not keep runtime hardcoded array as primary source after migration.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: focused one-time seed/backfill logic tied to known sample data.
  - Skills: `[]` — no extra skill needed.
  - Omitted: `ui-ux-pro-max` — backend/data-only task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 13 | Blocked By: 1

  **References**:
  - Pattern: `frontend/app/blog/[id]/page.tsx:8` — source hardcoded article corpus to migrate.
  - Pattern: `backend/internal/httpapi/routes.go:310` — existing DB transaction style in handlers.
  - Test: `backend/internal/httpapi/routes_test.go:203` — lifecycle assertions style.

  **Acceptance Criteria**:
  - [ ] Backfill inserts all existing sample blog posts once (idempotent guard).
  - [ ] Every inserted row has non-empty `slug`, `title`, `mdx_body`, and valid `status`.

  **QA Scenarios**:
  ```
  Scenario: initial backfill inserts expected rows
    Tool: Bash
    Steps: Boot test DB, invoke backfill hook, query count from `articles`.
    Expected: Count equals number of original hardcoded entries.
    Evidence: .sisyphus/evidence/task-2-article-backfill.txt

  Scenario: repeated backfill does not duplicate
    Tool: Bash
    Steps: Invoke backfill twice, query count + distinct legacy_id count.
    Expected: Counts unchanged on second run.
    Evidence: .sisyphus/evidence/task-2-article-backfill-idempotent.txt
  ```

  **Commit**: YES | Message: `feat(backend): backfill legacy blog articles` | Files: `backend/internal/httpapi/routes.go`, `backend/internal/httpapi/*_test.go` (or dedicated seed file)

- [x] 3. Add article repository/service layer

  **What to do**: Implement backend article data access/service functions for create/update/get/list/publish/unpublish with slug uniqueness and status filtering as reusable internal methods.
  **Must NOT do**: Do not embed authorization checks in repository layer.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: business rule centralization + query correctness.
  - Skills: `[]` — standard backend architecture.
  - Omitted: `frontend-ui-ux` — no frontend work.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 4,5,6 | Blocked By: 1

  **References**:
  - Pattern: `backend/internal/apikey/service.go:1` — service struct and DB interaction style.
  - Pattern: `backend/internal/user/service.go:1` — validation + error conventions.
  - API/Type: `backend/internal/model/entities.go:5` — model conventions.

  **Acceptance Criteria**:
  - [ ] Service layer supports public list/detail returning only published entries.
  - [ ] Service layer returns conflict error for duplicate `slug`.

  **QA Scenarios**:
  ```
  Scenario: create and query article lifecycle in service tests
    Tool: Bash
    Steps: Run targeted service tests for create/update/list/get.
    Expected: PASS with expected fields persisted and retrieved.
    Evidence: .sisyphus/evidence/task-3-article-service-tests.txt

  Scenario: duplicate slug rejected
    Tool: Bash
    Steps: Service test inserts same slug twice.
    Expected: second insert returns conflict-domain error.
    Evidence: .sisyphus/evidence/task-3-article-service-duplicate-slug.txt
  ```

  **Commit**: YES | Message: `feat(backend): add article service layer` | Files: `backend/internal/*` article service files + tests

- [x] 4. Expose public article endpoints

  **What to do**: Add public backend routes for article listing and detail-by-slug (published-only), with deterministic sort (newest published first) and stable response schema for frontend blog pages.
  **Must NOT do**: Do not expose draft content or admin-only fields.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: API contract + security boundary.
  - Skills: `[]` — existing route style already defined.
  - Omitted: `git-master` — not a git task.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 7,9,11,12,13 | Blocked By: 3

  **References**:
  - Pattern: `backend/internal/httpapi/routes.go:298` — public route registration patterns.
  - Pattern: `backend/internal/httpapi/routes.go:151` — public response DTO style.
  - Test: `backend/internal/httpapi/routes_test.go:16` — unauthenticated public endpoint tests.

  **Acceptance Criteria**:
  - [ ] `GET /public/articles` returns only published articles.
  - [ ] `GET /public/articles/{slug}` returns 200 for published, 404 for missing/draft.

  **QA Scenarios**:
  ```
  Scenario: published article is reachable publicly
    Tool: Bash
    Steps: curl GET public list/detail with a published slug.
    Expected: 200 and JSON includes slug/title/mdx_body payload as designed.
    Evidence: .sisyphus/evidence/task-4-public-articles-happy.txt

  Scenario: draft article hidden from public endpoint
    Tool: Bash
    Steps: curl GET `/public/articles/{draft-slug}`.
    Expected: 404 response.
    Evidence: .sisyphus/evidence/task-4-public-articles-draft-hidden.txt
  ```

  **Commit**: YES | Message: `feat(api): add public article endpoints` | Files: `backend/internal/httpapi/routes.go`, tests

- [x] 5. Add admin article CRUD endpoints

  **What to do**: Add admin-protected endpoints for create/update/delete/list article records, including payload validation (slug format, required fields, status transitions).
  **Must NOT do**: Do not allow non-admin access; do not leak raw DB errors in responses.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: RBAC + validation-heavy endpoint work.
  - Skills: `[]`.
  - Omitted: `frontend-ui-ux` — backend task.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 6,8,10,14 | Blocked By: 3

  **References**:
  - Pattern: `backend/internal/httpapi/routes.go:304` — admin route + middleware chaining style.
  - Pattern: `backend/internal/auth/middleware.go:76` — admin guard behavior.
  - Test: `backend/internal/httpapi/routes_test.go:178` — admin access-control test template.

  **Acceptance Criteria**:
  - [ ] Admin can create/update/delete article records through admin routes.
  - [ ] Non-admin receives 403 on all admin article routes.

  **QA Scenarios**:
  ```
  Scenario: admin create and update article
    Tool: Bash
    Steps: Obtain admin session token, curl POST then PUT admin article endpoint.
    Expected: 201 then 200; response reflects updated fields.
    Evidence: .sisyphus/evidence/task-5-admin-crud-happy.txt

  Scenario: non-admin blocked
    Tool: Bash
    Steps: Repeat POST with non-admin bearer token.
    Expected: 403 with `admin role required` style message.
    Evidence: .sisyphus/evidence/task-5-admin-crud-forbidden.txt
  ```

  **Commit**: YES | Message: `feat(api): add admin article CRUD endpoints` | Files: `backend/internal/httpapi/routes.go`, tests

- [x] 6. Add publish/unpublish admin actions

  **What to do**: Add dedicated admin endpoints/actions to publish and unpublish articles by slug, enforcing valid status transitions and setting/clearing `published_at`.
  **Must NOT do**: Do not allow publish/unpublish through public endpoints.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: bounded endpoint augmentation on top of task 5.
  - Skills: `[]`.
  - Omitted: `ui-ux-pro-max` — backend-only operation.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 8,10,14 | Blocked By: 5

  **References**:
  - Pattern: `backend/internal/httpapi/routes.go:304` — admin route chain.
  - Pattern: `backend/internal/httpapi/routes.go:237` — admin response DTO patterns.
  - Test: `backend/internal/httpapi/routes_test.go:203` — lifecycle test style.

  **Acceptance Criteria**:
  - [ ] Publish endpoint sets `status=published` and non-null `published_at`.
  - [ ] Unpublish endpoint sets `status=draft` and clears `published_at`.

  **QA Scenarios**:
  ```
  Scenario: publish then public visibility
    Tool: Bash
    Steps: Admin creates draft, calls publish endpoint, then calls public detail endpoint.
    Expected: publish returns 200 and public endpoint returns 200.
    Evidence: .sisyphus/evidence/task-6-publish-happy.txt

  Scenario: unpublish hides article publicly
    Tool: Bash
    Steps: Admin unpublishes article, then calls public detail endpoint.
    Expected: unpublish returns 200 and public endpoint returns 404.
    Evidence: .sisyphus/evidence/task-6-unpublish-failure-visibility.txt
  ```

  **Commit**: YES | Message: `feat(api): add article publish lifecycle endpoints` | Files: `backend/internal/httpapi/routes.go`, tests

- [x] 7. Add backend tests for public article API

  **What to do**: Add route tests covering public list/detail payload shape, sorting, and draft filtering.
  **Must NOT do**: Do not rely on external services or mutable shared state in tests.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: focused test implementation following existing test helpers.
  - Skills: `[]`.
  - Omitted: `frontend-ui-ux` — backend tests only.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 15 | Blocked By: 4

  **References**:
  - Pattern: `backend/internal/httpapi/routes_test.go:16` — public endpoint test scaffolding.
  - Pattern: `backend/internal/httpapi/auth_test_helpers_test.go:1` — helper usage.
  - Pattern: `backend/internal/httpapi/subscription_test.go:1` — payload assertions.

  **Acceptance Criteria**:
  - [ ] Tests cover at least one published and one draft article behavior path.
  - [ ] `go test ./internal/httpapi -run Article -v` passes.

  **QA Scenarios**:
  ```
  Scenario: public article tests pass
    Tool: Bash
    Steps: Run `go test ./internal/httpapi -run Article -v`.
    Expected: PASS; no flaky failures.
    Evidence: .sisyphus/evidence/task-7-public-article-tests.txt

  Scenario: regression guard for draft leak
    Tool: Bash
    Steps: Run test case asserting draft slug returns 404.
    Expected: PASS.
    Evidence: .sisyphus/evidence/task-7-draft-leak-guard.txt
  ```

  **Commit**: YES | Message: `test(api): cover public article routes` | Files: `backend/internal/httpapi/*_test.go`

- [x] 8. Add backend tests for admin article authorization and validation

  **What to do**: Add tests for admin-only enforcement and payload validation (missing fields, invalid slug, duplicate slug, invalid state transition).
  **Must NOT do**: Do not skip forbidden-path assertions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: broad negative-path security coverage.
  - Skills: `[]`.
  - Omitted: `ui-ux-pro-max` — backend task.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 15 | Blocked By: 5,6

  **References**:
  - Pattern: `backend/internal/httpapi/routes_test.go:178` — admin 401/403 assertions.
  - Pattern: `backend/internal/auth/middleware.go:76` — expected forbidden behavior.

  **Acceptance Criteria**:
  - [ ] Non-admin token on admin article endpoints returns 403.
  - [ ] Invalid payload and duplicate slug return deterministic 4xx responses.

  **QA Scenarios**:
  ```
  Scenario: auth guard tests
    Tool: Bash
    Steps: Run targeted admin article auth tests.
    Expected: PASS with 401/403 assertions.
    Evidence: .sisyphus/evidence/task-8-admin-auth-tests.txt

  Scenario: validation tests
    Tool: Bash
    Steps: Run tests for invalid slug, missing title, duplicate slug.
    Expected: PASS with expected 400/409 status mappings.
    Evidence: .sisyphus/evidence/task-8-admin-validation-tests.txt
  ```

  **Commit**: YES | Message: `test(api): enforce admin article auth and validation` | Files: `backend/internal/httpapi/*_test.go`

- [x] 9. Add frontend public article proxy routes

  **What to do**: Add Next.js route handlers under `frontend/app/api/public/articles` and `frontend/app/api/public/articles/[slug]` forwarding to backend public article endpoints.
  **Must NOT do**: Do not inject Authorization for public endpoints.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: follows established thin-proxy pattern.
  - Skills: `[]`.
  - Omitted: `unspecified-high` — low complexity proxy work.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 11,12 | Blocked By: 4

  **References**:
  - Pattern: `frontend/app/api/public/tiers/route.ts:1` — GET proxy pattern.
  - Pattern: `frontend/app/api/public/estimate/route.ts:11` — body forwarding pattern.
  - API/Type: `frontend/app/account/page.tsx:26` — typed payload style in frontend.

  **Acceptance Criteria**:
  - [ ] `/api/public/articles` and `/api/public/articles/{slug}` return backend payload/status transparently.
  - [ ] Misconfigured base URL still returns 500 with consistent error shape.

  **QA Scenarios**:
  ```
  Scenario: proxy happy path
    Tool: Bash
    Steps: curl frontend proxy routes while backend is running.
    Expected: status/body mirrors backend response.
    Evidence: .sisyphus/evidence/task-9-frontend-public-proxy.txt

  Scenario: config failure path
    Tool: Bash
    Steps: unset `NEXT_PUBLIC_API_BASE_URL` in test env and call proxy.
    Expected: 500 with `server misconfiguration` style message.
    Evidence: .sisyphus/evidence/task-9-frontend-public-proxy-error.txt
  ```

  **Commit**: YES | Message: `feat(frontend): add public articles proxy routes` | Files: `frontend/app/api/public/articles/**`

- [x] 10. Add frontend admin article proxy routes

  **What to do**: Add Next.js route handlers for admin article CRUD + publish/unpublish under `frontend/app/api/admin/articles/**`, forwarding Authorization header to backend.
  **Must NOT do**: Do not trust client-side role flags; backend must decide admin authorization.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: multiple methods and auth forwarding correctness.
  - Skills: `[]`.
  - Omitted: `ui-ux-pro-max` — API proxy layer only.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 14 | Blocked By: 5,6

  **References**:
  - Pattern: `frontend/app/api/subscription/route.ts:11` — GET/POST with Authorization forwarding.
  - Pattern: `frontend/app/api/api-keys/[id]/route.ts:11` — dynamic params proxy pattern.

  **Acceptance Criteria**:
  - [ ] Admin proxy supports list/create/update/delete/publish/unpublish mappings.
  - [ ] Authorization header passthrough is present on all protected operations.

  **QA Scenarios**:
  ```
  Scenario: admin proxy create flow
    Tool: Bash
    Steps: curl frontend admin proxy with admin bearer token.
    Expected: 201 and created article payload.
    Evidence: .sisyphus/evidence/task-10-admin-proxy-happy.txt

  Scenario: non-admin forbidden via proxy
    Tool: Bash
    Steps: call same proxy endpoint with non-admin token.
    Expected: 403 forwarded from backend.
    Evidence: .sisyphus/evidence/task-10-admin-proxy-forbidden.txt
  ```

  **Commit**: YES | Message: `feat(frontend): add admin articles proxy routes` | Files: `frontend/app/api/admin/articles/**`

- [x] 11. Migrate blog listing page to public article API

  **What to do**: Refactor `frontend/app/blog/page.tsx` to load list data from `/api/public/articles` instead of hardcoded `articles` array while preserving existing card layout and interactions.
  **Must NOT do**: Do not remove unrelated TechRadar interactions or alter visual design semantics.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: data-source migration with strict UI parity.
  - Skills: `["ui-ux-pro-max"]` — preserve visual quality while wiring new data source.
  - Omitted: `deep` — scope is UI data binding, not architecture redesign.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 15 | Blocked By: 9

  **References**:
  - Pattern: `frontend/app/blog/page.tsx:46` — current card field usage.
  - Pattern: `frontend/app/account/page.tsx:108` — client-side fetch + loading/error state handling.
  - API/Type: `frontend/app/api/public/tiers/route.ts:22` — no-store fetch contract style.

  **Acceptance Criteria**:
  - [ ] Blog cards render from API response and link to slug-based detail URLs.
  - [ ] Empty/error states are handled gracefully without runtime crash.

  **QA Scenarios**:
  ```
  Scenario: list page renders API-driven cards
    Tool: Playwright
    Steps: Open `/blog`, wait for article titles from seeded data, click first card.
    Expected: Navigates to `/blog/{slug}` and shows correct title.
    Evidence: .sisyphus/evidence/task-11-blog-list-playwright.md

  Scenario: backend unavailable fallback
    Tool: Bash
    Steps: stop backend, load `/blog`.
    Expected: user-visible error/empty state and no unhandled exception.
    Evidence: .sisyphus/evidence/task-11-blog-list-error.txt
  ```

  **Commit**: YES | Message: `refactor(frontend): load blog list from public articles API` | Files: `frontend/app/blog/page.tsx`

- [x] 12. Implement slug-based MDX detail rendering

  **What to do**: Create/replace detail route as `frontend/app/blog/[slug]/page.tsx` to fetch article by slug and render `mdx_body` using configured MDX pipeline + `mdx-components.tsx` mapping; preserve existing header/footer style.
  **Must NOT do**: Do not keep custom string parser as main rendering path.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: MDX rendering + layout parity.
  - Skills: `["ui-ux-pro-max"]` — maintain high-quality typographic rendering.
  - Omitted: `frontend-ui-ux` — custom project skill preferred.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 13,15 | Blocked By: 9

  **References**:
  - Pattern: `frontend/app/blog/[id]/page.tsx:427` — current article detail composition to preserve.
  - Pattern: `frontend/next.config.ts:1` — existing MDX enablement.
  - External: `https://nextjs.org/docs/app/guides/mdx` — App Router MDX usage and caveats.

  **Acceptance Criteria**:
  - [ ] `/blog/{slug}` renders persisted `mdx_body` with headings/lists/code blocks/links.
  - [ ] Unknown slug returns 404 view.

  **QA Scenarios**:
  ```
  Scenario: MDX detail happy path
    Tool: Playwright
    Steps: Open `/blog/{known-slug}` and assert heading/code/list elements are visible.
    Expected: Content matches backend article and renders styled prose blocks.
    Evidence: .sisyphus/evidence/task-12-blog-detail-mdx.md

  Scenario: unknown slug path
    Tool: Bash
    Steps: curl `/blog/not-existing-slug`.
    Expected: 404 status or Not Found page response signature.
    Evidence: .sisyphus/evidence/task-12-blog-detail-404.txt
  ```

  **Commit**: YES | Message: `feat(frontend): render blog detail with slug MDX pipeline` | Files: `frontend/app/blog/[slug]/page.tsx`, `frontend/mdx-components.tsx`

- [x] 13. Add legacy id route redirect compatibility

  **What to do**: Keep compatibility for existing `/blog/[id]` URLs by resolving `legacy_id` and issuing permanent redirect to canonical `/blog/[slug]`.
  **Must NOT do**: Do not serve duplicate canonical content under both id and slug URLs.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: focused compatibility route behavior.
  - Skills: `[]`.
  - Omitted: `unspecified-high` — small controlled task.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 15 | Blocked By: 2,4,12

  **References**:
  - Pattern: `frontend/app/blog/[id]/page.tsx:411` — current id param route exists.
  - API/Type: new public article API from task 4 includes `legacy_id` and `slug` mapping.

  **Acceptance Criteria**:
  - [ ] `/blog/1` returns redirect to `/blog/{mapped-slug}`.
  - [ ] Invalid legacy id returns 404.

  **QA Scenarios**:
  ```
  Scenario: valid id redirects to slug
    Tool: Bash
    Steps: curl -I `/blog/1`.
    Expected: 307/308 with `Location: /blog/{slug}`.
    Evidence: .sisyphus/evidence/task-13-id-redirect-happy.txt

  Scenario: invalid id route
    Tool: Bash
    Steps: curl -I `/blog/99999`.
    Expected: 404 status.
    Evidence: .sisyphus/evidence/task-13-id-redirect-404.txt
  ```

  **Commit**: YES | Message: `feat(frontend): add legacy blog id to slug redirect` | Files: `frontend/app/blog/[id]/page.tsx` (or replacement route file)

- [x] 14. Build admin article management page

  **What to do**: Add admin UI route (e.g., `frontend/app/admin/articles/page.tsx`) with list/create/edit/publish/unpublish controls, MDX textarea editor, preview panel, and clear loading/error states using existing session token flow.
  **Must NOT do**: Do not add rich text editor framework in this phase; keep textarea + preview.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: high-UI-density workflow screen.
  - Skills: `["ui-ux-pro-max"]` — improve usability while staying in project style.
  - Omitted: `frontend-ui-ux` — lower priority than project skill.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 15 | Blocked By: 10

  **References**:
  - Pattern: `frontend/app/account/page.tsx:75` — token-based authenticated workflows and form handling.
  - Pattern: `frontend/components/ui/*` — shared UI primitives and style language.
  - Pattern: `frontend/app/blog/[id]/page.tsx:447` — article metadata display structure.

  **Acceptance Criteria**:
  - [ ] Admin can create draft article and edit existing article from UI.
  - [ ] Publish/unpublish actions update status in UI and backend consistently.

  **QA Scenarios**:
  ```
  Scenario: full admin edit lifecycle in browser
    Tool: Playwright
    Steps: Login/admin token setup, open admin page, create draft, edit mdx body, publish.
    Expected: success notifications shown; article appears on public list after publish.
    Evidence: .sisyphus/evidence/task-14-admin-ui-lifecycle.md

  Scenario: unauthorized access attempt
    Tool: Playwright
    Steps: open admin page with no token or non-admin token.
    Expected: blocked workflow with explicit auth/permission error state.
    Evidence: .sisyphus/evidence/task-14-admin-ui-forbidden.md
  ```

  **Commit**: YES | Message: `feat(frontend): add admin article management interface` | Files: `frontend/app/admin/articles/page.tsx`, supporting components

- [x] 15. Run integrated verification and evidence capture

  **What to do**: Execute backend tests, frontend build, critical API curls, and UI navigation checks; save outputs to `.sisyphus/evidence/` and summarize pass/fail.
  **Must NOT do**: Do not mark completion without evidence files for each required check.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: cross-stack validation orchestration.
  - Skills: `[]`.
  - Omitted: `quick` — breadth is non-trivial.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: Final Verification Wave | Blocked By: 7,8,11,12,13,14

  **References**:
  - Pattern: `readme.md:29` — baseline verification command style.
  - Test: `backend/internal/httpapi/routes_test.go:16` — backend behavior expectations.
  - External: plan acceptance requirements in this file.

  **Acceptance Criteria**:
  - [ ] `go test ./...` in backend passes.
  - [ ] `npm -C frontend run build` passes.
  - [ ] curl checks for public/admin routes and status transitions pass and are logged.
  - [ ] Playwright checks for `/blog`, `/blog/{slug}`, and admin workflow produce evidence artifacts.

  **QA Scenarios**:
  ```
  Scenario: integrated happy path suite
    Tool: Bash
    Steps: run backend tests + frontend build + scripted curls in sequence.
    Expected: all commands exit 0.
    Evidence: .sisyphus/evidence/task-15-integration-suite.txt

  Scenario: negative path suite
    Tool: Bash
    Steps: execute scripted non-admin/admin-invalid payload checks.
    Expected: expected 4xx responses with no 5xx regressions.
    Evidence: .sisyphus/evidence/task-15-integration-negative.txt
  ```

  **Commit**: NO | Message: `chore(qa): capture mdx-admin verification evidence` | Files: `.sisyphus/evidence/*`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per task (T1..T15) when task changes are cohesive and independently verifiable.
- Commit message format: `type(scope): description`
- Suggested types: `feat`, `refactor`, `test`, `chore`.
- Do not squash unrelated backend/frontend changes into one commit.

## Success Criteria
- Public blog pages no longer rely on hardcoded article arrays for content rendering.
- Rich MDX content renders on article detail pages with controlled component mapping.
- Admin users can create, edit, publish, unpublish articles from UI.
- Non-admin users cannot access admin article actions.
- Legacy blog id links still resolve via redirect to canonical slug URLs.

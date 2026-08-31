# Auto API Key Ensure + 用户自建 Key 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新注册用户自动拥有可用的 auto API key（幂等 ensure，可重复调用），并支持用户在 /keys 页面自建 key（名字 + 分组）。

**Architecture:** 在 sub2api gateway 层新增 `EnsureDefaultUserKeys`（拉上游可用分组 → 过滤非订阅制标准组 → 按平台去重 → 复用现有 `EnsureUserKeyInGroup` 逐组 ensure）。触发点两处：注册成功后的异步钩子、`POST /api-keys/ensure-auto` 兜底端点。前端 /keys 页面新增创建表单并在加载时兜底调用 ensure；BFF `groups/available` 过滤策略改为「非订阅制标准组 ∪ 本地授权组」。防重复三重保证：创建前查重（主）+ 409 容忍 + 确定性幂等键（辅助）——**全程只创建、零更新**。

**Tech Stack:** Go 1.22+（net/http ServeMux、httptest、in-memory sqlite 测试）、Next.js App Router + next-intl。

**Spec:** `docs/superpowers/specs/2026-08-31-auto-apikey-ensure-design.md`（实现者应先通读）

**工作区要求（用户硬性要求）：** 所有实现工作在 **git worktree** 中进行 —— 执行前用 superpowers:using-git-worktrees 从最新 `main` 创建 worktree。本计划文档本身已提交在 main。

**验证命令约定：**
- 后端：`cd backend && go build ./... && go test ./...`
- 前端：`cd frontend && npm run lint && npm test && npm run build`

---

## Task 0: 创建 worktree（一次性）

- [ ] **Step 0.1: 从最新 main 创建 worktree**

```bash
git -C /Users/mac/MyProgram/AiProgram/aliang-official-website fetch origin main 2>/dev/null || true
git -C /Users/mac/MyProgram/AiProgram/aliang-official-website worktree add .claude/worktrees/auto-apikey-ensure -b feat/auto-apikey-ensure main
```

后续所有任务的工作目录均为 `/Users/mac/MyProgram/AiProgram/aliang-official-website/.claude/worktrees/auto-apikey-ensure`（下文简称 `$WT`）。

---

## Task 1: proxy 客户端 —— `AvailableGroup` 结构 + `ListAvailableGroups`

**Files:**
- Modify: `backend/internal/proxy/passthrough.go`（在 `AdminGroup` 结构定义（约 148 行）之后加结构体；`ListAvailableGroups` 放在 `ListAdminGroups`（约 968 行）之后）
- Test: `backend/internal/proxy/passthrough_test.go`

- [ ] **Step 1.1: 写失败测试**

在 `backend/internal/proxy/passthrough_test.go` 末尾追加（若 `httptest`/`context` 等 import 缺失则补上）：

```go
func TestListAvailableGroups(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[` +
			`{"id":3,"name":"Claude Basic","platform":"anthropic","status":"active","subscription_type":"","is_exclusive":false,"rate_multiplier":1.0},` +
			`{"id":7,"name":"GPT Sub","platform":"openai","status":"active","subscription_type":"subscription","is_exclusive":false,"rate_multiplier":1.5},` +
			`{"id":9,"name":"Exclusive","platform":"gemini","status":"active","subscription_type":"","is_exclusive":true,"rate_multiplier":2.0}]}`))
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := client.ListAvailableGroups(context.Background(), "bearer-1")
	if err != nil {
		t.Fatalf("ListAvailableGroups: %v", err)
	}
	if gotAuth != "Bearer bearer-1" {
		t.Errorf("Authorization header = %q", gotAuth)
	}
	if gotPath != "/api/v1/groups/available" {
		t.Errorf("upstream path = %q", gotPath)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("len(Data) = %d, want 3", len(resp.Data))
	}
	g := resp.Data[0]
	if g.ID != 3 || g.Platform != "anthropic" || g.Status != "active" || g.SubscriptionType != "" || g.IsExclusive {
		t.Errorf("group[0] parse mismatch: %+v", g)
	}
	if resp.Data[1].SubscriptionType != "subscription" || resp.Data[2].IsExclusive != true {
		t.Errorf("group[1]/group[2] parse mismatch: %+v %+v", resp.Data[1], resp.Data[2])
	}
}

func TestListAvailableGroups_EmptyBearer(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.ListAvailableGroups(context.Background(), "  "); err == nil {
		t.Error("expected error for empty bearer token")
	}
}
```

- [ ] **Step 1.2: 运行确认失败**

```bash
cd $WT/backend && go test ./internal/proxy/ -run TestListAvailableGroups -v
```
预期：FAIL（`client.ListAvailableGroups undefined`）

- [ ] **Step 1.3: 实现**

`backend/internal/proxy/passthrough.go`，在 `AdminGroup` 结构后追加：

```go
// AvailableGroup is a group visible to an end user on the upstream gateway
// (GET /api/v1/groups/available).
type AvailableGroup struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Platform         string  `json:"platform"`
	Status           string  `json:"status"`
	SubscriptionType string  `json:"subscription_type"`
	IsExclusive      bool    `json:"is_exclusive"`
	RateMultiplier   float64 `json:"rate_multiplier"`
}
```

在 `ListAdminGroups` 之后追加（仿照 `createUserAPIKeyAtPath` 的请求模式）：

```go
// ListAvailableGroups fetches the groups available to the given user from the
// upstream gateway using the user's bearer token.
func (c *Client) ListAvailableGroups(ctx context.Context, bearerToken string) (*ResponseEnvelope[[]AvailableGroup], error) {
	if strings.TrimSpace(bearerToken) == "" {
		return nil, errors.New("bearer token is required")
	}
	upstreamURL, err := BuildUpstreamURL(c.baseURL, "/api/v1/groups/available", "")
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodGet, upstreamURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build available groups request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read available groups response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp, body, http.MethodGet, "/api/v1/groups/available")
	}
	var decoded ResponseEnvelope[[]AvailableGroup]
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode available groups response: %w", err)
	}
	return &decoded, nil
}
```

- [ ] **Step 1.4: 运行确认通过**

```bash
cd $WT/backend && go test ./internal/proxy/ -run TestListAvailableGroups -v
```
预期：PASS

- [ ] **Step 1.5: Commit**

```bash
cd $WT && git add backend/internal/proxy/passthrough.go backend/internal/proxy/passthrough_test.go
git commit -m "feat(proxy): 用户侧 ListAvailableGroups 拉取上游可用分组"
```

---

## Task 2: gateway —— `EnsureUserKeyInGroupIdempotent` 重构（显式幂等键 + created 标志）

**Files:**
- Modify: `backend/internal/sub2api/gateway.go:128-160`（现有 `EnsureUserKeyInGroup`）
- Test: 既有 `backend/internal/sub2api/gateway_test.go` 必须保持通过（旧调用点行为不变）

背景：`EnsureUserKeyInGroup(ctx, userID, groupID, parentIdempotencyKey)` 现有唯一生产调用点在 `backend/internal/httpapi/routes.go:7552`（fulfillment），幂等键由 parent 派生。本任务把核心逻辑抽为可显式传幂等键、并返回「本次是否新建」的变体，旧签名变薄壳。

- [ ] **Step 2.1: 重构实现（行为保持，先改代码不新增测试）**

将 `EnsureUserKeyInGroup` 整体替换为：

```go
// EnsureUserKeyInGroup ensures the user has an auto-key in the specified group,
// tolerating 409 (already exists).
func (g *Gateway) EnsureUserKeyInGroup(ctx context.Context, userID int64, groupID int64, parentIdempotencyKey string) error {
	childKey := parentIdempotencyKey + ":ensure-key:" + strconv.FormatInt(groupID, 10)
	_, err := g.EnsureUserKeyInGroupIdempotent(ctx, userID, groupID, childKey)
	return err
}

// EnsureUserKeyInGroupIdempotent ensures an auto-key exists in the group using an
// explicit idempotency key, and reports whether a new key was created.
// Safety: it only ever creates — never updates or revokes existing keys; any
// pre-existing "auto-key" in the group (including a disabled one) short-circuits;
// 409 Conflict from concurrent creation is treated as success.
func (g *Gateway) EnsureUserKeyInGroupIdempotent(ctx context.Context, userID int64, groupID int64, idempotencyKey string) (bool, error) {
	if g == nil || g.proxy == nil || g.auth == nil {
		return false, nil
	}
	bearerToken, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get bearer token for user %d: %w", userID, err)
	}
	keys, err := g.proxy.ListUserAPIKeys(ctx, bearerToken, groupID, "auto-key")
	if err != nil {
		return false, err
	}
	for _, key := range keys.Data {
		if key.GroupID == groupID && strings.TrimSpace(key.Name) == "auto-key" {
			return false, nil
		}
	}
	_, createErr := g.proxy.CreateUserAPIKey(ctx, bearerToken, proxy.CreateUserAPIKeyRequest{
		Name:    "auto-key",
		GroupID: groupID,
	}, idempotencyKey)
	if createErr == nil {
		return true, nil
	}
	var apiErr *proxy.APIError
	if errors.As(createErr, &apiErr) && apiErr.IsConflict() {
		return false, nil // key already exists (concurrent creation)
	}
	return false, createErr
}
```

- [ ] **Step 2.2: 全量编译 + 既有测试回归**

```bash
cd $WT/backend && go build ./... && go test ./internal/sub2api/ ./internal/httpapi/ ./internal/proxy/
```
预期：全部 PASS（fulfillment 旧调用点无需任何改动）

- [ ] **Step 2.3: Commit**

```bash
cd $WT && git add backend/internal/sub2api/gateway.go
git commit -m "refactor(sub2api): EnsureUserKeyInGroup 抽出显式幂等键变体并返回 created 标志"
```

---

## Task 3: gateway —— 纯函数 `selectDefaultKeyGroups`

**Files:**
- Modify: `backend/internal/sub2api/gateway.go`（新增纯函数，放 `EnsureUserKeyInGroupIdempotent` 之后）
- Test: `backend/internal/sub2api/gateway_test.go`

- [ ] **Step 3.1: 写失败测试**

`backend/internal/sub2api/gateway_test.go` 末尾追加：

```go
func TestSelectDefaultKeyGroups(t *testing.T) {
	groups := []proxy.AvailableGroup{
		{ID: 3, Platform: "anthropic", Status: "active", SubscriptionType: ""},
		{ID: 4, Platform: "Anthropic", Status: "active", SubscriptionType: ""},     // 平台大小写去重 → 跳过
		{ID: 7, Platform: "openai", Status: "active", SubscriptionType: "subscription"}, // 订阅组 → 跳过
		{ID: 8, Platform: "openai", Status: "active", SubscriptionType: ""},        // openai 第一个标准组
		{ID: 9, Platform: "gemini", Status: "inactive", SubscriptionType: ""},      // 非active → 跳过
		{ID: 0, Platform: "gemini", Status: "active", SubscriptionType: ""},        // 非法ID → 跳过
		{ID: 11, Platform: "", Status: "active", SubscriptionType: ""},             // 空平台独立桶
	}
	got := selectDefaultKeyGroups(groups)
	want := []int64{3, 8, 11}
	if len(got) != len(want) {
		t.Fatalf("selectDefaultKeyGroups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selectDefaultKeyGroups = %v, want %v", got, want)
		}
	}
}

func TestSelectDefaultKeyGroups_Empty(t *testing.T) {
	if got := selectDefaultKeyGroups(nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}
```

- [ ] **Step 3.2: 运行确认失败**

```bash
cd $WT/backend && go test ./internal/sub2api/ -run TestSelectDefaultKeyGroups -v
```
预期：FAIL（undefined）

- [ ] **Step 3.3: 实现**

`backend/internal/sub2api/gateway.go` 追加：

```go
// selectDefaultKeyGroups picks at most one non-subscription active group per
// platform from the user's available groups, preserving input order. Groups
// with empty platform form their own bucket. This is the policy for which
// groups get an auto-key provisioned right after registration.
func selectDefaultKeyGroups(groups []proxy.AvailableGroup) []int64 {
	seenPlatforms := make(map[string]struct{})
	result := make([]int64, 0)
	for _, group := range groups {
		if group.ID <= 0 {
			continue
		}
		if status := strings.TrimSpace(group.Status); status != "" && !strings.EqualFold(status, "active") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(group.SubscriptionType), "subscription") {
			continue
		}
		platform := strings.ToLower(strings.TrimSpace(group.Platform))
		if _, seen := seenPlatforms[platform]; seen {
			continue
		}
		seenPlatforms[platform] = struct{}{}
		result = append(result, group.ID)
	}
	return result
}

// autoEnsureIdempotencyKey maps (user, group) to one stable idempotency key so
// repeated/concurrent ensure runs dedupe upstream.
func autoEnsureIdempotencyKey(userID, groupID int64) string {
	return fmt.Sprintf("auto-ensure:u%d:g%d", userID, groupID)
}
```

- [ ] **Step 3.4: 运行确认通过**

```bash
cd $WT/backend && go test ./internal/sub2api/ -run TestSelectDefaultKeyGroups -v
```
预期：PASS

- [ ] **Step 3.5: Commit**

```bash
cd $WT && git add backend/internal/sub2api/gateway.go backend/internal/sub2api/gateway_test.go
git commit -m "feat(sub2api): selectDefaultKeyGroups 平台去重选默认组策略"
```

---

## Task 4: gateway —— `EnsureDefaultUserKeys` 编排

**Files:**
- Modify: `backend/internal/sub2api/gateway.go`
- Test: `backend/internal/sub2api/gateway_test.go`

测试基座说明：Gateway 依赖具体类型 `*proxy.Client` + `*sub2apiauth.Service`。用 httptest 上游 + `proxy.NewClient(srv.URL)` + 真实 `sub2apiauth.Service`（内存 sqlite）。**先读 `backend/internal/sub2apiauth/service_test.go`，把 `setupTestDB`（约 275 行）、`testDialect`、`createUser`（约 274 行，向 `als_users` 插行 —— 必须复制，因为 `als_sub2api_auth_tokens.user_id` 有外键且 `db.Open` 开启 `PRAGMA foreign_keys=ON`，不建用户行 `UpsertToken` 会报 FK 错）三个 helper 复制进 gateway_test.go 并改名**（sqlite 内存库；若环境 `DB_DRIVER=postgres` 该 helper 会 skip，同样照搬）。

⚠️ 另注意：`sub2apiauth.UpsertTokenInput.AccessExpiresAt` 字段类型是 `*time.Time`（见 `backend/internal/sub2apiauth/service.go`），测试里必须传指针。

- [ ] **Step 4.1: 写失败测试**

`gateway_test.go` 末尾追加（`newEnsureTestGateway` 是本任务新加的 helper，放测试文件底部；`strPtr`/`timePtr` 若文件已有同名 helper 则复用）：

```go
func strPtr(s string) *string { return &s }
func timePtr(v time.Time) *time.Time { return &v }

// newEnsureTestGateway builds a Gateway backed by an httptest upstream and a
// real sub2apiauth service with user 1 having a stored token. The recorder
// captures upstream requests for assertions.
type ensureUpstreamRecorder struct {
	mu        sync.Mutex
	requests  []string // "METHOD PATH IDEMKEY" per request
	listBody  string   // body for GET /api/v1/groups/available
	keysBody  string   // body for GET /api/v1/api-keys (raw JSON array form)
	createErr string   // when non-empty, respond with this status code text on create
}

func newEnsureTestGateway(t *testing.T, rec *ensureUpstreamRecorder) *Gateway {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idem := r.Header.Get("Idempotency-Key")
		rec.mu.Lock()
		rec.requests = append(rec.requests, r.Method+" "+r.URL.Path+" "+idem)
		rec.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/available":
			_, _ = w.Write([]byte(rec.listBody))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/api-keys"):
			// 裸数组形态：ListResponseEnvelope 的自定义反序列化明确支持 `[` 开头。
			keysBody := rec.keysBody
			if keysBody == "" {
				keysBody = "[]"
			}
			_, _ = w.Write([]byte(keysBody))
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/api-keys" || r.URL.Path == "/api/v1/keys"):
			if rec.createErr != "" {
				http.Error(w, rec.createErr, http.StatusConflict)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":10,"key":"sk-x","name":"auto-key","group_id":3,"status":"active"}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := proxy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	database := setupEnsureTestDB(t)
	createEnsureTestUser(t, context.Background(), database, "u1@test.local", "User One", "user")
	authSvc := sub2apiauth.NewService(database)
	err = authSvc.UpsertToken(context.Background(), sub2apiauth.UpsertTokenInput{
		UserID:          1,
		AccessToken:     "access-1",
		RefreshToken:    strPtr("refresh-1"),
		AccessExpiresAt: timePtr(time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("UpsertToken: %v", err)
	}
	return NewGateway(client, authSvc)
}

const ensureAvailableGroupsBody = `{"data":[` +
	`{"id":3,"name":"Claude Basic","platform":"anthropic","status":"active","subscription_type":""},` +
	`{"id":7,"name":"GPT Sub","platform":"openai","status":"active","subscription_type":"subscription"},` +
	`{"id":8,"name":"GPT Basic","platform":"openai","status":"active","subscription_type":""}]}`

func TestEnsureDefaultUserKeys_CreatesOneKeyPerPlatform(t *testing.T) {
	rec := &ensureUpstreamRecorder{listBody: ensureAvailableGroupsBody}
	g := newEnsureTestGateway(t, rec)

	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnsureDefaultUserKeys: %v", err)
	}
	if result.Ensured != 2 || len(result.CreatedGroups) != 2 {
		t.Fatalf("result = %+v, want ensured=2 created=2 (groups 3 and 8)", result)
	}

	// 幂等键确定性：同一 (user, group) 永远同一键。
	wantKeys := map[string]bool{"auto-ensure:u1:g3": false, "auto-ensure:u1:g8": false}
	for _, req := range rec.requests {
		for key := range wantKeys {
			if strings.Contains(req, key) {
				wantKeys[key] = true
			}
		}
	}
	for key, seen := range wantKeys {
		if !seen {
			t.Errorf("expected upstream create with Idempotency-Key %q, requests=%v", key, rec.requests)
		}
	}
}

func TestEnsureDefaultUserKeys_IdempotentSecondRun(t *testing.T) {
	groupsBody := `{"data":[{"id":3,"name":"Claude Basic","platform":"anthropic","status":"active","subscription_type":""}]}`
	rec := &ensureUpstreamRecorder{listBody: groupsBody}
	g := newEnsureTestGateway(t, rec)

	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// 第二轮：组列表不变，但上游 api-keys 列表已含该组 auto-key → 不应再发起 create。
	rec.requests = nil
	rec.keysBody = `[{"id":10,"name":"auto-key","group_id":3,"status":"active"}]`
	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result.Ensured != 1 || len(result.CreatedGroups) != 0 {
		t.Fatalf("second run result = %+v, want ensured=1 created=0", result)
	}
	for _, req := range rec.requests {
		if strings.HasPrefix(req, "POST") {
			t.Errorf("second run must not create, got %q", req)
		}
	}
}

func TestEnsureDefaultUserKeys_ConflictTreatedAsEnsured(t *testing.T) {
	rec := &ensureUpstreamRecorder{
		listBody:  ensureAvailableGroupsBody,
		createErr: "conflict",
	}
	g := newEnsureTestGateway(t, rec)

	result, err := g.EnsureDefaultUserKeys(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnsureDefaultUserKeys: %v", err)
	}
	if result.Ensured != 2 || len(result.CreatedGroups) != 0 {
		t.Fatalf("result = %+v, want 409 counted as ensured-but-not-created", result)
	}
}

func TestEnsureDefaultUserKeys_NotConfigured(t *testing.T) {
	g := NewGateway(nil, nil)
	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err == nil {
		t.Error("expected error when gateway not configured")
	}
}

func TestEnsureDefaultUserKeys_UpstreamDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	client, err := proxy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	database := setupEnsureTestDB(t)
	createEnsureTestUser(t, context.Background(), database, "u2@test.local", "User Two", "user")
	authSvc := sub2apiauth.NewService(database)
	if err := authSvc.UpsertToken(context.Background(), sub2apiauth.UpsertTokenInput{
		UserID:          1,
		AccessToken:     "access-1",
		RefreshToken:    strPtr("refresh-1"),
		AccessExpiresAt: timePtr(time.Now().Add(time.Hour)),
	}); err != nil {
		t.Fatalf("UpsertToken: %v", err)
	}
	g := NewGateway(client, authSvc)
	if _, err := g.EnsureDefaultUserKeys(context.Background(), 1); err == nil {
		t.Error("expected error when upstream unavailable")
	}
}
```

同时在测试文件加 helpers（照搬 `backend/internal/sub2apiauth/service_test.go` 的实现，重命名避免冲突）：

```go
// setupEnsureTestDB / createEnsureTestUser / ensureTestDialect 一律从
// backend/internal/sub2apiauth/service_test.go 的 setupTestDB / createUser /
// testDialect 逐行复制后改名 —— 建库与迁移语句禁止凭空编写。
//
// createEnsureTestUser 签名（与原 createUser 相同）：
//
//	func createEnsureTestUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64
//
// 必须：先 createEnsureTestUser 再 UpsertToken —— als_sub2api_auth_tokens.user_id
// 外键引用 als_users(id)，且 db.Open 开启 PRAGMA foreign_keys=ON。
```

> ⚠️ 给执行者：这是本计划唯一一处「以现有代码为准」的复制点 —— 打开 `backend/internal/sub2apiauth/service_test.go`，逐行复制 `setupTestDB`（约 275 行）、`createUser`（约 274 行）、`testDialect`，仅重命名。测试文件需的 import（若缺）：`database/sql`、`context`、`net/http/httptest`、`strings`、`sync`、`time`、`os`、`fmt`。

- [ ] **Step 4.2: 运行确认失败**

```bash
cd $WT/backend && go test ./internal/sub2api/ -run TestEnsureDefaultUserKeys -v
```
预期：FAIL（`EnsureDefaultUserKeys undefined`、helper panic）

- [ ] **Step 4.3: 实现**

`backend/internal/sub2api/gateway.go` 追加（放在 `EnsureUserKeyInGroupIdempotent` 之后）：

```go
// EnsureDefaultResult summarizes one EnsureDefaultUserKeys run.
type EnsureDefaultResult struct {
	Ensured       int      // groups that now have an auto-key (pre-existing or newly created)
	CreatedGroups []int64  // groups where a key was created in this run
	FailedGroups  []string // groups that failed, formatted "<groupID>: <reason>"
}

// EnsureDefaultUserKeys idempotently provisions auto-keys for the user: for
// each platform among the user's non-subscription available groups, exactly
// one group gets an "auto-key". Create-only: existing keys are never touched.
func (g *Gateway) EnsureDefaultUserKeys(ctx context.Context, userID int64) (*EnsureDefaultResult, error) {
	if !g.IsConfigured() {
		return nil, errors.New("sub2api gateway is not configured")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	bearerToken, err := g.auth.GetBearerTokenByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get bearer token for user %d: %w", userID, err)
	}
	resp, err := g.proxy.ListAvailableGroups(ctx, bearerToken)
	if err != nil {
		return nil, fmt.Errorf("list available groups: %w", err)
	}

	result := &EnsureDefaultResult{CreatedGroups: []int64{}, FailedGroups: []string{}}
	for _, groupID := range selectDefaultKeyGroups(resp.Data) {
		created, ensureErr := g.EnsureUserKeyInGroupIdempotent(ctx, userID, groupID, autoEnsureIdempotencyKey(userID, groupID))
		if ensureErr != nil {
			result.FailedGroups = append(result.FailedGroups, fmt.Sprintf("%d: %v", groupID, ensureErr))
			continue
		}
		result.Ensured++
		if created {
			result.CreatedGroups = append(result.CreatedGroups, groupID)
		}
	}
	return result, nil
}
```

测试文件需要的 import（若缺）：`database/sql`、`net/http/httptest`、`sync`、`time`、`os`、`fmt`。

- [ ] **Step 4.4: 运行确认通过**

```bash
cd $WT/backend && go test ./internal/sub2api/ -v
```
预期：全部 PASS

- [ ] **Step 4.5: Commit**

```bash
cd $WT && git add backend/internal/sub2api/gateway.go backend/internal/sub2api/gateway_test.go
git commit -m "feat(sub2api): EnsureDefaultUserKeys 幂等确保每平台一个 auto-key"
```

---

## Task 5: routes —— `POST /api-keys/ensure-auto` 端点

**Files:**
- Modify: `backend/internal/httpapi/routes.go`（路由注册：`RegisterRoutesWithOptions` 内 `if r.proxyClient != nil` 分支，约 826 行 `POST /api-keys` 之后；handler 放 `handleAPIKeysCreatePassthrough`（约 1675 行）附近）

- [ ] **Step 5.1: 注册路由**

在 `mux.Handle("POST /api-keys", authenticated(http.HandlerFunc(r.handleAPIKeysCreatePassthrough)))` 之后加一行：

```go
		mux.Handle("POST /api-keys/ensure-auto", authenticated(http.HandlerFunc(r.handleEnsureAutoAPIKeys)))
```

（Go 1.22 ServeMux 按最具体 pattern 优先，`/api-keys/ensure-auto` 不会被 `/api-keys` 吞掉。）

- [ ] **Step 5.2: 实现 handler**

放在 `handleAPIKeysCreatePassthrough` 之前：

```go
type ensureAutoAPIKeysResponse struct {
	Ensured       int      `json:"ensured"`
	CreatedGroups []int64  `json:"created"`
	FailedGroups  []string `json:"failed,omitempty"`
}

// handleEnsureAutoAPIKeys idempotently provisions the user's auto keys. It is a
// best-effort bottom-up trigger: clients call it on keys-page load.
func (r *routes) handleEnsureAutoAPIKeys(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !r.sub2api.IsConfigured() {
		writeError(w, http.StatusBadGateway, "sub2api gateway is not configured")
		return
	}
	result, err := r.sub2api.EnsureDefaultUserKeys(req.Context(), user.ID)
	if err != nil {
		slog.Warn("ensure-auto api keys failed", "user_id", user.ID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to ensure api keys")
		return
	}
	writeJSON(w, http.StatusOK, ensureAutoAPIKeysResponse{
		Ensured:       result.Ensured,
		CreatedGroups: result.CreatedGroups,
		FailedGroups:  result.FailedGroups,
	})
}
```

- [ ] **Step 5.3: 编译 + 回归**

```bash
cd $WT/backend && go build ./... && go test ./internal/httpapi/
```
预期：PASS

- [ ] **Step 5.4: Commit**

```bash
cd $WT && git add backend/internal/httpapi/routes.go
git commit -m "feat(httpapi): POST /api-keys/ensure-auto 幂等确保 auto key 端点"
```

---

## Task 6: routes —— 注册成功后异步 ensure 钩子

**Files:**
- Modify: `backend/internal/httpapi/routes.go`（`handleAuthPassthrough` register 分支，约 2476-2500 行 `found` 块内、`injectLocalSessionIntoAuthResponse` 成功之后；helper 函数放 `handleAuthPassthrough` 之后）

- [ ] **Step 6.1: 实现 helper**

```go
// kickEnsureDefaultUserKeys best-effort provisions auto keys right after a
// fresh registration. It runs detached from the request, never blocks the
// registration response, and only logs failures.
func (r *routes) kickEnsureDefaultUserKeys(userID int64) {
	if !r.sub2api.IsConfigured() || userID <= 0 {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Warn("ensure default api keys panicked", "user_id", userID, "panic", rec)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result, err := r.sub2api.EnsureDefaultUserKeys(ctx, userID)
		if err != nil {
			slog.Warn("ensure default api keys after register failed", "user_id", userID, "error", err)
			return
		}
		slog.Info("ensure default api keys after register",
			"user_id", userID, "ensured", result.Ensured,
			"created", result.CreatedGroups, "failed", result.FailedGroups)
	}()
}
```

- [ ] **Step 6.2: 接入注册分支**

在 `handleAuthPassthrough` 内、`found` 分支中，`responseBody, err = injectLocalSessionIntoAuthResponse(...)` 的**错误检查之后**追加：

```go
			if upstreamPath == "/api/v1/auth/register" {
				r.kickEnsureDefaultUserKeys(localUserID)
			}
```

- [ ] **Step 6.3: 编译 + 回归**

```bash
cd $WT/backend && go build ./... && go test ./internal/httpapi/
```
预期：PASS（既有 `auth_passthrough_test.go`、`refresh_arbiter_test.go` 不受影响 —— 钩子仅在 register + found 路径触发且异步）

- [ ] **Step 6.4: Commit**

```bash
cd $WT && git add backend/internal/httpapi/routes.go
git commit -m "feat(httpapi): 注册成功后异步幂等预置 auto api key"
```

---

## Task 7: routes —— groups/available BFF 过滤策略调整

**Files:**
- Modify: `backend/internal/httpapi/routes.go`（重写 `handleFilteredGroupsAvailablePassthrough` 约 1694-1727 行；新增纯函数放 `filterGroupListPayloadByID` 附近）

策略（spec 3.5）：可见 = **非订阅制标准组（所有人）∪ 本地授权组**；上游失败时普通用户回退空列表（不再保留 502 路径）。

- [ ] **Step 7.1: 写失败测试（纯函数）**

`backend/internal/httpapi/routes_test.go` 末尾追加：

```go
func TestFilterGroupListPayloadByPolicy(t *testing.T) {
	payload := map[string]any{
		"data": []any{
			map[string]any{"id": json.Number("3"), "name": "Std", "subscription_type": ""},
			map[string]any{"id": json.Number("7"), "name": "Sub", "subscription_type": "subscription"},
			map[string]any{"id": json.Number("9"), "name": "Sub-Authorized", "subscription_type": "subscription"},
		},
	}
	authorized := map[int64]struct{}{9: {}}

	got, err := filterGroupListPayloadByPolicy(payload, authorized)
	if err != nil {
		t.Fatalf("filterGroupListPayloadByPolicy: %v", err)
	}
	root := got.(map[string]any)
	items := root["data"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 visible groups (standard + authorized subscription), got %d: %v", len(items), items)
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		id, _ := strconv.ParseInt(item["id"].(json.Number).String(), 10, 64)
		if id == 7 {
			t.Errorf("unauthorized subscription group 7 must be filtered out")
		}
	}
}

func TestFilterGroupListPayloadByPolicy_NoAuthorized(t *testing.T) {
	payload := map[string]any{
		"data": []any{
			map[string]any{"id": json.Number("3"), "subscription_type": ""},
			map[string]any{"id": json.Number("7"), "subscription_type": "subscription"},
		},
	}
	got, err := filterGroupListPayloadByPolicy(payload, map[int64]struct{}{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	items := got.(map[string]any)["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("fresh user should see only standard groups, got %v", items)
	}
}
```

（import 若缺：`encoding/json`、`strconv` —— routes_test.go 大概率已有。）

- [ ] **Step 7.2: 运行确认失败**

```bash
cd $WT/backend && go test ./internal/httpapi/ -run TestFilterGroupListPayloadByPolicy -v
```
预期：FAIL（undefined）

- [ ] **Step 7.3: 实现纯函数 + 重写 handler**

纯函数（放 `filterGroupListPayloadByID` 之后）：

```go
// groupPolicyVisible reports whether an upstream available-group item should be
// visible to the user: non-subscription groups are open to everyone (billed
// from balance); subscription groups require local authorization.
func groupPolicyVisible(item map[string]any, authorizedGroupIDs map[int64]struct{}) bool {
	groupID, _ := asInt64(item["id"])
	if _, allowed := authorizedGroupIDs[groupID]; allowed {
		return true
	}
	subscriptionType := strings.TrimSpace(stringFromAny(item["subscription_type"]))
	if subscriptionType == "" {
		subscriptionType = strings.TrimSpace(stringFromAny(item["type"]))
	}
	return !strings.EqualFold(subscriptionType, "subscription")
}

func filterGroupItemsByPolicy(items []any, authorizedGroupIDs map[int64]struct{}) []any {
	filtered := make([]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if groupPolicyVisible(item, authorizedGroupIDs) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterGroupListPayloadByPolicy(payload any, authorizedGroupIDs map[int64]struct{}) (any, error) {
	if root, ok := payload.(map[string]any); ok {
		cloned := cloneMap(root)
		if groups, ok := root["data"].([]any); ok {
			cloned["data"] = filterGroupItemsByPolicy(groups, authorizedGroupIDs)
			return cloned, nil
		}
	}
	if groups, ok := payload.([]any); ok {
		return filterGroupItemsByPolicy(groups, authorizedGroupIDs), nil
	}
	return payload, nil
}
```

重写 `handleFilteredGroupsAvailablePassthrough`（整体替换）：

```go
func (r *routes) handleFilteredGroupsAvailablePassthrough(w http.ResponseWriter, req *http.Request) {
	if r.proxyClient == nil {
		writeError(w, http.StatusInternalServerError, "proxy client is not configured")
		return
	}

	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if user.Role == "admin" {
		r.handleDashboardPassthrough(w, req, "/api/v1/groups/available")
		return
	}

	authorizedGroupIDs, err := r.loadAuthorizedGroupIDSet(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load authorized groups")
		return
	}

	// 注意：不再因无订阅而短路空列表 —— 标准组对所有人可见（spec 3.5）。
	filteredPayload, statusCode, headers, handled, err := r.filteredProxyJSONResponse(w, req, "/api/v1/groups/available", func(payload any) (any, error) {
		return filterGroupListPayloadByPolicy(payload, authorizedGroupIDs)
	})
	if err != nil {
		// 上游不可用：普通用户回退空列表，不暴露 502（订阅用户也不会误见未授权组）。
		slog.Warn("groups available fetch failed; falling back to empty list", "user_id", user.ID, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]any{}})
		return
	}
	if !handled {
		return
	}
	writeForwardedJSON(w, statusCode, headers, filteredPayload)
}
```

- [ ] **Step 7.4: 运行确认通过 + 回归**

```bash
cd $WT/backend && go test ./internal/httpapi/
```
预期：PASS（既有透传/鉴权测试不受影响；Step 7.1 测试里的 `if got[0] != nil {}` no-op 行若编译器报错可删除）

- [ ] **Step 7.5: Commit**

```bash
cd $WT && git add backend/internal/httpapi/routes.go backend/internal/httpapi/routes_test.go
git commit -m "feat(httpapi): groups/available 对普通用户放开非订阅制标准组"
```

---

## Task 8: 前端 —— /keys 页创建表单 + 加载兜底 ensure

**Files:**
- Create: `frontend/app/api-keys/ensure-auto/route.ts`（⚠️ 必需：Next BFF 只按固定路径转发，`/api-keys/ensure-auto` 否则会落进 `[id]` 动态路由且无 POST handler → 405，兜底触发变死功能）
- Modify: `frontend/app/(app)/keys/page.tsx`
- Modify: `frontend/messages/zh.json`、`frontend/messages/en.json`（`dashboard` 命名空间内、`tabApiKeys`（约 1099 行）附近）

- [ ] **Step 8.0: 新增 BFF 转发路由（先于其他步骤，静态段 `ensure-auto` 优先于 `[id]` 匹配）**

创建 `frontend/app/api-keys/ensure-auto/route.ts`（与 `frontend/app/api/api-keys/route.ts` 同款转发模式）：

```ts
import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function POST(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const upstream = await fetch(`${apiBaseUrl}/api-keys/ensure-auto`, {
    method: "POST",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
    },
    cache: "no-store",
  });

  try {
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "invalid json response from upstream" },
      { status: 502 },
    );
  }
}
```

- [ ] **Step 8.1: 增加 i18n 文案**

`frontend/messages/zh.json` 的 `"dashboard"` 对象内（`"confirmDeleteKey"` 之后）追加：

```json
    "createKeyTitle": "创建 API 密钥",
    "createKeyNameLabel": "名称",
    "createKeyNamePlaceholder": "例如 my-claude-key",
    "createKeyGroupLabel": "分组",
    "createKeySubmit": "创建",
    "createKeyCreating": "创建中…",
    "createKeySuccess": "API 密钥创建成功",
    "createKeyError": "创建 API 密钥失败",
```

`frontend/messages/en.json` 对应位置追加：

```json
    "createKeyTitle": "Create API Key",
    "createKeyNameLabel": "Name",
    "createKeyNamePlaceholder": "e.g. my-claude-key",
    "createKeyGroupLabel": "Group",
    "createKeySubmit": "Create",
    "createKeyCreating": "Creating…",
    "createKeySuccess": "API key created",
    "createKeyError": "Failed to create API key",
```

- [ ] **Step 8.2: keys 页面接入**

`frontend/app/(app)/keys/page.tsx`：

(a) import 增加（`extractApiError` 来自 `@/lib/api-response`，与 `account/page.tsx:7` 一致）：

```tsx
import { extractApiError } from "@/lib/api-response";
```

(b) state 增加（放在 `busyKeyId` 之后）：

```tsx
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyGroupId, setNewKeyGroupId] = useState<number | null>(null);
  const [creatingKey, setCreatingKey] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [createSuccess, setCreateSuccess] = useState<string | null>(null);
```

(c) `loadAll` 内、`Promise.all` 之前插入兜底 ensure（幂等、best-effort）：

```tsx
      // Best-effort: idempotently provision auto keys before listing, so a
      // fresh user sees keys on first visit. Failure must not block listing.
      await fetch("/api-keys/ensure-auto", { method: "POST", headers, cache: "no-store" }).catch(() => null);
```

(d) 新增创建 handler（放 `handleDelete` 之后）：

```tsx
  const handleCreateKey = async (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setCreateError(null);
    setCreateSuccess(null);
    if (!sessionToken || !newKeyName.trim() || !newKeyGroupId) return;
    setCreatingKey(true);
    try {
      const res = await fetch("/api-keys", {
        method: "POST",
        headers: authHeaders(sessionToken),
        body: JSON.stringify({ name: newKeyName.trim(), group_id: newKeyGroupId }),
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        throw new Error(extractApiError(payload, t("createKeyError")));
      }
      setCreateSuccess(t("createKeySuccess"));
      setNewKeyName("");
      setNewKeyGroupId(null);
      await loadAll();
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : t("createKeyError"));
    } finally {
      setCreatingKey(false);
    }
  };
```

(e) JSX：keys tab 内、「筛选」div 之前插入创建表单（沿用页面既有 clay-panel / var(--…) 风格）：

```tsx
          {/* 创建 key */}
          <form onSubmit={handleCreateKey} className="clay-panel flex flex-wrap items-end gap-3 p-4">
            <div className="flex flex-col gap-1">
              <label htmlFor="new-key-name" className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("createKeyNameLabel")}
              </label>
              <input
                id="new-key-name"
                value={newKeyName}
                onChange={(event) => setNewKeyName(event.target.value)}
                placeholder={t("createKeyNamePlaceholder")}
                maxLength={100}
                className="field w-56"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="new-key-group" className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">
                {t("createKeyGroupLabel")}
              </label>
              <select
                id="new-key-group"
                value={newKeyGroupId === null ? "" : String(newKeyGroupId)}
                onChange={(event) => setNewKeyGroupId(event.target.value ? Number(event.target.value) : null)}
                className="field w-56"
              >
                <option value="">—</option>
                {groups.map((group) => (
                  <option key={group.id} value={String(group.id)}>
                    {group.name}
                    {group.platform ? ` · ${platformBadgeLabel(group.platform)}` : ""}
                  </option>
                ))}
              </select>
            </div>
            <button
              type="submit"
              disabled={creatingKey || !newKeyName.trim() || !newKeyGroupId}
              className="rounded-full bg-[var(--ink)] px-5 py-2 text-sm font-bold text-[var(--paper)] transition-opacity disabled:opacity-50"
            >
              {creatingKey ? t("createKeyCreating") : t("createKeySubmit")}
            </button>
            {createSuccess ? <span className="text-xs font-bold text-emerald-500">{createSuccess}</span> : null}
            {createError ? <span className="text-xs font-bold text-red-500">{createError}</span> : null}
          </form>
```

- [ ] **Step 8.3: lint + 单测 + 构建**

```bash
cd $WT/frontend && npm run lint && npm test && npm run build
```
预期：全部通过（vitest 若无 keys 页测试则仅保证既有用例不回归）

- [ ] **Step 8.4: Commit**

```bash
cd $WT && git add frontend/app/api-keys/ensure-auto/route.ts "frontend/app/(app)/keys/page.tsx" frontend/messages/zh.json frontend/messages/en.json
git commit -m "feat(keys): 创建 API key 表单 + 页面加载幂等兜底 ensure（含 BFF 转发路由）"
```

---

## Task 9: 全量验证与收尾

- [ ] **Step 9.1: 后端全量**

```bash
cd $WT/backend && go vet ./... && go build ./... && go test ./...
```
预期：全部 PASS

- [ ] **Step 9.2: 前端全量**

```bash
cd $WT/frontend && npm run lint && npm test && npm run build
```
预期：全部 PASS

- [ ] **Step 9.3: 手动冒烟（可选但推荐，需本地 sub2api 环境）**

1. 注册新用户 → 观察 backend 日志出现 `ensure default api keys after register`
2. 打开 /keys 页 → 分组下拉出现标准组；列表出现各平台 `auto-key`
3. 用表单创建一个自定义 key → 列表刷新可见、可删除
4. 重复刷新 /keys → 不产生重复 auto-key

- [ ] **Step 9.4: 汇报**

向用户汇报：worktree 分支名、commit 列表、验证输出摘要。**不自行 merge 回 main**（由用户决定，遵循 worktree 隔离要求）。

---

## 风险与边界回顾（实现者必读）

1. **绝不更新/删除任何 key** —— 全链路只有 `CreateUserAPIKey`。
2. **查重是主保证**：同组存在任何 `auto-key`（含停用）即跳过；409 兜底；上游对 `Idempotency-Key` 的去重行为未承诺，仅作辅助。
3. **所有新失败路径不得影响既有流程**：注册响应、keys 列表、分组列表在上游故障时全部照常返回。
4. `auto-key` 受保护不可删（现状），勿改动相关拦截逻辑。
5. 遵循仓库既有代码风格（中文注释在业务语义处、英文注释在安全语义处均可见，保持就近一致）。

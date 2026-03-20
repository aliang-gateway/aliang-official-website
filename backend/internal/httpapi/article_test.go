package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-api-portal/backend/internal/article"
	"ai-api-portal/backend/internal/model"
)

func TestPublicListArticlesReturnsPublishedOnly(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload publicArticleListResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Articles) != 5 {
		t.Fatalf("expected 5 seeded articles, got %d", len(payload.Articles))
	}

	for i, a := range payload.Articles {
		if a.Slug == "" {
			t.Fatalf("article %d: slug is empty", i)
		}
		if a.Title == "" {
			t.Fatalf("article %d: title is empty", i)
		}
		if a.PublishedAt == "" {
			t.Fatalf("article %d: published_at is empty", i)
		}
	}
}

func TestPublicListArticlesOmitsMdxBody(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	raw := make(map[string]json.RawMessage)
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}

	var articles []json.RawMessage
	if err := json.Unmarshal(raw["articles"], &articles); err != nil {
		t.Fatalf("unmarshal articles: %v", err)
	}

	for i, a := range articles {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(a, &obj); err != nil {
			t.Fatalf("unmarshal article %d: %v", i, err)
		}
		if _, exists := obj["mdx_body"]; exists {
			t.Fatalf("article %d: mdx_body field should not be present in list response", i)
		}
	}
}

func TestPublicListArticlesSortedByPublishedAtDesc(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload publicArticleListResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	for i := 1; i < len(payload.Articles); i++ {
		if payload.Articles[i-1].PublishedAt < payload.Articles[i].PublishedAt {
			t.Fatalf(
				"articles not sorted by published_at desc: index %d has %q, index %d has %q",
				i-1, payload.Articles[i-1].PublishedAt,
				i, payload.Articles[i].PublishedAt,
			)
		}
	}
}

func TestPublicGetArticleReturnsDetailForPublished(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles/gpt4o-vs-claude35", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload publicArticleDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Article.Slug != "gpt4o-vs-claude35" {
		t.Fatalf("expected slug gpt4o-vs-claude35, got %q", payload.Article.Slug)
	}
	if !strings.Contains(payload.Article.Title, "GPT-4o") {
		t.Fatalf("expected title to contain GPT-4o, got %q", payload.Article.Title)
	}
	if payload.Article.MDXBody == "" {
		t.Fatal("expected non-empty mdx_body in detail response")
	}
	if payload.Article.PublishedAt == "" {
		t.Fatal("expected non-empty published_at")
	}
}

func TestPublicGetArticleNonExistentSlugReturns404(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles/non-existent-slug-ever", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPublicGetArticleDraftSlugReturns404(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	svc := article.NewService(database)

	draftArticle := &model.Article{
		Slug:    "draft-test-article",
		Title:   "Draft Only Article",
		MDXBody: "# This is a draft\n\nShould not be public.",
		Status:  "draft",
	}
	if err := svc.CreateArticle(context.Background(), draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles/draft-test-article", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for draft article, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPublicListArticlesExcludesDraftArticles(t *testing.T) {
	t.Parallel()

	database := setupTestDB(t)
	if err := article.SeedLegacyArticles(database); err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	svc := article.NewService(database)
	draftArticle := &model.Article{
		Slug:    "draft-hidden-article",
		Title:   "Hidden Draft",
		MDXBody: "# Hidden\n\nNot visible.",
		Status:  "draft",
	}
	if err := svc.CreateArticle(context.Background(), draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/articles", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload publicArticleListResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Articles) != 5 {
		t.Fatalf("expected 5 published articles (draft excluded), got %d", len(payload.Articles))
	}

	for _, a := range payload.Articles {
		if a.Slug == "draft-hidden-article" {
			t.Fatal("draft article should not appear in public list")
		}
	}
}

func TestAdminArticleLifecycleCRUDPublishUnpublish(t *testing.T) {
	t.Parallel()

	_, mux, adminSessionToken, _ := setupAdminArticleTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/admin/articles", strings.NewReader(`{"slug":"admin-lifecycle-article","title":"Admin Lifecycle","mdx_body":"# Lifecycle","status":"draft"}`))
	setBearerAuth(createReq, adminSessionToken)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createRec.Code)
	}

	var created adminArticleDTO
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Slug != "admin-lifecycle-article" {
		t.Fatalf("expected slug admin-lifecycle-article, got %q", created.Slug)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/articles/admin-lifecycle-article", nil)
	setBearerAuth(getReq, adminSessionToken)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRec.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/admin/articles/admin-lifecycle-article", strings.NewReader(`{"title":"Admin Lifecycle Updated"}`))
	setBearerAuth(updateReq, adminSessionToken)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateRec.Code)
	}

	var updated adminArticleDTO
	if err := json.NewDecoder(updateRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Title != "Admin Lifecycle Updated" {
		t.Fatalf("expected updated title, got %q", updated.Title)
	}

	publishReq := httptest.NewRequest(http.MethodPost, "/admin/articles/admin-lifecycle-article/publish", nil)
	setBearerAuth(publishReq, adminSessionToken)
	publishRec := httptest.NewRecorder()
	mux.ServeHTTP(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, publishRec.Code)
	}

	var published adminArticleDTO
	if err := json.NewDecoder(publishRec.Body).Decode(&published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if published.Status != "published" {
		t.Fatalf("expected published status, got %q", published.Status)
	}

	unpublishReq := httptest.NewRequest(http.MethodPost, "/admin/articles/admin-lifecycle-article/unpublish", nil)
	setBearerAuth(unpublishReq, adminSessionToken)
	unpublishRec := httptest.NewRecorder()
	mux.ServeHTTP(unpublishRec, unpublishReq)
	if unpublishRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, unpublishRec.Code)
	}

	var unpublished adminArticleDTO
	if err := json.NewDecoder(unpublishRec.Body).Decode(&unpublished); err != nil {
		t.Fatalf("decode unpublish response: %v", err)
	}
	if unpublished.Status != "draft" {
		t.Fatalf("expected draft status after unpublish, got %q", unpublished.Status)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/articles/admin-lifecycle-article", nil)
	setBearerAuth(deleteReq, adminSessionToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deleteRec.Code)
	}
}

func TestAdminArticleEndpointsRequireAuthAndAdmin(t *testing.T) {
	t.Parallel()

	_, mux, _, nonAdminSessionToken := setupAdminArticleTestServer(t)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/admin/articles"},
		{name: "create", method: http.MethodPost, path: "/admin/articles", body: `{"slug":"guard-create-article","title":"Guard Create","mdx_body":"# Guard"}`},
		{name: "get", method: http.MethodGet, path: "/admin/articles/guard-article"},
		{name: "update", method: http.MethodPut, path: "/admin/articles/guard-article", body: `{"title":"Guard Update"}`},
		{name: "delete", method: http.MethodDelete, path: "/admin/articles/guard-article"},
		{name: "publish", method: http.MethodPost, path: "/admin/articles/guard-article/publish"},
		{name: "unpublish", method: http.MethodPost, path: "/admin/articles/guard-article/unpublish"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			unauthReq := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			unauthRec := httptest.NewRecorder()
			mux.ServeHTTP(unauthRec, unauthReq)
			if unauthRec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d for unauthenticated request, got %d", http.StatusUnauthorized, unauthRec.Code)
			}

			nonAdminReq := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			setBearerAuth(nonAdminReq, nonAdminSessionToken)
			nonAdminRec := httptest.NewRecorder()
			mux.ServeHTTP(nonAdminRec, nonAdminReq)
			if nonAdminRec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d for non-admin request, got %d", http.StatusForbidden, nonAdminRec.Code)
			}
		})
	}
}

func TestAdminCreateArticleValidationAndConflict(t *testing.T) {
	t.Parallel()

	database, mux, adminSessionToken, _ := setupAdminArticleTestServer(t)
	createAdminArticleFixture(t, database, "existing-article", "draft")

	testCases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "duplicate slug", body: `{"slug":"existing-article","title":"Duplicate","mdx_body":"# Body"}`, wantStatus: http.StatusConflict},
		{name: "invalid slug format", body: `{"slug":"Invalid Slug","title":"Invalid Slug","mdx_body":"# Body"}`, wantStatus: http.StatusBadRequest},
		{name: "missing required fields", body: `{"slug":"missing-fields-article","mdx_body":"# Body"}`, wantStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/articles", strings.NewReader(tc.body))
			setBearerAuth(req, adminSessionToken)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}

			var payload errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if payload.Error == "" {
				t.Fatal("expected non-empty error message")
			}
		})
	}
}

func TestAdminUpdateArticleInvalidStatusTransitionReturns400(t *testing.T) {
	t.Parallel()

	database, mux, adminSessionToken, _ := setupAdminArticleTestServer(t)
	ctx := context.Background()
	createAdminArticleFixture(t, database, "status-transition-article", "draft")

	if _, err := database.ExecContext(ctx, `DROP TABLE articles;`); err != nil {
		t.Fatalf("drop articles table: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			legacy_id INTEGER,
			slug TEXT NOT NULL,
			title TEXT NOT NULL,
			excerpt TEXT,
			cover_image_url TEXT,
			tag TEXT,
			read_time TEXT,
			author_name TEXT,
			author_avatar_url TEXT,
			author_icon TEXT,
			mdx_body TEXT NOT NULL,
			status TEXT NOT NULL,
			published_at TIMESTAMP,
			created_by_user_id INTEGER,
			updated_by_user_id INTEGER,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("create permissive articles table: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO articles(slug, title, mdx_body, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`, "status-transition-article", "Corrupted Article", "# Corrupted", "archived"); err != nil {
		t.Fatalf("insert corrupted article: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/admin/articles/status-transition-article", strings.NewReader(`{"status":"draft"}`))
	setBearerAuth(req, adminSessionToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAdminGetAndDeleteNonExistentArticleReturn404(t *testing.T) {
	t.Parallel()

	_, mux, adminSessionToken, _ := setupAdminArticleTestServer(t)

	getReq := httptest.NewRequest(http.MethodGet, "/admin/articles/non-existent-article", nil)
	setBearerAuth(getReq, adminSessionToken)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, getRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/articles/non-existent-article", nil)
	setBearerAuth(deleteReq, adminSessionToken)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, deleteRec.Code)
	}
}

func setupAdminArticleTestServer(t *testing.T) (*sql.DB, *http.ServeMux, string, string) {
	t.Helper()

	database := setupTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})

	_, adminSessionToken := createUserViaAPI(t, mux, "admin-article@example.com", "Admin Article", "admin", "test-admin-secret")
	_, nonAdminSessionToken := createUserViaAPI(t, mux, "member-article@example.com", "Member Article", "user", "")

	return database, mux, adminSessionToken, nonAdminSessionToken
}

func createAdminArticleFixture(t *testing.T, database *sql.DB, slug, status string) {
	t.Helper()

	svc := article.NewService(database)
	entry := &model.Article{
		Slug:    slug,
		Title:   "Fixture " + slug,
		MDXBody: "# Fixture",
		Status:  status,
	}
	if err := svc.CreateArticle(context.Background(), entry); err != nil {
		t.Fatalf("create fixture article: %v", err)
	}
}

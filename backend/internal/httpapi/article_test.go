package httpapi

import (
	"context"
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

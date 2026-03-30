package article

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

func TestCreateArticleAndGetByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)
	createdBy := createUser(t, ctx, database, "article1@example.com", "Article One", "admin")

	legacyID := int64(101)
	excerpt := "A concise excerpt"
	author := "Alice"
	article := &model.Article{
		LegacyID:        &legacyID,
		Slug:            "first-article",
		Title:           "First Article",
		Excerpt:         &excerpt,
		MDXBody:         "# Hello World",
		Status:          "draft",
		CreatedByUserID: &createdBy,
		UpdatedByUserID: &createdBy,
		AuthorName:      &author,
	}

	if err := service.CreateArticle(ctx, article); err != nil {
		t.Fatalf("CreateArticle() error = %v", err)
	}

	if article.ID <= 0 {
		t.Fatalf("expected article ID to be set")
	}
	if article.CreatedAt.IsZero() || article.UpdatedAt.IsZero() {
		t.Fatalf("expected create/update timestamps to be set")
	}
	if !article.CreatedAt.Equal(article.UpdatedAt) {
		t.Fatalf("expected created_at and updated_at to match on create")
	}

	found, err := service.GetArticleByID(ctx, article.ID)
	if err != nil {
		t.Fatalf("GetArticleByID() error = %v", err)
	}
	if found.Slug != "first-article" || found.Title != "First Article" || found.Status != "draft" {
		t.Fatalf("unexpected article payload: %+v", found)
	}
}

func TestCreateArticleDuplicateSlug(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	first := &model.Article{Slug: "dup-slug", Title: "First", MDXBody: "one", Status: "draft"}
	if err := service.CreateArticle(ctx, first); err != nil {
		t.Fatalf("CreateArticle() first error = %v", err)
	}

	second := &model.Article{Slug: "dup-slug", Title: "Second", MDXBody: "two", Status: "published"}
	err := service.CreateArticle(ctx, second)
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestUpdateArticleAndDuplicateSlugCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	first := &model.Article{Slug: "update-me", Title: "Original", MDXBody: "body", Status: "draft"}
	second := &model.Article{Slug: "occupied-slug", Title: "Other", MDXBody: "body2", Status: "draft"}
	if err := service.CreateArticle(ctx, first); err != nil {
		t.Fatalf("create first article error = %v", err)
	}
	if err := service.CreateArticle(ctx, second); err != nil {
		t.Fatalf("create second article error = %v", err)
	}

	update := &model.Article{Slug: "renamed-article", Title: "Updated", MDXBody: "new body", Status: "published"}
	if err := service.UpdateArticle(ctx, "update-me", update); err != nil {
		t.Fatalf("UpdateArticle() error = %v", err)
	}

	updated, err := service.GetArticleByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetArticleByID() error = %v", err)
	}
	if updated.Slug != "renamed-article" || updated.Status != "published" {
		t.Fatalf("unexpected updated article: %+v", updated)
	}
	if updated.PublishedAt == nil {
		t.Fatalf("expected published_at to be set when published")
	}

	conflictingUpdate := &model.Article{Slug: "occupied-slug", Title: "Conflict", MDXBody: "body", Status: "draft"}
	err = service.UpdateArticle(ctx, "renamed-article", conflictingUpdate)
	if !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug on update, got %v", err)
	}
}

func TestListAndPublicAccessFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	draft := &model.Article{Slug: "draft-only", Title: "Draft", MDXBody: "draft body", Status: "draft"}
	pub := &model.Article{Slug: "public-one", Title: "Published", MDXBody: "pub body", Status: "published"}
	if err := service.CreateArticle(ctx, draft); err != nil {
		t.Fatalf("create draft error = %v", err)
	}
	if err := service.CreateArticle(ctx, pub); err != nil {
		t.Fatalf("create published error = %v", err)
	}

	all, err := service.ListArticles(ctx, ListArticlesFilters{})
	if err != nil {
		t.Fatalf("ListArticles(all) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 als_articles in all list, got %d", len(all))
	}

	drafts, err := service.ListArticles(ctx, ListArticlesFilters{Status: "draft"})
	if err != nil {
		t.Fatalf("ListArticles(draft) error = %v", err)
	}
	if len(drafts) != 1 || drafts[0].Slug != "draft-only" {
		t.Fatalf("unexpected draft list: %+v", drafts)
	}

	publicList, err := service.ListPublishedArticles(ctx)
	if err != nil {
		t.Fatalf("ListPublishedArticles() error = %v", err)
	}
	if len(publicList) != 1 || publicList[0].Slug != "public-one" {
		t.Fatalf("unexpected published list: %+v", publicList)
	}

	if _, err := service.GetArticleBySlug(ctx, "draft-only"); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("expected draft to be hidden in GetArticleBySlug, got %v", err)
	}
	if _, err := service.GetArticleBySlug(ctx, "public-one"); err != nil {
		t.Fatalf("expected published slug lookup success, got %v", err)
	}
}

func TestPublishUnpublishAndDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	article := &model.Article{Slug: "stateful", Title: "Stateful", MDXBody: "body", Status: "draft"}
	if err := service.CreateArticle(ctx, article); err != nil {
		t.Fatalf("create article error = %v", err)
	}

	if err := service.PublishArticle(ctx, "stateful"); err != nil {
		t.Fatalf("PublishArticle() error = %v", err)
	}
	published, err := service.GetArticleByID(ctx, article.ID)
	if err != nil {
		t.Fatalf("GetArticleByID() after publish error = %v", err)
	}
	if published.Status != "published" || published.PublishedAt == nil {
		t.Fatalf("expected published status and published_at, got %+v", published)
	}

	if err := service.UnpublishArticle(ctx, "stateful"); err != nil {
		t.Fatalf("UnpublishArticle() error = %v", err)
	}
	draftAgain, err := service.GetArticleByID(ctx, article.ID)
	if err != nil {
		t.Fatalf("GetArticleByID() after unpublish error = %v", err)
	}
	if draftAgain.Status != "draft" || draftAgain.PublishedAt != nil {
		t.Fatalf("expected draft status and nil published_at, got %+v", draftAgain)
	}

	if err := service.DeleteArticle(ctx, "stateful"); err != nil {
		t.Fatalf("DeleteArticle() error = %v", err)
	}
	if _, err := service.GetArticleByID(ctx, article.ID); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("expected deleted article to be not found, got %v", err)
	}
}

func TestListArticlesInvalidStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := setupTestDB(t)
	service := NewService(database)

	_, err := service.ListArticles(ctx, ListArticlesFilters{Status: "archived"})
	if err == nil {
		t.Fatalf("expected error for invalid status filter")
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(ctx, "sqlite", dbFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.ApplyMigrations(ctx, database, "sqlite"); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	return database
}

func createUser(t *testing.T, ctx context.Context, database *sql.DB, email, name, role string) int64 {
	t.Helper()

	result, err := database.ExecContext(ctx, `INSERT INTO als_users(email, name, role) VALUES (?, ?, ?);`, email, name, role)
	if err != nil {
		t.Fatalf("createUser insert error = %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("createUser LastInsertId error = %v", err)
	}

	return id
}

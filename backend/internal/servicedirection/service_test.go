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
		Status:  "done",
		PhaseZh: "阶段", PhaseEn: "phase",
		TitleZh: "标题", TitleEn: "title",
		DescZh: "描述", DescEn: "desc",
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

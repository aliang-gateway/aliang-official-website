package download

import (
	"context"
	"path/filepath"
	"testing"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

func setupTestService(t *testing.T) *Service {
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

	return NewService(database, "sqlite")
}

func TestCreateDownloadAcceptsMultiDigitVersionSegments(t *testing.T) {
	t.Parallel()

	svc := setupTestService(t)
	download := &model.Download{
		SoftwareName: "aliang-helper",
		Platform:     "darwin",
		FileType:     "dmg",
		DownloadURL:  "https://example.com/aliang-helper-v1.23.123.dmg",
		Version:      "v1.23.123",
	}

	if err := svc.CreateDownload(context.Background(), download); err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}

	if download.ID <= 0 {
		t.Fatalf("expected positive ID, got %d", download.ID)
	}
}

func TestCheckVersionUsesMultiDigitNumericComparison(t *testing.T) {
	t.Parallel()

	svc := setupTestService(t)
	ctx := context.Background()

	downloads := []*model.Download{
		{
			SoftwareName: "aliang-helper",
			Platform:     "darwin",
			FileType:     "dmg",
			DownloadURL:  "https://example.com/aliang-helper-v1.9.9.dmg",
			Version:      "v1.9.9",
		},
		{
			SoftwareName: "aliang-helper",
			Platform:     "darwin",
			FileType:     "dmg",
			DownloadURL:  "https://example.com/aliang-helper-v1.23.123.dmg",
			Version:      "v1.23.123",
			ForceUpdate:  true,
		},
	}

	for _, item := range downloads {
		if err := svc.CreateDownload(ctx, item); err != nil {
			t.Fatalf("CreateDownload(%s) error = %v", item.Version, err)
		}
	}

	result, err := svc.CheckVersion(ctx, "darwin", "aliang-helper", "v1.10.2")
	if err != nil {
		t.Fatalf("CheckVersion() error = %v", err)
	}

	if result.LatestVer != "v1.23.123" {
		t.Fatalf("expected latest_version v1.23.123, got %q", result.LatestVer)
	}
	if !result.NeedsUpdate {
		t.Fatalf("expected needs_update=true")
	}
	if !result.ForceUpdate {
		t.Fatalf("expected force_update=true")
	}
}

func TestCheckVersionRejectsVersionWithoutVPrefix(t *testing.T) {
	t.Parallel()

	svc := setupTestService(t)

	_, err := svc.CheckVersion(context.Background(), "darwin", "aliang-helper", "1.23.123")
	if err != ErrInvalidVersion {
		t.Fatalf("expected ErrInvalidVersion, got %v", err)
	}
}

package download

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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

// preferLatestDownload 对"版本相等的重复行"必须给出确定性裁决:updated_at
// 新者、再退 id 大者——否则 sort.Slice 非稳定排序让 latest(force_update/
// changelog)跨请求随机翻转。(0030 唯一约束防新重复行,此比较器兜历史数据
// 与约束前迁移窗口。)
func TestPreferLatestDownloadDeterministic(t *testing.T) {
	t.Parallel()

	newer := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	forced := model.Download{ID: 2, Version: "v2.0.0", ForceUpdate: true, UpdatedAt: newer}
	soft := model.Download{ID: 1, Version: "v2.0.0", ForceUpdate: false, UpdatedAt: older}

	// 等版本:updated_at 新者胜 —— force=true 的那行成为 latest,跨调用稳定。
	if !preferLatestDownload(forced, soft) {
		t.Fatal("newer-updated forced row should sort first")
	}
	if preferLatestDownload(soft, forced) {
		t.Fatal("older-updated soft row should not sort first")
	}

	// updated_at 也相等:退 id 大者。
	idNewer := model.Download{ID: 9, Version: "v2.0.0", UpdatedAt: newer}
	idOlder := model.Download{ID: 3, Version: "v2.0.0", UpdatedAt: newer}
	if !preferLatestDownload(idNewer, idOlder) {
		t.Fatal("higher-id row should sort first when updated_at equal")
	}

	// 版本不等:版本降序优先于一切。
	higher := model.Download{ID: 1, Version: "v3.0.0", UpdatedAt: older}
	lower := model.Download{ID: 2, Version: "v2.9.9", UpdatedAt: newer}
	if !preferLatestDownload(higher, lower) {
		t.Fatal("higher version should sort first regardless of updated_at")
	}

	// 版本非法(脏数据):退 updated_at/id 比较,不得 panic。
	junk := model.Download{ID: 5, Version: "garbage", UpdatedAt: newer}
	if !preferLatestDownload(junk, soft) {
		t.Fatal("invalid-version row falls back to updated_at compare")
	}
}

// 0030 迁移的唯一约束必须在位:同 (software, platform, version) 二次插入被拒。
func TestMigrationsRejectDuplicateSoftwarePlatformVersion(t *testing.T) {
	t.Parallel()

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

	insert := func(url string) error {
		_, err := database.ExecContext(ctx,
			`INSERT INTO als_downloads (software_name, platform, file_type, download_url, version, force_update, changelog)
			 VALUES ('aliang-vibe-phone', 'ios', 'ipa', ?, 'v1.0.6', FALSE, '')`,
			url)
		return err
	}
	if err := insert("https://www.aliang.one/first"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert("https://www.aliang.one/duplicate"); err == nil {
		t.Fatal("duplicate (software, platform, version) insert succeeded; unique constraint missing")
	}
}

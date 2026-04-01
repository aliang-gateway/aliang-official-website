package download

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	db "ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

type Service struct {
	db         *sql.DB
	sqlDialect string
}

func NewService(database *sql.DB, sqlDialect string) *Service {
	return &Service{db: database, sqlDialect: sqlDialect}
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidVersion = errors.New("invalid version format, expected vMAJOR.MINOR.PATCH with single digits")
)

var versionPattern = regexp.MustCompile(`^v(\d)\.(\d)\.(\d)$`)

func (s *Service) rebind(q string) string {
	return db.Rebind(s.sqlDialect, q)
}

// --------------- Download CRUD ---------------

func (s *Service) CreateDownload(ctx context.Context, d *model.Download) error {
	d.SoftwareName = strings.TrimSpace(d.SoftwareName)
	d.Platform = strings.TrimSpace(d.Platform)
	d.FileType = strings.TrimSpace(d.FileType)
	d.DownloadURL = strings.TrimSpace(d.DownloadURL)
	d.Version = strings.TrimSpace(d.Version)

	if d.SoftwareName == "" || d.Platform == "" || d.FileType == "" || d.DownloadURL == "" || d.Version == "" {
		return errors.New("software_name, platform, file_type, download_url and version are required")
	}
	if !versionPattern.MatchString(d.Version) {
		return ErrInvalidVersion
	}

	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_downloads (software_name, platform, file_type, download_url, version, force_update, changelog, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"id",
		d.SoftwareName, d.Platform, d.FileType, d.DownloadURL, d.Version, d.ForceUpdate, d.Changelog, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert download: %w", err)
	}
	d.ID = id
	d.CreatedAt = now
	d.UpdatedAt = now
	return nil
}

func (s *Service) GetDownload(ctx context.Context, id int64) (*model.Download, error) {
	var d model.Download
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, software_name, platform, file_type, download_url, version, force_update, changelog, created_at, updated_at
		FROM als_downloads WHERE id = ?`), id,
	).Scan(&d.ID, &d.SoftwareName, &d.Platform, &d.FileType, &d.DownloadURL, &d.Version, &d.ForceUpdate, &d.Changelog, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query download: %w", err)
	}
	return &d, nil
}

func (s *Service) UpdateDownload(ctx context.Context, id int64, d *model.Download) error {
	d.SoftwareName = strings.TrimSpace(d.SoftwareName)
	d.Platform = strings.TrimSpace(d.Platform)
	d.FileType = strings.TrimSpace(d.FileType)
	d.DownloadURL = strings.TrimSpace(d.DownloadURL)
	d.Version = strings.TrimSpace(d.Version)

	if d.Version != "" && !versionPattern.MatchString(d.Version) {
		return ErrInvalidVersion
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_downloads
		SET software_name = ?, platform = ?, file_type = ?, download_url = ?, version = ?, force_update = ?, changelog = ?, updated_at = ?
		WHERE id = ?`),
		d.SoftwareName, d.Platform, d.FileType, d.DownloadURL, d.Version, d.ForceUpdate, d.Changelog, now, id,
	)
	if err != nil {
		return fmt.Errorf("update download: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	d.UpdatedAt = now
	return nil
}

func (s *Service) DeleteDownload(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_downloads WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete download: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListDownloads(ctx context.Context) ([]model.Download, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, software_name, platform, file_type, download_url, version, force_update, changelog, created_at, updated_at
		FROM als_downloads ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query downloads: %w", err)
	}
	defer rows.Close()

	var downloads []model.Download
	for rows.Next() {
		var d model.Download
		if err := rows.Scan(&d.ID, &d.SoftwareName, &d.Platform, &d.FileType, &d.DownloadURL, &d.Version, &d.ForceUpdate, &d.Changelog, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan download: %w", err)
		}
		downloads = append(downloads, d)
	}
	return downloads, rows.Err()
}

// --------------- Version Check ---------------

// parseVersion parses "vMAJOR.MINOR.PATCH" into three integers.
func parseVersion(v string) (major, minor, patch int, ok bool) {
	m := versionPattern.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return major, minor, patch, true
}

// compareVersions returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(aMajor, aMinor, aPatch, bMajor, bMinor, bPatch int) int {
	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1
		}
		return 1
	}
	if aPatch != bPatch {
		if aPatch < bPatch {
			return -1
		}
		return 1
	}
	return 0
}

type CheckResult struct {
	SoftwareName string `json:"software_name"`
	Platform     string `json:"platform"`
	CurrentVer   string `json:"current_version"`
	LatestVer    string `json:"latest_version"`
	DownloadURL  string `json:"download_url"`
	FileType     string `json:"file_type"`
	ForceUpdate  bool   `json:"force_update"`
	NeedsUpdate  bool   `json:"needs_update"`
	Changelog    string `json:"changelog,omitempty"`
}

// CheckVersion finds the latest download for the given platform and software,
// compares versions, and returns whether the user needs to update.
func (s *Service) CheckVersion(ctx context.Context, platform, softwareName, userVersion string) (*CheckResult, error) {
	platform = strings.TrimSpace(platform)
	softwareName = strings.TrimSpace(softwareName)
	userVersion = strings.TrimSpace(userVersion)

	if platform == "" || userVersion == "" {
		return nil, errors.New("platform and version are required")
	}

	uMajor, uMinor, uPatch, ok := parseVersion(userVersion)
	if !ok {
		return nil, ErrInvalidVersion
	}

	// Query all downloads for this platform (and optionally software_name)
	query := `
		SELECT id, software_name, platform, file_type, download_url, version, force_update, changelog, created_at, updated_at
		FROM als_downloads WHERE platform = ?`
	args := []any{platform}

	if softwareName != "" {
		query += ` AND software_name = ?`
		args = append(args, softwareName)
	}

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query downloads for check: %w", err)
	}
	defer rows.Close()

	var candidates []model.Download
	for rows.Next() {
		var d model.Download
		if err := rows.Scan(&d.ID, &d.SoftwareName, &d.Platform, &d.FileType, &d.DownloadURL, &d.Version, &d.ForceUpdate, &d.Changelog, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan download: %w", err)
		}
		candidates = append(candidates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, ErrNotFound
	}

	// Sort by version descending to find the latest
	sort.Slice(candidates, func(i, j int) bool {
		iMaj, iMin, iPat, _ := parseVersion(candidates[i].Version)
		jMaj, jMin, jPat, _ := parseVersion(candidates[j].Version)
		return compareVersions(iMaj, iMin, iPat, jMaj, jMin, jPat) > 0
	})

	latest := candidates[0]
	lMajor, lMinor, lPatch, _ := parseVersion(latest.Version)
	cmp := compareVersions(uMajor, uMinor, uPatch, lMajor, lMinor, lPatch)

	needsUpdate := cmp < 0
	forceUpdate := needsUpdate && latest.ForceUpdate

	return &CheckResult{
		SoftwareName: latest.SoftwareName,
		Platform:     latest.Platform,
		CurrentVer:   userVersion,
		LatestVer:    latest.Version,
		DownloadURL:  latest.DownloadURL,
		FileType:     latest.FileType,
		ForceUpdate:  forceUpdate,
		NeedsUpdate:  needsUpdate,
		Changelog:    latest.Changelog,
	}, nil
}

// ListPlatforms returns all distinct platforms that have downloads.
func (s *Service) ListPlatforms(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT platform FROM als_downloads ORDER BY platform`)
	if err != nil {
		return nil, fmt.Errorf("query platforms: %w", err)
	}
	defer rows.Close()

	var platforms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan platform: %w", err)
		}
		platforms = append(platforms, p)
	}
	return platforms, rows.Err()
}

// ListSoftwareNames returns all distinct software names that have downloads.
func (s *Service) ListSoftwareNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT software_name FROM als_downloads ORDER BY software_name`)
	if err != nil {
		return nil, fmt.Errorf("query software names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan software name: %w", err)
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

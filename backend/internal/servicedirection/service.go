package servicedirection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	db "ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

// Service manages the als_service_directions table backing the /services page.
type Service struct {
	db         *sql.DB
	sqlDialect string
}

func NewService(database *sql.DB, sqlDialect string) *Service {
	return &Service{db: database, sqlDialect: sqlDialect}
}

var ErrNotFound = errors.New("not found")

var validStatuses = map[string]bool{"research": true, "done": true}

func (s *Service) rebind(q string) string {
	return db.Rebind(s.sqlDialect, q)
}

const columns = `id, status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, sort_order, is_published, created_at, updated_at`

func scanServiceDirection(sd *model.ServiceDirection, scanner interface{ Scan(...any) error }) error {
	return scanner.Scan(
		&sd.ID, &sd.Status, &sd.PhaseZh, &sd.PhaseEn, &sd.TitleZh, &sd.TitleEn, &sd.DescZh, &sd.DescEn,
		&sd.SortOrder, &sd.IsPublished, &sd.CreatedAt, &sd.UpdatedAt,
	)
}

func (s *Service) normalizeAndValidate(sd *model.ServiceDirection) error {
	sd.Status = strings.TrimSpace(sd.Status)
	sd.PhaseZh = strings.TrimSpace(sd.PhaseZh)
	sd.PhaseEn = strings.TrimSpace(sd.PhaseEn)
	sd.TitleZh = strings.TrimSpace(sd.TitleZh)
	sd.TitleEn = strings.TrimSpace(sd.TitleEn)
	sd.DescZh = strings.TrimSpace(sd.DescZh)
	sd.DescEn = strings.TrimSpace(sd.DescEn)
	if !validStatuses[sd.Status] {
		return errors.New("status must be 'research' or 'done'")
	}
	if sd.TitleZh == "" || sd.TitleEn == "" {
		return errors.New("title_zh and title_en are required")
	}
	if sd.PhaseZh == "" || sd.PhaseEn == "" {
		return errors.New("phase_zh and phase_en are required")
	}
	if sd.DescZh == "" || sd.DescEn == "" {
		return errors.New("desc_zh and desc_en are required")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, sd *model.ServiceDirection) error {
	if err := s.normalizeAndValidate(sd); err != nil {
		return err
	}
	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_service_directions (status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, sort_order, is_published, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"id",
		sd.Status, sd.PhaseZh, sd.PhaseEn, sd.TitleZh, sd.TitleEn, sd.DescZh, sd.DescEn, sd.SortOrder, sd.IsPublished, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert service direction: %w", err)
	}
	sd.ID = id
	sd.CreatedAt = now
	sd.UpdatedAt = now
	return nil
}

func (s *Service) Get(ctx context.Context, id int64) (*model.ServiceDirection, error) {
	var sd model.ServiceDirection
	err := scanServiceDirection(&sd, s.db.QueryRowContext(ctx, s.rebind(
		`SELECT `+columns+` FROM als_service_directions WHERE id = ?`), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query service direction: %w", err)
	}
	return &sd, nil
}

func (s *Service) Update(ctx context.Context, id int64, sd *model.ServiceDirection) error {
	if err := s.normalizeAndValidate(sd); err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_service_directions
		SET status = ?, phase_zh = ?, phase_en = ?, title_zh = ?, title_en = ?, desc_zh = ?, desc_en = ?, sort_order = ?, is_published = ?, updated_at = ?
		WHERE id = ?`),
		sd.Status, sd.PhaseZh, sd.PhaseEn, sd.TitleZh, sd.TitleEn, sd.DescZh, sd.DescEn, sd.SortOrder, sd.IsPublished, now, id,
	)
	if err != nil {
		return fmt.Errorf("update service direction: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	sd.UpdatedAt = now
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_service_directions WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete service direction: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAll returns every direction (including drafts), ordered by sort_order then id.
func (s *Service) ListAll(ctx context.Context) ([]model.ServiceDirection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+columns+` FROM als_service_directions ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query service directions: %w", err)
	}
	defer rows.Close()
	return collect(rows)
}

// ListPublic returns only published directions, ordered by sort_order then id.
func (s *Service) ListPublic(ctx context.Context) ([]model.ServiceDirection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+columns+` FROM als_service_directions WHERE is_published = true ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query public service directions: %w", err)
	}
	defer rows.Close()
	return collect(rows)
}

func collect(rows *sql.Rows) ([]model.ServiceDirection, error) {
	var items []model.ServiceDirection
	for rows.Next() {
		var sd model.ServiceDirection
		if err := scanServiceDirection(&sd, rows); err != nil {
			return nil, fmt.Errorf("scan service direction: %w", err)
		}
		items = append(items, sd)
	}
	return items, rows.Err()
}

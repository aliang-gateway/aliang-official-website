package doc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/model"
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
)

type Service struct {
	db         *sql.DB
	sqlDialect string
}

type rowScanner interface {
	Scan(dest ...any) error
}

func NewService(database *sql.DB, sqlDialect string) *Service {
	return &Service{db: database, sqlDialect: sqlDialect}
}

func (s *Service) rebind(q string) string {
	return db.Rebind(s.sqlDialect, q)
}

// ── Category CRUD ──────────────────────────────────────────────────

func (s *Service) CreateCategory(ctx context.Context, cat *model.DocCategory) error {
	if cat == nil {
		return errors.New("category is required")
	}
	normalizeCategory(cat)
	if err := validateCategory(cat); err != nil {
		return err
	}
	if err := s.ensureCategorySlugUnique(ctx, cat.Slug, nil); err != nil {
		return err
	}

	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_doc_categories(slug, title, description, icon, sort_order, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", cat.Slug, cat.Title, cat.Description, cat.Icon, cat.SortOrder, cat.Status, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("insert doc category: %w", err)
	}

	cat.ID = id
	cat.CreatedAt = now
	cat.UpdatedAt = now
	return nil
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, cat *model.DocCategory) error {
	if cat == nil {
		return errors.New("category is required")
	}

	existing, err := s.GetCategoryByID(ctx, id)
	if err != nil {
		return err
	}

	mergeCategory(existing, cat)
	if err := validateCategory(cat); err != nil {
		return err
	}
	if err := s.ensureCategorySlugUnique(ctx, cat.Slug, &existing.ID); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_categories
		SET slug = ?, title = ?, description = ?, icon = ?, sort_order = ?, status = ?, updated_at = ?
		WHERE id = ?;
	`), cat.Slug, cat.Title, cat.Description, cat.Icon, cat.SortOrder, cat.Status, now, existing.ID)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("update doc category: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read doc category update rows affected: %w", err)
	}
	if affected == 0 {
		return ErrCategoryNotFound
	}

	cat.ID = existing.ID
	cat.CreatedAt = existing.CreatedAt
	cat.UpdatedAt = now
	return nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_doc_categories WHERE id = ?;`), id)
	if err != nil {
		return fmt.Errorf("delete doc category: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete doc category rows affected: %w", err)
	}
	if affected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (s *Service) GetCategoryByID(ctx context.Context, id int64) (*model.DocCategory, error) {
	var cat model.DocCategory
	err := scanCategory(
		s.db.QueryRowContext(ctx, s.rebind(`
			SELECT id, slug, title, description, icon, sort_order, status, created_at, updated_at
			FROM als_doc_categories
			WHERE id = ?
			LIMIT 1;
		`), id),
		&cat,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query doc category by id: %w", err)
	}
	return &cat, nil
}

type ListCategoriesFilter struct {
	Status string
}

func (s *Service) ListCategories(ctx context.Context, filter ListCategoriesFilter) ([]model.DocCategory, error) {
	query := `SELECT id, slug, title, description, icon, sort_order, status, created_at, updated_at FROM als_doc_categories`
	args := make([]any, 0, 1)

	status := strings.TrimSpace(strings.ToLower(filter.Status))
	if status != "" {
		if !isValidStatus(status) {
			return nil, errors.New("invalid status filter")
		}
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY sort_order ASC, id ASC;"

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query doc categories: %w", err)
	}
	defer rows.Close()

	categories := make([]model.DocCategory, 0)
	for rows.Next() {
		var cat model.DocCategory
		if err := scanCategory(rows, &cat); err != nil {
			return nil, fmt.Errorf("scan doc category: %w", err)
		}
		categories = append(categories, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate doc categories: %w", err)
	}
	return categories, nil
}

func (s *Service) PublishCategory(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_categories SET status = ?, updated_at = ? WHERE id = ?;
	`), statusPublished, now, id)
	if err != nil {
		return fmt.Errorf("publish doc category: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read publish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (s *Service) UnpublishCategory(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_categories SET status = ?, updated_at = ? WHERE id = ?;
	`), statusDraft, now, id)
	if err != nil {
		return fmt.Errorf("unpublish doc category: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read unpublish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// ── Page CRUD ──────────────────────────────────────────────────────

func (s *Service) CreatePage(ctx context.Context, page *model.DocPage) error {
	if page == nil {
		return errors.New("page is required")
	}
	normalizePage(page)
	if err := validatePage(page); err != nil {
		return err
	}
	if err := s.ensurePageSlugUnique(ctx, page.Slug, nil); err != nil {
		return err
	}

	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_doc_pages(slug, title, category_id, mdx_body, sort_order, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", page.Slug, page.Title, page.CategoryID, page.MDXBody, page.SortOrder, page.Status, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("insert doc page: %w", err)
	}

	page.ID = id
	page.CreatedAt = now
	page.UpdatedAt = now
	return nil
}

func (s *Service) UpdatePage(ctx context.Context, id int64, page *model.DocPage) error {
	if page == nil {
		return errors.New("page is required")
	}

	existing, err := s.GetPageByID(ctx, id)
	if err != nil {
		return err
	}

	mergePage(existing, page)
	if err := validatePage(page); err != nil {
		return err
	}
	if err := s.ensurePageSlugUnique(ctx, page.Slug, &existing.ID); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_pages
		SET slug = ?, title = ?, category_id = ?, mdx_body = ?, sort_order = ?, status = ?, updated_at = ?
		WHERE id = ?;
	`), page.Slug, page.Title, page.CategoryID, page.MDXBody, page.SortOrder, page.Status, now, existing.ID)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("update doc page: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read doc page update rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPageNotFound
	}

	page.ID = existing.ID
	page.CreatedAt = existing.CreatedAt
	page.UpdatedAt = now
	return nil
}

func (s *Service) DeletePage(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_doc_pages WHERE id = ?;`), id)
	if err != nil {
		return fmt.Errorf("delete doc page: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete doc page rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPageNotFound
	}
	return nil
}

func (s *Service) GetPageByID(ctx context.Context, id int64) (*model.DocPage, error) {
	var page model.DocPage
	err := scanPage(
		s.db.QueryRowContext(ctx, s.rebind(`
			SELECT id, slug, title, category_id, mdx_body, sort_order, status, created_at, updated_at
			FROM als_doc_pages
			WHERE id = ?
			LIMIT 1;
		`), id),
		&page,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query doc page by id: %w", err)
	}
	return &page, nil
}

func (s *Service) GetPageBySlug(ctx context.Context, slug string) (*model.DocPage, error) {
	var page model.DocPage
	err := scanPage(
		s.db.QueryRowContext(ctx, s.rebind(`
			SELECT id, slug, title, category_id, mdx_body, sort_order, status, created_at, updated_at
			FROM als_doc_pages
			WHERE slug = ? AND status = ?
			LIMIT 1;
		`), strings.TrimSpace(slug), statusPublished),
		&page,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query doc page by slug: %w", err)
	}
	return &page, nil
}

func (s *Service) GetPageBySlugAdmin(ctx context.Context, slug string) (*model.DocPage, error) {
	var page model.DocPage
	err := scanPage(
		s.db.QueryRowContext(ctx, s.rebind(`
			SELECT id, slug, title, category_id, mdx_body, sort_order, status, created_at, updated_at
			FROM als_doc_pages
			WHERE slug = ?
			LIMIT 1;
		`), strings.TrimSpace(slug)),
		&page,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query doc page by slug admin: %w", err)
	}
	return &page, nil
}

type ListPagesFilter struct {
	Status     string
	CategoryID *int64
}

func (s *Service) ListPages(ctx context.Context, filter ListPagesFilter) ([]model.DocPage, error) {
	query := `SELECT id, slug, title, category_id, mdx_body, sort_order, status, created_at, updated_at FROM als_doc_pages`
	args := make([]any, 0, 2)
	conditions := make([]string, 0, 2)

	status := strings.TrimSpace(strings.ToLower(filter.Status))
	if status != "" {
		if !isValidStatus(status) {
			return nil, errors.New("invalid status filter")
		}
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	if filter.CategoryID != nil {
		conditions = append(conditions, "category_id = ?")
		args = append(args, *filter.CategoryID)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY sort_order ASC, id ASC;"

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query doc pages: %w", err)
	}
	defer rows.Close()

	pages := make([]model.DocPage, 0)
	for rows.Next() {
		var page model.DocPage
		if err := scanPage(rows, &page); err != nil {
			return nil, fmt.Errorf("scan doc page: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate doc pages: %w", err)
	}
	return pages, nil
}

func (s *Service) PublishPage(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_pages SET status = ?, updated_at = ? WHERE id = ?;
	`), statusPublished, now, id)
	if err != nil {
		return fmt.Errorf("publish doc page: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read publish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPageNotFound
	}
	return nil
}

func (s *Service) UnpublishPage(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_doc_pages SET status = ?, updated_at = ? WHERE id = ?;
	`), statusDraft, now, id)
	if err != nil {
		return fmt.Errorf("unpublish doc page: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read unpublish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPageNotFound
	}
	return nil
}

// ── Aggregated query for public sidebar ────────────────────────────

type CategoryWithPages struct {
	Category model.DocCategory
	Pages    []model.DocPage
}

func (s *Service) ListPublishedCategoriesWithPages(ctx context.Context) ([]CategoryWithPages, error) {
	categories, err := s.ListCategories(ctx, ListCategoriesFilter{Status: statusPublished})
	if err != nil {
		return nil, err
	}

	result := make([]CategoryWithPages, 0, len(categories))
	for _, cat := range categories {
		pages, err := s.ListPages(ctx, ListPagesFilter{Status: statusPublished, CategoryID: &cat.ID})
		if err != nil {
			return nil, fmt.Errorf("list pages for category %d: %w", cat.ID, err)
		}
		if len(pages) > 0 {
			result = append(result, CategoryWithPages{Category: cat, Pages: pages})
		}
	}
	return result, nil
}

// ── Helpers ────────────────────────────────────────────────────────

func normalizeCategory(cat *model.DocCategory) {
	cat.Slug = strings.TrimSpace(cat.Slug)
	cat.Title = strings.TrimSpace(cat.Title)
	cat.Status = normalizeStatus(cat.Status)
}

func mergeCategory(existing, cat *model.DocCategory) {
	cat.Slug = strings.TrimSpace(cat.Slug)
	if cat.Slug == "" {
		cat.Slug = existing.Slug
	}
	cat.Title = strings.TrimSpace(cat.Title)
	if cat.Title == "" {
		cat.Title = existing.Title
	}
	if cat.Description == nil {
		cat.Description = existing.Description
	}
	if cat.Icon == nil {
		cat.Icon = existing.Icon
	}
	cat.Status = normalizeStatus(cat.Status)
	if cat.Status == "" {
		cat.Status = existing.Status
	}
}

func validateCategory(cat *model.DocCategory) error {
	if cat.Slug == "" {
		return errors.New("category slug is required")
	}
	if cat.Title == "" {
		return errors.New("category title is required")
	}
	if !isValidStatus(cat.Status) {
		return errors.New("invalid category status")
	}
	return nil
}

func normalizePage(page *model.DocPage) {
	page.Slug = strings.TrimSpace(page.Slug)
	page.Title = strings.TrimSpace(page.Title)
	page.MDXBody = strings.TrimSpace(page.MDXBody)
	page.Status = normalizeStatus(page.Status)
}

func mergePage(existing, page *model.DocPage) {
	page.Slug = strings.TrimSpace(page.Slug)
	if page.Slug == "" {
		page.Slug = existing.Slug
	}
	page.Title = strings.TrimSpace(page.Title)
	if page.Title == "" {
		page.Title = existing.Title
	}
	page.MDXBody = strings.TrimSpace(page.MDXBody)
	if page.MDXBody == "" {
		page.MDXBody = existing.MDXBody
	}
	page.Status = normalizeStatus(page.Status)
	if page.Status == "" {
		page.Status = existing.Status
	}
	if page.CategoryID == 0 {
		page.CategoryID = existing.CategoryID
	}
}

func validatePage(page *model.DocPage) error {
	if page.Slug == "" {
		return errors.New("page slug is required")
	}
	if page.Title == "" {
		return errors.New("page title is required")
	}
	if page.MDXBody == "" {
		return errors.New("page mdx body is required")
	}
	if page.CategoryID == 0 {
		return errors.New("page category_id is required")
	}
	if !isValidStatus(page.Status) {
		return errors.New("invalid page status")
	}
	return nil
}

func normalizeStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return statusDraft
	}
	return status
}

func isValidStatus(status string) bool {
	return status == statusDraft || status == statusPublished
}

func isUniqueConstraintError(err error) bool {
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unique")
}

func (s *Service) ensureCategorySlugUnique(ctx context.Context, slug string, exceptID *int64) error {
	query := `SELECT id FROM als_doc_categories WHERE slug = ?`
	args := []any{slug}
	if exceptID != nil {
		query += ` AND id != ?`
		args = append(args, *exceptID)
	}
	query += ` LIMIT 1;`

	var existingID int64
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check category slug uniqueness: %w", err)
	}
	return ErrDuplicateSlug
}

func (s *Service) ensurePageSlugUnique(ctx context.Context, slug string, exceptID *int64) error {
	query := `SELECT id FROM als_doc_pages WHERE slug = ?`
	args := []any{slug}
	if exceptID != nil {
		query += ` AND id != ?`
		args = append(args, *exceptID)
	}
	query += ` LIMIT 1;`

	var existingID int64
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check page slug uniqueness: %w", err)
	}
	return ErrDuplicateSlug
}

func scanCategory(scanner rowScanner, cat *model.DocCategory) error {
	var (
		description sql.NullString
		icon        sql.NullString
	)
	err := scanner.Scan(
		&cat.ID,
		&cat.Slug,
		&cat.Title,
		&description,
		&icon,
		&cat.SortOrder,
		&cat.Status,
		&cat.CreatedAt,
		&cat.UpdatedAt,
	)
	if err != nil {
		return err
	}
	cat.Description = nullStringPtr(description)
	cat.Icon = nullStringPtr(icon)
	return nil
}

func scanPage(scanner rowScanner, page *model.DocPage) error {
	err := scanner.Scan(
		&page.ID,
		&page.Slug,
		&page.Title,
		&page.CategoryID,
		&page.MDXBody,
		&page.SortOrder,
		&page.Status,
		&page.CreatedAt,
		&page.UpdatedAt,
	)
	return err
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

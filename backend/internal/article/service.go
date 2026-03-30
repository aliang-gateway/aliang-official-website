package article

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-api-portal/backend/internal/model"
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
)

type Service struct {
	db *sql.DB
}

type ListArticlesFilters struct {
	Status string
}

type rowScanner interface {
	Scan(dest ...any) error
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) CreateArticle(ctx context.Context, article *model.Article) error {
	if article == nil {
		return errors.New("article is required")
	}

	normalizeArticleForCreate(article)
	if err := validateArticle(article); err != nil {
		return err
	}
	if err := s.ensureSlugUnique(ctx, article.Slug, nil); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO als_articles(
			legacy_id,
			slug,
			title,
			excerpt,
			cover_image_url,
			tag,
			read_time,
			author_name,
			author_avatar_url,
			author_icon,
			mdx_body,
			status,
			published_at,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`,
		article.LegacyID,
		article.Slug,
		article.Title,
		article.Excerpt,
		article.CoverImageURL,
		article.Tag,
		article.ReadTime,
		article.AuthorName,
		article.AuthorAvatarURL,
		article.AuthorIcon,
		article.MDXBody,
		article.Status,
		article.PublishedAt,
		article.CreatedByUserID,
		article.UpdatedByUserID,
		now,
		now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("insert article: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read article insert id: %w", err)
	}

	article.ID = id
	article.CreatedAt = now
	article.UpdatedAt = now
	return nil
}

func (s *Service) UpdateArticle(ctx context.Context, slug string, article *model.Article) error {
	if article == nil {
		return errors.New("article is required")
	}

	existing, err := s.getArticleBySlugAnyStatus(ctx, strings.TrimSpace(slug))
	if err != nil {
		return err
	}

	normalizeArticleForUpdate(existing, article)
	if err := validateArticle(article); err != nil {
		return err
	}
	if err := s.ensureSlugUnique(ctx, article.Slug, &existing.ID); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE als_articles
		SET
			legacy_id = ?,
			slug = ?,
			title = ?,
			excerpt = ?,
			cover_image_url = ?,
			tag = ?,
			read_time = ?,
			author_name = ?,
			author_avatar_url = ?,
			author_icon = ?,
			mdx_body = ?,
			status = ?,
			published_at = ?,
			updated_by_user_id = ?,
			updated_at = ?
		WHERE id = ?;
	`,
		article.LegacyID,
		article.Slug,
		article.Title,
		article.Excerpt,
		article.CoverImageURL,
		article.Tag,
		article.ReadTime,
		article.AuthorName,
		article.AuthorAvatarURL,
		article.AuthorIcon,
		article.MDXBody,
		article.Status,
		article.PublishedAt,
		article.UpdatedByUserID,
		now,
		existing.ID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("update article: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read article update rows affected: %w", err)
	}
	if affected == 0 {
		return ErrArticleNotFound
	}

	article.ID = existing.ID
	article.CreatedAt = existing.CreatedAt
	article.UpdatedAt = now
	return nil
}

func (s *Service) GetArticleBySlug(ctx context.Context, slug string) (*model.Article, error) {
	var article model.Article
	err := scanArticle(
		s.db.QueryRowContext(ctx, `
			SELECT
				id,
				legacy_id,
				slug,
				title,
				excerpt,
				cover_image_url,
				tag,
				read_time,
				author_name,
				author_avatar_url,
				author_icon,
				mdx_body,
				status,
				published_at,
				created_by_user_id,
				updated_by_user_id,
				created_at,
				updated_at
			FROM als_articles
			WHERE slug = ? AND status = ?
			LIMIT 1;
		`, strings.TrimSpace(slug), statusPublished),
		&article,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrArticleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query article by slug: %w", err)
	}
	return &article, nil
}

func (s *Service) GetArticleByID(ctx context.Context, id int64) (*model.Article, error) {
	var article model.Article
	err := scanArticle(
		s.db.QueryRowContext(ctx, `
			SELECT
				id,
				legacy_id,
				slug,
				title,
				excerpt,
				cover_image_url,
				tag,
				read_time,
				author_name,
				author_avatar_url,
				author_icon,
				mdx_body,
				status,
				published_at,
				created_by_user_id,
				updated_by_user_id,
				created_at,
				updated_at
			FROM als_articles
			WHERE id = ?
			LIMIT 1;
		`, id),
		&article,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrArticleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query article by id: %w", err)
	}
	return &article, nil
}

func (s *Service) ListArticles(ctx context.Context, filters ListArticlesFilters) ([]model.Article, error) {
	query := `
		SELECT
			id,
			legacy_id,
			slug,
			title,
			excerpt,
			cover_image_url,
			tag,
			read_time,
			author_name,
			author_avatar_url,
			author_icon,
			mdx_body,
			status,
			published_at,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		FROM als_articles
	`
	args := make([]any, 0, 1)

	status := strings.TrimSpace(strings.ToLower(filters.Status))
	if status != "" {
		if !isValidStatus(status) {
			return nil, errors.New("invalid article status")
		}
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC, id DESC;"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query als_articles: %w", err)
	}
	defer rows.Close()

	als_articles := make([]model.Article, 0)
	for rows.Next() {
		var article model.Article
		if err := scanArticle(rows, &article); err != nil {
			return nil, fmt.Errorf("scan article: %w", err)
		}
		als_articles = append(als_articles, article)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate als_articles: %w", err)
	}

	return als_articles, nil
}

func (s *Service) ListPublishedArticles(ctx context.Context) ([]model.Article, error) {
	return s.ListArticles(ctx, ListArticlesFilters{Status: statusPublished})
}

func (s *Service) PublishArticle(ctx context.Context, slug string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE als_articles
		SET status = ?, published_at = ?, updated_at = ?
		WHERE slug = ?;
	`, statusPublished, now, now, strings.TrimSpace(slug))
	if err != nil {
		return fmt.Errorf("publish article: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read publish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrArticleNotFound
	}

	return nil
}

func (s *Service) UnpublishArticle(ctx context.Context, slug string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE als_articles
		SET status = ?, published_at = NULL, updated_at = ?
		WHERE slug = ?;
	`, statusDraft, now, strings.TrimSpace(slug))
	if err != nil {
		return fmt.Errorf("unpublish article: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read unpublish rows affected: %w", err)
	}
	if affected == 0 {
		return ErrArticleNotFound
	}

	return nil
}

func (s *Service) DeleteArticle(ctx context.Context, slug string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM als_articles WHERE slug = ?;`, strings.TrimSpace(slug))
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete article rows affected: %w", err)
	}
	if affected == 0 {
		return ErrArticleNotFound
	}

	return nil
}

func (s *Service) getArticleBySlugAnyStatus(ctx context.Context, slug string) (*model.Article, error) {
	var article model.Article
	err := scanArticle(
		s.db.QueryRowContext(ctx, `
			SELECT
				id,
				legacy_id,
				slug,
				title,
				excerpt,
				cover_image_url,
				tag,
				read_time,
				author_name,
				author_avatar_url,
				author_icon,
				mdx_body,
				status,
				published_at,
				created_by_user_id,
				updated_by_user_id,
				created_at,
				updated_at
			FROM als_articles
			WHERE slug = ?
			LIMIT 1;
		`, slug),
		&article,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrArticleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query article by slug: %w", err)
	}
	return &article, nil
}

func (s *Service) ensureSlugUnique(ctx context.Context, slug string, exceptID *int64) error {
	query := `SELECT id FROM als_articles WHERE slug = ?`
	args := []any{slug}
	if exceptID != nil {
		query += ` AND id != ?`
		args = append(args, *exceptID)
	}
	query += ` LIMIT 1;`

	var existingID int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check article slug uniqueness: %w", err)
	}
	return ErrDuplicateSlug
}

func normalizeArticleForCreate(article *model.Article) {
	article.Slug = strings.TrimSpace(article.Slug)
	article.Title = strings.TrimSpace(article.Title)
	article.MDXBody = strings.TrimSpace(article.MDXBody)
	article.Status = normalizeStatus(article.Status)

	if article.Status == statusPublished && article.PublishedAt == nil {
		now := time.Now().UTC()
		article.PublishedAt = &now
	}
	if article.Status == statusDraft {
		article.PublishedAt = nil
	}
}

func normalizeArticleForUpdate(existing, article *model.Article) {
	article.Slug = strings.TrimSpace(article.Slug)
	if article.Slug == "" {
		article.Slug = existing.Slug
	}

	article.Title = strings.TrimSpace(article.Title)
	if article.Title == "" {
		article.Title = existing.Title
	}

	article.MDXBody = strings.TrimSpace(article.MDXBody)
	if article.MDXBody == "" {
		article.MDXBody = existing.MDXBody
	}

	article.Status = normalizeStatus(article.Status)
	if article.Status == "" {
		article.Status = existing.Status
	}

	if article.LegacyID == nil {
		article.LegacyID = existing.LegacyID
	}
	if article.Excerpt == nil {
		article.Excerpt = existing.Excerpt
	}
	if article.CoverImageURL == nil {
		article.CoverImageURL = existing.CoverImageURL
	}
	if article.Tag == nil {
		article.Tag = existing.Tag
	}
	if article.ReadTime == nil {
		article.ReadTime = existing.ReadTime
	}
	if article.AuthorName == nil {
		article.AuthorName = existing.AuthorName
	}
	if article.AuthorAvatarURL == nil {
		article.AuthorAvatarURL = existing.AuthorAvatarURL
	}
	if article.AuthorIcon == nil {
		article.AuthorIcon = existing.AuthorIcon
	}
	if article.UpdatedByUserID == nil {
		article.UpdatedByUserID = existing.UpdatedByUserID
	}

	if article.Status == statusPublished && article.PublishedAt == nil {
		if existing.PublishedAt != nil {
			article.PublishedAt = existing.PublishedAt
		} else {
			now := time.Now().UTC()
			article.PublishedAt = &now
		}
	}
	if article.Status == statusDraft {
		article.PublishedAt = nil
	}
}

func validateArticle(article *model.Article) error {
	if article.Slug == "" {
		return errors.New("article slug is required")
	}
	if article.Title == "" {
		return errors.New("article title is required")
	}
	if article.MDXBody == "" {
		return errors.New("article mdx body is required")
	}
	if !isValidStatus(article.Status) {
		return errors.New("invalid article status")
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

func scanArticle(scanner rowScanner, article *model.Article) error {
	var (
		legacyID        sql.NullInt64
		excerpt         sql.NullString
		coverImageURL   sql.NullString
		tag             sql.NullString
		readTime        sql.NullString
		authorName      sql.NullString
		authorAvatarURL sql.NullString
		authorIcon      sql.NullString
		publishedAt     sql.NullTime
		createdByUserID sql.NullInt64
		updatedByUserID sql.NullInt64
	)

	err := scanner.Scan(
		&article.ID,
		&legacyID,
		&article.Slug,
		&article.Title,
		&excerpt,
		&coverImageURL,
		&tag,
		&readTime,
		&authorName,
		&authorAvatarURL,
		&authorIcon,
		&article.MDXBody,
		&article.Status,
		&publishedAt,
		&createdByUserID,
		&updatedByUserID,
		&article.CreatedAt,
		&article.UpdatedAt,
	)
	if err != nil {
		return err
	}

	article.LegacyID = nullInt64Ptr(legacyID)
	article.Excerpt = nullStringPtr(excerpt)
	article.CoverImageURL = nullStringPtr(coverImageURL)
	article.Tag = nullStringPtr(tag)
	article.ReadTime = nullStringPtr(readTime)
	article.AuthorName = nullStringPtr(authorName)
	article.AuthorAvatarURL = nullStringPtr(authorAvatarURL)
	article.AuthorIcon = nullStringPtr(authorIcon)
	article.PublishedAt = nullTimePtr(publishedAt)
	article.CreatedByUserID = nullInt64Ptr(createdByUserID)
	article.UpdatedByUserID = nullInt64Ptr(updatedByUserID)

	return nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}

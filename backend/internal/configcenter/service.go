package configcenter

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

type Service struct {
	db         *sql.DB
	sqlDialect string
}

func NewService(database *sql.DB, sqlDialect string) *Service {
	return &Service{db: database, sqlDialect: sqlDialect}
}

// --------------- Errors ---------------

var (
	ErrNotFound          = errors.New("not found")
	ErrDuplicateCode     = errors.New("duplicate software code")
	ErrDuplicateTag      = errors.New("duplicate tag")
	ErrDuplicateVarKey   = errors.New("duplicate var key")
	ErrDuplicateUUID     = errors.New("duplicate uuid")
	ErrNoDefaultTemplate = errors.New("no default template found")
)

// rebind is a shorthand for db.Rebind(s.sqlDialect, q).
func (s *Service) rebind(q string) string {
	return db.Rebind(s.sqlDialect, q)
}

// --------------- Software Config CRUD ---------------

func (s *Service) CreateSoftwareConfig(ctx context.Context, cfg *model.SoftwareConfig) error {
	cfg.SoftwareCode = strings.TrimSpace(cfg.SoftwareCode)
	cfg.SoftwareName = strings.TrimSpace(cfg.SoftwareName)
	if cfg.SoftwareCode == "" || cfg.SoftwareName == "" {
		return errors.New("software_code and software_name are required")
	}

	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_software_configs (software_code, software_name, group_id, description, is_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"id",
		cfg.SoftwareCode, cfg.SoftwareName, cfg.GroupID, cfg.Description, cfg.IsEnabled, now, now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateCode
		}
		return fmt.Errorf("insert software config: %w", err)
	}

	cfg.ID = id
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	return nil
}

func (s *Service) GetSoftwareConfigByCode(ctx context.Context, code string) (*model.SoftwareConfig, error) {
	var cfg model.SoftwareConfig
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, software_code, software_name, group_id, description, is_enabled, created_at, updated_at
		FROM als_software_configs WHERE software_code = ?`), strings.TrimSpace(code),
	).Scan(&cfg.ID, &cfg.SoftwareCode, &cfg.SoftwareName, &cfg.GroupID, &cfg.Description, &cfg.IsEnabled, &cfg.CreatedAt, &cfg.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query software config: %w", err)
	}
	return &cfg, nil
}

func (s *Service) ListSoftwareConfigs(ctx context.Context) ([]model.SoftwareConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, software_code, software_name, group_id, description, is_enabled, created_at, updated_at
		FROM als_software_configs ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query software configs: %w", err)
	}
	defer rows.Close()

	var configs []model.SoftwareConfig
	for rows.Next() {
		var cfg model.SoftwareConfig
		if err := rows.Scan(&cfg.ID, &cfg.SoftwareCode, &cfg.SoftwareName, &cfg.GroupID, &cfg.Description, &cfg.IsEnabled, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan software config: %w", err)
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (s *Service) UpdateSoftwareConfig(ctx context.Context, code string, cfg *model.SoftwareConfig) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_software_configs
		SET software_name = ?, group_id = ?, description = ?, is_enabled = ?, updated_at = ?
		WHERE software_code = ?`),
		cfg.SoftwareName, cfg.GroupID, cfg.Description, cfg.IsEnabled, now, strings.TrimSpace(code),
	)
	if err != nil {
		return fmt.Errorf("update software config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	cfg.UpdatedAt = now
	return nil
}

func (s *Service) DeleteSoftwareConfig(ctx context.Context, code string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_software_configs WHERE software_code = ?`), strings.TrimSpace(code))
	if err != nil {
		return fmt.Errorf("delete software config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// --------------- Tag Management ---------------

func (s *Service) AddTag(ctx context.Context, softwareConfigID int64, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return errors.New("tag is required")
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO als_software_tags (software_config_id, tag, created_at)
		VALUES (?, ?, ?)`), softwareConfigID, tag, time.Now().UTC(),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrDuplicateTag
		}
		return fmt.Errorf("insert tag: %w", err)
	}
	return nil
}

func (s *Service) RemoveTag(ctx context.Context, softwareConfigID int64, tag string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_software_tags WHERE software_config_id = ? AND tag = ?`),
		softwareConfigID, strings.TrimSpace(tag))
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListTags(ctx context.Context, softwareConfigID int64) ([]model.SoftwareTag, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT id, software_config_id, tag, created_at
		FROM als_software_tags WHERE software_config_id = ? ORDER BY tag`), softwareConfigID)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var tags []model.SoftwareTag
	for rows.Next() {
		var t model.SoftwareTag
		if err := rows.Scan(&t.ID, &t.SoftwareConfigID, &t.Tag, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// --------------- Config Template CRUD ---------------

func (s *Service) CreateTemplate(ctx context.Context, tpl *model.ConfigTemplate) error {
	tpl.Name = strings.TrimSpace(tpl.Name)
	tpl.Content = strings.TrimSpace(tpl.Content)
	if tpl.Name == "" || tpl.Content == "" {
		return errors.New("template name and content are required")
	}
	if tpl.Format == "" {
		tpl.Format = "json"
	}

	now := time.Now().UTC()
	id, err := db.InsertID(ctx, s.sqlDialect, s.db, `
		INSERT INTO als_config_templates (software_config_id, name, format, content, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"id",
		tpl.SoftwareConfigID, tpl.Name, tpl.Format, tpl.Content, tpl.IsDefault, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}
	tpl.ID = id
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	return nil
}

func (s *Service) UpdateTemplate(ctx context.Context, templateID int64, tpl *model.ConfigTemplate) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE als_config_templates
		SET name = ?, format = ?, content = ?, is_default = ?, updated_at = ?
		WHERE id = ?`),
		tpl.Name, tpl.Format, tpl.Content, tpl.IsDefault, now, templateID,
	)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	tpl.UpdatedAt = now
	return nil
}

func (s *Service) DeleteTemplate(ctx context.Context, templateID int64) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_config_templates WHERE id = ?`), templateID)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListTemplates(ctx context.Context, softwareConfigID int64) ([]model.ConfigTemplate, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT id, software_config_id, name, format, content, is_default, created_at, updated_at
		FROM als_config_templates WHERE software_config_id = ? ORDER BY is_default DESC, name`),
		softwareConfigID)
	if err != nil {
		return nil, fmt.Errorf("query templates: %w", err)
	}
	defer rows.Close()

	var templates []model.ConfigTemplate
	for rows.Next() {
		var tpl model.ConfigTemplate
		if err := rows.Scan(&tpl.ID, &tpl.SoftwareConfigID, &tpl.Name, &tpl.Format, &tpl.Content, &tpl.IsDefault, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, tpl)
	}
	return templates, rows.Err()
}

func (s *Service) GetDefaultTemplate(ctx context.Context, softwareConfigID int64) (*model.ConfigTemplate, error) {
	var tpl model.ConfigTemplate
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, software_config_id, name, format, content, is_default, created_at, updated_at
		FROM als_config_templates
		WHERE software_config_id = ?
		ORDER BY is_default DESC, id ASC
		LIMIT 1`), softwareConfigID,
	).Scan(&tpl.ID, &tpl.SoftwareConfigID, &tpl.Name, &tpl.Format, &tpl.Content, &tpl.IsDefault, &tpl.CreatedAt, &tpl.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDefaultTemplate
	}
	if err != nil {
		return nil, fmt.Errorf("query default template: %w", err)
	}
	return &tpl, nil
}

// --------------- Global Template Variables ---------------

func (s *Service) SetGlobalVar(ctx context.Context, varKey, varValue, description string) error {
	varKey = strings.TrimSpace(varKey)
	if varKey == "" {
		return errors.New("var_key is required")
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO als_global_template_vars (var_key, var_value, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(var_key) DO UPDATE SET var_value = ?, description = ?, updated_at = ?`),
		varKey, varValue, description, now, now,
		varValue, description, now,
	)
	if err != nil {
		return fmt.Errorf("upsert global var: %w", err)
	}
	_ = result
	return nil
}

func (s *Service) GetGlobalVar(ctx context.Context, varKey string) (*model.GlobalTemplateVar, error) {
	var v model.GlobalTemplateVar
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, var_key, var_value, description, created_at, updated_at
		FROM als_global_template_vars WHERE var_key = ?`), strings.TrimSpace(varKey),
	).Scan(&v.ID, &v.VarKey, &v.VarValue, &v.Description, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query global var: %w", err)
	}
	return &v, nil
}

func (s *Service) ListGlobalVars(ctx context.Context) ([]model.GlobalTemplateVar, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, var_key, var_value, description, created_at, updated_at
		FROM als_global_template_vars ORDER BY var_key`)
	if err != nil {
		return nil, fmt.Errorf("query global vars: %w", err)
	}
	defer rows.Close()

	var vars []model.GlobalTemplateVar
	for rows.Next() {
		var v model.GlobalTemplateVar
		if err := rows.Scan(&v.ID, &v.VarKey, &v.VarValue, &v.Description, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan global var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (s *Service) DeleteGlobalVar(ctx context.Context, varKey string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_global_template_vars WHERE var_key = ?`), strings.TrimSpace(varKey))
	if err != nil {
		return fmt.Errorf("delete global var: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// --------------- Default Config Resolution ---------------

type DefaultConfigResult struct {
	SoftwareCode string `json:"software_code"`
	SoftwareName string `json:"software_name"`
	TemplateName string `json:"template_name"`
	Format       string `json:"format"`
	Content      string `json:"content"`
	GroupID      int64  `json:"-"`
}

// GetDefaultConfigByTag resolves a tag to the default config template content.
// Known global template variables are rendered server-side, while unresolved
// placeholders are preserved for the client to fill in.
func (s *Service) GetDefaultConfigByTag(ctx context.Context, tag string) (*DefaultConfigResult, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("tag is required")
	}

	// 1. Find software config by tag
	var cfg model.SoftwareConfig
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT sc.id, sc.software_code, sc.software_name, sc.group_id, sc.description, sc.is_enabled, sc.created_at, sc.updated_at
		FROM als_software_configs sc
		JOIN als_software_tags st ON st.software_config_id = sc.id
		WHERE st.tag = ? AND sc.is_enabled = TRUE
		LIMIT 1`), tag,
	).Scan(&cfg.ID, &cfg.SoftwareCode, &cfg.SoftwareName, &cfg.GroupID, &cfg.Description, &cfg.IsEnabled, &cfg.CreatedAt, &cfg.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query config by tag: %w", err)
	}

	// 2. Get default template
	tpl, err := s.GetDefaultTemplate(ctx, cfg.ID)
	if err != nil {
		return nil, fmt.Errorf("get default template: %w", err)
	}

	globalVars, err := s.ListGlobalVars(ctx)
	if err != nil {
		return nil, fmt.Errorf("list global vars: %w", err)
	}

	content := tpl.Content
	for _, v := range globalVars {
		content = strings.ReplaceAll(content, "{{"+v.VarKey+"}}", v.VarValue)
	}

	return &DefaultConfigResult{
		SoftwareCode: cfg.SoftwareCode,
		SoftwareName: cfg.SoftwareName,
		TemplateName: tpl.Name,
		Format:       tpl.Format,
		Content:      content,
		GroupID:      cfg.GroupID,
	}, nil
}

// --------------- User Synced Configs ---------------

func (s *Service) SyncConfigs(ctx context.Context, userID int64, configs []model.UserSyncedConfig) error {
	now := time.Now().UTC()
	for i := range configs {
		configs[i].UserID = userID
		c := configs[i]
		_, err := s.db.ExecContext(ctx, s.rebind(`
			INSERT INTO als_user_synced_configs (user_id, uuid, software, name, file_path, version, in_use, selected, format, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, uuid) DO UPDATE SET
				software = ?, name = ?, file_path = ?, version = ?,
				in_use = ?, selected = ?, format = ?, content = ?, updated_at = ?`),
			userID, c.UUID, c.Software, c.Name, c.FilePath, c.Version,
			c.InUse, c.Selected, c.Format, c.Content, now, now,
			c.Software, c.Name, c.FilePath, c.Version,
			c.InUse, c.Selected, c.Format, c.Content, now,
		)
		if err != nil {
			return fmt.Errorf("sync config uuid=%s: %w", c.UUID, err)
		}
	}
	return nil
}

type PullConfigsOptions struct {
	Software     string
	UpdatedAfter *time.Time
	Page         int
	PageSize     int
}

func (s *Service) PullConfigs(ctx context.Context, userID int64, opts PullConfigsOptions) ([]model.UserSyncedConfig, error) {
	query := `
		SELECT id, user_id, uuid, software, name, file_path, version, in_use, selected, format, content, created_at, updated_at
		FROM als_user_synced_configs WHERE user_id = ?`
	args := []any{userID}

	if opts.Software != "" {
		query += ` AND software = ?`
		args = append(args, opts.Software)
	}
	if opts.UpdatedAfter != nil {
		query += ` AND updated_at > ?`
		args = append(args, *opts.UpdatedAfter)
	}

	query += ` ORDER BY updated_at DESC`

	if opts.PageSize > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.PageSize)
		if opts.Page > 0 {
			query += ` OFFSET ?`
			args = append(args, opts.Page*opts.PageSize)
		}
	}

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("pull configs: %w", err)
	}
	defer rows.Close()

	var configs []model.UserSyncedConfig
	for rows.Next() {
		var c model.UserSyncedConfig
		if err := rows.Scan(&c.ID, &c.UserID, &c.UUID, &c.Software, &c.Name, &c.FilePath, &c.Version, &c.InUse, &c.Selected, &c.Format, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan synced config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

type CompareItem struct {
	UUID    string `json:"uuid"`
	Version string `json:"version"`
}

func (s *Service) CompareConfigs(ctx context.Context, userID int64, items []CompareItem) (map[string]string, error) {
	result := make(map[string]string)
	for _, item := range items {
		var version string
		var updatedAt time.Time
		err := s.db.QueryRowContext(ctx, s.rebind(`
			SELECT version, updated_at FROM als_user_synced_configs
			WHERE user_id = ? AND uuid = ?`), userID, item.UUID,
		).Scan(&version, &updatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			result[item.UUID] = "not_found"
		} else if err != nil {
			return nil, fmt.Errorf("compare config uuid=%s: %w", item.UUID, err)
		} else if version != item.Version {
			result[item.UUID] = "conflict"
		} else {
			result[item.UUID] = "match"
		}
	}
	return result, nil
}

func (s *Service) DeleteSyncedConfig(ctx context.Context, userID int64, uuid string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM als_user_synced_configs WHERE user_id = ? AND uuid = ?`), userID, uuid)
	if err != nil {
		return fmt.Errorf("delete synced config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListSoftware(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT DISTINCT software FROM als_user_synced_configs WHERE user_id = ? ORDER BY software`), userID)
	if err != nil {
		return nil, fmt.Errorf("list software: %w", err)
	}
	defer rows.Close()

	var software []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan software: %w", err)
		}
		software = append(software, s)
	}
	return software, rows.Err()
}

type SyncStatus struct {
	TotalConfigs int        `json:"total_configs"`
	SoftwareList []string   `json:"software_list"`
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
}

func (s *Service) GetSyncStatus(ctx context.Context, userID int64) (*SyncStatus, error) {
	var total int
	var lastSync sql.NullTime
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT COUNT(*), MAX(updated_at) FROM als_user_synced_configs WHERE user_id = ?`), userID,
	).Scan(&total, &lastSync)
	if err != nil {
		return nil, fmt.Errorf("get sync status: %w", err)
	}

	softwareList, err := s.ListSoftware(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list software for status: %w", err)
	}

	status := &SyncStatus{
		TotalConfigs: total,
		SoftwareList: softwareList,
	}
	if lastSync.Valid {
		status.LastSyncAt = &lastSync.Time
	}
	return status, nil
}

// --------------- Helpers ---------------

func isUniqueConstraintError(err error) bool {
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unique") || strings.Contains(errText, "constraint")
}

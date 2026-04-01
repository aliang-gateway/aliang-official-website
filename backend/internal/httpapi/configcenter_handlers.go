package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/configcenter"
	"ai-api-portal/backend/internal/model"
)

// -------------------------------------------------------
// Shared response types
// -------------------------------------------------------

type softwareConfigResponse struct {
	ID           int64    `json:"id"`
	SoftwareCode string   `json:"software_code"`
	SoftwareName string   `json:"software_name"`
	GroupID      int64    `json:"group_id"`
	Description  string   `json:"description"`
	IsEnabled    bool     `json:"is_enabled"`
	Tags         []string `json:"tags,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (r *routes) loadTagsForConfig(ctx context.Context, softwareConfigID int64) []string {
	tagRows, err := r.configCenterSvc.ListTags(ctx, softwareConfigID)
	if err != nil {
		return nil
	}
	tags := make([]string, 0, len(tagRows))
	for _, t := range tagRows {
		tags = append(tags, t.Tag)
	}
	return tags
}

// -------------------------------------------------------
// Admin: Software Config CRUD
// -------------------------------------------------------

func (r *routes) handleAdminListSoftwareConfigs(w http.ResponseWriter, req *http.Request) {
	configs, err := r.configCenterSvc.ListSoftwareConfigs(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list software configs")
		return
	}

	var result []softwareConfigResponse
	for _, cfg := range configs {
		tags := r.loadTagsForConfig(req.Context(), cfg.ID)
		result = append(result, softwareConfigResponse{
			ID:           cfg.ID,
			SoftwareCode: cfg.SoftwareCode,
			SoftwareName: cfg.SoftwareName,
			GroupID:      cfg.GroupID,
			Description:  cfg.Description,
			IsEnabled:    cfg.IsEnabled,
			Tags:         tags,
			CreatedAt:    cfg.CreatedAt,
			UpdatedAt:    cfg.UpdatedAt,
		})
	}
	if result == nil {
		result = []softwareConfigResponse{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"configs": result})
}

func (r *routes) handleAdminGetSoftwareConfig(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	tags := r.loadTagsForConfig(req.Context(), cfg.ID)

	templates, err := r.configCenterSvc.ListTemplates(req.Context(), cfg.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	type templateResponse struct {
		ID               int64     `json:"id"`
		SoftwareConfigID int64     `json:"software_config_id"`
		Name             string    `json:"name"`
		Format           string    `json:"format"`
		Content          string    `json:"content"`
		IsDefault        bool      `json:"is_default"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
	}

	templateResponses := make([]templateResponse, 0, len(templates))
	for _, tpl := range templates {
		templateResponses = append(templateResponses, templateResponse{
			ID:               tpl.ID,
			SoftwareConfigID: tpl.SoftwareConfigID,
			Name:             tpl.Name,
			Format:           tpl.Format,
			Content:          tpl.Content,
			IsDefault:        tpl.IsDefault,
			CreatedAt:        tpl.CreatedAt,
			UpdatedAt:        tpl.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":            cfg.ID,
		"software_code": cfg.SoftwareCode,
		"software_name": cfg.SoftwareName,
		"group_id":      cfg.GroupID,
		"description":   cfg.Description,
		"is_enabled":    cfg.IsEnabled,
		"tags":          tags,
		"templates":     templateResponses,
		"created_at":    cfg.CreatedAt,
		"updated_at":    cfg.UpdatedAt,
	})
}

type createSoftwareConfigRequest struct {
	SoftwareCode string `json:"software_code"`
	SoftwareName string `json:"software_name"`
	GroupID      int64  `json:"group_id"`
	Description  string `json:"description"`
	IsEnabled    *bool  `json:"is_enabled,omitempty"`
}

func (r *routes) handleAdminCreateSoftwareConfig(w http.ResponseWriter, req *http.Request) {
	var payload createSoftwareConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.SoftwareCode = strings.TrimSpace(payload.SoftwareCode)
	payload.SoftwareName = strings.TrimSpace(payload.SoftwareName)
	if payload.SoftwareCode == "" || payload.SoftwareName == "" {
		writeError(w, http.StatusBadRequest, "software_code and software_name are required")
		return
	}

	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}

	cfg := &model.SoftwareConfig{
		SoftwareCode: payload.SoftwareCode,
		SoftwareName: payload.SoftwareName,
		GroupID:      payload.GroupID,
		Description:  payload.Description,
		IsEnabled:    isEnabled,
	}
	if err := r.configCenterSvc.CreateSoftwareConfig(req.Context(), cfg); err != nil {
		if errors.Is(err, configcenter.ErrDuplicateCode) {
			writeError(w, http.StatusConflict, "software code already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create software config")
		return
	}

	writeJSON(w, http.StatusCreated, softwareConfigResponse{
		ID:           cfg.ID,
		SoftwareCode: cfg.SoftwareCode,
		SoftwareName: cfg.SoftwareName,
		GroupID:      cfg.GroupID,
		Description:  cfg.Description,
		IsEnabled:    cfg.IsEnabled,
		CreatedAt:    cfg.CreatedAt,
		UpdatedAt:    cfg.UpdatedAt,
	})
}

type updateSoftwareConfigRequest struct {
	SoftwareName string `json:"software_name"`
	GroupID      int64  `json:"group_id"`
	Description  string `json:"description"`
	IsEnabled    *bool  `json:"is_enabled,omitempty"`
}

func (r *routes) handleAdminUpdateSoftwareConfig(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	var payload updateSoftwareConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.SoftwareName = strings.TrimSpace(payload.SoftwareName)
	if payload.SoftwareName == "" {
		payload.SoftwareName = cfg.SoftwareName
	}
	isEnabled := cfg.IsEnabled
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}

	cfg.SoftwareName = payload.SoftwareName
	cfg.GroupID = payload.GroupID
	cfg.Description = payload.Description
	cfg.IsEnabled = isEnabled

	if err := r.configCenterSvc.UpdateSoftwareConfig(req.Context(), code, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update software config")
		return
	}

	tags := r.loadTagsForConfig(req.Context(), cfg.ID)
	writeJSON(w, http.StatusOK, softwareConfigResponse{
		ID:           cfg.ID,
		SoftwareCode: cfg.SoftwareCode,
		SoftwareName: cfg.SoftwareName,
		GroupID:      cfg.GroupID,
		Description:  cfg.Description,
		IsEnabled:    cfg.IsEnabled,
		Tags:         tags,
		CreatedAt:    cfg.CreatedAt,
		UpdatedAt:    cfg.UpdatedAt,
	})
}

func (r *routes) handleAdminDeleteSoftwareConfig(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	if err := r.configCenterSvc.DeleteSoftwareConfig(req.Context(), code); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete software config")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// -------------------------------------------------------
// Admin: Tag Management
// -------------------------------------------------------

type addTagRequest struct {
	Tag string `json:"tag"`
}

func (r *routes) handleAdminAddTag(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	var payload addTagRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	payload.Tag = strings.TrimSpace(payload.Tag)
	if payload.Tag == "" {
		writeError(w, http.StatusBadRequest, "tag is required")
		return
	}

	if err := r.configCenterSvc.AddTag(req.Context(), cfg.ID, payload.Tag); err != nil {
		if errors.Is(err, configcenter.ErrDuplicateTag) {
			writeError(w, http.StatusConflict, "tag already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add tag")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"tag": payload.Tag})
}

func (r *routes) handleAdminRemoveTag(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	tag := strings.TrimSpace(req.PathValue("tag"))
	if code == "" || tag == "" {
		writeError(w, http.StatusBadRequest, "code and tag are required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	if err := r.configCenterSvc.RemoveTag(req.Context(), cfg.ID, tag); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "tag not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to remove tag")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"removed": true})
}

// -------------------------------------------------------
// Admin: Template Management
// -------------------------------------------------------

type createTemplateRequest struct {
	Name      string `json:"name"`
	Format    string `json:"format"`
	Content   string `json:"content"`
	IsDefault *bool  `json:"is_default,omitempty"`
}

func (r *routes) handleAdminListTemplates(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	templates, err := r.configCenterSvc.ListTemplates(req.Context(), cfg.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	type templateResponse struct {
		ID               int64     `json:"id"`
		SoftwareConfigID int64     `json:"software_config_id"`
		Name             string    `json:"name"`
		Format           string    `json:"format"`
		Content          string    `json:"content"`
		IsDefault        bool      `json:"is_default"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
	}

	result := make([]templateResponse, 0, len(templates))
	for _, tpl := range templates {
		result = append(result, templateResponse{
			ID:               tpl.ID,
			SoftwareConfigID: tpl.SoftwareConfigID,
			Name:             tpl.Name,
			Format:           tpl.Format,
			Content:          tpl.Content,
			IsDefault:        tpl.IsDefault,
			CreatedAt:        tpl.CreatedAt,
			UpdatedAt:        tpl.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"templates": result})
}

func (r *routes) handleAdminCreateTemplate(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.PathValue("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	cfg, err := r.configCenterSvc.GetSoftwareConfigByCode(req.Context(), code)
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "software config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get software config")
		return
	}

	var payload createTemplateRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.Name == "" || payload.Content == "" {
		writeError(w, http.StatusBadRequest, "name and content are required")
		return
	}
	isDefault := false
	if payload.IsDefault != nil {
		isDefault = *payload.IsDefault
	}

	tpl := &model.ConfigTemplate{
		SoftwareConfigID: cfg.ID,
		Name:             payload.Name,
		Format:           payload.Format,
		Content:          payload.Content,
		IsDefault:        isDefault,
	}
	if err := r.configCenterSvc.CreateTemplate(req.Context(), tpl); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                tpl.ID,
		"software_config_id": tpl.SoftwareConfigID,
		"name":              tpl.Name,
		"format":            tpl.Format,
		"content":           tpl.Content,
		"is_default":        tpl.IsDefault,
		"created_at":        tpl.CreatedAt,
		"updated_at":        tpl.UpdatedAt,
	})
}

type updateTemplateRequest struct {
	Name      string `json:"name"`
	Format    string `json:"format"`
	Content   string `json:"content"`
	IsDefault *bool  `json:"is_default,omitempty"`
}

func (r *routes) handleAdminUpdateTemplate(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimSpace(req.PathValue("id"))
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "template id is required")
		return
	}
	templateID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	var payload updateTemplateRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.Name == "" || payload.Content == "" {
		writeError(w, http.StatusBadRequest, "name and content are required")
		return
	}
	isDefault := false
	if payload.IsDefault != nil {
		isDefault = *payload.IsDefault
	}

	tpl := &model.ConfigTemplate{
		Name:      payload.Name,
		Format:    payload.Format,
		Content:   payload.Content,
		IsDefault: isDefault,
	}
	if err := r.configCenterSvc.UpdateTemplate(req.Context(), templateID, tpl); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         templateID,
		"name":       tpl.Name,
		"format":     tpl.Format,
		"content":    tpl.Content,
		"is_default": tpl.IsDefault,
		"updated_at": tpl.UpdatedAt,
	})
}

func (r *routes) handleAdminDeleteTemplate(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimSpace(req.PathValue("id"))
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "template id is required")
		return
	}
	templateID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template id")
		return
	}

	if err := r.configCenterSvc.DeleteTemplate(req.Context(), templateID); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// -------------------------------------------------------
// Admin: Global Variables
// -------------------------------------------------------

type setGlobalVarRequest struct {
	VarKey      string `json:"var_key"`
	VarValue    string `json:"var_value"`
	Description string `json:"description"`
}

func (r *routes) handleAdminListGlobalVars(w http.ResponseWriter, req *http.Request) {
	vars, err := r.configCenterSvc.ListGlobalVars(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list global vars")
		return
	}

	type globalVarResponse struct {
		ID          int64     `json:"id"`
		VarKey      string    `json:"var_key"`
		VarValue    string    `json:"var_value"`
		Description string    `json:"description"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	result := make([]globalVarResponse, 0, len(vars))
	for _, v := range vars {
		result = append(result, globalVarResponse{
			ID:          v.ID,
			VarKey:      v.VarKey,
			VarValue:    v.VarValue,
			Description: v.Description,
			CreatedAt:   v.CreatedAt,
			UpdatedAt:   v.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"vars": result})
}

func (r *routes) handleAdminSetGlobalVar(w http.ResponseWriter, req *http.Request) {
	var payload setGlobalVarRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.VarKey = strings.TrimSpace(payload.VarKey)
	if payload.VarKey == "" {
		writeError(w, http.StatusBadRequest, "var_key is required")
		return
	}

	if err := r.configCenterSvc.SetGlobalVar(req.Context(), payload.VarKey, payload.VarValue, payload.Description); err != nil {
		if errors.Is(err, configcenter.ErrDuplicateVarKey) {
			writeError(w, http.StatusConflict, "var key already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set global var")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"var_key": payload.VarKey})
}

func (r *routes) handleAdminDeleteGlobalVar(w http.ResponseWriter, req *http.Request) {
	key := strings.TrimSpace(req.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	if err := r.configCenterSvc.DeleteGlobalVar(req.Context(), key); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "global var not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete global var")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// -------------------------------------------------------
// User: Default Config by Tag
// -------------------------------------------------------

func (r *routes) handleGetDefaultConfig(w http.ResponseWriter, req *http.Request) {
	tag := strings.TrimSpace(req.URL.Query().Get("tag"))
	if tag == "" {
		writeError(w, http.StatusBadRequest, "tag query parameter is required")
		return
	}

	// API key substitution is not possible server-side because keys are stored hashed.
	// The config template is returned with placeholders intact; the client fills in the API key.
	result, err := r.configCenterSvc.GetDefaultConfigByTag(req.Context(), tag, "")
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "default config not found for tag")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to resolve config: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// -------------------------------------------------------
// User: Config Sync
// -------------------------------------------------------

type syncConfigItem struct {
	UUID     string `json:"uuid"`
	Software string `json:"software"`
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Version  string `json:"version"`
	InUse    bool   `json:"in_use"`
	Selected bool   `json:"selected"`
	Format   string `json:"format"`
	Content  string `json:"content"`
}

func (r *routes) handleSyncConfigs(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var items []syncConfigItem
	if err := json.NewDecoder(req.Body).Decode(&items); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	configs := make([]model.UserSyncedConfig, 0, len(items))
	for _, item := range items {
		configs = append(configs, model.UserSyncedConfig{
			UUID:     item.UUID,
			Software: item.Software,
			Name:     item.Name,
			FilePath: item.FilePath,
			Version:  item.Version,
			InUse:    item.InUse,
			Selected: item.Selected,
			Format:   item.Format,
			Content:  item.Content,
		})
	}

	if err := r.configCenterSvc.SyncConfigs(req.Context(), user.ID, configs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync configs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"synced": len(configs)})
}

func (r *routes) handlePullConfigs(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	opts := configcenter.PullConfigsOptions{
		Software: req.URL.Query().Get("software"),
		Page:     0,
		PageSize: 0,
	}
	if ps := req.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			opts.PageSize = v
		}
	}
	if pg := req.URL.Query().Get("page"); pg != "" {
		if v, err := strconv.Atoi(pg); err == nil && v >= 0 {
			opts.Page = v
		}
	}
	if ua := req.URL.Query().Get("updated_after"); ua != "" {
		if t, err := time.Parse(time.RFC3339, ua); err == nil {
			opts.UpdatedAfter = &t
		}
	}

	configs, err := r.configCenterSvc.PullConfigs(req.Context(), user.ID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pull configs")
		return
	}

	if configs == nil {
		configs = []model.UserSyncedConfig{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"configs": configs})
}

type compareRequest struct {
	Items []configcenter.CompareItem `json:"items"`
}

func (r *routes) handleCompareConfigs(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var payload compareRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := r.configCenterSvc.CompareConfigs(req.Context(), user.ID, payload.Items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compare configs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": result})
}

func (r *routes) handleDeleteSyncedConfig(w http.ResponseWriter, req *http.Request) {
	uuid := strings.TrimSpace(req.PathValue("uuid"))
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "uuid is required")
		return
	}

	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.configCenterSvc.DeleteSyncedConfig(req.Context(), user.ID, uuid); err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeError(w, http.StatusNotFound, "synced config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete synced config")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (r *routes) handleListSoftware(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	software, err := r.configCenterSvc.ListSoftware(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list software")
		return
	}

	if software == nil {
		software = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"software": software})
}

func (r *routes) handleGetSyncStatus(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	status, err := r.configCenterSvc.GetSyncStatus(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get sync status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/doc"
	"ai-api-portal/backend/internal/model"
)

// ── Helper converters ──────────────────────────────────────────────

func toAdminDocCategoryDTO(cat model.DocCategory) adminDocCategoryDTO {
	return adminDocCategoryDTO{
		ID:          cat.ID,
		Slug:        cat.Slug,
		Title:       cat.Title,
		Description: cat.Description,
		Icon:        cat.Icon,
		SortOrder:   cat.SortOrder,
		Status:      cat.Status,
		CreatedAt:   cat.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   cat.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminDocPageDTO(page model.DocPage) adminDocPageDTO {
	return adminDocPageDTO{
		ID:         page.ID,
		Slug:       page.Slug,
		Title:      page.Title,
		CategoryID: page.CategoryID,
		MDXBody:    page.MDXBody,
		SortOrder:  page.SortOrder,
		Status:     page.Status,
		CreatedAt:  page.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  page.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ── Public handlers ────────────────────────────────────────────────

func (r *routes) handlePublicListDocCategoriesWithPages(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	categories, err := r.docSvc.ListPublishedCategoriesWithPages(ctx)
	if err != nil {
		log.Printf("[ERROR] handlePublicListDocCategoriesWithPages: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list doc categories")
		return
	}

	dtos := make([]publicDocCategoryWithPagesDTO, 0, len(categories))
	for _, cwp := range categories {
		pages := make([]publicDocPageSummary, 0, len(cwp.Pages))
		for _, p := range cwp.Pages {
			pages = append(pages, publicDocPageSummary{
				Slug:    p.Slug,
				Title:   p.Title,
				MDXBody: p.MDXBody,
			})
		}
		dtos = append(dtos, publicDocCategoryWithPagesDTO{
			Slug:        cwp.Category.Slug,
			Title:       cwp.Category.Title,
			Description: cwp.Category.Description,
			Pages:       pages,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"categories": dtos})
}

func (r *routes) handlePublicGetDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	slug := strings.TrimSpace(req.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	page, err := r.docSvc.GetPageBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handlePublicGetDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get doc page")
		return
	}

	writeJSON(w, http.StatusOK, publicDocPageDetailResponse{
		Page: publicDocPageSummary{
			Slug:    page.Slug,
			Title:   page.Title,
			MDXBody: page.MDXBody,
		},
	})
}

// ── Admin Category handlers ───────────────────────────────────────

func (r *routes) handleAdminListDocCategories(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	categories, err := r.docSvc.ListCategories(ctx, doc.ListCategoriesFilter{})
	if err != nil {
		log.Printf("[ERROR] handleAdminListDocCategories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list doc categories")
		return
	}

	dtos := make([]adminDocCategoryDTO, 0, len(categories))
	for _, cat := range categories {
		dtos = append(dtos, toAdminDocCategoryDTO(cat))
	}

	writeJSON(w, http.StatusOK, adminDocCategoryListResponse{Categories: dtos})
}

type createDocCategoryRequest struct {
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	SortOrder   int64   `json:"sort_order"`
	Status      string  `json:"status"`
}

func (r *routes) handleAdminCreateDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var payload createDocCategoryRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Slug = strings.TrimSpace(payload.Slug)
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Slug == "" || payload.Title == "" {
		writeError(w, http.StatusBadRequest, "slug and title are required")
		return
	}

	cat := &model.DocCategory{
		Slug:        payload.Slug,
		Title:       payload.Title,
		Description: trimOptionalString(payload.Description),
		Icon:        trimOptionalString(payload.Icon),
		SortOrder:   payload.SortOrder,
		Status:      payload.Status,
	}

	if err := r.docSvc.CreateCategory(ctx, cat); err != nil {
		if errors.Is(err, doc.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		log.Printf("[ERROR] handleAdminCreateDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create doc category")
		return
	}

	writeJSON(w, http.StatusCreated, toAdminDocCategoryDTO(*cat))
}

func (r *routes) handleAdminGetDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	cat, err := r.docSvc.GetCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, doc.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "doc category not found")
			return
		}
		log.Printf("[ERROR] handleAdminGetDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get doc category")
		return
	}

	writeJSON(w, http.StatusOK, adminDocCategoryDetailResponse{Category: toAdminDocCategoryDTO(*cat)})
}

type updateDocCategoryRequest struct {
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	SortOrder   int64   `json:"sort_order"`
	Status      string  `json:"status"`
}

func (r *routes) handleAdminUpdateDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var payload updateDocCategoryRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	cat := &model.DocCategory{
		Slug:        strings.TrimSpace(payload.Slug),
		Title:       strings.TrimSpace(payload.Title),
		Description: trimOptionalString(payload.Description),
		Icon:        trimOptionalString(payload.Icon),
		SortOrder:   payload.SortOrder,
		Status:      payload.Status,
	}

	if err := r.docSvc.UpdateCategory(ctx, id, cat); err != nil {
		if errors.Is(err, doc.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "doc category not found")
			return
		}
		if errors.Is(err, doc.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		log.Printf("[ERROR] handleAdminUpdateDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update doc category")
		return
	}

	refreshed, err := r.docSvc.GetCategoryByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminUpdateDocCategory: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc category")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocCategoryDTO(*refreshed))
}

func (r *routes) handleAdminDeleteDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.DeleteCategory(ctx, id); err != nil {
		if errors.Is(err, doc.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "doc category not found")
			return
		}
		log.Printf("[ERROR] handleAdminDeleteDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete doc category")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (r *routes) handleAdminPublishDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.PublishCategory(ctx, id); err != nil {
		if errors.Is(err, doc.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "doc category not found")
			return
		}
		log.Printf("[ERROR] handleAdminPublishDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to publish doc category")
		return
	}

	refreshed, err := r.docSvc.GetCategoryByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminPublishDocCategory: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc category")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocCategoryDTO(*refreshed))
}

func (r *routes) handleAdminUnpublishDocCategory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.UnpublishCategory(ctx, id); err != nil {
		if errors.Is(err, doc.ErrCategoryNotFound) {
			writeError(w, http.StatusNotFound, "doc category not found")
			return
		}
		log.Printf("[ERROR] handleAdminUnpublishDocCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to unpublish doc category")
		return
	}

	refreshed, err := r.docSvc.GetCategoryByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminUnpublishDocCategory: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc category")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocCategoryDTO(*refreshed))
}

// ── Admin Page handlers ───────────────────────────────────────────

func (r *routes) handleAdminListDocPages(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	pages, err := r.docSvc.ListPages(ctx, doc.ListPagesFilter{})
	if err != nil {
		log.Printf("[ERROR] handleAdminListDocPages: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list doc pages")
		return
	}

	dtos := make([]adminDocPageDTO, 0, len(pages))
	for _, page := range pages {
		dtos = append(dtos, toAdminDocPageDTO(page))
	}

	writeJSON(w, http.StatusOK, adminDocPageListResponse{Pages: dtos})
}

type createDocPageRequest struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	CategoryID int64  `json:"category_id"`
	MDXBody    string `json:"mdx_body"`
	SortOrder  int64  `json:"sort_order"`
	Status     string `json:"status"`
}

func (r *routes) handleAdminCreateDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var payload createDocPageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.Slug = strings.TrimSpace(payload.Slug)
	payload.Title = strings.TrimSpace(payload.Title)
	payload.MDXBody = strings.TrimSpace(payload.MDXBody)
	if payload.Slug == "" || payload.Title == "" || payload.MDXBody == "" || payload.CategoryID == 0 {
		writeError(w, http.StatusBadRequest, "slug, title, category_id, and mdx_body are required")
		return
	}

	page := &model.DocPage{
		Slug:       payload.Slug,
		Title:      payload.Title,
		CategoryID: payload.CategoryID,
		MDXBody:    payload.MDXBody,
		SortOrder:  payload.SortOrder,
		Status:     payload.Status,
	}

	if err := r.docSvc.CreatePage(ctx, page); err != nil {
		if errors.Is(err, doc.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		log.Printf("[ERROR] handleAdminCreateDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create doc page")
		return
	}

	writeJSON(w, http.StatusCreated, toAdminDocPageDTO(*page))
}

func (r *routes) handleAdminGetDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	page, err := r.docSvc.GetPageByID(ctx, id)
	if err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handleAdminGetDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get doc page")
		return
	}

	writeJSON(w, http.StatusOK, adminDocPageDetailResponse{Page: toAdminDocPageDTO(*page)})
}

type updateDocPageRequest struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	CategoryID int64  `json:"category_id"`
	MDXBody    string `json:"mdx_body"`
	SortOrder  int64  `json:"sort_order"`
	Status     string `json:"status"`
}

func (r *routes) handleAdminUpdateDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	_, err = r.docSvc.GetPageByID(ctx, id)
	if err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handleAdminUpdateDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get doc page")
		return
	}

	var payload updateDocPageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	page := &model.DocPage{
		Slug:       strings.TrimSpace(payload.Slug),
		Title:      strings.TrimSpace(payload.Title),
		CategoryID: payload.CategoryID,
		MDXBody:    strings.TrimSpace(payload.MDXBody),
		SortOrder:  payload.SortOrder,
		Status:     payload.Status,
	}

	if err := r.docSvc.UpdatePage(ctx, id, page); err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		if errors.Is(err, doc.ErrDuplicateSlug) {
			writeError(w, http.StatusConflict, "slug already exists")
			return
		}
		log.Printf("[ERROR] handleAdminUpdateDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update doc page")
		return
	}

	refreshed, err := r.docSvc.GetPageByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminUpdateDocPage: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc page")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocPageDTO(*refreshed))
}

func (r *routes) handleAdminDeleteDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.DeletePage(ctx, id); err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handleAdminDeleteDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete doc page")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (r *routes) handleAdminPublishDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.PublishPage(ctx, id); err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handleAdminPublishDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to publish doc page")
		return
	}

	refreshed, err := r.docSvc.GetPageByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminPublishDocPage: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc page")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocPageDTO(*refreshed))
}

func (r *routes) handleAdminUnpublishDocPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.docSvc.UnpublishPage(ctx, id); err != nil {
		if errors.Is(err, doc.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "doc page not found")
			return
		}
		log.Printf("[ERROR] handleAdminUnpublishDocPage: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to unpublish doc page")
		return
	}

	refreshed, err := r.docSvc.GetPageByID(ctx, id)
	if err != nil {
		log.Printf("[ERROR] handleAdminUnpublishDocPage: refresh: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get updated doc page")
		return
	}

	writeJSON(w, http.StatusOK, toAdminDocPageDTO(*refreshed))
}

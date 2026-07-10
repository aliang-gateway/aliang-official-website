package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/model"
	"ai-api-portal/backend/internal/servicedirection"
)

// serviceDirectionResponse is the full admin representation (all fields, both languages).
type serviceDirectionResponse struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	PhaseZh     string    `json:"phase_zh"`
	PhaseEn     string    `json:"phase_en"`
	TitleZh     string    `json:"title_zh"`
	TitleEn     string    `json:"title_en"`
	DescZh      string    `json:"desc_zh"`
	DescEn      string    `json:"desc_en"`
	SortOrder   int       `json:"sort_order"`
	IsPublished bool      `json:"is_published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func serviceDirectionToResponse(sd *model.ServiceDirection) serviceDirectionResponse {
	return serviceDirectionResponse{
		ID:          sd.ID,
		Status:      sd.Status,
		PhaseZh:     sd.PhaseZh,
		PhaseEn:     sd.PhaseEn,
		TitleZh:     sd.TitleZh,
		TitleEn:     sd.TitleEn,
		DescZh:      sd.DescZh,
		DescEn:      sd.DescEn,
		SortOrder:   sd.SortOrder,
		IsPublished: sd.IsPublished,
		CreatedAt:   sd.CreatedAt,
		UpdatedAt:   sd.UpdatedAt,
	}
}

// publicServiceDirectionResponse is the public representation, localized to one language.
type publicServiceDirectionResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Title  string `json:"title"`
	Desc   string `json:"desc"`
}

func publicServiceDirectionToResponse(sd *model.ServiceDirection, lang string) publicServiceDirectionResponse {
	resp := publicServiceDirectionResponse{ID: sd.ID, Status: sd.Status}
	if lang == "en" {
		resp.Phase = sd.PhaseEn
		resp.Title = sd.TitleEn
		resp.Desc = sd.DescEn
	} else {
		resp.Phase = sd.PhaseZh
		resp.Title = sd.TitleZh
		resp.Desc = sd.DescZh
	}
	return resp
}

type serviceDirectionRequest struct {
	Status      string `json:"status"`
	PhaseZh     string `json:"phase_zh"`
	PhaseEn     string `json:"phase_en"`
	TitleZh     string `json:"title_zh"`
	TitleEn     string `json:"title_en"`
	DescZh      string `json:"desc_zh"`
	DescEn      string `json:"desc_en"`
	SortOrder   int    `json:"sort_order"`
	IsPublished bool   `json:"is_published"`
}

func (req *serviceDirectionRequest) toModel() *model.ServiceDirection {
	return &model.ServiceDirection{
		Status:      req.Status,
		PhaseZh:     req.PhaseZh,
		PhaseEn:     req.PhaseEn,
		TitleZh:     req.TitleZh,
		TitleEn:     req.TitleEn,
		DescZh:      req.DescZh,
		DescEn:      req.DescEn,
		SortOrder:   req.SortOrder,
		IsPublished: req.IsPublished,
	}
}

func parseServiceID(req *http.Request) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(req.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func (r *routes) handleAdminListServiceDirections(w http.ResponseWriter, req *http.Request) {
	items, err := r.serviceDirectionSvc.ListAll(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list service directions")
		return
	}
	result := make([]serviceDirectionResponse, 0, len(items))
	for i := range items {
		result = append(result, serviceDirectionToResponse(&items[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": result})
}

func (r *routes) handleAdminCreateServiceDirection(w http.ResponseWriter, req *http.Request) {
	var payload serviceDirectionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	sd := payload.toModel()
	if err := r.serviceDirectionSvc.Create(req.Context(), sd); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, serviceDirectionToResponse(sd))
}

func (r *routes) handleAdminGetServiceDirection(w http.ResponseWriter, req *http.Request) {
	id, err := parseServiceID(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sd, err := r.serviceDirectionSvc.Get(req.Context(), id)
	if err != nil {
		if errors.Is(err, servicedirection.ErrNotFound) {
			writeError(w, http.StatusNotFound, "service direction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get service direction")
		return
	}
	writeJSON(w, http.StatusOK, serviceDirectionToResponse(sd))
}

func (r *routes) handleAdminUpdateServiceDirection(w http.ResponseWriter, req *http.Request) {
	id, err := parseServiceID(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var payload serviceDirectionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	sd := payload.toModel()
	if err := r.serviceDirectionSvc.Update(req.Context(), id, sd); err != nil {
		if errors.Is(err, servicedirection.ErrNotFound) {
			writeError(w, http.StatusNotFound, "service direction not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sd.ID = id
	writeJSON(w, http.StatusOK, serviceDirectionToResponse(sd))
}

func (r *routes) handleAdminDeleteServiceDirection(w http.ResponseWriter, req *http.Request) {
	id, err := parseServiceID(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := r.serviceDirectionSvc.Delete(req.Context(), id); err != nil {
		if errors.Is(err, servicedirection.ErrNotFound) {
			writeError(w, http.StatusNotFound, "service direction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete service direction")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (r *routes) handlePublicListServiceDirections(w http.ResponseWriter, req *http.Request) {
	lang := strings.TrimSpace(req.URL.Query().Get("lang"))
	if lang != "en" {
		lang = "zh"
	}
	items, err := r.serviceDirectionSvc.ListPublic(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list service directions")
		return
	}
	result := make([]publicServiceDirectionResponse, 0, len(items))
	for i := range items {
		result = append(result, publicServiceDirectionToResponse(&items[i], lang))
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": result})
}

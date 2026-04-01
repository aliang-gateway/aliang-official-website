package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-api-portal/backend/internal/download"
	"ai-api-portal/backend/internal/model"
)

type downloadResponse struct {
	ID           int64     `json:"id"`
	SoftwareName string    `json:"software_name"`
	Platform     string    `json:"platform"`
	FileType     string    `json:"file_type"`
	DownloadURL  string    `json:"download_url"`
	Version      string    `json:"version"`
	ForceUpdate  bool      `json:"force_update"`
	Changelog   string    `json:"changelog"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func downloadToResponse(d *model.Download) downloadResponse {
	return downloadResponse{
		ID:           d.ID,
		SoftwareName: d.SoftwareName,
		Platform:     d.Platform,
		FileType:     d.FileType,
		DownloadURL:  d.DownloadURL,
		Version:      d.Version,
		ForceUpdate:  d.ForceUpdate,
		Changelog:    d.Changelog,
		IsDefault:    d.IsDefault,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

func (r *routes) handleAdminListDownloads(w http.ResponseWriter, req *http.Request) {
	downloads, err := r.downloadSvc.ListDownloads(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list downloads center")
		return
	}

	result := make([]downloadResponse, 0, len(downloads))
	for i := range downloads {
		result = append(result, downloadToResponse(&downloads[i]))
	}
	if result == nil {
		result = []downloadResponse{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": result})
}

type createDownloadRequest struct {
	SoftwareName string `json:"software_name"`
	Platform     string `json:"platform"`
	FileType     string `json:"file_type"`
	DownloadURL  string `json:"download_url"`
	Version      string `json:"version"`
	ForceUpdate  bool   `json:"force_update"`
	Changelog    string `json:"changelog"`
	IsDefault    bool   `json:"is_default"`
}

func (r *routes) handleAdminCreateDownload(w http.ResponseWriter, req *http.Request) {
	var payload createDownloadRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	d := &model.Download{
		SoftwareName: payload.SoftwareName,
		Platform:     payload.Platform,
		FileType:     payload.FileType,
		DownloadURL:  payload.DownloadURL,
		Version:      payload.Version,
		ForceUpdate:  payload.ForceUpdate,
		Changelog:    payload.Changelog,
		IsDefault:    payload.IsDefault,
	}

	if err := r.downloadSvc.CreateDownload(req.Context(), d); err != nil {
		if errors.Is(err, download.ErrInvalidVersion) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create download")
		return
	}

	writeJSON(w, http.StatusCreated, downloadToResponse(d))
}

func (r *routes) handleAdminGetDownload(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimSpace(req.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	d, err := r.downloadSvc.GetDownload(req.Context(), id)
	if err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "download not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get download")
		return
	}

	writeJSON(w, http.StatusOK, downloadToResponse(d))
}

type updateDownloadRequest struct {
	SoftwareName string `json:"software_name"`
	Platform     string `json:"platform"`
	FileType     string `json:"file_type"`
	DownloadURL  string `json:"download_url"`
	Version      string `json:"version"`
	ForceUpdate  bool   `json:"force_update"`
	Changelog    string `json:"changelog"`
	IsDefault    bool   `json:"is_default"`
}

func (r *routes) handleAdminUpdateDownload(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimSpace(req.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var payload updateDownloadRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	d := &model.Download{
		SoftwareName: payload.SoftwareName,
		Platform:     payload.Platform,
		FileType:     payload.FileType,
		DownloadURL:  payload.DownloadURL,
		Version:      payload.Version,
		ForceUpdate:  payload.ForceUpdate,
		Changelog:    payload.Changelog,
		IsDefault:    payload.IsDefault,
	}

	if err := r.downloadSvc.UpdateDownload(req.Context(), id, d); err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "download not found")
			return
		}
		if errors.Is(err, download.ErrInvalidVersion) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update download")
		return
	}

	d.ID = id
	writeJSON(w, http.StatusOK, downloadToResponse(d))
}

func (r *routes) handleAdminDeleteDownload(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimSpace(req.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := r.downloadSvc.DeleteDownload(req.Context(), id); err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "download not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete download")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (r *routes) handlePublicVersionCheck(w http.ResponseWriter, req *http.Request) {
	platform := strings.TrimSpace(req.URL.Query().Get("platform"))
	software := strings.TrimSpace(req.URL.Query().Get("software"))
	userVersion := strings.TrimSpace(req.URL.Query().Get("version"))

	result, err := r.downloadSvc.CheckVersion(req.Context(), platform, software, userVersion)
	if err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no downloads found for the given criteria")
			return
		}
		if errors.Is(err, download.ErrInvalidVersion) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (r *routes) handlePublicListDownloads(w http.ResponseWriter, req *http.Request) {
	platform := strings.TrimSpace(req.URL.Query().Get("platform"))

	downloads, err := r.downloadSvc.ListPublicDownloads(req.Context(), platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list downloads")
		return
	}

	result := make([]downloadResponse, 0, len(downloads))
	for i := range downloads {
		result = append(result, downloadToResponse(&downloads[i]))
	}
	if result == nil {
		result = []downloadResponse{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": result})
}

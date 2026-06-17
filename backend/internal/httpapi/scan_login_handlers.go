package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"ai-api-portal/backend/internal/auth"
	"ai-api-portal/backend/internal/scanlogin"
)

type scanCodeRequest struct {
	Code string `json:"code"`
}

func (r *routes) handleScanInit(w http.ResponseWriter, req *http.Request) {
	res, err := r.scanLogin.Init(req.Context(), scanClientIP(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create scan code")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (r *routes) handleScanStatus(w http.ResponseWriter, req *http.Request) {
	res, err := r.scanLogin.Status(req.Context(), req.URL.Query().Get("device_code"))
	if err != nil {
		if errors.Is(err, scanlogin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "scan code not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query status")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (r *routes) handleScanScan(w http.ResponseWriter, req *http.Request) {
	u, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Scan(req.Context(), body.Code, u.ID); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "scanned"})
}

func (r *routes) handleScanConfirm(w http.ResponseWriter, req *http.Request) {
	u, ok := auth.UserFromContext(req.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Confirm(req.Context(), body.Code, u.ID); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "authorized"})
}

func (r *routes) handleScanDeny(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.UserFromContext(req.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeScanCodeBody(w, req)
	if !ok {
		return
	}
	if err := r.scanLogin.Deny(req.Context(), body.Code); err != nil {
		writeScanStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "denied"})
}

func decodeScanCodeBody(w http.ResponseWriter, req *http.Request) (*scanCodeRequest, bool) {
	var body scanCodeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	body.Code = strings.TrimSpace(body.Code)
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return nil, false
	}
	return &body, true
}

func writeScanStateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scanlogin.ErrNotFound):
		writeError(w, http.StatusNotFound, "scan code not found")
	case errors.Is(err, scanlogin.ErrInvalidState):
		writeError(w, http.StatusConflict, "scan code is not in a valid state")
	default:
		writeError(w, http.StatusInternalServerError, "scan operation failed")
	}
}

// scanClientIP 取发起端 IP（去端口），用于审计。用 net.SplitHostPort 以正确处理 IPv6。
func scanClientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(req.RemoteAddr)
	}
	return strings.TrimSpace(host)
}

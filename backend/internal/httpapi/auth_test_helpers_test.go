package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setBearerAuth(req *http.Request, sessionToken string) {
	req.Header.Set("Authorization", "Bearer "+sessionToken)
}

func createUserViaAPI(t *testing.T, mux *http.ServeMux, email, name, role, adminBootstrapSecret string) (int64, string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"email": email,
		"name":  name,
		"role":  role,
	})
	if err != nil {
		t.Fatalf("marshal create user request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	if role == "admin" && adminBootstrapSecret != "" {
		req.Header.Set("X-Admin-Bootstrap-Secret", adminBootstrapSecret)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		ID           int64  `json:"id"`
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode create user response: %v", err)
	}
	if response.ID <= 0 {
		t.Fatalf("expected positive user id, got %d", response.ID)
	}
	if response.SessionToken == "" {
		t.Fatalf("expected non-empty session token")
	}

	return response.ID, response.SessionToken
}

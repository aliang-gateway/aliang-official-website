package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateUserRequiresAuthenticatedAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		role       string // seeded session role; empty → no auth at all
		bodyRole   string
		secret     string
		wantStatus int
	}{
		{name: "anonymous is rejected", role: "", bodyRole: "user", wantStatus: http.StatusUnauthorized},
		{name: "plain user is rejected", role: "user", bodyRole: "user", wantStatus: http.StatusForbidden},
		{name: "distributor is rejected", role: "distributor", bodyRole: "user", wantStatus: http.StatusForbidden},
		{name: "admin may create a plain user", role: "admin", bodyRole: "user", wantStatus: http.StatusCreated},
		{name: "admin creating admin still needs bootstrap secret", role: "admin", bodyRole: "admin", wantStatus: http.StatusForbidden},
		{name: "admin plus secret may create admin", role: "admin", bodyRole: "admin", secret: "test-admin-secret", wantStatus: http.StatusCreated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			database := setupTestDB(t)
			m := http.NewServeMux()
			RegisterRoutesWithOptions(m, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret", SQLDialect: "sqlite"})

			body := map[string]string{"email": "created@example.com", "name": "Created"}
			if tc.bodyRole != "" {
				body["role"] = tc.bodyRole
			}
			payload, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")

			if tc.role != "" {
				token := seedUserWithSession(t, database, tc.role+"@example.com", tc.role)
				setBearerAuth(req, token)
			}
			if tc.secret != "" {
				req.Header.Set("X-Admin-Bootstrap-Secret", tc.secret)
			}

			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

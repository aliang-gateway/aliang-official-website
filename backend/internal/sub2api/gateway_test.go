package sub2api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"ai-api-portal/backend/internal/proxy"
	"ai-api-portal/backend/internal/sub2apiauth"
)

// --- mocks ---

type mockAuth struct {
	token string
	err   error
}

func (m *mockAuth) GetBearerTokenByUserID(_ context.Context, _ int64) (string, error) {
	return m.token, m.err
}

func (m *mockAuth) UpsertToken(_ context.Context, _ sub2apiauth.UpsertTokenInput) error {
	return nil
}

type mockProxy struct {
	apiKeyResp *proxy.ResponseEnvelope[proxy.APIKey]
	err        error
}

func (m *mockProxy) CreateUserAPIKey(_ context.Context, _ string, _ proxy.CreateUserAPIKeyRequest, _ string) (*proxy.ResponseEnvelope[proxy.APIKey], error) {
	return m.apiKeyResp, m.err
}

type mockResolver struct {
	userID int64
	found  bool
	err    error

	role      string
	roleFound bool
	roleErr   error
}

func (m *mockResolver) FindUserIDBySession(_ context.Context, _ string) (int64, bool, error) {
	return m.userID, m.found, m.err
}

func (m *mockResolver) FindUserRoleByID(_ context.Context, _ int64) (string, bool, error) {
	return m.role, m.roleFound, m.roleErr
}

// Since Gateway uses concrete types, we test via the Gateway struct
// with real sub2apiauth.Service (in-memory) or by testing individual methods
// that only depend on the auth interface.

func TestNewGateway(t *testing.T) {
	g := NewGateway(nil, nil)
	if g == nil {
		t.Fatal("expected non-nil gateway")
	}
}

func TestGateway_IsConfigured(t *testing.T) {
	tests := []struct {
		name    string
		gateway *Gateway
		want    bool
	}{
		{"nil gateway", nil, false},
		{"nil deps", NewGateway(nil, nil), false},
		{"only proxy", NewGateway(&proxy.Client{}, nil), false},
		{"only auth", NewGateway(nil, &sub2apiauth.Service{}), false},
		{"both present", NewGateway(&proxy.Client{}, &sub2apiauth.Service{}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gateway.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGateway_HasUpstreamToken_NilGateway(t *testing.T) {
	var g *Gateway
	ok, err := g.HasUpstreamToken(context.Background(), 1)
	if ok || err != nil {
		t.Errorf("expected (false, nil), got (%v, %v)", ok, err)
	}
}

func TestGateway_CaptureTokens_NilGateway(t *testing.T) {
	var g *Gateway
	err := g.CaptureTokens(context.Background(), sub2apiauth.UpsertTokenInput{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_NilGateway(t *testing.T) {
	var g *Gateway
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test")
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_NilHeaders(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	err := g.ReplaceAuthHeader(context.Background(), nil, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_EmptyAuth(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_InvalidBearer(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Basic abc123")
	err := g.ReplaceAuthHeader(context.Background(), headers, &mockResolver{})
	if err != nil {
		t.Errorf("expected nil error for invalid bearer, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_UserNotFound(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Bearer session-token")
	resolver := &mockResolver{userID: 0, found: false}
	err := g.ReplaceAuthHeader(context.Background(), headers, resolver)
	if err != nil {
		t.Errorf("expected nil error for user not found, got %v", err)
	}
}

func TestGateway_ReplaceAuthHeader_ResolverError(t *testing.T) {
	g := NewGateway(&proxy.Client{}, &sub2apiauth.Service{})
	headers := http.Header{}
	headers.Set("Authorization", "Bearer session-token")
	resolver := &mockResolver{err: errors.New("db down")}
	err := g.ReplaceAuthHeader(context.Background(), headers, resolver)
	if err == nil {
		t.Error("expected error from resolver")
	}
}

func TestGateway_EnsureUserKeyInGroup_NilDeps(t *testing.T) {
	g := NewGateway(nil, &sub2apiauth.Service{})
	err := g.EnsureUserKeyInGroup(context.Background(), 1, 10, "parent-key")
	if err != nil {
		t.Errorf("expected nil for nil proxy, got %v", err)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"Bearer abc123", "abc123", false},
		{"Bearer   abc123   ", "abc123", false},
		{"bearer abc123", "", true}, // case-sensitive
		{"Basic abc123", "", true},
		{"", "", true},
		{"Bearer ", "", true},
		{"Bearer", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := extractBearerToken(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBearerToken(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

package guard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTurnstileVerifyAcceptsValidToken(t *testing.T) {
	t.Parallel()

	var gotForm map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			t.Fatalf("parse siteverify form: %v", err)
		}
		gotForm = map[string]string{
			"secret":   req.Form.Get("secret"),
			"response": req.Form.Get("response"),
			"remoteip": req.Form.Get("remoteip"),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer server.Close()

	v := NewTurnstileVerifier("unit-secret", server.URL)
	if err := v.Verify(context.Background(), "token-1", "9.9.9.9"); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if gotForm["secret"] != "unit-secret" || gotForm["response"] != "token-1" || gotForm["remoteip"] != "9.9.9.9" {
		t.Fatalf("siteverify payload mismatch: %+v", gotForm)
	}
}

func TestTurnstileVerifyRejectsFailedChallenge(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"error-codes": []string{"invalid-input-response"},
		})
	}))
	defer server.Close()

	v := NewTurnstileVerifier("unit-secret", server.URL)
	err := v.Verify(context.Background(), "bogus", "9.9.9.9")
	if err == nil {
		t.Fatal("expected rejection for success=false")
	}
	if !errors.Is(err, ErrChallengeFailed) {
		t.Fatalf("expected ErrChallengeFailed, got %v", err)
	}
}

func TestTurnstileVerifyFailsClosedOnServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	v := NewTurnstileVerifier("unit-secret", server.URL)
	if err := v.Verify(context.Background(), "token-1", ""); err == nil {
		t.Fatal("expected fail-closed when siteverify responds 5xx")
	}
}

func TestTurnstileVerifyFailsClosedOnTransportError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
	server.Close() // listener shut down → dial error

	v := NewTurnstileVerifier("unit-secret", server.URL)
	if err := v.Verify(context.Background(), "token-1", ""); err == nil {
		t.Fatal("expected fail-closed when siteverify is unreachable")
	}
}

func TestTurnstileVerifyRejectsEmptyTokenWithoutRemoteCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Fatal("siteverify must not be called for an empty token")
	}))
	defer server.Close()

	v := NewTurnstileVerifier("unit-secret", server.URL)
	if err := v.Verify(context.Background(), "  ", ""); !errors.Is(err, ErrChallengeFailed) {
		t.Fatalf("expected ErrChallengeFailed for empty token, got %v", err)
	}
}

package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestStartAuthorizationCodeFlow(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Fatalf("grant_type = %q, want authorization_code", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code_verifier") == "" {
			t.Fatal("code_verifier is empty")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "oauth-token"})
	}))
	defer tokenServer.Close()

	port := freePort(t)
	callbackAddr := fmt.Sprintf("127.0.0.1:%d", port)
	redirectURL := fmt.Sprintf("http://localhost:%d/auth/callback", port)

	var opened bool
	client := &Client{
		HTTPClient: tokenServer.Client(),
		OpenBrowser: func(target string) error {
			opened = true
			u, err := url.Parse(target)
			if err != nil {
				return err
			}
			state := u.Query().Get("state")
			_, err = http.Get(redirectURL + "?code=test-code&state=" + url.QueryEscape(state))
			return err
		},
	}

	settings := Settings{
		AuthorizeURL: "https://example.com/authorize",
		TokenURL:     tokenServer.URL,
		ClientID:     "client-id",
		Scopes:       "openid profile",
		CallbackAddr: callbackAddr,
		RedirectURL:  redirectURL,
		Originator:   "pi",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := client.StartAuthorizationCodeFlow(ctx, settings)
	if err != nil {
		t.Fatalf("StartAuthorizationCodeFlow() error: %v", err)
	}
	if !opened {
		t.Fatal("browser was not opened")
	}
	if got.OAuthToken != "oauth-token" {
		t.Fatalf("OAuthToken = %q, want oauth-token", got.OAuthToken)
	}
	if got.ClientID != "client-id" {
		t.Fatalf("ClientID = %q, want client-id", got.ClientID)
	}
}

func TestStartAuthorizationCodeFlowStateMismatch(t *testing.T) {
	port := freePort(t)
	callbackAddr := fmt.Sprintf("127.0.0.1:%d", port)
	redirectURL := fmt.Sprintf("http://localhost:%d/auth/callback", port)

	client := &Client{
		HTTPClient: http.DefaultClient,
		OpenBrowser: func(_ string) error {
			_, err := http.Get(redirectURL + "?code=test-code&state=wrong-state")
			return err
		},
	}

	settings := Settings{
		AuthorizeURL: "https://example.com/authorize",
		TokenURL:     "https://example.com/token",
		ClientID:     "client-id",
		Scopes:       "openid profile",
		CallbackAddr: callbackAddr,
		RedirectURL:  redirectURL,
		Originator:   "pi",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.StartAuthorizationCodeFlow(ctx, settings)
	if err == nil {
		t.Fatal("StartAuthorizationCodeFlow() error = nil, want error")
	}
	if err.Error() != "state mismatch" {
		t.Fatalf("error = %v, want state mismatch", err)
	}
}

func TestStartAuthorizationCodeFlowAuthorizationError(t *testing.T) {
	port := freePort(t)
	callbackAddr := fmt.Sprintf("127.0.0.1:%d", port)
	redirectURL := fmt.Sprintf("http://localhost:%d/auth/callback", port)

	client := &Client{
		HTTPClient: http.DefaultClient,
		OpenBrowser: func(target string) error {
			u, err := url.Parse(target)
			if err != nil {
				return err
			}
			state := u.Query().Get("state")
			_, err = http.Get(redirectURL +
				"?error=invalid_scope&error_description=scope+not+allowed&state=" + url.QueryEscape(state))
			return err
		},
	}

	settings := Settings{
		AuthorizeURL: "https://example.com/authorize",
		TokenURL:     "https://example.com/token",
		ClientID:     "client-id",
		Scopes:       "openid profile",
		CallbackAddr: callbackAddr,
		RedirectURL:  redirectURL,
		Originator:   "pi",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.StartAuthorizationCodeFlow(ctx, settings)
	if err == nil {
		t.Fatal("StartAuthorizationCodeFlow() error = nil, want error")
	}
	var authErr *AuthorizationError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %v, want AuthorizationError", err)
	}
	if authErr.Code != "invalid_scope" {
		t.Fatalf("code = %q, want invalid_scope", authErr.Code)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	defaultAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	defaultTokenURL     = "https://auth.openai.com/oauth/token"
	defaultClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultScopes       = "openid profile email offline_access"
	defaultCallbackAddr = "127.0.0.1:1455"
	defaultRedirectURL  = "http://localhost:1455/auth/callback"
	defaultOriginator   = "pi"
)

// Settings configures the OAuth flow.
type Settings struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	Scopes       string
	CallbackAddr string
	RedirectURL  string
	Originator   string
}

// TokenPair represents the persisted auth data.
type TokenPair struct {
	OAuthToken string
	ClientID   string
}

// BrowserOpener opens an URL in a browser.
type BrowserOpener func(url string) error

// Client runs OAuth flows.
type Client struct {
	HTTPClient   *http.Client
	OpenBrowser  BrowserOpener
	Now          func() time.Time
	RandomReader io.Reader
}

type callbackResult struct {
	Code  string
	State string
	Err   error
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// AuthorizationError represents an OAuth authorization failure returned in callback query params.
type AuthorizationError struct {
	Code        string
	Description string
}

func (e *AuthorizationError) Error() string {
	if strings.TrimSpace(e.Description) == "" {
		return fmt.Sprintf("oauth authorization error: %s", e.Code)
	}
	return fmt.Sprintf("oauth authorization error: %s (%s)", e.Code, e.Description)
}

// SettingsFromEnv loads settings from optional env vars.
func SettingsFromEnv() Settings {
	return Settings{
		AuthorizeURL: envOrDefault("CODEX_OAUTH_AUTHORIZE_URL", defaultAuthorizeURL),
		TokenURL:     envOrDefault("CODEX_OAUTH_TOKEN_URL", defaultTokenURL),
		ClientID:     envOrDefault("CODEX_OAUTH_CLIENT_ID", defaultClientID),
		Scopes:       envOrDefault("CODEX_OAUTH_SCOPES", defaultScopes),
		CallbackAddr: envOrDefault("CODEX_OAUTH_CALLBACK_ADDR", defaultCallbackAddr),
		RedirectURL:  envOrDefault("CODEX_OAUTH_REDIRECT_URL", defaultRedirectURL),
		Originator:   envOrDefault("CODEX_OAUTH_ORIGINATOR", defaultOriginator),
	}
}

// NewClient returns a client with sane defaults.
func NewClient() *Client {
	return &Client{
		HTTPClient:   http.DefaultClient,
		OpenBrowser:  OpenURLInBrowser,
		Now:          time.Now,
		RandomReader: rand.Reader,
	}
}

// StartAuthorizationCodeFlow runs Authorization Code + PKCE and returns access token.
func (c *Client) StartAuthorizationCodeFlow(ctx context.Context, settings Settings) (TokenPair, error) {
	if c == nil {
		c = NewClient()
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	if c.OpenBrowser == nil {
		c.OpenBrowser = OpenURLInBrowser
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.RandomReader == nil {
		c.RandomReader = rand.Reader
	}

	state, err := c.randomCode(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generating state: %w", err)
	}
	codeVerifier, err := c.randomCode(48)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generating code verifier: %w", err)
	}
	codeChallenge := pkceS256(codeVerifier)

	listener, err := net.Listen("tcp", settings.CallbackAddr)
	if err != nil {
		return TokenPair{}, fmt.Errorf("listening callback address: %w", err)
	}
	defer listener.Close()

	callbackCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		gotState := r.URL.Query().Get("state")
		if gotState != "" && gotState != state {
			http.Error(w, "Invalid state.", http.StatusBadRequest)
			select {
			case callbackCh <- callbackResult{Err: errors.New("state mismatch")}:
			default:
			}
			return
		}
		if authErrCode := r.URL.Query().Get("error"); authErrCode != "" {
			authErrDesc := strings.TrimSpace(r.URL.Query().Get("error_description"))
			http.Error(w, fmt.Sprintf("OAuth failed: %s", authErrCode), http.StatusBadRequest)
			select {
			case callbackCh <- callbackResult{Err: &AuthorizationError{Code: authErrCode, Description: authErrDesc}}:
			default:
			}
			return
		}

		code := r.URL.Query().Get("code")
		if gotState == "" || code == "" {
			http.Error(w, "Missing state or code.", http.StatusBadRequest)
			select {
			case callbackCh <- callbackResult{Err: errors.New("missing state or code")}:
			default:
			}
			return
		}
		_, _ = io.WriteString(w, "Authentication complete. You can return to the terminal.")
		select {
		case callbackCh <- callbackResult{Code: code, State: gotState}:
		default:
		}
	})

	server := &http.Server{Handler: mux}
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	authorizeURL, err := buildAuthorizeURL(settings, state, codeChallenge)
	if err != nil {
		return TokenPair{}, err
	}
	if err := c.OpenBrowser(authorizeURL); err != nil {
		return TokenPair{}, fmt.Errorf("opening browser: %w", err)
	}

	var result callbackResult
	select {
	case <-ctx.Done():
		return TokenPair{}, ctx.Err()
	case err := <-serverErrCh:
		return TokenPair{}, fmt.Errorf("callback server failed: %w", err)
	case result = <-callbackCh:
	}
	if result.Err != nil {
		return TokenPair{}, result.Err
	}
	if result.State != state {
		return TokenPair{}, errors.New("state mismatch")
	}

	token, err := c.exchangeCode(ctx, settings, result.Code, codeVerifier)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		OAuthToken: token,
		ClientID:   settings.ClientID,
	}, nil
}

func (c *Client) exchangeCode(ctx context.Context, settings Settings, code, codeVerifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", settings.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", settings.RedirectURL)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("token response missing access_token")
	}
	return parsed.AccessToken, nil
}

func buildAuthorizeURL(settings Settings, state, codeChallenge string) (string, error) {
	u, err := url.Parse(settings.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("invalid authorize url: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", settings.ClientID)
	q.Set("redirect_uri", settings.RedirectURL)
	q.Set("scope", settings.Scopes)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", settings.Originator)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func pkceS256(codeVerifier string) string {
	sum := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (c *Client) randomCode(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(c.RandomReader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// OpenURLInBrowser opens the provided URL in the OS browser.
func OpenURLInBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

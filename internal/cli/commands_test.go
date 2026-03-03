package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leandrotocalini/codelens/internal/config"
	"github.com/leandrotocalini/codelens/internal/oauth"
	"github.com/leandrotocalini/codelens/internal/summarizer"
)

type fakeOAuthRunner struct {
	called   bool
	settings oauth.Settings
	pair     oauth.TokenPair
	err      error
}

func (f *fakeOAuthRunner) StartAuthorizationCodeFlow(_ context.Context, s oauth.Settings) (oauth.TokenPair, error) {
	f.called = true
	f.settings = s
	if f.err != nil {
		return oauth.TokenPair{}, f.err
	}
	return f.pair, nil
}

type fakeSummarizer struct{}

func (f *fakeSummarizer) Summarize(_ context.Context, prompt string) (string, error) {
	if strings.Contains(strings.ToLower(prompt), "executive summary") || strings.Contains(strings.ToLower(prompt), "module summaries") {
		return "Project summary from fake model.", nil
	}
	return "**Responsibility**: Fake summary.\n**Key types**: None\n**Key functions**: None", nil
}

func TestRootCommandMissingConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout: &out,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(stderr.String(), config.SetupInstructions()) {
		t.Fatalf("stderr = %q, want setup instructions", stderr.String())
	}
}

func TestConfigureOpensOAuthFlowAndSavesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeOAuthRunner{
		pair: oauth.TokenPair{
			OAuthToken: "oauth-token",
			ClientID:   "client-id",
		},
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout:      &out,
		Stderr:      &stderr,
		Stdin:       strings.NewReader(""),
		OAuthClient: fake,
	})
	cmd.SetArgs([]string{"configure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !fake.called {
		t.Fatal("OAuth flow was not called")
	}
	if !strings.Contains(out.String(), "Your browser will open to sign in with Codex OAuth") {
		t.Fatalf("stdout = %q, missing browser message", out.String())
	}

	path := filepath.Join(home, ".codelens", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	var saved config.GlobalConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if saved.OAuthToken != "oauth-token" {
		t.Fatalf("oauthToken = %q, want oauth-token", saved.OAuthToken)
	}
	if saved.ClientID != "client-id" {
		t.Fatalf("clientId = %q, want client-id", saved.ClientID)
	}
}

func TestConfigureManualTokenFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeOAuthRunner{
		err: errors.New("browser open failed"),
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout:      &out,
		Stderr:      &stderr,
		Stdin:       strings.NewReader("manual-token\n"),
		OAuthClient: fake,
	})
	cmd.SetArgs([]string{"configure"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	cfg, err := config.Load(t.TempDir(), config.CLIFlags{})
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.OAuthToken != "manual-token" {
		t.Fatalf("OAuthToken = %q, want manual-token", cfg.OAuthToken)
	}
}

func TestConfigureWithFlagsSkipsBrowserFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeOAuthRunner{
		err: errors.New("should not be called"),
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout:      &out,
		Stderr:      &stderr,
		Stdin:       strings.NewReader(""),
		OAuthClient: fake,
	})
	cmd.SetArgs([]string{
		"configure",
		"--oauth-token", "flag-token",
		"--client-id", "flag-client",
		"--model", "gpt-5.3-codex",
		"--concurrency", "9",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if fake.called {
		t.Fatal("OAuth browser flow should not be called when --oauth-token is provided")
	}

	cfg, err := config.Load(t.TempDir(), config.CLIFlags{})
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.OAuthToken != "flag-token" {
		t.Fatalf("OAuthToken = %q, want flag-token", cfg.OAuthToken)
	}
	if cfg.ClientID != "flag-client" {
		t.Fatalf("ClientID = %q, want flag-client", cfg.ClientID)
	}
	if cfg.Model != "gpt-5.3-codex" {
		t.Fatalf("Model = %q, want gpt-5.3-codex", cfg.Model)
	}
	if cfg.Concurrency != 9 {
		t.Fatalf("Concurrency = %d, want 9", cfg.Concurrency)
	}
}

func TestConfigureAuthorizationErrorReturnsActionableMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeOAuthRunner{
		err: &oauth.AuthorizationError{
			Code:        "invalid_scope",
			Description: "Client is not allowed to request scope 'model.request'",
		},
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout:      &out,
		Stderr:      &stderr,
		Stdin:       strings.NewReader("manual-token\n"),
		OAuthClient: fake,
	})
	cmd.SetArgs([]string{"configure"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if strings.Contains(out.String(), "Paste your Codex OAuth token") {
		t.Fatalf("manual token fallback should not run for authorization errors: %s", out.String())
	}
	if !strings.Contains(stderr.String(), "CODEX_OAUTH_SCOPES") {
		t.Fatalf("stderr missing actionable scopes guidance: %s", stderr.String())
	}
}

func TestRootCommandRunsPipeline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()

	if err := config.SaveGlobal(config.GlobalConfig{
		OAuthToken:  "token",
		ClientID:    "client",
		Model:       "codex",
		Concurrency: 2,
	}); err != nil {
		t.Fatalf("SaveGlobal() error: %v", err)
	}

	goFile := `package sample

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }
`
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte(goFile), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(Deps{
		Stdout: &out,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
		SummarizerFactory: func(_ string, _ string, _ bool, _ io.Writer) (summarizer.Summarizer, error) {
			return &fakeSummarizer{}, nil
		},
	})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(repo, "CODELENS.md"))
	if err != nil {
		t.Fatalf("ReadFile(CODELENS.md) error: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Project summary from fake model.") {
		t.Fatalf("CODELENS.md missing project summary: %s", text)
	}
	if !strings.Contains(text, "## CLI Commands") {
		t.Fatalf("CODELENS.md missing CLI section: %s", text)
	}
	if !strings.Contains(text, "codelens configure") {
		t.Fatalf("CODELENS.md missing configure command details: %s", text)
	}
}

func TestBuildCLIReferenceIncludesConfigureFlags(t *testing.T) {
	cmd := NewRootCommand(Deps{})
	ref := buildCLIReference(cmd)
	if !strings.Contains(ref, "## CLI Commands") {
		t.Fatalf("ref missing section header: %s", ref)
	}
	if !strings.Contains(ref, "codelens configure") {
		t.Fatalf("ref missing configure command: %s", ref)
	}
	if !strings.Contains(ref, "--oauth-token") {
		t.Fatalf("ref missing configure params: %s", ref)
	}
	if !strings.Contains(ref, "--debug") {
		t.Fatalf("ref missing debug flag: %s", ref)
	}
}

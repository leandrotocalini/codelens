package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := Load(t.TempDir(), CLIFlags{})
	if err == nil {
		t.Fatal("Load() error = nil, want missing config error")
	}
	if !IsMissingConfigError(err) {
		t.Fatalf("Load() error = %v, want missing config error", err)
	}
	if SetupInstructions() != "Configuración no encontrada en ~/.codelens/config.json. Ejecutá: codelens configure" {
		t.Fatalf("unexpected setup instructions: %q", SetupInstructions())
	}
}

func TestLoadUsesGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := SaveGlobal(GlobalConfig{
		OAuthToken:  "oauth-token",
		ClientID:    "client-id",
		Model:       "codex",
		Concurrency: 7,
	}); err != nil {
		t.Fatalf("SaveGlobal() error: %v", err)
	}

	cfg, err := Load(t.TempDir(), CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.OAuthToken != "oauth-token" {
		t.Fatalf("OAuthToken = %q, want oauth-token", cfg.OAuthToken)
	}
	if cfg.ClientID != "client-id" {
		t.Fatalf("ClientID = %q, want client-id", cfg.ClientID)
	}
	if cfg.Concurrency != 7 {
		t.Fatalf("Concurrency = %d, want 7", cfg.Concurrency)
	}
}

func TestSaveGlobalRequiresCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := SaveGlobal(GlobalConfig{ClientID: "client-id"})
	if err == nil {
		t.Fatal("SaveGlobal() error = nil, want error")
	}
	if !errors.Is(err, os.ErrInvalid) && err.Error() != "oauthToken is required" {
		t.Fatalf("SaveGlobal() error = %v", err)
	}
}

func TestGlobalConfigPathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath() error: %v", err)
	}
	want := filepath.Join(home, ".codelens", "config.json")
	if path != want {
		t.Fatalf("GlobalConfigPath() = %q, want %q", path, want)
	}
}

func TestLoadRepoAndFlagsOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()

	if err := SaveGlobal(GlobalConfig{
		OAuthToken:  "oauth-token",
		ClientID:    "client-id",
		Model:       "codex",
		Concurrency: 5,
	}); err != nil {
		t.Fatalf("SaveGlobal() error: %v", err)
	}

	repoCfg := `{"model":"gpt-5.3-codex","output":"docs/CODELENS.md","exclude":["vendor"],"maxFiles":99}`
	if err := os.WriteFile(filepath.Join(repo, ".codelens.json"), []byte(repoCfg), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	cfg, err := Load(repo, CLIFlags{
		Model:       "gpt-5.2-codex",
		Concurrency: 3,
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Model != "gpt-5.2-codex" {
		t.Fatalf("Model = %q, want gpt-5.2-codex", cfg.Model)
	}
	if cfg.Output != "docs/CODELENS.md" {
		t.Fatalf("Output = %q, want docs/CODELENS.md", cfg.Output)
	}
	if cfg.MaxFiles != 99 {
		t.Fatalf("MaxFiles = %d, want 99", cfg.MaxFiles)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "vendor" {
		t.Fatalf("Exclude = %#v, want [vendor]", cfg.Exclude)
	}
	if cfg.Concurrency != 3 {
		t.Fatalf("Concurrency = %d, want 3", cfg.Concurrency)
	}
}

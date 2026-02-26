package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-haiku-4-5-20251001")
	}
	if cfg.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, 5)
	}
	if cfg.Output != "CODELENS.md" {
		t.Errorf("Output = %q, want %q", cfg.Output, "CODELENS.md")
	}
	if cfg.MaxFiles != 50 {
		t.Errorf("MaxFiles = %d, want %d", cfg.MaxFiles, 50)
	}
}

func TestLoadRepoConfig(t *testing.T) {
	dir := t.TempDir()
	repoConfig := `{"model": "gpt-4o", "provider": "openai", "output": "docs/CODELENS.md", "exclude": ["vendor", ".git"], "maxFiles": 100}`
	if err := os.WriteFile(filepath.Join(dir, ".codelens.json"), []byte(repoConfig), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
	}
	if cfg.Output != "docs/CODELENS.md" {
		t.Errorf("Output = %q, want %q", cfg.Output, "docs/CODELENS.md")
	}
	if cfg.MaxFiles != 100 {
		t.Errorf("MaxFiles = %d, want %d", cfg.MaxFiles, 100)
	}
}

func TestFlagOverride(t *testing.T) {
	dir := t.TempDir()
	repoConfig := `{"model": "gpt-4o", "provider": "openai"}`
	if err := os.WriteFile(filepath.Join(dir, ".codelens.json"), []byte(repoConfig), 0644); err != nil {
		t.Fatal(err)
	}
	flags := CLIFlags{
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
		APIKey:      "test-key",
		Concurrency: 10,
		Full:        true,
		Verbose:     true,
	}
	cfg, err := Load(dir, flags)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-sonnet-4-20250514")
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, 10)
	}
	if !cfg.Full {
		t.Error("Full = false, want true")
	}
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
}

func TestEnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "env-key-123")
	cfg, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.APIKey != "env-key-123" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "env-key-123")
	}
}

func TestFlagOverridesEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	flags := CLIFlags{APIKey: "flag-key"}
	cfg, err := Load(dir, flags)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "flag-key")
	}
}

func TestSetupInstructions(t *testing.T) {
	msg := SetupInstructions()
	if msg == "" {
		t.Error("SetupInstructions() returned empty string")
	}
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobalConfig holds settings from ~/.config/codelens/config.json.
type GlobalConfig struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	APIKey      string `json:"apiKey"`
	Concurrency int    `json:"concurrency"`
}

// RepoConfig holds per-repo settings from .codelens.json.
type RepoConfig struct {
	Model    string   `json:"model,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Output   string   `json:"output,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	MaxFiles int      `json:"maxFiles,omitempty"`
}

// Config is the merged configuration used at runtime.
type Config struct {
	Provider    string
	Model       string
	APIKey      string
	Concurrency int
	Output      string
	Exclude     []string
	MaxFiles    int
	Full        bool
	Verbose     bool
	RepoRoot    string
}

// CLIFlags holds values passed via CLI flags.
type CLIFlags struct {
	Provider    string
	Model       string
	APIKey      string
	Concurrency int
	Output      string
	Exclude     string
	MaxFiles    int
	Full        bool
	Verbose     bool
}

var defaultExcludes = []string{
	"vendor", "node_modules", ".git", "testdata", "build", "target",
}

// Load merges config from all sources: defaults -> global -> repo -> env -> flags.
func Load(repoRoot string, flags CLIFlags) (*Config, error) {
	cfg := &Config{
		Provider:    "anthropic",
		Model:       "claude-haiku-4-5-20251001",
		Concurrency: 5,
		Output:      "CODELENS.md",
		Exclude:     defaultExcludes,
		MaxFiles:    50,
		RepoRoot:    repoRoot,
	}

	// Global config
	global, err := loadGlobal()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading global config: %w", err)
	}
	if global != nil {
		if global.Provider != "" {
			cfg.Provider = global.Provider
		}
		if global.Model != "" {
			cfg.Model = global.Model
		}
		if global.APIKey != "" {
			cfg.APIKey = global.APIKey
		}
		if global.Concurrency > 0 {
			cfg.Concurrency = global.Concurrency
		}
	}

	// Repo config
	repo, err := loadRepo(repoRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading repo config: %w", err)
	}
	if repo != nil {
		if repo.Provider != "" {
			cfg.Provider = repo.Provider
		}
		if repo.Model != "" {
			cfg.Model = repo.Model
		}
		if repo.Output != "" {
			cfg.Output = repo.Output
		}
		if len(repo.Exclude) > 0 {
			cfg.Exclude = repo.Exclude
		}
		if repo.MaxFiles > 0 {
			cfg.MaxFiles = repo.MaxFiles
		}
	}

	// Env vars
	applyEnvVars(cfg)

	// CLI flags (highest precedence)
	applyFlags(cfg, flags)

	return cfg, nil
}

// HasAPIKey returns true if an API key is configured from any source.
func (c *Config) HasAPIKey() bool {
	return c.APIKey != ""
}

// SetupInstructions returns the message shown when no config is found.
func SetupInstructions() string {
	return `No configuration found. Either:

  1. Create ~/.config/codelens/config.json:
     { "provider": "anthropic", "model": "claude-haiku-4-5-20251001", "apiKey": "your-key" }

  2. Pass flags directly:
     codelens --provider anthropic --api-key sk-ant-...

  3. Set an environment variable:
     export ANTHROPIC_API_KEY=sk-ant-...

Supported providers: anthropic, openrouter, openai`
}

func loadGlobal() (*GlobalConfig, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "codelens", "config.json")
	return loadJSONFile[GlobalConfig](path)
}

func loadRepo(repoRoot string) (*RepoConfig, error) {
	path := filepath.Join(repoRoot, ".codelens.json")
	return loadJSONFile[RepoConfig](path)
}

func loadJSONFile[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &v, nil
}

func applyEnvVars(cfg *Config) {
	envKeys := map[string]string{
		"ANTHROPIC_API_KEY":   "anthropic",
		"OPENROUTER_API_KEY":  "openrouter",
		"OPENAI_API_KEY":      "openai",
	}
	for envVar, provider := range envKeys {
		if key := os.Getenv(envVar); key != "" {
			cfg.APIKey = key
			if cfg.Provider == "" || cfg.Provider == provider {
				cfg.Provider = provider
			}
			break
		}
	}
}

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.Provider != "" {
		cfg.Provider = flags.Provider
	}
	if flags.Model != "" {
		cfg.Model = flags.Model
	}
	if flags.APIKey != "" {
		cfg.APIKey = flags.APIKey
	}
	if flags.Concurrency > 0 {
		cfg.Concurrency = flags.Concurrency
	}
	if flags.Output != "" {
		cfg.Output = flags.Output
	}
	if flags.Exclude != "" {
		cfg.Exclude = strings.Split(flags.Exclude, ",")
	}
	if flags.MaxFiles > 0 {
		cfg.MaxFiles = flags.MaxFiles
	}
	cfg.Full = flags.Full
	cfg.Verbose = flags.Verbose
}

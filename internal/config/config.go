package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const missingConfigMessage = "Configuración no encontrada en ~/.codelens/config.json. Ejecutá: codelens configure"

var ErrMissingGlobalConfig = errors.New("global config not found")

// GlobalConfig holds settings from ~/.codelens/config.json.
type GlobalConfig struct {
	OAuthToken  string `json:"oauthToken"`
	ClientID    string `json:"clientId"`
	Model       string `json:"model,omitempty"`
	Concurrency int    `json:"concurrency"`
}

// RepoConfig holds per-repo settings from .codelens.json.
type RepoConfig struct {
	Model    string   `json:"model,omitempty"`
	Output   string   `json:"output,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	MaxFiles int      `json:"maxFiles,omitempty"`
}

// Config is the merged configuration used at runtime.
type Config struct {
	Model       string
	OAuthToken  string
	ClientID    string
	Concurrency int
	Output      string
	Exclude     []string
	MaxFiles    int
	Full        bool
	Verbose     bool
	Debug       bool
	RepoRoot    string
}

// CLIFlags holds values passed via CLI flags.
type CLIFlags struct {
	Model       string
	Concurrency int
	Output      string
	Exclude     string
	MaxFiles    int
	Full        bool
	Verbose     bool
	Debug       bool
}

var defaultExcludes = []string{
	"vendor", "node_modules", ".git", "testdata", "build", "target",
}

// Load merges config from all sources: defaults -> global -> repo -> env -> flags.
func Load(repoRoot string, flags CLIFlags) (*Config, error) {
	cfg := &Config{
		Model:       "codex",
		Concurrency: 5,
		Output:      "CODELENS.md",
		Exclude:     defaultExcludes,
		MaxFiles:    50,
		RepoRoot:    repoRoot,
	}

	// Global config
	global, err := loadGlobal()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMissingGlobalConfig
		}
		return nil, fmt.Errorf("reading global config: %w", err)
	}
	if global.Model != "" {
		cfg.Model = global.Model
	}
	if global.Concurrency > 0 {
		cfg.Concurrency = global.Concurrency
	}
	cfg.OAuthToken = global.OAuthToken
	cfg.ClientID = global.ClientID
	if cfg.OAuthToken == "" || cfg.ClientID == "" {
		return nil, errors.New("invalid global config: oauthToken and clientId are required")
	}

	// Repo config
	repo, err := loadRepo(repoRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading repo config: %w", err)
	}
	if repo != nil {
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

	// CLI flags (highest precedence)
	applyFlags(cfg, flags)

	return cfg, nil
}

// HasOAuthToken returns true if a token is available.
func (c *Config) HasOAuthToken() bool {
	return c.OAuthToken != ""
}

// SetupInstructions returns the message shown when no config is found.
func SetupInstructions() string {
	return missingConfigMessage
}

// IsMissingConfigError reports whether an error is caused by a missing config file.
func IsMissingConfigError(err error) bool {
	return errors.Is(err, ErrMissingGlobalConfig)
}

// GlobalConfigPath returns the full path to the global config file.
func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codelens", "config.json"), nil
}

// SaveGlobal writes ~/.codelens/config.json with restrictive permissions.
func SaveGlobal(cfg GlobalConfig) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}
	if cfg.OAuthToken == "" {
		return errors.New("oauthToken is required")
	}
	if cfg.ClientID == "" {
		return errors.New("clientId is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func loadGlobal() (*GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}
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

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.Model != "" {
		cfg.Model = flags.Model
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
	cfg.Debug = flags.Debug
}

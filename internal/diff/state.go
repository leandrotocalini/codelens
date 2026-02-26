package diff

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/leandrotocalini/codelens/internal/parser"
)

// State represents the persisted indexing state.
type State struct {
	Version    int                    `json:"version"`
	LastCommit string                 `json:"lastCommit"`
	Timestamp  time.Time              `json:"timestamp"`
	Model      string                 `json:"model"`
	Modules    map[string]ModuleState `json:"modules"`
	Summaries  map[string]string      `json:"summaries"`
}

// ModuleState stores the hash and file list for a module.
type ModuleState struct {
	Hash  string   `json:"hash"`
	Files []string `json:"files"`
}

const stateDir = ".codelens"
const stateFile = "state.json"

// LoadState reads the state from .codelens/state.json.
func LoadState(repoRoot string) (*State, error) {
	path := filepath.Join(repoRoot, stateDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}

	return &state, nil
}

// SaveState writes the state to .codelens/state.json.
func SaveState(repoRoot, commitHash, model string, modules []parser.Module, summaries map[string]string) error {
	dir := filepath.Join(repoRoot, stateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	state := State{
		Version:    1,
		LastCommit: commitHash,
		Timestamp:  time.Now().UTC(),
		Model:      model,
		Modules:    make(map[string]ModuleState),
		Summaries:  summaries,
	}

	for _, m := range modules {
		files := make([]string, 0, len(m.Files))
		for _, f := range m.Files {
			files = append(files, f.Path)
		}
		sort.Strings(files)

		state.Modules[m.Name] = ModuleState{
			Hash:  computeModuleHash(repoRoot, m),
			Files: files,
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	path := filepath.Join(dir, stateFile)
	return os.WriteFile(path, data, 0644)
}

// computeModuleHash computes a SHA256 hash of a module's file contents.
func computeModuleHash(repoRoot string, module parser.Module) string {
	h := sha256.New()

	files := make([]string, 0, len(module.Files))
	for _, f := range module.Files {
		files = append(files, f.Path)
	}
	sort.Strings(files)

	for _, path := range files {
		data, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			continue
		}
		h.Write([]byte(path))
		h.Write(data)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

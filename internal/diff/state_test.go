package diff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leandrotocalini/codelens/internal/parser"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()

	modules := []parser.Module{
		{
			Name:     "internal/handler",
			Language: "go",
			Files: []parser.File{
				{Path: "internal/handler/handler.go", Language: "go", LOC: 50},
			},
		},
		{
			Name:     "internal/model",
			Language: "go",
			Files: []parser.File{
				{Path: "internal/model/user.go", Language: "go", LOC: 30},
			},
		},
	}

	summaries := map[string]string{
		"internal/handler": "Handler summary",
		"internal/model":   "Model summary",
	}

	err := SaveState(dir, "abc123def", "claude-haiku", modules, summaries)
	if err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	// Verify file was created
	statePath := filepath.Join(dir, ".codelens", "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state.json not created")
	}

	// Load it back
	state, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if state == nil {
		t.Fatal("LoadState() returned nil")
	}

	if state.Version != 1 {
		t.Errorf("Version = %d, want 1", state.Version)
	}
	if state.LastCommit != "abc123def" {
		t.Errorf("LastCommit = %q, want %q", state.LastCommit, "abc123def")
	}
	if state.Model != "claude-haiku" {
		t.Errorf("Model = %q, want %q", state.Model, "claude-haiku")
	}
	if len(state.Modules) != 2 {
		t.Errorf("Modules count = %d, want 2", len(state.Modules))
	}
	if len(state.Summaries) != 2 {
		t.Errorf("Summaries count = %d, want 2", len(state.Summaries))
	}
	if state.Summaries["internal/handler"] != "Handler summary" {
		t.Errorf("Summary mismatch for internal/handler")
	}
}

func TestLoadStateNotExist(t *testing.T) {
	dir := t.TempDir()
	state, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if state != nil {
		t.Error("LoadState() should return nil for non-existent state")
	}
}

func TestAffectedModules(t *testing.T) {
	modules := []parser.Module{
		{
			Name: "internal/handler",
			Files: []parser.File{
				{Path: "internal/handler/handler.go"},
				{Path: "internal/handler/routes.go"},
			},
		},
		{
			Name: "internal/model",
			Files: []parser.File{
				{Path: "internal/model/user.go"},
			},
		},
		{
			Name: "internal/config",
			Files: []parser.File{
				{Path: "internal/config/config.go"},
			},
		},
	}

	changedFiles := []string{
		"internal/handler/handler.go",
		"internal/config/config.go",
	}

	affected := AffectedModules(changedFiles, modules)
	if len(affected) != 2 {
		t.Errorf("affected = %d, want 2", len(affected))
	}

	names := make(map[string]bool)
	for _, m := range affected {
		names[m.Name] = true
	}
	if !names["internal/handler"] {
		t.Error("expected internal/handler to be affected")
	}
	if !names["internal/config"] {
		t.Error("expected internal/config to be affected")
	}
	if names["internal/model"] {
		t.Error("internal/model should not be affected")
	}
}

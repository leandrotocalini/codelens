package graph

import (
	"strings"
	"testing"

	"github.com/leandrotocalini/codelens/internal/parser"
)

func TestBuild(t *testing.T) {
	modules := []parser.Module{
		{
			Name:     "cmd/app",
			Language: "go",
			Symbols: []parser.Symbol{
				{Kind: parser.SymbolImport, ImportPath: "example.com/internal/handler", Language: "go"},
				{Kind: parser.SymbolImport, ImportPath: "example.com/internal/config", Language: "go"},
			},
		},
		{
			Name:     "internal/handler",
			Language: "go",
			Symbols: []parser.Symbol{
				{Kind: parser.SymbolImport, ImportPath: "example.com/internal/model", Language: "go"},
			},
		},
		{
			Name:     "internal/model",
			Language: "go",
		},
		{
			Name:     "internal/config",
			Language: "go",
		},
	}

	g := Build(modules)

	if len(g.Modules) != 4 {
		t.Errorf("Modules count = %d, want 4", len(g.Modules))
	}

	// cmd/app depends on handler and config
	deps := g.DependenciesOf("cmd/app")
	if len(deps) != 2 {
		t.Errorf("cmd/app deps = %d, want 2", len(deps))
	}

	// internal/handler depends on model
	deps = g.DependenciesOf("internal/handler")
	if len(deps) != 1 {
		t.Errorf("internal/handler deps = %d, want 1", len(deps))
	}

	// model has no deps
	deps = g.DependenciesOf("internal/model")
	if len(deps) != 0 {
		t.Errorf("internal/model deps = %d, want 0", len(deps))
	}

	// model is depended on by handler
	dependents := g.DependentsOf("internal/model")
	if len(dependents) != 1 || dependents[0] != "internal/handler" {
		t.Errorf("internal/model dependents = %v, want [internal/handler]", dependents)
	}
}

func TestEntryPoints(t *testing.T) {
	modules := []parser.Module{
		{Name: "cmd/app", Language: "go", Symbols: []parser.Symbol{
			{Kind: parser.SymbolImport, ImportPath: "internal/handler", Language: "go"},
		}},
		{Name: "internal/handler", Language: "go"},
	}

	g := Build(modules)
	entries := g.EntryPoints()

	if len(entries) != 1 || entries[0] != "cmd/app" {
		t.Errorf("EntryPoints() = %v, want [cmd/app]", entries)
	}
}

func TestRender(t *testing.T) {
	modules := []parser.Module{
		{Name: "cmd/app", Language: "go", Symbols: []parser.Symbol{
			{Kind: parser.SymbolImport, ImportPath: "internal/handler", Language: "go"},
			{Kind: parser.SymbolImport, ImportPath: "internal/config", Language: "go"},
		}},
		{Name: "internal/handler", Language: "go", Symbols: []parser.Symbol{
			{Kind: parser.SymbolImport, ImportPath: "internal/model", Language: "go"},
		}},
		{Name: "internal/model", Language: "go"},
		{Name: "internal/config", Language: "go"},
	}

	g := Build(modules)
	output := Render(g)

	if !strings.Contains(output, "cmd/app") {
		t.Error("render output should contain cmd/app")
	}
	if !strings.Contains(output, "internal/handler") {
		t.Error("render output should contain internal/handler")
	}
	if !strings.Contains(output, "internal/model") {
		t.Error("render output should contain internal/model")
	}
	if !strings.Contains(output, "internal/config") {
		t.Error("render output should contain internal/config")
	}
	if !strings.Contains(output, "├── ") && !strings.Contains(output, "└── ") {
		t.Fatalf("render output should contain tree connectors, got:\n%s", output)
	}
}

func TestRenderCycle(t *testing.T) {
	modules := []parser.Module{
		{Name: "a", Language: "go", Symbols: []parser.Symbol{
			{Kind: parser.SymbolImport, ImportPath: "b", Language: "go"},
		}},
		{Name: "b", Language: "go", Symbols: []parser.Symbol{
			{Kind: parser.SymbolImport, ImportPath: "a", Language: "go"},
		}},
	}

	g := Build(modules)
	output := Render(g)

	if !strings.Contains(output, "(cycle)") {
		t.Error("render output should contain (cycle) for circular dependencies")
	}
}

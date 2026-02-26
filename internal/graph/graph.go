package graph

import (
	"sort"
	"strings"

	"github.com/leandrotocalini/codelens/internal/parser"
)

// Graph represents a dependency graph between modules.
type Graph struct {
	// Forward edges: module -> modules it depends on
	Dependencies map[string][]string
	// Reverse edges: module -> modules that depend on it
	Dependents map[string][]string
	// All module names
	Modules []string
}

// Build constructs a dependency graph from modules by analyzing their import symbols.
func Build(modules []parser.Module) *Graph {
	g := &Graph{
		Dependencies: make(map[string][]string),
		Dependents:   make(map[string][]string),
	}

	// Index module names for lookup
	moduleNames := make(map[string]bool)
	for _, m := range modules {
		moduleNames[m.Name] = true
		g.Modules = append(g.Modules, m.Name)
	}
	sort.Strings(g.Modules)

	// For each module, resolve its imports to other known modules
	for _, m := range modules {
		deps := make(map[string]bool)
		for _, sym := range m.Symbols {
			if sym.Kind != parser.SymbolImport {
				continue
			}
			target := resolveImport(sym.ImportPath, m.Language, moduleNames)
			if target != "" && target != m.Name {
				deps[target] = true
			}
		}

		var depList []string
		for dep := range deps {
			depList = append(depList, dep)
		}
		sort.Strings(depList)
		g.Dependencies[m.Name] = depList

		for _, dep := range depList {
			g.Dependents[dep] = append(g.Dependents[dep], m.Name)
		}
	}

	// Sort dependents
	for k := range g.Dependents {
		sort.Strings(g.Dependents[k])
	}

	return g
}

// EntryPoints returns modules that nothing depends on (typically cmd/ or main).
func (g *Graph) EntryPoints() []string {
	var entries []string
	for _, m := range g.Modules {
		if len(g.Dependents[m]) == 0 {
			entries = append(entries, m)
		}
	}
	sort.Strings(entries)
	return entries
}

// DependenciesOf returns the direct dependencies of a module.
func (g *Graph) DependenciesOf(module string) []string {
	return g.Dependencies[module]
}

// DependentsOf returns modules that depend on the given module.
func (g *Graph) DependentsOf(module string) []string {
	return g.Dependents[module]
}

// resolveImport tries to match an import path to a known module.
func resolveImport(importPath, language string, knownModules map[string]bool) string {
	// Direct match
	if knownModules[importPath] {
		return importPath
	}

	switch language {
	case "go":
		// For Go, try to match the last path segments
		for mod := range knownModules {
			if strings.HasSuffix(importPath, "/"+mod) || strings.HasSuffix(importPath, mod) {
				return mod
			}
		}
	case "java":
		// For Java, imports are like com.example.auth.AuthService
		// Try matching package prefix
		parts := strings.Split(importPath, ".")
		for i := len(parts) - 1; i > 0; i-- {
			candidate := strings.Join(parts[:i], ".")
			if knownModules[candidate] {
				return candidate
			}
		}
	case "typescript", "javascript":
		// For TS/JS, try relative path resolution
		// Remove leading ./ or ../
		clean := importPath
		clean = strings.TrimPrefix(clean, "./")
		clean = strings.TrimPrefix(clean, "../")
		for mod := range knownModules {
			if strings.HasSuffix(clean, mod) || strings.HasPrefix(mod, clean) {
				return mod
			}
		}
	case "python":
		// For Python, try matching package path
		dotPath := strings.ReplaceAll(importPath, ".", "/")
		for mod := range knownModules {
			if mod == dotPath || strings.HasPrefix(dotPath, mod+"/") {
				return mod
			}
		}
	}

	return ""
}

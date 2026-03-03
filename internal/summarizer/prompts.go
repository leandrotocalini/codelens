package summarizer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/leandrotocalini/codelens/internal/parser"
)

// ModulePrompt builds the LLM prompt for summarizing a single module.
func ModulePrompt(module parser.Module) string {
	var sb strings.Builder
	runtimeFiles, testFiles := splitModuleFiles(module.Files)
	isTestHeavy := len(runtimeFiles) == 0 && len(testFiles) > 0

	if isTestHeavy {
		sb.WriteString("You are a code analyst. Summarize this TEST module with focus on coverage intent and risk.\n")
		sb.WriteString("Be concrete and concise. Do not describe generic testing philosophy.\n\n")
	} else {
		sb.WriteString("You are a code analyst. Summarize this RUNTIME module for future code changes.\n")
		sb.WriteString("Prioritize real execution behavior, contracts, and impact of modifications.\n\n")
	}

	fmt.Fprintf(&sb, "Module: %s\n", module.Name)
	fmt.Fprintf(&sb, "Language: %s\n", module.Language)
	if isTestHeavy {
		sb.WriteString("ModuleClass: test\n")
	} else {
		sb.WriteString("ModuleClass: runtime\n")
	}

	// File list
	sb.WriteString("Files: ")
	selectedFiles := runtimeFiles
	if isTestHeavy {
		selectedFiles = testFiles
	}
	fileNames := make([]string, 0, len(selectedFiles))
	for _, f := range selectedFiles {
		fileNames = append(fileNames, f.Path)
	}
	if len(fileNames) == 0 {
		for _, f := range module.Files {
			fileNames = append(fileNames, f.Path)
		}
	}
	sb.WriteString(strings.Join(fileNames, ", "))
	sb.WriteString("\n")

	// Filter symbols for high-signal context.
	sb.WriteString("Symbols:\n---\n")
	count := 0
	for _, sym := range relevantSymbols(selectedFiles, module.Symbols, isTestHeavy) {
		if !isTestHeavy && !sym.Exported && !isRuntimeCriticalSymbol(sym.Name) {
			continue
		}
		if sym.Kind == parser.SymbolImport {
			continue
		}
		if !isTestHeavy && isTestSymbol(sym.Name) {
			continue
		}
		if isTestHeavy && !isTestSymbol(sym.Name) && !sym.Exported {
			continue
		}
		signature := strings.TrimSpace(sym.Signature)
		if signature != "" {
			sb.WriteString(signature)
		} else {
			fmt.Fprintf(&sb, "%s %s", sym.Kind, sym.Name)
		}
		sb.WriteString("\n")
		count++
	}
	if count == 0 {
		sb.WriteString("(no exported symbols)\n")
	}
	sb.WriteString("---\n\n")

	if isTestHeavy {
		sb.WriteString("Respond with exactly this format:\n")
		sb.WriteString("**Responsibility**: <what behavior these tests validate>\n")
		sb.WriteString("**Key types**: <important fixtures/mocks/types or 'none'>\n")
		sb.WriteString("**Key functions**: <main test functions>\n")
		sb.WriteString("**Change impact**: <what runtime areas are risky if changed>\n")
	} else {
		sb.WriteString("Respond with exactly this format:\n")
		sb.WriteString("**Responsibility**: <runtime responsibility, no fluff>\n")
		sb.WriteString("**Key types**: <runtime types/interfaces, comma-separated>\n")
		sb.WriteString("**Key functions**: <runtime entrypoints/constructors/handlers>\n")
		sb.WriteString("**Change impact**: <what can break if this module changes>\n")
	}

	return sb.String()
}

// ProjectPrompt builds the LLM prompt for the project-level summary.
func ProjectPrompt(modules []parser.Module, summaries map[string]string, depGraph string) string {
	var sb strings.Builder

	sb.WriteString("You are preparing a high-signal engineering brief for maintainers.\n")
	sb.WriteString("Focus on what matters to change code safely in any project.\n")
	sb.WriteString("Prefer concrete contracts, execution flow, and change risk over generic descriptions.\n\n")

	sb.WriteString("Module Summaries:\n")

	// Sort module names for deterministic output
	names := make([]string, 0, len(summaries))
	for name := range summaries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(&sb, "\n### %s\n%s\n", name, summaries[name])
	}

	if depGraph != "" {
		sb.WriteString("\nDependency Graph:\n```\n")
		sb.WriteString(depGraph)
		sb.WriteString("```\n")
	}

	sb.WriteString("\nRespond with EXACTLY these markdown sections and no duplicates:\n")
	sb.WriteString("### Project Summary\n")
	sb.WriteString("### Libraries & Frameworks\n")
	sb.WriteString("### External Dependencies\n")
	sb.WriteString("### Code Architecture\n")
	sb.WriteString("### Critical Paths & Change Impact\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Mention auth/provider/config/env/API/file contracts when present.\n")
	sb.WriteString("- Keep tests as secondary signal unless they define critical behavior.\n")
	sb.WriteString("- Do not repeat paragraphs.\n")

	return sb.String()
}

// FormatModuleSymbols formats module symbols for the output, respecting maxFiles.
func FormatModuleSymbols(module parser.Module, maxFiles int) string {
	files := module.Files
	truncated := 0
	if len(files) > maxFiles {
		// Sort by LOC descending, take top N
		sort.Slice(files, func(i, j int) bool {
			return files[i].LOC > files[j].LOC
		})
		truncated = len(files) - maxFiles
		files = files[:maxFiles]
	}

	var sb strings.Builder
	for _, f := range files {
		for _, sym := range f.Symbols {
			if !sym.Exported || sym.Kind == parser.SymbolImport {
				continue
			}
			if sym.Signature != "" {
				sb.WriteString(sym.Signature)
			} else {
				fmt.Fprintf(&sb, "%s %s", sym.Kind, sym.Name)
			}
			sb.WriteString("\n")
		}
	}

	if truncated > 0 {
		fmt.Fprintf(&sb, "\n... and %d more files\n", truncated)
	}

	return sb.String()
}

func splitModuleFiles(files []parser.File) (runtimeFiles []parser.File, testFiles []parser.File) {
	for _, f := range files {
		if isTestPath(f.Path) {
			testFiles = append(testFiles, f)
			continue
		}
		runtimeFiles = append(runtimeFiles, f)
	}
	return runtimeFiles, testFiles
}

func relevantSymbols(files []parser.File, fallback []parser.Symbol, isTestHeavy bool) []parser.Symbol {
	if len(files) == 0 {
		return fallback
	}
	var out []parser.Symbol
	for _, f := range files {
		for _, sym := range f.Symbols {
			if isTestHeavy {
				out = append(out, sym)
				continue
			}
			if isTestSymbol(sym.Name) {
				continue
			}
			out = append(out, sym)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func isRuntimeCriticalSymbol(name string) bool {
	switch name {
	case "main", "init", "Run", "Execute", "Start", "Handle", "ServeHTTP":
		return true
	default:
		return false
	}
}

func isTestSymbol(name string) bool {
	return strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Example") ||
		strings.HasPrefix(name, "Fuzz")
}

func isTestPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_spec.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_spec.py") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, "test.java") ||
		strings.HasSuffix(base, "tests.swift") {
		return true
	}

	segments := strings.Split(lower, "/")
	for _, seg := range segments {
		switch seg {
		case "test", "tests", "__tests__", "spec", "specs", "mocks", "fixtures":
			return true
		}
	}
	return false
}

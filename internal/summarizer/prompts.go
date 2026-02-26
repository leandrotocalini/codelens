package summarizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leandrotocalini/codelens/internal/parser"
)

// ModulePrompt builds the LLM prompt for summarizing a single module.
func ModulePrompt(module parser.Module) string {
	var sb strings.Builder

	sb.WriteString("You are a code analyst. Given a module's source symbols, ")
	sb.WriteString("write a concise summary. Be specific — use actual type and function ")
	sb.WriteString("names. 3-5 sentences max.\n\n")

	fmt.Fprintf(&sb, "Module: %s\n", module.Name)
	fmt.Fprintf(&sb, "Language: %s\n", module.Language)

	// File list
	sb.WriteString("Files: ")
	fileNames := make([]string, 0, len(module.Files))
	for _, f := range module.Files {
		fileNames = append(fileNames, f.Path)
	}
	sb.WriteString(strings.Join(fileNames, ", "))
	sb.WriteString("\n")

	// Exported symbols only
	sb.WriteString("Symbols:\n---\n")
	count := 0
	for _, sym := range module.Symbols {
		if !sym.Exported {
			continue
		}
		if sym.Kind == parser.SymbolImport {
			continue
		}
		if sym.Signature != "" {
			sb.WriteString(sym.Signature)
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

	sb.WriteString("Respond with exactly this format:\n")
	sb.WriteString("**Responsibility**: <2-3 sentences>\n")
	sb.WriteString("**Key types**: <comma-separated>\n")
	sb.WriteString("**Key functions**: <comma-separated>\n")

	return sb.String()
}

// ProjectPrompt builds the LLM prompt for the project-level summary.
func ProjectPrompt(modules []parser.Module, summaries map[string]string, depGraph string) string {
	var sb strings.Builder

	sb.WriteString("You are a code analyst. Given module summaries and a dependency graph, ")
	sb.WriteString("write a 2-3 paragraph executive summary of the project. ")
	sb.WriteString("Describe what it does, its architecture, and main technologies.\n\n")

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

	sb.WriteString("\nRespond with a 2-3 paragraph summary. No headers or formatting beyond paragraphs.\n")

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

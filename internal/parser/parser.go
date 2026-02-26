package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Language extension mapping.
var extensionToLanguage = map[string]string{
	".go":   "go",
	".java": "java",
	".ts":   "typescript",
	".tsx":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".py":   "python",
	".rs":   "rust",
}

// Parse walks the repository, parses files, resolves modules, and returns
// the list of modules and aggregate stats.
func Parse(repoRoot string, excludes []string) ([]Module, Stats, error) {
	files, err := walkFiles(repoRoot, excludes)
	if err != nil {
		return nil, Stats{}, err
	}

	// Extract symbols from each file
	for i := range files {
		symbols, err := extractSymbols(files[i].Path, files[i].Language)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}
		files[i].Symbols = symbols
	}

	// Resolve modules
	modules := ResolveModules(repoRoot, files)

	// Compute stats
	stats := computeStats(modules)

	return modules, stats, nil
}

func walkFiles(repoRoot string, excludes []string) ([]File, error) {
	var files []File

	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return nil
		}

		// Check exclude patterns
		if shouldExclude(rel, info.IsDir(), excludes) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		lang, ok := extensionToLanguage[ext]
		if !ok {
			return nil // skip unsupported languages
		}

		loc, err := countLOC(path)
		if err != nil {
			return nil
		}

		files = append(files, File{
			Path:     rel,
			Language: lang,
			LOC:      loc,
		})

		return nil
	})

	return files, err
}

func shouldExclude(relPath string, isDir bool, excludes []string) bool {
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, part := range parts {
		for _, excl := range excludes {
			if matched, _ := filepath.Match(excl, part); matched {
				return true
			}
		}
	}

	// Also check full path match for glob patterns
	for _, excl := range excludes {
		if matched, _ := filepath.Match(excl, relPath); matched {
			return true
		}
		if matched, _ := filepath.Match(excl, filepath.Base(relPath)); matched {
			return true
		}
	}
	return false
}

func countLOC(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			count++
		}
	}
	return count, scanner.Err()
}

func computeStats(modules []Module) Stats {
	stats := Stats{
		LanguageBreakdown: make(map[string]int),
	}
	stats.TotalPackages = len(modules)

	for _, m := range modules {
		for _, f := range m.Files {
			stats.TotalFiles++
			stats.TotalLOC += f.LOC
			stats.LanguageBreakdown[f.Language] += f.LOC
		}
		for _, s := range m.Symbols {
			if !s.Exported {
				continue
			}
			switch s.Kind {
			case SymbolFunction, SymbolMethod:
				stats.ExportedFunctions++
			case SymbolType, SymbolInterface:
				stats.ExportedTypes++
			}
		}
	}

	return stats
}

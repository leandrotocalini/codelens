package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var javaPackageRe = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)

// mavenTestPrefix and mavenMainPrefix for stripping standard Maven/Gradle paths.
var (
	mavenMainPrefixes = []string{"src/main/java/", "app/src/main/java/"}
	mavenTestPrefixes = []string{"src/test/java/", "app/src/test/java/"}
)

// ResolveModules groups files into modules based on language-specific strategies.
func ResolveModules(repoRoot string, files []File) []Module {
	byLang := make(map[string][]File)
	for _, f := range files {
		byLang[f.Language] = append(byLang[f.Language], f)
	}

	var modules []Module

	for lang, langFiles := range byLang {
		switch lang {
		case "java":
			modules = append(modules, resolveJavaModules(repoRoot, langFiles)...)
		case "python":
			modules = append(modules, resolvePythonModules(repoRoot, langFiles)...)
		case "swift":
			modules = append(modules, resolveSwiftModules(repoRoot, langFiles)...)
		default:
			// Go, TypeScript, JavaScript, Rust: group by directory
			modules = append(modules, resolveDirectoryModules(langFiles)...)
		}
	}

	// Sort modules by name
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	// Aggregate symbols
	for i := range modules {
		var allSymbols []Symbol
		for _, f := range modules[i].Files {
			allSymbols = append(allSymbols, f.Symbols...)
		}
		modules[i].Symbols = allSymbols
	}

	return modules
}

// resolveDirectoryModules groups files by their directory path.
func resolveDirectoryModules(files []File) []Module {
	groups := make(map[string][]File)
	langs := make(map[string]string)

	for _, f := range files {
		dir := filepath.Dir(f.Path)
		groups[dir] = append(groups[dir], f)
		langs[dir] = f.Language
	}

	var modules []Module
	for dir, dirFiles := range groups {
		modules = append(modules, Module{
			Name:     dir,
			Language: langs[dir],
			Files:    dirFiles,
		})
	}
	return modules
}

// resolveJavaModules groups Java files by their package declaration.
func resolveJavaModules(repoRoot string, files []File) []Module {
	groups := make(map[string][]File)

	for _, f := range files {
		pkg := detectJavaPackage(filepath.Join(repoRoot, f.Path))
		if pkg == "" {
			// Fall back to directory-based grouping
			pkg = javaPathToPackage(f.Path)
		}

		// Associate test files with main module
		if isJavaTestFile(f.Path) {
			// Just use the package name - tests merge with main
			groups[pkg] = append(groups[pkg], f)
		} else {
			groups[pkg] = append(groups[pkg], f)
		}
	}

	var modules []Module
	for pkg, pkgFiles := range groups {
		modules = append(modules, Module{
			Name:     pkg,
			Language: "java",
			Files:    pkgFiles,
		})
	}
	return modules
}

// detectJavaPackage reads the package declaration from a Java file.
func detectJavaPackage(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		line := scanner.Text()
		if matches := javaPackageRe.FindStringSubmatch(line); matches != nil {
			return matches[1]
		}
	}
	return ""
}

// javaPathToPackage converts a file path to a Java-style package name.
func javaPathToPackage(path string) string {
	// Strip Maven/Gradle prefixes
	for _, prefix := range mavenMainPrefixes {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}
	for _, prefix := range mavenTestPrefixes {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}

	dir := filepath.Dir(path)
	return strings.ReplaceAll(dir, string(filepath.Separator), ".")
}

func isJavaTestFile(path string) bool {
	for _, prefix := range mavenTestPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// resolveSwiftModules groups Swift files into modules.
// Swift projects typically use SPM (Sources/<Target>/) or Xcode project structure.
// Falls back to directory-based grouping.
func resolveSwiftModules(repoRoot string, files []File) []Module {
	groups := make(map[string][]File)

	for _, f := range files {
		moduleName := resolveSwiftModuleName(f.Path)
		groups[moduleName] = append(groups[moduleName], f)
	}

	var modules []Module
	for name, modFiles := range groups {
		modules = append(modules, Module{
			Name:     name,
			Language: "swift",
			Files:    modFiles,
		})
	}
	return modules
}

// resolveSwiftModuleName determines the module name for a Swift file.
// Recognizes SPM layout (Sources/<Target>/), Xcode patterns, and falls back to directory.
func resolveSwiftModuleName(path string) string {
	parts := strings.Split(path, string(filepath.Separator))

	// SPM layout: Sources/<TargetName>/...
	for i, part := range parts {
		if part == "Sources" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// Tests layout: Tests/<TargetName>/...
	for i, part := range parts {
		if part == "Tests" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// Fall back to directory
	return filepath.Dir(path)
}

// resolvePythonModules groups Python files by package (directory with __init__.py)
// or as standalone modules.
func resolvePythonModules(repoRoot string, files []File) []Module {
	// Build set of directories that have __init__.py
	packageDirs := make(map[string]bool)
	for _, f := range files {
		if filepath.Base(f.Path) == "__init__.py" {
			packageDirs[filepath.Dir(f.Path)] = true
		}
	}

	groups := make(map[string][]File)
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		if packageDirs[dir] {
			// Part of a package
			groups[dir] = append(groups[dir], f)
		} else {
			// Standalone module - check parent dirs
			found := false
			current := dir
			for current != "." && current != "" {
				if packageDirs[current] {
					groups[current] = append(groups[current], f)
					found = true
					break
				}
				current = filepath.Dir(current)
			}
			if !found {
				// Standalone file as its own module
				groups[f.Path] = append(groups[f.Path], f)
			}
		}
	}

	var modules []Module
	for name, modFiles := range groups {
		modules = append(modules, Module{
			Name:     name,
			Language: "python",
			Files:    modFiles,
		})
	}
	return modules
}

package parser

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// Regex patterns for symbol extraction per language.
var (
	// Go
	goFuncRe      = regexp.MustCompile(`^func\s+(\w+)\s*\(`)
	goMethodRe    = regexp.MustCompile(`^func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)\s*\(`)
	goTypeRe      = regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface)\s*\{`)
	goConstRe     = regexp.MustCompile(`^\s*(\w+)\s*=`)
	goImportRe    = regexp.MustCompile(`^\s*"([^"]+)"`)
	goImportOneRe = regexp.MustCompile(`^import\s+"([^"]+)"`)

	// Java
	javaClassRe     = regexp.MustCompile(`^(?:public\s+)?(?:abstract\s+)?(?:final\s+)?(?:class|interface|enum|record)\s+(\w+)`)
	javaMethodRe    = regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(?:static\s+)?(?:final\s+)?(?:abstract\s+)?(?:synchronized\s+)?(?:\w+(?:<[^>]*>)?(?:\[\])*)\s+(\w+)\s*\(([^)]*)\)`)
	javaImportRe    = regexp.MustCompile(`^import\s+(?:static\s+)?([\w.]+);`)
	javaFieldRe     = regexp.MustCompile(`^\s*(?:public|protected|private)\s+(?:static\s+)?(?:final\s+)?(\w+(?:<[^>]*>)?(?:\[\])*)\s+(\w+)\s*[=;]`)

	// TypeScript/JavaScript
	tsFuncRe     = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	tsClassRe    = regexp.MustCompile(`^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	tsIfaceRe    = regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`)
	tsTypeRe     = regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)`)
	tsConstRe    = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)`)
	tsArrowRe    = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`)
	tsImportRe   = regexp.MustCompile(`^import\s+.*?from\s+['"]([^'"]+)['"]`)

	// Python
	pyFuncRe   = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)`)
	pyClassRe  = regexp.MustCompile(`^class\s+(\w+)`)
	pyImportRe = regexp.MustCompile(`^(?:from\s+([\w.]+)\s+)?import\s+(.+)`)
)

func extractSymbols(path, language string) ([]Symbol, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var symbols []Symbol
	scanner := bufio.NewScanner(f)
	lineNum := 0
	inImportBlock := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		switch language {
		case "go":
			syms := extractGoSymbols(trimmed, line, lineNum, &inImportBlock)
			symbols = append(symbols, syms...)
		case "java":
			syms := extractJavaSymbols(trimmed, line, lineNum)
			symbols = append(symbols, syms...)
		case "typescript", "javascript":
			syms := extractTSSymbols(trimmed, line, lineNum)
			symbols = append(symbols, syms...)
		case "python":
			syms := extractPythonSymbols(trimmed, line, lineNum)
			symbols = append(symbols, syms...)
		}
	}

	return symbols, scanner.Err()
}

func extractGoSymbols(trimmed, line string, lineNum int, inImportBlock *bool) []Symbol {
	var symbols []Symbol

	// Track import blocks
	if strings.HasPrefix(trimmed, "import (") {
		*inImportBlock = true
		return nil
	}
	if *inImportBlock {
		if trimmed == ")" {
			*inImportBlock = false
			return nil
		}
		if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
			symbols = append(symbols, Symbol{
				Name:       m[1],
				Kind:       SymbolImport,
				ImportPath: m[1],
				Line:       lineNum,
				Language:   "go",
			})
		}
		return symbols
	}

	// Single-line import
	if m := goImportOneRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:       m[1],
			Kind:       SymbolImport,
			ImportPath: m[1],
			Line:       lineNum,
			Language:   "go",
		})
		return symbols
	}

	// Method
	if m := goMethodRe.FindStringSubmatch(trimmed); m != nil {
		name := m[3]
		receiver := m[2]
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolMethod,
			Exported:  isGoExported(name),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "go",
			Receiver:  receiver,
		})
		return symbols
	}

	// Function
	if m := goFuncRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolFunction,
			Exported:  isGoExported(name),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "go",
		})
		return symbols
	}

	// Type
	if m := goTypeRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		kind := SymbolType
		if m[2] == "interface" {
			kind = SymbolInterface
		}
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      kind,
			Exported:  isGoExported(name),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "go",
		})
		return symbols
	}

	return symbols
}

func extractJavaSymbols(trimmed, line string, lineNum int) []Symbol {
	var symbols []Symbol

	// Import
	if m := javaImportRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:       m[1],
			Kind:       SymbolImport,
			ImportPath: m[1],
			Line:       lineNum,
			Language:   "java",
		})
		return symbols
	}

	// Class/Interface/Enum
	if m := javaClassRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		kind := SymbolType
		if strings.Contains(trimmed, "interface ") {
			kind = SymbolInterface
		}
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      kind,
			Exported:  strings.HasPrefix(trimmed, "public"),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "java",
		})
		return symbols
	}

	// Method
	if m := javaMethodRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		// Skip constructors and common keywords that match the regex
		if name == "if" || name == "for" || name == "while" || name == "switch" || name == "catch" || name == "return" {
			return nil
		}
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolMethod,
			Exported:  strings.Contains(trimmed, "public"),
			Signature: strings.TrimSpace(trimmed),
			Line:      lineNum,
			Language:  "java",
		})
		return symbols
	}

	// Field
	if m := javaFieldRe.FindStringSubmatch(trimmed); m != nil {
		name := m[2]
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolVariable,
			Exported:  strings.Contains(trimmed, "public"),
			Signature: strings.TrimSpace(trimmed),
			Line:      lineNum,
			Language:  "java",
		})
	}

	return symbols
}

func extractTSSymbols(trimmed, line string, lineNum int) []Symbol {
	var symbols []Symbol

	// Import
	if m := tsImportRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:       m[1],
			Kind:       SymbolImport,
			ImportPath: m[1],
			Line:       lineNum,
			Language:   "typescript",
		})
		return symbols
	}

	exported := strings.HasPrefix(trimmed, "export ")

	// Class
	if m := tsClassRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:      m[1],
			Kind:      SymbolType,
			Exported:  exported,
			Signature: trimmed,
			Line:      lineNum,
			Language:  "typescript",
		})
		return symbols
	}

	// Interface
	if m := tsIfaceRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:      m[1],
			Kind:      SymbolInterface,
			Exported:  exported,
			Signature: trimmed,
			Line:      lineNum,
			Language:  "typescript",
		})
		return symbols
	}

	// Type alias
	if m := tsTypeRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:      m[1],
			Kind:      SymbolType,
			Exported:  exported,
			Signature: trimmed,
			Line:      lineNum,
			Language:  "typescript",
		})
		return symbols
	}

	// Arrow function
	if m := tsArrowRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:      m[1],
			Kind:      SymbolFunction,
			Exported:  exported,
			Signature: trimmed,
			Line:      lineNum,
			Language:  "typescript",
		})
		return symbols
	}

	// Function
	if m := tsFuncRe.FindStringSubmatch(trimmed); m != nil {
		symbols = append(symbols, Symbol{
			Name:      m[1],
			Kind:      SymbolFunction,
			Exported:  exported,
			Signature: trimmed,
			Line:      lineNum,
			Language:  "typescript",
		})
		return symbols
	}

	// const/let/var (but not arrow functions, handled above)
	if m := tsConstRe.FindStringSubmatch(trimmed); m != nil {
		if !strings.Contains(trimmed, "(") {
			symbols = append(symbols, Symbol{
				Name:      m[1],
				Kind:      SymbolVariable,
				Exported:  exported,
				Signature: trimmed,
				Line:      lineNum,
				Language:  "typescript",
			})
		}
	}

	return symbols
}

func extractPythonSymbols(trimmed, line string, lineNum int) []Symbol {
	var symbols []Symbol

	// Import
	if m := pyImportRe.FindStringSubmatch(trimmed); m != nil {
		importPath := m[1]
		if importPath == "" {
			importPath = m[2]
		}
		symbols = append(symbols, Symbol{
			Name:       strings.TrimSpace(importPath),
			Kind:       SymbolImport,
			ImportPath: strings.TrimSpace(importPath),
			Line:       lineNum,
			Language:   "python",
		})
		return symbols
	}

	// Class
	if m := pyClassRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolType,
			Exported:  !strings.HasPrefix(name, "_"),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "python",
		})
		return symbols
	}

	// Function/method
	if m := pyFuncRe.FindStringSubmatch(trimmed); m != nil {
		name := m[1]
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      SymbolFunction,
			Exported:  !strings.HasPrefix(name, "_"),
			Signature: trimmed,
			Line:      lineNum,
			Language:  "python",
		})
		return symbols
	}

	return symbols
}

func isGoExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

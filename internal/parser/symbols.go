package parser

// SymbolKind represents the kind of a code symbol.
type SymbolKind int

const (
	SymbolFunction  SymbolKind = iota
	SymbolType
	SymbolMethod
	SymbolImport
	SymbolConstant
	SymbolInterface
	SymbolVariable
)

func (k SymbolKind) String() string {
	switch k {
	case SymbolFunction:
		return "function"
	case SymbolType:
		return "type"
	case SymbolMethod:
		return "method"
	case SymbolImport:
		return "import"
	case SymbolConstant:
		return "constant"
	case SymbolInterface:
		return "interface"
	case SymbolVariable:
		return "variable"
	default:
		return "unknown"
	}
}

// Symbol represents a parsed code symbol (function, type, import, etc.).
type Symbol struct {
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	Exported   bool       `json:"exported"`
	Signature  string     `json:"signature"`
	Line       int        `json:"line"`
	Language   string     `json:"language"`
	Receiver   string     `json:"receiver,omitempty"`   // For methods
	ImportPath string     `json:"importPath,omitempty"` // For imports
}

// File represents a parsed source file.
type File struct {
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Symbols  []Symbol `json:"symbols"`
	LOC      int      `json:"loc"`
}

// Module represents a logical grouping of files (package, module, etc.).
type Module struct {
	Name     string   `json:"name"`
	Language string   `json:"language"`
	Files    []File   `json:"files"`
	Symbols  []Symbol `json:"symbols"` // Aggregated from files
}

// Stats holds aggregate statistics for the parsed codebase.
type Stats struct {
	TotalFiles         int            `json:"totalFiles"`
	TotalLOC           int            `json:"totalLOC"`
	TotalPackages      int            `json:"totalPackages"`
	ExportedFunctions  int            `json:"exportedFunctions"`
	ExportedTypes      int            `json:"exportedTypes"`
	LanguageBreakdown  map[string]int `json:"languageBreakdown"` // language -> LOC
}

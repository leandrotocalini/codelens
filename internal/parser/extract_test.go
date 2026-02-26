package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractGoSymbols(t *testing.T) {
	path := filepath.Join(testdataDir(t), "go-project", "internal", "handler", "handler.go")
	symbols, err := extractSymbols(path, "go")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	// Check exported symbols
	exportedFuncs := 0
	exportedTypes := 0
	unexportedFuncs := 0
	imports := 0
	for _, s := range symbols {
		switch {
		case s.Kind == SymbolImport:
			imports++
		case s.Exported && (s.Kind == SymbolFunction || s.Kind == SymbolMethod):
			exportedFuncs++
		case !s.Exported && (s.Kind == SymbolFunction || s.Kind == SymbolMethod):
			unexportedFuncs++
		case s.Exported && s.Kind == SymbolType:
			exportedTypes++
		}
	}

	if exportedTypes == 0 {
		t.Error("expected at least 1 exported type (Handler)")
	}
	if exportedFuncs == 0 {
		t.Error("expected at least 1 exported function (New)")
	}
	if imports == 0 {
		t.Error("expected at least 1 import")
	}
}

func TestExtractJavaSymbols(t *testing.T) {
	path := filepath.Join(testdataDir(t), "java-project", "src", "main", "java", "com", "example", "auth", "AuthService.java")
	symbols, err := extractSymbols(path, "java")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	// Check for class and methods
	hasClass := false
	hasMethods := false
	hasImports := false
	for _, s := range symbols {
		if s.Kind == SymbolType && s.Name == "AuthService" {
			hasClass = true
		}
		if s.Kind == SymbolMethod {
			hasMethods = true
		}
		if s.Kind == SymbolImport {
			hasImports = true
		}
	}

	if !hasClass {
		t.Error("expected AuthService class")
	}
	if !hasMethods {
		t.Error("expected methods")
	}
	if !hasImports {
		t.Error("expected imports")
	}
}

func TestExtractTSSymbols(t *testing.T) {
	path := filepath.Join(testdataDir(t), "ts-project", "src", "components", "Button.tsx")
	symbols, err := extractSymbols(path, "typescript")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	hasInterface := false
	hasFunction := false
	hasImport := false
	for _, s := range symbols {
		if s.Kind == SymbolInterface && s.Name == "ButtonOptions" {
			hasInterface = true
		}
		if s.Kind == SymbolFunction && s.Name == "Button" {
			hasFunction = true
		}
		if s.Kind == SymbolImport {
			hasImport = true
		}
	}

	if !hasInterface {
		t.Error("expected ButtonOptions interface")
	}
	if !hasFunction {
		t.Error("expected Button function")
	}
	if !hasImport {
		t.Error("expected imports")
	}
}

func TestExtractPythonSymbols(t *testing.T) {
	path := filepath.Join(testdataDir(t), "python-project", "myapp", "app.py")
	symbols, err := extractSymbols(path, "python")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	hasClass := false
	hasFunc := false
	hasPrivate := false
	for _, s := range symbols {
		if s.Kind == SymbolType && s.Name == "Application" {
			hasClass = true
		}
		if s.Kind == SymbolFunction && s.Name == "create_app" {
			hasFunc = true
		}
		if s.Name == "_internal_setup" && !s.Exported {
			hasPrivate = true
		}
	}

	if !hasClass {
		t.Error("expected Application class")
	}
	if !hasFunc {
		t.Error("expected create_app function")
	}
	if !hasPrivate {
		t.Error("expected _internal_setup as non-exported")
	}
}

func TestIsGoExported(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Handler", true},
		{"handler", false},
		{"New", true},
		{"new", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGoExported(tt.name)
			if got != tt.want {
				t.Errorf("isGoExported(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestExtractFromTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	content := `package test

import "fmt"

type Config struct {
	Name string
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) Validate() error {
	return nil
}

func helperFunc() {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	symbols, err := extractSymbols(path, "go")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	var exported, unexported int
	for _, s := range symbols {
		if s.Kind == SymbolImport {
			continue
		}
		if s.Exported {
			exported++
		} else {
			unexported++
		}
	}

	if exported != 3 { // Config, NewConfig, Validate
		t.Errorf("exported = %d, want 3", exported)
	}
	if unexported != 1 { // helperFunc
		t.Errorf("unexported = %d, want 1", unexported)
	}
}

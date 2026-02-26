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

func TestExtractSwiftSymbols(t *testing.T) {
	path := filepath.Join(testdataDir(t), "swift-project", "Sources", "MyApp", "App.swift")
	symbols, err := extractSymbols(path, "swift")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("no symbols extracted")
	}

	var (
		hasClass    bool
		hasStruct   bool
		hasEnum     bool
		hasProtocol bool
		hasFunc     bool
		hasInit     bool
		hasImport   bool
		hasPrivate  bool
		hasTypealias bool
	)
	for _, s := range symbols {
		switch {
		case s.Kind == SymbolType && s.Name == "Application":
			hasClass = true
		case s.Kind == SymbolType && s.Name == "AppConfig":
			hasStruct = true
		case s.Kind == SymbolType && s.Name == "Environment":
			hasEnum = true
		case s.Kind == SymbolInterface && s.Name == "AppDelegate":
			hasProtocol = true
		case s.Kind == SymbolFunction && s.Name == "createApp":
			hasFunc = true
		case s.Kind == SymbolMethod && s.Name == "init":
			hasInit = true
		case s.Kind == SymbolImport:
			hasImport = true
		case s.Kind == SymbolFunction && s.Name == "cleanup" && !s.Exported:
			hasPrivate = true
		case s.Kind == SymbolType && s.Name == "CompletionHandler":
			hasTypealias = true
		}
	}

	if !hasClass {
		t.Error("expected Application class")
	}
	if !hasStruct {
		t.Error("expected AppConfig struct")
	}
	if !hasEnum {
		t.Error("expected Environment enum")
	}
	if !hasProtocol {
		t.Error("expected AppDelegate protocol")
	}
	if !hasFunc {
		t.Error("expected createApp function")
	}
	if !hasInit {
		t.Error("expected init method")
	}
	if !hasImport {
		t.Error("expected imports")
	}
	if !hasPrivate {
		t.Error("expected private cleanup function")
	}
	if !hasTypealias {
		t.Error("expected CompletionHandler typealias")
	}
}

func TestSwiftVisibility(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.swift")
	content := `import UIKit

public class PublicClass {}
open class OpenClass {}
class InternalClass {}
private class PrivateClass {}
fileprivate class FilePrivateClass {}

public func publicFunc() {}
func internalFunc() {}
private func privateFunc() {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	symbols, err := extractSymbols(path, "swift")
	if err != nil {
		t.Fatalf("extractSymbols() error: %v", err)
	}

	visibility := make(map[string]bool) // name -> exported
	for _, s := range symbols {
		if s.Kind != SymbolImport {
			visibility[s.Name] = s.Exported
		}
	}

	// public, open, internal (default) should be exported
	if !visibility["PublicClass"] {
		t.Error("PublicClass should be exported")
	}
	if !visibility["OpenClass"] {
		t.Error("OpenClass should be exported")
	}
	if !visibility["InternalClass"] {
		t.Error("InternalClass should be exported (internal = module-visible)")
	}
	// private, fileprivate should NOT be exported
	if visibility["PrivateClass"] {
		t.Error("PrivateClass should not be exported")
	}
	if visibility["FilePrivateClass"] {
		t.Error("FilePrivateClass should not be exported")
	}
	if !visibility["publicFunc"] {
		t.Error("publicFunc should be exported")
	}
	if !visibility["internalFunc"] {
		t.Error("internalFunc should be exported (internal = default)")
	}
	if visibility["privateFunc"] {
		t.Error("privateFunc should not be exported")
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

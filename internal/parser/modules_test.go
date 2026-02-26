package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirectoryModules(t *testing.T) {
	files := []File{
		{Path: "internal/handler/handler.go", Language: "go"},
		{Path: "internal/handler/routes.go", Language: "go"},
		{Path: "internal/model/user.go", Language: "go"},
		{Path: "cmd/app/main.go", Language: "go"},
	}

	modules := resolveDirectoryModules(files)
	if len(modules) != 3 {
		t.Errorf("got %d modules, want 3", len(modules))
	}

	// Check that files are grouped correctly
	moduleMap := make(map[string]int)
	for _, m := range modules {
		moduleMap[m.Name] = len(m.Files)
	}

	if moduleMap["internal/handler"] != 2 {
		t.Errorf("internal/handler has %d files, want 2", moduleMap["internal/handler"])
	}
	if moduleMap["internal/model"] != 1 {
		t.Errorf("internal/model has %d files, want 1", moduleMap["internal/model"])
	}
}

func TestResolveJavaModules(t *testing.T) {
	// Create temp Java files with package declarations
	dir := t.TempDir()

	authDir := filepath.Join(dir, "src", "main", "java", "com", "example", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := "package com.example.auth;\n\npublic class Service {}\n"
	if err := os.WriteFile(filepath.Join(authDir, "Service.java"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	content2 := "package com.example.auth;\n\npublic class Config {}\n"
	if err := os.WriteFile(filepath.Join(authDir, "Config.java"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	files := []File{
		{Path: "src/main/java/com/example/auth/Service.java", Language: "java"},
		{Path: "src/main/java/com/example/auth/Config.java", Language: "java"},
	}

	modules := resolveJavaModules(dir, files)
	if len(modules) != 1 {
		t.Errorf("got %d modules, want 1", len(modules))
		for _, m := range modules {
			t.Logf("  module: %s (%d files)", m.Name, len(m.Files))
		}
		return
	}

	if modules[0].Name != "com.example.auth" {
		t.Errorf("module name = %q, want %q", modules[0].Name, "com.example.auth")
	}
	if len(modules[0].Files) != 2 {
		t.Errorf("module has %d files, want 2", len(modules[0].Files))
	}
}

func TestResolvePythonModules(t *testing.T) {
	dir := t.TempDir()

	// Create package with __init__.py
	pkgDir := filepath.Join(dir, "myapp")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "__init__.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	files := []File{
		{Path: "myapp/__init__.py", Language: "python"},
		{Path: "myapp/app.py", Language: "python"},
		{Path: "myapp/config.py", Language: "python"},
		{Path: "standalone.py", Language: "python"},
	}

	modules := resolvePythonModules(dir, files)

	// Should have 2 modules: myapp (package) and standalone.py
	if len(modules) != 2 {
		t.Errorf("got %d modules, want 2", len(modules))
		for _, m := range modules {
			t.Logf("  module: %s (%d files)", m.Name, len(m.Files))
		}
	}
}

func TestJavaPathToPackage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"src/main/java/com/example/auth/Service.java", "com.example.auth"},
		{"src/test/java/com/example/auth/Test.java", "com.example.auth"},
		{"app/src/main/java/com/example/App.java", "com.example"},
		{"com/example/Service.java", "com.example"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := javaPathToPackage(tt.path)
			if got != tt.want {
				t.Errorf("javaPathToPackage(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectJavaPackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Test.java")

	content := "package com.example.auth;\n\npublic class Test {}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pkg := detectJavaPackage(path)
	if pkg != "com.example.auth" {
		t.Errorf("detectJavaPackage() = %q, want %q", pkg, "com.example.auth")
	}
}

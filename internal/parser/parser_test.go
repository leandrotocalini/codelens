package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkFiles(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "go-project")
	files, err := walkFiles(repoRoot, []string{"vendor", "node_modules", ".git"})
	if err != nil {
		t.Fatalf("walkFiles() error: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("walkFiles() returned no files")
	}

	// Check that we found Go files
	found := false
	for _, f := range files {
		if f.Language == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no Go files found")
	}
}

func TestWalkFilesExclude(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "go-project")
	files, err := walkFiles(repoRoot, []string{"internal"})
	if err != nil {
		t.Fatalf("walkFiles() error: %v", err)
	}

	for _, f := range files {
		if filepath.Dir(f.Path) == "internal" || filepath.Dir(filepath.Dir(f.Path)) == "internal" {
			t.Errorf("file %s should have been excluded", f.Path)
		}
	}
}

func TestCountLOC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loc, err := countLOC(path)
	if err != nil {
		t.Fatalf("countLOC() error: %v", err)
	}
	if loc != 4 { // 4 non-empty lines
		t.Errorf("countLOC() = %d, want %d", loc, 4)
	}
}

func TestParseGoProject(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "go-project")
	modules, stats, err := Parse(repoRoot, []string{})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(modules) == 0 {
		t.Fatal("Parse() returned no modules")
	}

	if stats.TotalFiles == 0 {
		t.Error("stats.TotalFiles = 0")
	}
	if stats.TotalLOC == 0 {
		t.Error("stats.TotalLOC = 0")
	}
	if stats.TotalPackages == 0 {
		t.Error("stats.TotalPackages = 0")
	}
}

func TestParseJavaProject(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "java-project")
	modules, stats, err := Parse(repoRoot, []string{})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(modules) == 0 {
		t.Fatal("Parse() returned no modules")
	}

	// Java should resolve to package names
	found := false
	for _, m := range modules {
		if m.Name == "com.example.auth" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(modules))
		for i, m := range modules {
			names[i] = m.Name
		}
		t.Errorf("expected module com.example.auth, got modules: %v", names)
	}

	if stats.TotalFiles == 0 {
		t.Error("stats.TotalFiles = 0")
	}
}

func TestParseTSProject(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "ts-project")
	modules, stats, err := Parse(repoRoot, []string{})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(modules) == 0 {
		t.Fatal("Parse() returned no modules")
	}
	if stats.TotalFiles == 0 {
		t.Error("stats.TotalFiles = 0")
	}
}

func TestParsePythonProject(t *testing.T) {
	repoRoot := filepath.Join(testdataDir(t), "python-project")
	modules, stats, err := Parse(repoRoot, []string{})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(modules) == 0 {
		t.Fatal("Parse() returned no modules")
	}
	if stats.TotalFiles == 0 {
		t.Error("stats.TotalFiles = 0")
	}
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		isDir    bool
		excludes []string
		want     bool
	}{
		{"vendor dir", "vendor", true, []string{"vendor"}, true},
		{"nested vendor", "some/vendor", true, []string{"vendor"}, true},
		{"file in vendor", "vendor/pkg.go", false, []string{"vendor"}, true},
		{"normal file", "main.go", false, []string{"vendor"}, false},
		{"glob match", "file.pb.go", false, []string{"*.pb.go"}, true},
		{"no excludes", "main.go", false, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExclude(tt.path, tt.isDir, tt.excludes)
			if got != tt.want {
				t.Errorf("shouldExclude(%q, %v, %v) = %v, want %v", tt.path, tt.isDir, tt.excludes, got, tt.want)
			}
		})
	}
}

func testdataDir(t *testing.T) string {
	t.Helper()
	// Walk up to find testdata
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		td := filepath.Join(dir, "testdata")
		if info, err := os.Stat(td); err == nil && info.IsDir() {
			return td
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("testdata directory not found")
		}
		dir = parent
	}
}

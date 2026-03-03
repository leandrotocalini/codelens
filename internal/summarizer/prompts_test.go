package summarizer

import (
	"strings"
	"testing"

	"github.com/leandrotocalini/codelens/internal/parser"
)

func TestModulePrompt(t *testing.T) {
	module := parser.Module{
		Name:     "internal/handler",
		Language: "go",
		Files: []parser.File{
			{Path: "internal/handler/handler.go"},
		},
		Symbols: []parser.Symbol{
			{Name: "Handler", Kind: parser.SymbolType, Exported: true, Signature: "type Handler struct {"},
			{Name: "New", Kind: parser.SymbolFunction, Exported: true, Signature: "func New() *Handler"},
			{Name: "helper", Kind: parser.SymbolFunction, Exported: false, Signature: "func helper()"},
			{Name: "net/http", Kind: parser.SymbolImport, ImportPath: "net/http"},
		},
	}

	prompt := ModulePrompt(module)

	if !strings.Contains(prompt, "internal/handler") {
		t.Error("prompt should contain module name")
	}
	if !strings.Contains(prompt, "type Handler struct") {
		t.Error("prompt should contain exported type")
	}
	if !strings.Contains(prompt, "func New()") {
		t.Error("prompt should contain exported function")
	}
	if strings.Contains(prompt, "func helper()") {
		t.Error("prompt should not contain unexported function")
	}
	if strings.Contains(prompt, "net/http") {
		t.Error("prompt should not contain imports in symbols section")
	}
}

func TestProjectPrompt(t *testing.T) {
	modules := []parser.Module{
		{Name: "internal/handler", Language: "go"},
		{Name: "internal/model", Language: "go"},
	}

	summaries := map[string]string{
		"internal/handler": "Handler summary",
		"internal/model":   "Model summary",
	}

	prompt := ProjectPrompt(modules, summaries, "handler -> model\n")

	if !strings.Contains(prompt, "Handler summary") {
		t.Error("prompt should contain module summaries")
	}
	if !strings.Contains(prompt, "handler -> model") {
		t.Error("prompt should contain dep graph")
	}
	if !strings.Contains(prompt, "### Project Summary") {
		t.Error("prompt should request project summary section")
	}
	if !strings.Contains(prompt, "### Libraries & Frameworks") {
		t.Error("prompt should request libraries/frameworks section")
	}
	if !strings.Contains(prompt, "### External Dependencies") {
		t.Error("prompt should request external dependencies section")
	}
	if !strings.Contains(prompt, "### Code Architecture") {
		t.Error("prompt should request code architecture section")
	}
	if !strings.Contains(prompt, "### Critical Paths & Change Impact") {
		t.Error("prompt should request critical paths section")
	}
}

func TestFormatModuleSymbols(t *testing.T) {
	module := parser.Module{
		Name: "test",
		Files: []parser.File{
			{
				Path: "a.go",
				LOC:  100,
				Symbols: []parser.Symbol{
					{Name: "Foo", Kind: parser.SymbolFunction, Exported: true, Signature: "func Foo()"},
				},
			},
		},
	}

	output := FormatModuleSymbols(module, 50)
	if !strings.Contains(output, "func Foo()") {
		t.Error("output should contain Foo")
	}
}

func TestFormatModuleSymbolsTruncation(t *testing.T) {
	var files []parser.File
	for i := 0; i < 60; i++ {
		files = append(files, parser.File{
			Path: "file_" + string(rune('a'+i%26)) + ".go",
			LOC:  100 - i,
		})
	}

	module := parser.Module{Name: "test", Files: files}
	output := FormatModuleSymbols(module, 50)

	if !strings.Contains(output, "and 10 more") {
		t.Error("output should mention truncated files")
	}
}

func TestModulePromptTestHeavyModule(t *testing.T) {
	module := parser.Module{
		Name:     "internal/parser",
		Language: "go",
		Files: []parser.File{
			{
				Path: "internal/parser/parser_test.go",
				Symbols: []parser.Symbol{
					{Name: "TestParse", Kind: parser.SymbolFunction, Exported: true, Signature: "func TestParse(t *testing.T)"},
				},
			},
		},
	}

	prompt := ModulePrompt(module)
	if !strings.Contains(prompt, "ModuleClass: test") {
		t.Error("test-heavy module should be marked as test")
	}
	if !strings.Contains(prompt, "**Change impact**:") {
		t.Error("prompt should request change impact in response format")
	}
}

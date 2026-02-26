package summarizer

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/leandrotocalini/codelens/internal/graph"
	"github.com/leandrotocalini/codelens/internal/parser"
)

// MockSummarizer implements the Summarizer interface for testing.
type MockSummarizer struct {
	CallCount atomic.Int32
	Response  string
	Err       error
}

func (m *MockSummarizer) Summarize(_ context.Context, prompt string) (string, error) {
	m.CallCount.Add(1)
	if m.Err != nil {
		return "", m.Err
	}
	if m.Response != "" {
		return m.Response, nil
	}
	return "**Responsibility**: Test module.\n**Key types**: TestType\n**Key functions**: TestFunc()", nil
}

func TestSummarizeAll(t *testing.T) {
	modules := []parser.Module{
		{Name: "pkg/a", Language: "go"},
		{Name: "pkg/b", Language: "go"},
		{Name: "pkg/c", Language: "go"},
	}

	mock := &MockSummarizer{}
	g := graph.Build(modules)

	summaries, projectSummary, err := SummarizeAll(context.Background(), modules, g, mock, 2, nil)
	if err != nil {
		t.Fatalf("SummarizeAll() error: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("summaries count = %d, want 3", len(summaries))
	}

	if projectSummary == "" {
		t.Error("projectSummary is empty")
	}

	// 3 module calls + 1 project call = 4
	if mock.CallCount.Load() != 4 {
		t.Errorf("call count = %d, want 4", mock.CallCount.Load())
	}
}

func TestSummarizePartial(t *testing.T) {
	allModules := []parser.Module{
		{Name: "pkg/a", Language: "go"},
		{Name: "pkg/b", Language: "go"},
		{Name: "pkg/c", Language: "go"},
	}

	affected := []parser.Module{
		{Name: "pkg/b", Language: "go"},
	}

	cached := map[string]string{
		"pkg/a": "cached summary for a",
		"pkg/c": "cached summary for c",
	}

	mock := &MockSummarizer{}
	g := graph.Build(allModules)

	summaries, projectSummary, err := SummarizePartial(
		context.Background(), affected, cached, allModules, g, mock, 2, nil)
	if err != nil {
		t.Fatalf("SummarizePartial() error: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("summaries count = %d, want 3", len(summaries))
	}

	// Cached summaries should be preserved
	if summaries["pkg/a"] != "cached summary for a" {
		t.Errorf("pkg/a summary = %q, want cached", summaries["pkg/a"])
	}
	if summaries["pkg/c"] != "cached summary for c" {
		t.Errorf("pkg/c summary = %q, want cached", summaries["pkg/c"])
	}

	if projectSummary == "" {
		t.Error("projectSummary is empty")
	}

	// 1 module call + 1 project call = 2
	if mock.CallCount.Load() != 2 {
		t.Errorf("call count = %d, want 2", mock.CallCount.Load())
	}
}

func TestSummarizeAllError(t *testing.T) {
	modules := []parser.Module{
		{Name: "pkg/a", Language: "go"},
	}

	mock := &MockSummarizer{Err: fmt.Errorf("API error")}
	g := graph.Build(modules)

	_, _, err := SummarizeAll(context.Background(), modules, g, mock, 1, nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestSummarizeProgress(t *testing.T) {
	modules := []parser.Module{
		{Name: "pkg/a", Language: "go"},
		{Name: "pkg/b", Language: "go"},
	}

	mock := &MockSummarizer{}
	g := graph.Build(modules)

	var progressCalls int
	progress := func(completed, total int) {
		progressCalls++
	}

	_, _, err := SummarizeAll(context.Background(), modules, g, mock, 1, progress)
	if err != nil {
		t.Fatalf("SummarizeAll() error: %v", err)
	}

	if progressCalls != 2 {
		t.Errorf("progress called %d times, want 2", progressCalls)
	}
}

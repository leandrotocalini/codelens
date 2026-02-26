package summarizer

import (
	"context"
	"fmt"
	"sync"

	"github.com/leandrotocalini/codelens/internal/graph"
	"github.com/leandrotocalini/codelens/internal/parser"
)

// ProgressFunc is called to report summarization progress.
type ProgressFunc func(completed, total int)

// SummarizeAll summarizes all modules and produces a project summary.
func SummarizeAll(ctx context.Context, modules []parser.Module, g *graph.Graph, s Summarizer, concurrency int, progress ProgressFunc) (map[string]string, string, error) {
	summaries, err := summarizeModules(ctx, modules, s, concurrency, progress)
	if err != nil {
		return nil, "", err
	}

	// Project summary
	depGraph := ""
	if g != nil {
		depGraph = graph.RenderFlat(g)
	}
	projectPrompt := ProjectPrompt(modules, summaries, depGraph)
	projectSummary, err := s.Summarize(ctx, projectPrompt)
	if err != nil {
		return summaries, "", fmt.Errorf("project summary: %w", err)
	}

	return summaries, projectSummary, nil
}

// SummarizePartial only summarizes affected modules, merging with cached summaries.
func SummarizePartial(ctx context.Context, affected []parser.Module, cached map[string]string, allModules []parser.Module, g *graph.Graph, s Summarizer, concurrency int, progress ProgressFunc) (map[string]string, string, error) {
	// Start with cached summaries
	summaries := make(map[string]string)
	for k, v := range cached {
		summaries[k] = v
	}

	// Summarize affected modules
	newSummaries, err := summarizeModules(ctx, affected, s, concurrency, progress)
	if err != nil {
		return nil, "", err
	}

	// Merge new summaries
	for k, v := range newSummaries {
		summaries[k] = v
	}

	// Remove summaries for modules that no longer exist
	moduleSet := make(map[string]bool)
	for _, m := range allModules {
		moduleSet[m.Name] = true
	}
	for k := range summaries {
		if !moduleSet[k] {
			delete(summaries, k)
		}
	}

	// Regenerate project summary
	depGraph := ""
	if g != nil {
		depGraph = graph.RenderFlat(g)
	}
	projectPrompt := ProjectPrompt(allModules, summaries, depGraph)
	projectSummary, err := s.Summarize(ctx, projectPrompt)
	if err != nil {
		return summaries, "", fmt.Errorf("project summary: %w", err)
	}

	return summaries, projectSummary, nil
}

func summarizeModules(ctx context.Context, modules []parser.Module, s Summarizer, concurrency int, progress ProgressFunc) (map[string]string, error) {
	if concurrency < 1 {
		concurrency = 5
	}

	type result struct {
		name    string
		summary string
		err     error
	}

	results := make(chan result, len(modules))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for _, m := range modules {
		wg.Add(1)
		go func(mod parser.Module) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prompt := ModulePrompt(mod)
			summary, err := s.Summarize(ctx, prompt)
			results <- result{name: mod.Name, summary: summary, err: err}
		}(m)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	summaries := make(map[string]string)
	completed := 0
	var firstErr error

	for r := range results {
		completed++
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("summarizing %s: %w", r.name, r.err)
			}
			continue
		}
		summaries[r.name] = r.summary
		if progress != nil {
			progress(completed, len(modules))
		}
	}

	return summaries, firstErr
}

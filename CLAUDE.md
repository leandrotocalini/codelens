# CLAUDE.md — CodeLens

## Project Overview

CodeLens is a single Go binary that generates a structured Markdown summary (`CODELENS.md`) of any codebase. It uses tree-sitter for parsing and an LLM for summarization. On subsequent runs it performs incremental updates by diffing against the last indexed commit.

## Build & Run

```bash
# Build
go build -o codelens ./cmd/codelens

# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Lint
golangci-lint run ./...

# Run the binary
./codelens              # incremental (or full on first run)
./codelens --full       # force full re-index
./codelens --verbose    # detailed progress
```

## Project Structure

```
cmd/codelens/main.go           — CLI entry point (cobra/pflag)
internal/config/config.go      — Config loading: flags + .codelens.json + env vars
internal/parser/parser.go      — Tree-sitter parsing, file walking, symbol extraction
internal/parser/modules.go     — Language-specific module resolution
internal/parser/symbols.go     — Symbol types: Function, Type, Import
internal/parser/queries/*.scm  — Tree-sitter queries per language
internal/summarizer/           — LLM summarization (orchestration, prompts, provider interface)
internal/graph/                — Dependency graph construction + ASCII rendering
internal/diff/                 — Git diff detection + state.json persistence
internal/output/markdown.go    — CODELENS.md assembly
```

## Key Architectural Decisions

- **Module resolution is language-specific**: Go uses directory paths, Java uses `package` declarations, Python uses `__init__.py` presence. See `internal/parser/modules.go`.
- **Incremental updates**: State stored in `.codelens/state.json`. Only modules with changed files get re-summarized via LLM. Parsing and graph building always run fully (they're fast).
- **LLM provider abstraction**: `internal/summarizer/provider.go` defines a `Summarizer` interface. Implementations for Anthropic, OpenRouter, OpenAI.
- **Bounded concurrency**: LLM calls run in parallel with a semaphore (default: 5 concurrent).
- **Exported symbols only**: To manage token budgets, only exported/public signatures are sent to the LLM, not full function bodies.

## Code Conventions

- Go standard project layout: `cmd/` for binaries, `internal/` for private packages
- Use `errors.New` / `fmt.Errorf` with `%w` wrapping — no custom error types unless needed
- Table-driven tests
- No global mutable state — pass dependencies explicitly
- Interfaces defined where consumed, not where implemented
- Context (`context.Context`) threaded through for cancellation support

## Environment Variables

- `ANTHROPIC_API_KEY` — required when using Anthropic provider (default)
- `OPENROUTER_API_KEY` — required when using OpenRouter provider
- `OPENAI_API_KEY` — required when using OpenAI provider

## Testing Strategy

- Unit tests per package with `_test.go` files
- Parser tests use fixture files in `testdata/` directories
- Summarizer tests mock the LLM provider interface
- Integration test: run full pipeline on a small embedded test repo
- `go test ./...` must pass before any commit

## Common Tasks

- **Add a new language**: Add tree-sitter grammar dependency to `go.mod`, create `internal/parser/queries/<lang>.scm`, add module resolution logic in `modules.go`, add language detection in `parser.go`
- **Add a new LLM provider**: Implement the `Summarizer` interface in `internal/summarizer/provider.go`, register it in the provider factory
- **Change output format**: Modify `internal/output/markdown.go`

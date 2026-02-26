# CodeLens — Implementation Plan

## Phase 0: Project Scaffolding

### 0.1 — Go module & dependencies
- `go mod init github.com/leandrotocalini/codelens`
- Add dependencies: `cobra` (CLI), `sitter` (tree-sitter Go bindings), tree-sitter grammar packages for Go/Java/TypeScript/Python
- Add `golangci-lint` config (`.golangci.yml`)
- Create `.gitignore` (binaries, `.codelens/`, vendor if not vendoring)

### 0.2 — Directory skeleton
Create all directories matching the architecture:
```
cmd/codelens/main.go
internal/config/
internal/parser/
internal/parser/queries/
internal/summarizer/
internal/graph/
internal/diff/
internal/output/
```
Each package gets a minimal `.go` file with the package declaration.

### 0.3 — CI basics
- Makefile with targets: `build`, `test`, `lint`, `all`
- Goreleaser config for cross-platform builds (optional, can defer)

---

## Phase 1: Config & CLI (`internal/config` + `cmd/codelens`)

### 1.1 — Config types
- Define `Config` struct: `Model`, `Provider`, `Output`, `Exclude []string`, `MaxFiles int`, `Full bool`, `Verbose bool`
- Defaults matching README spec

### 1.2 — Config loading
- `config.Load()` reads (in order): defaults → `.codelens.json` → env vars → CLI flags
- CLI flags override everything
- Unit tests: defaults, JSON override, flag override

### 1.3 — CLI entry point
- `cmd/codelens/main.go` using `cobra` (or raw `flag` if simpler)
- Parse flags, call `config.Load()`, then dispatch to the main pipeline
- Wire up `--help` with the flag descriptions from README

**Deliverable**: `codelens --help` works, config loading is tested.

---

## Phase 2: Parser Foundation (`internal/parser`)

### 2.1 — Symbol types (`symbols.go`)
- Define types: `Symbol` (name, kind, visibility, signature, line), `SymbolKind` enum (Function, Type, Method, Import, Constant, Interface)
- Define `File` struct: path, language, symbols, LOC
- Define `Module` struct: name, language, files, symbols (aggregated)

### 2.2 — File walker (`parser.go`)
- Walk repo root, respect exclude globs from config
- Detect language by file extension (`.go`, `.java`, `.ts`, `.tsx`, `.js`, `.py`, `.rs`)
- Count LOC per file
- Return `[]File` grouped by detected language

### 2.3 — Module resolution (`modules.go`)
- `ResolveModules(files []File) []Module`
- Implement per-language strategies:
  - **Go**: group by directory path
  - **Java**: parse `package` declaration from first few lines (regex, no tree-sitter needed here), group by package. Strip Maven/Gradle prefixes (`src/main/java/`, `src/test/java/`). Associate test files with main module.
  - **TypeScript/JS**: group by directory
  - **Python**: group by directory if `__init__.py` exists, else standalone file = module
- Unit tests with fixture directories in `testdata/`

### 2.4 — Tree-sitter parsing (`parser.go` + `queries/*.scm`)
- Integrate Go tree-sitter bindings
- Write `.scm` queries for each P0 language to extract:
  - Function/method declarations (name, signature, visibility)
  - Type/class/struct/interface declarations
  - Import statements
- Parse each file, extract symbols, attach to `File`
- Fallback for unsupported languages: LOC count only, no symbols

### 2.5 — Stats collection
- Aggregate stats across all modules: total files, LOC, packages, exported functions/types, language breakdown
- Define `Stats` struct

**Deliverable**: `parser.Parse(root, config) -> ([]Module, Stats, error)` works on Go, Java, TS, Python repos. Comprehensive tests with fixture repos.

---

## Phase 3: Dependency Graph (`internal/graph`)

### 3.1 — Graph construction (`graph.go`)
- `Build(modules []Module) *Graph`
- For each module, look at its import symbols
- Resolve imports to other known modules (best-effort matching)
- Store as adjacency list: `map[string][]string` (module -> dependencies)
- Also compute reverse edges (used by)

### 3.2 — ASCII rendering (`render.go`)
- `Render(graph *Graph, entrypoints []string) string`
- Produce the tree-style output shown in README
- Entry points: modules that nothing depends on (typically `cmd/` or `main`)
- Handle cycles gracefully (mark with `(cycle)`)

**Deliverable**: Given modules with imports, produce a correct ASCII dependency tree. Tests with known graph structures.

---

## Phase 4: LLM Summarizer (`internal/summarizer`)

### 4.1 — Provider interface (`provider.go`)
- Define `Summarizer` interface:
  ```go
  type Summarizer interface {
      Summarize(ctx context.Context, prompt string) (string, error)
  }
  ```
- Implement `AnthropicSummarizer` (using Anthropic Messages API)
- Implement `OpenAISummarizer` (OpenAI chat completions)
- Implement `OpenRouterSummarizer` (OpenRouter API, same as OpenAI format)
- Factory function: `NewSummarizer(provider, model, apiKey string) Summarizer`

### 4.2 — Prompt templates (`prompts.go`)
- Module summary prompt (from README spec)
- Project summary prompt
- Format symbol data for the prompt (exported only, signatures not bodies)
- Token truncation: if module > 50 files, take top 50 by LOC + "and N more"

### 4.3 — Orchestration (`summarizer.go`)
- `SummarizeAll(ctx, modules []Module, summarizer Summarizer) (map[string]string, string, error)`
  - Returns module summaries + project summary
  - Bounded parallel execution with semaphore (default: 5)
  - Progress callback for the progress bar
- `SummarizePartial(ctx, affected []Module, cached map[string]string, all []Module, summarizer Summarizer) (map[string]string, string, error)`
  - Only LLM-calls affected modules, merges with cached summaries
  - Regenerates project summary

### 4.4 — Testing
- Mock `Summarizer` implementation for tests
- Test prompt construction
- Test parallel execution and error handling

**Deliverable**: Given modules + provider config, produce summaries for all modules. Works with all three providers.

---

## Phase 5: Diff & State (`internal/diff`)

### 5.1 — State persistence (`state.go`)
- `State` struct matching the JSON schema in README
- `Save(path, commitHash, modules)` — write `.codelens/state.json`
- `Load(path) (*State, error)` — read state, return nil if not exists
- Compute per-module content hash (SHA256 of sorted file contents)

### 5.2 — Git diff detection (`diff.go`)
- `Changed(lastCommit string) ([]string, error)` — run `git diff --name-only <lastCommit> HEAD`
- `AffectedModules(changedFiles []string, modules []Module) []Module`
  - Map changed files to their modules
  - Handle edge cases: new modules (not in state), deleted modules (in state but not in parser output), moved files
- `IsGitRepo() bool` — check if cwd is a git repo
- `HeadCommit() (string, error)` — get current HEAD
- `IsDirty() bool` — check for uncommitted changes

**Deliverable**: Given a previous state and current HEAD, determine which modules changed. Tests with mock git commands.

---

## Phase 6: Markdown Output (`internal/output`)

### 6.1 — CODELENS.md assembly (`markdown.go`)
- `Write(path string, projectSummary string, modules []Module, summaries map[string]string, graph *Graph, stats Stats) error`
- Generate the full CODELENS.md following the format from README:
  - Header comment with commit hash
  - Project Summary section
  - Module Map (sorted alphabetically, each module's summary)
  - Dependency Graph (ASCII tree)
  - Stats table
- Ensure idempotent output (same input -> same output)

**Deliverable**: Given all computed data, produce a well-formatted CODELENS.md. Golden-file tests comparing expected output.

---

## Phase 7: Main Pipeline & Integration

### 7.1 — Wire everything together in `main.go`
- First run flow:
  1. `config.Load()`
  2. `parser.Parse(repoRoot)`
  3. `graph.Build(modules)`
  4. `summarizer.SummarizeAll(modules)`
  5. `output.Write(...)`
  6. `state.Save(...)`

- Incremental flow:
  1. `config.Load()`
  2. `state.Load()`
  3. `diff.Changed(lastCommit)`
  4. `diff.AffectedModules(changedFiles)`
  5. `parser.Parse(repoRoot)`
  6. `graph.Build(modules)`
  7. `summarizer.SummarizePartial(affected, cached)`
  8. `output.Write(...)`
  9. `state.Save(HEAD, modules)`

- `--full` flag: skip state loading, always do full run
- Progress output: parsing, summarizing (with progress bar), writing

### 7.2 — Progress display
- Simple progress bar for summarization: `Summarizing modules... ████ 12/47`
- Use `--verbose` for detailed per-file output
- Clean non-verbose output matching the UX examples in README

### 7.3 — Error handling
- Missing API key: clear error message with which env var to set
- LLM API errors: retry with backoff (3 attempts), then fail gracefully
- Parse errors: skip file, warn, continue
- No git repo: warn, always do full index

### 7.4 — Integration tests
- Small test repo (embedded or in `testdata/`)
- Test full pipeline end-to-end with mock LLM
- Test incremental pipeline: index, modify files, re-index
- Test `--full` flag forces full re-index

**Deliverable**: `codelens` runs end-to-end on a real repo, producing correct output.

---

## Phase 8: Polish & P1 Languages

### 8.1 — Rust support (P1)
- Tree-sitter Rust grammar
- Module resolution: `Cargo.toml` boundaries, `mod.rs` / `lib.rs`
- Query file: `queries/rust.scm`

### 8.2 — Edge cases
- Empty repos
- Repos with only unsupported languages
- Very large modules (>50 files truncation)
- Binary files (skip)
- Symlinks (follow or skip, configurable)

### 8.3 — Performance
- Profile on a large repo (500+ files)
- Ensure parsing is fast (tree-sitter is native, should be fine)
- Ensure bounded LLM concurrency doesn't bottleneck
- Target: 200-file repo < 60s, 500-file < 120s

---

## Implementation Order Summary

| Order | Package | Estimated Effort | Dependencies |
|-------|---------|-----------------|--------------|
| 1 | Scaffolding (Phase 0) | Small | None |
| 2 | `internal/config` | Small | None |
| 3 | `cmd/codelens` (basic) | Small | config |
| 4 | `internal/parser/symbols.go` | Small | None |
| 5 | `internal/parser/parser.go` (file walker) | Medium | config |
| 6 | `internal/parser/modules.go` | Medium | parser |
| 7 | `internal/parser` (tree-sitter) | Large | modules |
| 8 | `internal/graph` | Medium | parser |
| 9 | `internal/summarizer/provider.go` | Medium | None |
| 10 | `internal/summarizer/prompts.go` | Small | parser |
| 11 | `internal/summarizer/summarizer.go` | Medium | provider, prompts |
| 12 | `internal/diff` | Medium | parser |
| 13 | `internal/output` | Medium | parser, graph, summarizer |
| 14 | Main pipeline wiring | Medium | All |
| 15 | Integration tests | Medium | All |
| 16 | Rust support (P1) | Medium | parser |

The heaviest piece is tree-sitter integration (step 7) — writing correct `.scm` queries for 4 languages and handling edge cases. The LLM provider implementations (step 9) are straightforward HTTP calls. Everything else is standard Go plumbing.

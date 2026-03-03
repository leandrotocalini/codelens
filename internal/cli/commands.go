package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leandrotocalini/codelens/internal/config"
	"github.com/leandrotocalini/codelens/internal/diff"
	"github.com/leandrotocalini/codelens/internal/graph"
	"github.com/leandrotocalini/codelens/internal/oauth"
	"github.com/leandrotocalini/codelens/internal/output"
	"github.com/leandrotocalini/codelens/internal/parser"
	"github.com/leandrotocalini/codelens/internal/summarizer"
	"github.com/spf13/cobra"
)

type oauthRunner interface {
	StartAuthorizationCodeFlow(ctx context.Context, settings oauth.Settings) (oauth.TokenPair, error)
}

type summarizerFactory func(model, oauthToken string, debug bool, debugOut io.Writer) (summarizer.Summarizer, error)

// Deps provides command runtime dependencies.
type Deps struct {
	Stdout            io.Writer
	Stderr            io.Writer
	Stdin             io.Reader
	OAuthClient       oauthRunner
	SummarizerFactory summarizerFactory
	Now               func() time.Time
}

// NewRootCommand returns the codelens root command.
func NewRootCommand(deps Deps) *cobra.Command {
	if deps.Stdout == nil {
		deps.Stdout = os.Stdout
	}
	if deps.Stderr == nil {
		deps.Stderr = os.Stderr
	}
	if deps.Stdin == nil {
		deps.Stdin = os.Stdin
	}
	if deps.OAuthClient == nil {
		deps.OAuthClient = oauth.NewClient()
	}
	if deps.SummarizerFactory == nil {
		deps.SummarizerFactory = func(model, oauthToken string, debug bool, debugOut io.Writer) (summarizer.Summarizer, error) {
			return summarizer.NewSummarizer("codex", model, oauthToken, debug, debugOut)
		}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}

	flags := config.CLIFlags{}
	root := &cobra.Command{
		Use:          "codelens",
		Short:        "Generate structured codebase summaries",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := runPipeline(cmd.Context(), deps, flags, cmd)
			if err != nil {
				if config.IsMissingConfigError(err) {
					_, _ = fmt.Fprintln(deps.Stderr, config.SetupInstructions())
					return errors.New("missing global config")
				}
				return err
			}
			return nil
		},
	}

	root.Flags().StringVar(&flags.Model, "model", "", "Override model (default from global config)")
	root.Flags().IntVar(&flags.Concurrency, "concurrency", 0, "Override max parallel model calls")
	root.Flags().StringVar(&flags.Output, "output", "", "Output file path")
	root.Flags().StringVar(&flags.Exclude, "exclude", "", "Comma-separated glob patterns to exclude")
	root.Flags().IntVar(&flags.MaxFiles, "max-files", 0, "Max files per module before truncating")
	root.Flags().BoolVar(&flags.Full, "full", false, "Force full re-index")
	root.Flags().BoolVar(&flags.Verbose, "verbose", false, "Show detailed progress")
	root.Flags().BoolVar(&flags.Debug, "debug", false, "Show Codex request/response debug logs")

	root.AddCommand(newConfigureCommand(deps))
	return root
}

func newConfigureCommand(deps Deps) *cobra.Command {
	var oauthTokenFlag string
	var clientIDFlag string
	var modelFlag string
	var concurrencyFlag int

	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Authenticate with Codex OAuth and save ~/.codelens/config.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings := oauth.SettingsFromEnv()

			if modelFlag == "" {
				modelFlag = "codex"
			}
			if concurrencyFlag <= 0 {
				concurrencyFlag = 5
			}

			var tokenPair oauth.TokenPair
			if oauthTokenFlag != "" {
				clientID := clientIDFlag
				if clientID == "" {
					clientID = settings.ClientID
				}
				tokenPair = oauth.TokenPair{
					OAuthToken: oauthTokenFlag,
					ClientID:   clientID,
				}
			} else {
				_, _ = fmt.Fprintln(deps.Stdout, "Your browser will open to sign in with Codex OAuth...")

				ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
				defer cancel()

				var err error
				tokenPair, err = deps.OAuthClient.StartAuthorizationCodeFlow(ctx, settings)
				if err != nil {
					_, _ = fmt.Fprintf(deps.Stderr, "Browser OAuth flow failed: %v\n", err)
					var authErr *oauth.AuthorizationError
					if errors.As(err, &authErr) {
						if authErr.Code == "invalid_scope" && strings.Contains(authErr.Description, "model.request") {
							_, _ = fmt.Fprintln(deps.Stderr, `This OAuth client is not allowed to request "model.request".`)
							_, _ = fmt.Fprintln(deps.Stderr, `Run: export CODEX_OAUTH_SCOPES="openid profile email offline_access"`)
							_, _ = fmt.Fprintln(deps.Stderr, `Then run: codelens configure`)
						}
						return err
					}
					manualToken, readErr := readManualToken(deps.Stdin, deps.Stdout)
					if readErr != nil {
						return readErr
					}
					if manualToken == "" {
						return errors.New("manual token not provided")
					}
					tokenPair = oauth.TokenPair{
						OAuthToken: manualToken,
						ClientID:   settings.ClientID,
					}
				}
			}

			globalCfg := config.GlobalConfig{
				OAuthToken:  tokenPair.OAuthToken,
				ClientID:    tokenPair.ClientID,
				Model:       modelFlag,
				Concurrency: concurrencyFlag,
			}
			if err := config.SaveGlobal(globalCfg); err != nil {
				return err
			}

			path, err := config.GlobalConfigPath()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(deps.Stdout, "Saved configuration to %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&oauthTokenFlag, "oauth-token", "", "OAuth token. If provided, skip browser redirect and save config directly")
	cmd.Flags().StringVar(&clientIDFlag, "client-id", "", "OAuth client ID (default: CODEX_OAUTH_CLIENT_ID or built-in default)")
	cmd.Flags().StringVar(&modelFlag, "model", "codex", "Default model to store in config")
	cmd.Flags().IntVar(&concurrencyFlag, "concurrency", 5, "Default concurrency to store in config")
	return cmd
}

func readManualToken(in io.Reader, out io.Writer) (string, error) {
	_, _ = fmt.Fprint(out, "Paste your Codex OAuth token (leave empty to cancel): ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func runPipeline(ctx context.Context, deps Deps, flags config.CLIFlags, rootCmd *cobra.Command) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(repoRoot, flags)
	if err != nil {
		return err
	}

	llm, err := deps.SummarizerFactory(cfg.Model, cfg.OAuthToken, cfg.Debug, deps.Stderr)
	if err != nil {
		return fmt.Errorf("initializing summarizer: %w", err)
	}

	modules, stats, err := parser.Parse(repoRoot, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("parsing repository: %w", err)
	}
	if len(modules) == 0 {
		return errors.New("no supported source files found in this repository")
	}
	g := graph.Build(modules)

	gitRepo := diff.IsGitRepo(repoRoot)
	commitDisplay := "unversioned"
	commitHash := ""
	if gitRepo {
		commitDisplay, err = diff.HeadCommitShort(repoRoot)
		if err != nil {
			return err
		}
		commitHash, err = diff.HeadCommit(repoRoot)
		if err != nil {
			return err
		}
	}

	state, err := diff.LoadState(repoRoot)
	if err != nil {
		return err
	}

	progress := progressFunc(deps.Stdout, cfg.Verbose)

	fullIndex := cfg.Full || !gitRepo || state == nil || state.Model != cfg.Model
	var summaries map[string]string
	var projectSummary string

	if fullIndex {
		summaries, projectSummary, err = summarizer.SummarizeAll(ctx, modules, g, llm, cfg.Concurrency, progress)
	} else {
		changedFiles, changedErr := diff.ChangedFiles(repoRoot, state.LastCommit)
		if changedErr != nil {
			summaries, projectSummary, err = summarizer.SummarizeAll(ctx, modules, g, llm, cfg.Concurrency, progress)
		} else {
			affected := diff.AffectedModules(changedFiles, modules)
			summaries, projectSummary, err = summarizer.SummarizePartial(
				ctx,
				affected,
				state.Summaries,
				modules,
				g,
				llm,
				cfg.Concurrency,
				progress,
			)
		}
	}
	if cfg.Verbose {
		_, _ = fmt.Fprintln(deps.Stdout)
	}
	if err != nil {
		return err
	}

	outputPath := cfg.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(repoRoot, outputPath)
	}
	cliReference := buildCLIReference(rootCmd)
	if err := output.Write(outputPath, commitDisplay, projectSummary, modules, summaries, g, stats, cliReference); err != nil {
		return err
	}

	if gitRepo && commitHash != "" {
		if err := diff.SaveState(repoRoot, commitHash, cfg.Model, modules, summaries); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintf(deps.Stdout, "Wrote %s\n", outputPath)
	return nil
}

func progressFunc(out io.Writer, verbose bool) summarizer.ProgressFunc {
	if !verbose {
		return nil
	}
	return func(completed, total int) {
		_, _ = fmt.Fprintf(out, "\rSummarizing modules... %d/%d", completed, total)
	}
}

func buildCLIReference(rootCmd *cobra.Command) string {
	if rootCmd == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## CLI Commands\n\n")
	sb.WriteString("### `codelens`\n\n")
	sb.WriteString("```text\n")
	sb.WriteString(strings.TrimSpace(rootCmd.UsageString()))
	sb.WriteString("\n```\n\n")

	for _, sub := range rootCmd.Commands() {
		if sub.Name() != "configure" {
			continue
		}
		sb.WriteString("### `codelens configure`\n\n")
		sb.WriteString("```text\n")
		sb.WriteString(strings.TrimSpace(sub.UsageString()))
		sb.WriteString("\n```\n")
		break
	}

	return sb.String()
}

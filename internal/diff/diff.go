package diff

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/leandrotocalini/codelens/internal/parser"
)

// IsGitRepo checks if the given directory is a git repository.
func IsGitRepo(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// HeadCommit returns the current HEAD commit hash.
func HeadCommit(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// HeadCommitShort returns the short form of the current HEAD commit.
func HeadCommitShort(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsDirty checks for uncommitted changes.
func IsDirty(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// ChangedFiles returns files changed between lastCommit and HEAD.
func ChangedFiles(repoRoot, lastCommit string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--name-only", lastCommit, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// AffectedModules maps changed files to their modules.
func AffectedModules(changedFiles []string, modules []parser.Module) []parser.Module {
	// Build file -> module index
	fileToModule := make(map[string]string)
	for _, m := range modules {
		for _, f := range m.Files {
			fileToModule[f.Path] = m.Name
		}
	}

	// Find affected module names
	affected := make(map[string]bool)
	for _, f := range changedFiles {
		if modName, ok := fileToModule[f]; ok {
			affected[modName] = true
		}
	}

	// Also mark new modules (in parser output but not in fileToModule coverage)
	// This happens when new files don't match any known module path,
	// handled by being parsed fresh

	// Collect affected modules
	var result []parser.Module
	for _, m := range modules {
		if affected[m.Name] {
			result = append(result, m)
		}
	}

	return result
}

// CommitTimestamp returns the timestamp of a commit as a human-readable string.
func CommitTimestamp(repoRoot, commit string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "log", "-1", "--format=%cr", commit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting commit time: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

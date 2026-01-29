package git

import (
	"fmt"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var gitLog = l.Logger.WithField("logger", "GitCli")

// Init initializes a new git repository in the specified directory.
func (g *GitCli) Init(workdir string) error {
	gitLog.Debugf("[command]: git init (in %s)", workdir)

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "init")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git init failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

// SetSparseCheckout configures sparse checkout for the given directories.
// Runs: git config --local core.sparseCheckout true && git sparse-checkout set <directories...>
func (g *GitCli) SetSparseCheckout(workdir string, directories []string) error {
	gitLog.Debugf("Configuring sparse checkout: %v", directories)
	if len(directories) == 0 {
		return fmt.Errorf("directories parameter empty")
	}

	// Enable sparse checkout
	if err := g.ConfigLocal(workdir, "core.sparseCheckout", "true"); err != nil {
		return fmt.Errorf("failed to enable sparse checkout: %w", err)
	}

	// Write sparse checkout patterns using git sparse-checkout set
	args := append([]string{"sparse-checkout", "set"}, directories...)
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", args...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to set sparse checkout directories: %v (stderr: %s)", err, stderr)
	}
	return nil
}

// ConfigLocal sets a git config value locally in the repository.
func (g *GitCli) ConfigLocal(workdir, key, value string) error {
	gitArgs := []string{"config", "--local", key, value}
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git config failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

// Commit creates a commit with the specified message.
// Runs: git commit -m <message>
func (g *GitCli) Commit(workdir, message string) (string, error) {
	gitArgs := []string{"commit", "-m", message}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git commit failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// Merge merges the specified ref into the current branch with the given commit message.
// Uses --no-ff to always create a merge commit. Returns the merge output.
// If the merge is already up-to-date, no commit is created and no error is returned.
// Runs: git merge -m <message> --no-ff --allow-unrelated-histories <ref>
func (g *GitCli) Merge(workdir, ref, message string) (string, error) {
	gitArgs := []string{"merge", "-m", message, "--no-ff", "--allow-unrelated-histories", ref}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git merge failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

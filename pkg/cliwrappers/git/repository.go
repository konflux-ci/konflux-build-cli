package git

import (
	"fmt"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// Init initializes a new git repository in the specified directory.
func (g *Cli) Init(workdir string) error {
	l.Logger.Infof("[command]:\ngit init (in %s)", workdir)

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "init")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git init failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

func (g *Cli) SetSparseCheckout(workdir string, directories []string) error {
	if len(directories) == 0 {
		return fmt.Errorf("directories parameter empty")
	}
	l.Logger.Infof("Configuring sparse checkout: %v", directories)

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
func (g *Cli) ConfigLocal(workdir, key, value string) error {
	gitArgs := []string{"config", "--local", key, value}
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil {
		return fmt.Errorf("git config failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

func (g *Cli) Commit(workdir, targetBranch, remote, resultSHA string) (string, error) {
	gitArgs := []string{"commit", "-m", fmt.Sprintf("Merge branch '%s' from %s into %s", targetBranch, remote, resultSHA)}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil {
		return "", fmt.Errorf("git commit failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

func (g *Cli) Merge(workdir, fetchHead string) (string, error) {
	gitArgs := []string{"merge", fetchHead, "--no-commit", "--no-ff", "--allow-unrelated-histories"}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil {
		return "", fmt.Errorf("git merge failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

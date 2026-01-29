package git

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

// RemoteAdd adds a new remote with the given name and URL.
// Runs: git remote add <name> <url>
func (g *GitCli) RemoteAdd(workdir, name, url string) (string, error) {
	gitArgs := []string{"remote", "add", name, url}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git remote add failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// FetchWithRefspec fetches a specific refspec from a remote with optional depth and retry
func (g *GitCli) FetchWithRefspec(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
	gitArgs := []string{"fetch"}

	if submodules {
		gitArgs = append(gitArgs, "--recurse-submodules=yes")
	}

	if depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", depth))
	}

	gitArgs = append(gitArgs, remote, "--update-head-ok", "--force")

	if refspec != "" {
		gitArgs = append(gitArgs, refspec)
	}

	retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
		return g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	}).WithMaxAttempts(maxAttempts)

	_, stderr, exitCode, err := retryer.Run()
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git fetch failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

// Checkout checks out the specified ref (branch, tag, or commit SHA).
// Runs: git checkout <ref>
func (g *GitCli) Checkout(workdir, ref string) error {
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "checkout", ref)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git checkout failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

// SubmoduleUpdate initializes and/or updates submodules recursively.
// Runs: git submodule update --recursive [--init] [--force] [--depth=N] [paths...]
func (g *GitCli) SubmoduleUpdate(workdir string, init bool, depth int, paths []string) error {
	gitArgs := []string{"submodule", "update", "--recursive"}

	if init {
		gitArgs = append(gitArgs, "--init")
	}

	gitArgs = append(gitArgs, "--force")

	if depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", depth))
	}

	gitArgs = append(gitArgs, paths...)

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git submodule update failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

package git

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

func (g *Cli) RemoteAdd(workdir, name, url string) (string, error) {
	gitArgs := []string{"remote", "add", name, url}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git remote add failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// FetchTags fetches all tags from the remote and returns the list of tags.
func (g *Cli) FetchTags(workdir string) ([]string, error) {
	// Fetch tags from remote
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "fetch", "--tags")
	if err != nil {
		return nil, fmt.Errorf("git fetch --tags failed: %w (stderr: %s)", err, stderr)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("git fetch --tags failed with exit code %d (stderr: %s)", exitCode, stderr)
	}

	// List all tags
	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "tag", "-l")
	if err != nil {
		return nil, fmt.Errorf("git tag -l failed: %w (stderr: %s)", err, stderr)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("git tag -l failed with exit code %d (stderr: %s)", exitCode, stderr)
	}

	// Parse tags from output (one per line)
	tags := []string{}
	for _, tag := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// FetchWithRefspec fetches a specific refspec from a remote with optional depth and retry
func (g *Cli) FetchWithRefspec(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
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

// Checkout checks out a specific ref (branch, tag, or commit)
func (g *Cli) Checkout(workdir, ref string) error {
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "checkout", ref)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git checkout failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

// SubmoduleUpdate initializes and/or updates submodules
func (g *Cli) SubmoduleUpdate(workdir string, init bool, depth int, paths []string) error {
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

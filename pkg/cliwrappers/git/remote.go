package git

import (
	"fmt"
	"strings"
)

func (g *Cli) RemoteAdd(workdir, name, url string) (string, error) {
	gitArgs := []string{"remote", "add", name, url}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git remote add failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

<<<<<<< Updated upstream
func (g *Cli) Fetch(workdir, repository string, depth int) (string, error) {
	gitArgs := []string{"fetch", repository}

	if depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", depth))
	}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil {
		return "", fmt.Errorf("git fetch failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
=======
func (g *Cli) FetchTags(workdir string) (string, error) {
	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "fetch", "--tags")
	if err != nil {
		return "", fmt.Errorf("git fetch --tags failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return stdout, nil
>>>>>>> Stashed changes
}

// FetchWithRefspec fetches a specific refspec from a remote with optional depth
func (g *Cli) FetchWithRefspec(workdir, remote, refspec string, depth int) error {
	gitArgs := []string{"fetch"}

	if depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", depth))
	}

	gitArgs = append(gitArgs, remote)

	if refspec != "" {
		gitArgs = append(gitArgs, refspec)
	}

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
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
func (g *Cli) SubmoduleUpdate(workdir string, init bool, paths string) error {
	gitArgs := []string{"submodule", "update"}

	if init {
		gitArgs = append(gitArgs, "--init")
	}

	if paths != "" {
		// Split comma-separated paths
		for _, path := range strings.Split(paths, ",") {
			path = strings.TrimSpace(path)
			if path != "" {
				gitArgs = append(gitArgs, path)
			}
		}
	}

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("git submodule update failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return nil
}

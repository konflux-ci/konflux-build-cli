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


package git

import (
	"fmt"
	"strings"
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

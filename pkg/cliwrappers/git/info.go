package git

import (
	"fmt"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

func (g *Cli) RevParse(workdir string, ref string, short bool, length int) (string, error) {
	gitArgs := []string{"rev-parse"}

	if short {
		if length > 0 {
			gitArgs = append(gitArgs, fmt.Sprintf("--short=%d", length))
		} else {
			gitArgs = append(gitArgs, "--short")
		}
	}
	gitArgs = append(gitArgs, ref)

	l.Logger.Debugf("[command]:\ngit %s (in %s)", strings.Join(gitArgs, " "), workdir)

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git rev-parse failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

// Log runs git log and returns the result
func (g *Cli) Log(workdir string, format string, count int) (string, error) {
	gitArgs := []string{"log"}

	if count > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("-%d", count))
	}
	if format != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--pretty=%s", format))
	}

	l.Logger.Debugf("[command]:\ngit %s (in %s)", strings.Join(gitArgs, " "), workdir)

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("git log failed with exit code %d: %v (stderr: %s)", exitCode, err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

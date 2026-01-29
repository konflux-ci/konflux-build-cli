package git

import (
	"fmt"

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

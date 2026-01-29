package git

import (
	"fmt"

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

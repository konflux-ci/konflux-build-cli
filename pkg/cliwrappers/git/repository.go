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

func (g *Cli) SetSparseCheckout(workdir, sparseCheckoutDirectories string) error {
	if sparseCheckoutDirectories == "" {
		return fmt.Errorf("sparseCheckoutDirectories parameter empty")
	}
	l.Logger.Infof("Configuring sparse checkout: %s", sparseCheckoutDirectories)

	// Enable sparse checkout
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(workdir, "git", "config", "core.sparseCheckout", "true")
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to enable sparse checkout: %v (stderr: %s)", err, stderr)
	}

	// Write sparse checkout patterns using git sparse-checkout set
	args := append([]string{"sparse-checkout", "set"}, strings.Split(sparseCheckoutDirectories, ",")...)
	_, stderr, exitCode, err = g.Executor.ExecuteInDir(workdir, "git", args...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to set sparse checkout directories: %v (stderr: %s)", err, stderr)
	}
	return nil
}

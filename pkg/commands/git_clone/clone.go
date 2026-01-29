package git_clone

import (
	"fmt"
	"os"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// performClone initializes a git repo, fetches the requested revision, and checks it out.
func (c *GitClone) performClone() error {
	checkoutDir := c.getCheckoutDir()

	// Ensure checkout directory exists
	if err := os.MkdirAll(checkoutDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkout directory: %w", err)
	}

	l.Logger.Debug("Initializing git repository")
	if err := c.CliWrappers.GitCli.Init(checkoutDir); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	return nil
}

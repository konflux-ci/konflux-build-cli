package git_clone

import (
	"fmt"
	"os"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

func (c *GitClone) performClone() error {
	checkoutDir := c.getCheckoutDir()

	// Ensure checkout directory exists
	if err := os.MkdirAll(checkoutDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkout directory: %w", err)
	}

	l.Logger.Info("Initializing git repository")
	if err := c.CliWrappers.GitCli.Init(checkoutDir); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	l.Logger.Infof("Adding remote origin: %s", c.Params.Url)
	if _, err := c.CliWrappers.GitCli.RemoteAdd(checkoutDir, "origin", c.Params.Url); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}

	if err := c.fetchRevision(checkoutDir); err != nil {
		return err
	}

	l.Logger.Info("Checking out FETCH_HEAD")
	if err := c.CliWrappers.GitCli.Checkout(checkoutDir, "FETCH_HEAD"); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}
	return nil
}

// fetchRevision fetches the appropriate refs based on refspec and revision parameters
func (c *GitClone) fetchRevision(checkoutDir string) error {
	// Determine what to fetch
	refspec := c.Params.Refspec
	if refspec == "" && c.Params.Revision != "" {
		// If no refspec but we have a revision, fetch that specific ref
		refspec = c.Params.Revision
	}

	l.Logger.Infof("Fetching from origin (depth=%d, refspec=%s)", c.Params.Depth, refspec)

	// Ensure at least 1 attempt
	maxAttempts := c.Params.RetryMaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Retry logic for network operations
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			l.Logger.Infof("Retrying fetch (attempt %d of %d)", attempt, maxAttempts)
		}

		err := c.CliWrappers.GitCli.FetchWithRefspec(checkoutDir, "origin", refspec, c.Params.Depth)
		if err == nil {
			return nil
		}
		lastErr = err
		l.Logger.Warnf("Fetch attempt %d failed: %v", attempt, err)
	}

	return fmt.Errorf("git fetch failed after %d attempts: %w", maxAttempts, lastErr)
}

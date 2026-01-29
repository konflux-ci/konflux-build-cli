package git_clone

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

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

	l.Logger.Debugf("Adding remote origin: %s", c.Params.URL)
	if _, err := c.CliWrappers.GitCli.RemoteAdd(checkoutDir, "origin", c.Params.URL); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}

	if err := c.fetchRevision(checkoutDir); err != nil {
		return err
	}

	// If both refspec and revision are set, the refspec is fetched first,
	// then the specific revision is checked out. Otherwise, check out FETCH_HEAD.
	checkoutRef := "FETCH_HEAD"
	if c.Params.Refspec != "" && c.Params.Revision != "" {
		checkoutRef = c.Params.Revision
	}

	l.Logger.Debugf("Checking out %s", checkoutRef)
	if err := c.CliWrappers.GitCli.Checkout(checkoutDir, checkoutRef); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	if c.Params.Submodules {
		l.Logger.Debug("Updating submodules")
		paths, err := parseCSV(c.Params.SubmodulePaths)
		if err != nil {
			return fmt.Errorf("failed to parse submodule-paths: %w", err)
		}
		if err := c.CliWrappers.GitCli.SubmoduleUpdate(checkoutDir, true, c.Params.Depth, paths); err != nil {
			return fmt.Errorf("git submodule update failed: %w", err)
		}
	}

	return nil
}

// fetchRevision fetches refs from the remote based on refspec and revision parameters.
// If a refspec is provided, it is fetched directly. Otherwise, the revision is used as the refspec.
func (c *GitClone) fetchRevision(checkoutDir string) error {
	// Determine what to fetch
	refspec := c.Params.Refspec
	if refspec == "" && c.Params.Revision != "" {
		// If no refspec but we have a revision, fetch that specific ref
		refspec = c.Params.Revision
	}

	l.Logger.Debugf("Fetching from origin (depth=%d, refspec=%s)", c.Params.Depth, refspec)

	// Ensure at least 1 attempt
	maxAttempts := c.Params.RetryMaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	err := c.CliWrappers.GitCli.FetchWithRefspec(checkoutDir, "origin", refspec, c.Params.Depth, c.Params.Submodules, maxAttempts)
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	return nil
}

// parseCSV parses a comma-separated string into a slice of trimmed values.
func parseCSV(input string) ([]string, error) {
	if input == "" {
		return nil, nil
	}
	reader := csv.NewReader(strings.NewReader(input))
	reader.TrimLeadingSpace = true
	return reader.Read()
}

package git_clone

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// cleanCheckoutDir removes all contents from the checkout directory.
func (c *GitClone) cleanCheckoutDir() error {
	checkoutDir := c.getCheckoutDir()

	info, err := os.Stat(checkoutDir)
	if os.IsNotExist(err) {
		// Directory doesn't exist, nothing to clean
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat checkout directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("checkout path exists but is not a directory: %s", checkoutDir)
	}

	l.Logger.Infof("Cleaning existing checkout directory: %s", checkoutDir)

	entries, err := os.ReadDir(checkoutDir)
	if err != nil {
		return fmt.Errorf("failed to read checkout directory: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(checkoutDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entryPath, err)
		}
	}

	return nil
}

// setupProxies sets HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
// if the corresponding parameters are provided.
func (c *GitClone) setupProxies() {
	if c.Params.HttpProxy != "" {
		l.Logger.Infof("Setting HTTP_PROXY=%s", c.Params.HttpProxy)
		os.Setenv("HTTP_PROXY", c.Params.HttpProxy)
	}
	if c.Params.HttpsProxy != "" {
		l.Logger.Infof("Setting HTTPS_PROXY=%s", c.Params.HttpsProxy)
		os.Setenv("HTTPS_PROXY", c.Params.HttpsProxy)
	}
	if c.Params.NoProxy != "" {
		l.Logger.Infof("Setting NO_PROXY=%s", c.Params.NoProxy)
		os.Setenv("NO_PROXY", c.Params.NoProxy)
	}
}


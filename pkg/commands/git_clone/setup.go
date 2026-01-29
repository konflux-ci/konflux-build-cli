package git_clone

import (
	"fmt"
	"os"
	"path/filepath"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// cleanCheckoutDir removes all contents from the checkout directory while preserving
// the directory itself. We iterate over entries rather than using os.RemoveAll on the
// directory because the checkout directory may be a mount point (e.g., a Kubernetes
// volume) that should not be removed.
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

// setupGitConfig configures git settings for SSL verification and CA bundle.
func (c *GitClone) setupGitConfig() error {
	if !c.Params.SSLVerify {
		l.Logger.Info("Disabling SSL verification (http.sslVerify=false)")
		if err := c.CliWrappers.GitCli.ConfigLocal("", "http.sslVerify", "false"); err != nil {
			return fmt.Errorf("failed to configure http.sslVerify: %w", err)
		}
	}

	caBundlePath := c.Params.CaBundlePath
	if caBundlePath == "" {
		caBundlePath = "/mnt/trusted-ca/ca-bundle.crt"
	}
	if _, err := os.Stat(caBundlePath); err == nil {
		l.Logger.Infof("Using mounted CA bundle: %s", caBundlePath)
		if err := c.CliWrappers.GitCli.ConfigLocal("", "http.sslCAInfo", caBundlePath); err != nil {
			return fmt.Errorf("failed to configure http.sslCAInfo: %w", err)
		}
	}

	return nil
}

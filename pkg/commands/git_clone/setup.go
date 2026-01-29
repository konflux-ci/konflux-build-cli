package git_clone

import (
	"fmt"
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

// setupGitConfig configures git settings for SSL verification and CA bundle.
func (c *GitClone) setupGitConfig() error {
	if !c.Params.SslVerify {
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

// setupBasicAuth sets up git credentials from a basic-auth workspace.
// Supports two formats:
// 1. .git-credentials and .gitconfig files (copied directly)
// 2. username and password files (kubernetes.io/basic-auth secret format)
func (c *GitClone) setupBasicAuth() error {
	if c.Params.BasicAuthDirectory == "" {
		return nil
	}

	authDir := c.Params.BasicAuthDirectory
	userHome := c.Params.UserHome

	// Check if the auth directory exists
	if _, err := os.Stat(authDir); os.IsNotExist(err) {
		l.Logger.Infof("Basic auth directory not found: %s", authDir)
		return nil
	}

	gitCredentialsPath := filepath.Join(authDir, ".git-credentials")
	gitConfigPath := filepath.Join(authDir, ".gitconfig")
	usernamePath := filepath.Join(authDir, "username")
	passwordPath := filepath.Join(authDir, "password")

	// Format 1: .git-credentials and .gitconfig files
	if fileExists(gitCredentialsPath) && fileExists(gitConfigPath) {
		l.Logger.Info("Setting up basic auth from .git-credentials and .gitconfig")

		destCredentials := filepath.Join(userHome, ".git-credentials")
		destConfig := filepath.Join(userHome, ".gitconfig")

		if err := copyFile(gitCredentialsPath, destCredentials, 0400); err != nil {
			return fmt.Errorf("failed to copy .git-credentials: %w", err)
		}
		if err := copyFile(gitConfigPath, destConfig, 0400); err != nil {
			return fmt.Errorf("failed to copy .gitconfig: %w", err)
		}

		l.Logger.Info("Basic auth credentials configured")
		return nil
	}

	// Format 2: kubernetes.io/basic-auth secret (username and password files)
	if fileExists(usernamePath) && fileExists(passwordPath) {
		l.Logger.Info("Setting up basic auth from username/password files")

		username, err := os.ReadFile(usernamePath)
		if err != nil {
			return fmt.Errorf("failed to read username file: %w", err)
		}

		password, err := os.ReadFile(passwordPath)
		if err != nil {
			return fmt.Errorf("failed to read password file: %w", err)
		}

		// Extract hostname from URL
		parsedURL, err := url.Parse(c.Params.Url)
		if err != nil {
			return fmt.Errorf("failed to parse repository URL: %w", err)
		}
		hostname := parsedURL.Host

		// Create .git-credentials file
		credentialsContent := fmt.Sprintf("https://%s:%s@%s\n",
			strings.TrimSpace(string(username)),
			strings.TrimSpace(string(password)),
			hostname)

		destCredentials := filepath.Join(userHome, ".git-credentials")
		if err := os.WriteFile(destCredentials, []byte(credentialsContent), 0400); err != nil {
			return fmt.Errorf("failed to write .git-credentials: %w", err)
		}

		// Create .gitconfig file
		gitConfigContent := fmt.Sprintf("[credential \"https://%s\"]\n  helper = store\n", hostname)
		destConfig := filepath.Join(userHome, ".gitconfig")
		if err := os.WriteFile(destConfig, []byte(gitConfigContent), 0400); err != nil {
			return fmt.Errorf("failed to write .gitconfig: %w", err)
		}

		l.Logger.Infof("Basic auth credentials configured for %s", hostname)
		return nil
	}

	return fmt.Errorf("unknown basic-auth workspace format: expected .git-credentials/.gitconfig or username/password files")
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// copyFile copies a file from src to dest with the specified permissions.
func copyFile(src, dest string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, perm)
}

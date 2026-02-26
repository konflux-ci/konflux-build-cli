package git_clone

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
// Uses environment variables instead of git config to avoid modifying global git state.
func (c *GitClone) setupGitConfig() error {
	if !c.Params.SSLVerify {
		l.Logger.Info("Disabling SSL verification (GIT_SSL_NO_VERIFY=true)")
		if err := os.Setenv("GIT_SSL_NO_VERIFY", "true"); err != nil {
			return err
		}
	}

	caBundlePath := c.Params.CaBundlePath
	if caBundlePath == "" {
		caBundlePath = "/mnt/trusted-ca/ca-bundle.crt"
	}
	if _, err := os.Stat(caBundlePath); err == nil {
		l.Logger.Infof("Using mounted CA bundle: %s", caBundlePath)
		if err := os.Setenv("GIT_SSL_CAINFO", caBundlePath); err != nil {
			return err
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

	// Check if the auth directory exists
	if _, err := os.Stat(authDir); os.IsNotExist(err) {
		l.Logger.Infof("Basic auth directory not found: %s", authDir)
		return nil
	}

	gitCredentialsPath := filepath.Join(authDir, ".git-credentials")
	gitConfigPath := filepath.Join(authDir, ".gitconfig")
	usernamePath := filepath.Join(authDir, "username")
	passwordPath := filepath.Join(authDir, "password")

	destCredentials := filepath.Join(c.internalDir, ".git-credentials")
	destConfig := filepath.Join(c.internalDir, ".gitconfig")

	// Format 1: .git-credentials and .gitconfig files
	if fileExists(gitCredentialsPath) && fileExists(gitConfigPath) {
		l.Logger.Info("Setting up basic auth from .git-credentials and .gitconfig")

		if err := copyFile(gitCredentialsPath, destCredentials, 0400); err != nil {
			return fmt.Errorf("failed to copy .git-credentials: %w", err)
		}

		// Read the original .gitconfig and rewrite credential helper to use explicit --file path
		configContent, err := os.ReadFile(gitConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read .gitconfig: %w", err)
		}
		rewritten := rewriteGitConfigCredentialHelper(string(configContent), destCredentials)
		if err := os.WriteFile(destConfig, []byte(rewritten), 0400); err != nil {
			return fmt.Errorf("failed to write .gitconfig: %w", err)
		}

		if err := os.Setenv("GIT_CONFIG_GLOBAL", destConfig); err != nil {
			return err
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
		parsedURL, err := url.Parse(c.Params.URL)
		if err != nil {
			return fmt.Errorf("failed to parse repository URL: %w", err)
		}
		hostname := parsedURL.Host

		// Create .git-credentials file
		credentialsContent := fmt.Sprintf("https://%s:%s@%s\n",
			strings.TrimSpace(string(username)),
			strings.TrimSpace(string(password)),
			hostname)

		if err := os.WriteFile(destCredentials, []byte(credentialsContent), 0400); err != nil {
			return fmt.Errorf("failed to write .git-credentials: %w", err)
		}

		// Create .gitconfig file with explicit --file path for credential store
		gitConfigContent := fmt.Sprintf("[credential \"https://%s\"]\n  helper = store --file=%s\n", hostname, destCredentials)
		if err := os.WriteFile(destConfig, []byte(gitConfigContent), 0400); err != nil {
			return fmt.Errorf("failed to write .gitconfig: %w", err)
		}

		if err := os.Setenv("GIT_CONFIG_GLOBAL", destConfig); err != nil {
			return err
		}

		l.Logger.Infof("Basic auth credentials configured for %s", hostname)
		return nil
	}

	return fmt.Errorf("unknown basic-auth workspace format: expected .git-credentials/.gitconfig or username/password files")
}

// rewriteGitConfigCredentialHelper rewrites "helper = store" lines in a git config
// to include an explicit --file flag pointing to the given credentials path.
func rewriteGitConfigCredentialHelper(configContent, credentialsPath string) string {
	lines := strings.Split(configContent, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "helper = store" {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = fmt.Sprintf("%shelper = store --file=%s", indent, credentialsPath)
		}
	}
	return strings.Join(lines, "\n")
}

// setupSSH sets up SSH keys from an ssh-directory workspace.
// SSH files are copied to c.internalDir/.ssh/ and GIT_SSH_COMMAND is set
// with explicit flags so that git uses the custom SSH config without modifying $HOME.
func (c *GitClone) setupSSH() error {
	if c.Params.SSHDirectory == "" {
		return nil
	}

	sshDir := c.Params.SSHDirectory

	// Check if the SSH directory exists
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		l.Logger.Infof("SSH directory not found: %s", sshDir)
		return nil
	}

	l.Logger.Infof("Setting up SSH keys from %s", sshDir)

	destSSHDir := filepath.Join(c.internalDir, ".ssh")

	// Create destination .ssh directory
	if err := os.MkdirAll(destSSHDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Copy all files from source to destination
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return fmt.Errorf("failed to read SSH directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		srcPath := filepath.Join(sshDir, entry.Name())
		destPath := filepath.Join(destSSHDir, entry.Name())

		if err := copyFile(srcPath, destPath, 0400); err != nil {
			return fmt.Errorf("failed to copy SSH file %s: %w", entry.Name(), err)
		}
	}

	// Build GIT_SSH_COMMAND with explicit flags
	sshCmd := "ssh"

	// Add identity files for private keys (files matching id_* without .pub suffix)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "id_") && !strings.HasSuffix(name, ".pub") && !entry.IsDir() {
			sshCmd += fmt.Sprintf(" -i %s", filepath.Join(destSSHDir, name))
		}
	}

	// Add known_hosts if present
	knownHostsPath := filepath.Join(destSSHDir, "known_hosts")
	if fileExists(knownHostsPath) {
		sshCmd += fmt.Sprintf(" -o UserKnownHostsFile=%s", knownHostsPath)
	}

	if err := os.Setenv("GIT_SSH_COMMAND", sshCmd); err != nil {
		return err
	}

	l.Logger.Infof("SSH keys configured (GIT_SSH_COMMAND=%s)", sshCmd)
	return nil
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

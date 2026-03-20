package gitclone

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/spf13/cobra"
)

type CliWrappers struct {
	GitCli cliwrappers.GitCliInterface
}

type GitClone struct {
	Params        *Params
	CliWrappers   CliWrappers
	Results       Results
	ResultsWriter common.ResultsWriterInterface
}

func New(cmd *cobra.Command) (*GitClone, error) {
	gitClone := &GitClone{}

	params := &Params{}
	if err := common.ParseParameters(cmd, ParamsConfig, params); err != nil {
		return nil, err
	}
	gitClone.Params = params

	gitClone.ResultsWriter = common.NewResultsWriter()

	return gitClone, nil
}

func (c *GitClone) initCliWrappers() error {
	executor := cliwrappers.NewCliExecutor()

	gitCli, err := cliwrappers.NewGitCli(executor, c.getCheckoutDir())
	if err != nil {
		return err
	}
	c.CliWrappers.GitCli = gitCli

	return nil
}

func (c *GitClone) Run() error {
	c.logParams()

	if err := c.validateParams(); err != nil {
		return err
	}

	if c.CliWrappers.GitCli == nil {
		if err := c.initCliWrappers(); err != nil {
			return err
		}
	}

	return nil
}

func (c *GitClone) logParams() {
	// Log URL params separately with sanitization to avoid leaking credentials
	l.Logger.Infof("[param] url: %s", sanitizeURL(c.Params.URL))
	if c.Params.MergeSourceRepoURL != "" {
		l.Logger.Infof("[param] merge-source-repo-url: %s", sanitizeURL(c.Params.MergeSourceRepoURL))
	}
	common.LogParameters(ParamsConfig, c.Params, "url", "merge-source-repo-url")
}

// sanitizeURL removes credentials from a URL for safe logging.
func sanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
	}
	return parsed.String()
}

func (c *GitClone) validateParams() error {
	if c.Params.URL == "" {
		return fmt.Errorf("url parameter is required")
	}
	if c.Params.Depth < 0 {
		return fmt.Errorf("depth must be >= 0 (0 means full history)")
	}
	if c.Params.MergeSourceDepth < 0 {
		return fmt.Errorf("merge-source-depth must be >= 0 (0 means full history)")
	}
	if c.Params.RetryMaxAttempts < 1 {
		return fmt.Errorf("retry-max-attempts must be >= 1, got %d", c.Params.RetryMaxAttempts)
	}
	if c.Params.RetryMaxAttempts > 100 {
		return fmt.Errorf("retry-max-attempts must be <= 100, got %d", c.Params.RetryMaxAttempts)
	}

	// Validate subdirectory for path traversal
	if c.Params.Subdirectory != "" {
		if filepath.IsAbs(c.Params.Subdirectory) {
			return fmt.Errorf("subdirectory must be a relative path, got absolute path: %s", c.Params.Subdirectory)
		}
		if strings.Contains(c.Params.Subdirectory, "..") {
			return fmt.Errorf("subdirectory must not contain path traversal (..): %s", c.Params.Subdirectory)
		}
		// Verify the resolved path stays within OutputDir.
		baseDir := c.Params.OutputDir
		if baseDir == "" {
			baseDir = "."
		}
		absOutput, err := filepath.Abs(baseDir)
		if err != nil {
			return fmt.Errorf("failed to resolve output dir: %w", err)
		}
		absCheckout, err := filepath.Abs(filepath.Join(baseDir, c.Params.Subdirectory))
		if err != nil {
			return fmt.Errorf("failed to resolve checkout dir: %w", err)
		}
		if absCheckout != absOutput && !strings.HasPrefix(absCheckout, absOutput+string(filepath.Separator)) {
			return fmt.Errorf("subdirectory must not escape output directory")
		}
	}

	return nil
}

func (c *GitClone) getCheckoutDir() string {
	return filepath.Join(c.Params.OutputDir, c.Params.Subdirectory)
}

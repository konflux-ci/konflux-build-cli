package gitclone

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers/git"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/spf13/cobra"
)

type CliWrappers struct {
	GitCli git.CliInterface
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

	gitCli, err := git.NewGitCli(executor, c.getCheckoutDir())
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
	l.Logger.Infof("[param] URL: %s", sanitizeURL(c.Params.URL))
	l.Logger.Infof("[param] Depth: %d", c.Params.Depth)
	l.Logger.Infof("[param] Submodules: %t", c.Params.Submodules)
	l.Logger.Infof("[param] SSL verify: %t", c.Params.SSLVerify)
	l.Logger.Infof("[param] Delete existing: %t", c.Params.DeleteExisting)
	l.Logger.Infof("[param] Enable symlink check: %t", c.Params.EnableSymlinkCheck)
	l.Logger.Infof("[param] Fetch tags: %t", c.Params.FetchTags)
	l.Logger.Infof("[param] Merge target branch: %t", c.Params.MergeTargetBranch)
	logIfNotEmpty("[param] Revision", c.Params.Revision)
	logIfNotEmpty("[param] Refspec", c.Params.Refspec)
	logIfNotEmpty("[param] Submodule paths", c.Params.SubmodulePaths)
	logIfNotEmpty("[param] Output dir", c.Params.OutputDir)
	logIfNotEmpty("[param] Subdirectory", c.Params.Subdirectory)
	logIfNotEmpty("[param] Sparse checkout directories", c.Params.SparseCheckoutDirectories)
	logIfNotEmpty("[param] Target branch", c.Params.TargetBranch)
	logIfNotEmpty("[param] Merge source repo URL", sanitizeURL(c.Params.MergeSourceRepoURL))
	logIfNotEmpty("[param] Basic auth directory", c.Params.BasicAuthDirectory)
	logIfNotEmpty("[param] SSH directory", c.Params.SSHDirectory)
	logIfNotEmpty("[param] Merge commit author name", c.Params.MergeCommitAuthorName)
	logIfNotEmpty("[param] Merge commit author email", c.Params.MergeCommitAuthorEmail)
	if c.Params.MergeSourceDepth != 0 {
		l.Logger.Infof("[param] Merge source depth: %d", c.Params.MergeSourceDepth)
	}
	if c.Params.ShortCommitLength != 7 {
		l.Logger.Infof("[param] Short commit length: %d", c.Params.ShortCommitLength)
	}
	if c.Params.RetryMaxAttempts != 10 {
		l.Logger.Infof("[param] Retry max attempts: %d", c.Params.RetryMaxAttempts)
	}
}

func logIfNotEmpty(label, value string) {
	if value != "" {
		l.Logger.Infof("%s: %s", label, value)
	}
}

// sanitizeURL removes credentials from a URL for safe logging.
func sanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return as-is if parsing fails
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

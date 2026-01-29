package git_clone

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

	if err := gitClone.initCliWrappers(); err != nil {
		return nil, err
	}

	gitClone.ResultsWriter = common.NewResultsWriter()

	return gitClone, nil
}

func (c *GitClone) initCliWrappers() error {
	executor := cliwrappers.NewCliExecutor()

	gitCli, err := git.NewCli(executor)
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

	// Clean checkout directory if requested
	if c.Params.DeleteExisting {
		if err := c.cleanCheckoutDir(); err != nil {
			return err
		}
	}

	if err := c.performClone(); err != nil {
		return err
	}

	if c.Params.MergeTargetBranch {
		if err := c.mergeTargetBranch(); err != nil {
			return err
		}
	}

	if c.Params.EnableSymlinkCheck {
		if err := common.CheckSymlinks(c.getCheckoutDir()); err != nil {
			return fmt.Errorf("symlink check: %w", err)
		}
	}

	if err := c.gatherCommitInfo(); err != nil {
		return err
	}

	if c.Params.FetchTags {
		if _, err := c.CliWrappers.GitCli.FetchTags(c.getCheckoutDir()); err != nil {
			return err
		}
	}

	return c.outputResults()
}

func (c *GitClone) logParams() {
	// Always log required and important params
	l.Logger.Infof("[param] URL: %s", sanitizeURL(c.Params.URL))
	l.Logger.Infof("[param] Depth: %d", c.Params.Depth)
	l.Logger.Infof("[param] Submodules: %t", c.Params.Submodules)
	l.Logger.Infof("[param] SSL verify: %t", c.Params.SSLVerify)
	l.Logger.Infof("[param] Output dir: %s", c.Params.OutputDir)
	l.Logger.Infof("[param] Delete existing: %t", c.Params.DeleteExisting)
	l.Logger.Infof("[param] Enable symlink check: %t", c.Params.EnableSymlinkCheck)
	l.Logger.Infof("[param] Fetch tags: %t", c.Params.FetchTags)
	l.Logger.Infof("[param] Merge target branch: %t", c.Params.MergeTargetBranch)
	l.Logger.Infof("[param] Short commit length: %d", c.Params.ShortCommitLength)
	l.Logger.Infof("[param] Retry max attempts: %d", c.Params.RetryMaxAttempts)

	// Only log optional string params if they have values
	if c.Params.Revision != "" {
		l.Logger.Infof("[param] Revision: %s", c.Params.Revision)
	}
	if c.Params.Refspec != "" {
		l.Logger.Infof("[param] Refspec: %s", c.Params.Refspec)
	}
	if c.Params.SubmodulePaths != "" {
		l.Logger.Infof("[param] Submodule paths: %s", c.Params.SubmodulePaths)
	}
	if c.Params.Subdirectory != "" {
		l.Logger.Infof("[param] Subdirectory: %s", c.Params.Subdirectory)
	}
	if c.Params.SparseCheckoutDirectories != "" {
		l.Logger.Infof("[param] Sparse checkout directories: %s", c.Params.SparseCheckoutDirectories)
	}
	if c.Params.TargetBranch != "" {
		l.Logger.Infof("[param] Target branch: %s", c.Params.TargetBranch)
	}
	if c.Params.MergeSourceRepoURL != "" {
		l.Logger.Infof("[param] Merge source repo URL: %s", sanitizeURL(c.Params.MergeSourceRepoURL))
	}
	if c.Params.MergeSourceDepth != 0 {
		l.Logger.Infof("[param] Merge source depth: %d", c.Params.MergeSourceDepth)
	}
	if c.Params.BasicAuthDirectory != "" {
		l.Logger.Infof("[param] Basic auth directory: %s", c.Params.BasicAuthDirectory)
	}
	if c.Params.SSHDirectory != "" {
		l.Logger.Infof("[param] SSH directory: %s", c.Params.SSHDirectory)
	}
	if c.Params.CaBundlePath != "" {
		l.Logger.Infof("[param] CA bundle path: %s", c.Params.CaBundlePath)
	}
}

// sanitizeURL removes credentials from a URL for safe logging.
func sanitizeURL(rawURL string) string {
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

	// Validate subdirectory for path traversal
	if c.Params.Subdirectory != "" {
		if filepath.IsAbs(c.Params.Subdirectory) {
			return fmt.Errorf("subdirectory must be a relative path, got absolute path: %s", c.Params.Subdirectory)
		}
		if strings.Contains(c.Params.Subdirectory, "..") {
			return fmt.Errorf("subdirectory must not contain path traversal (..): %s", c.Params.Subdirectory)
		}
	}

	return nil
}

func (c *GitClone) getCheckoutDir() string {
	return filepath.Join(c.Params.OutputDir, c.Params.Subdirectory)
}

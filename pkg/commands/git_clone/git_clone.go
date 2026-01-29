package git_clone

import (
	"fmt"
	"os"
	"path/filepath"

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
	internalDir   string
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

	internalDir, err := os.MkdirTemp("", "git-clone-internal-*")
	if err != nil {
		return fmt.Errorf("failed to create internal directory: %w", err)
	}
	c.internalDir = internalDir
	defer func() {
		_ = os.RemoveAll(c.internalDir)
		_ = os.Unsetenv("GIT_CONFIG_GLOBAL")
	}()

	// Set proxy environment variables if there are any
	if err := c.setupProxies(); err != nil {
		return err
	}

	if err := c.setupGitConfig(); err != nil {
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
	l.Logger.Infof("[param] URL: %s", c.Params.URL)
	l.Logger.Infof("[param] Revision: %s", c.Params.Revision)
	l.Logger.Infof("[param] Refspec: %s", c.Params.Refspec)
	l.Logger.Infof("[param] Depth: %d", c.Params.Depth)
	l.Logger.Infof("[param] Submodules: %t", c.Params.Submodules)
	l.Logger.Infof("[param] Submodule paths: %s", c.Params.SubmodulePaths)
	l.Logger.Infof("[param] SSL verify: %t", c.Params.SSLVerify)
	l.Logger.Infof("[param] Output dir: %s", c.Params.OutputDir)
	l.Logger.Infof("[param] Subdirectory: %s", c.Params.Subdirectory)
	l.Logger.Infof("[param] Sparse checkout directories: %s", c.Params.SparseCheckoutDirectories)
	l.Logger.Infof("[param] Delete existing: %t", c.Params.DeleteExisting)
	l.Logger.Infof("[param] Enable symlink check: %t", c.Params.EnableSymlinkCheck)
	l.Logger.Infof("[param] Fetch tags: %t", c.Params.FetchTags)
	l.Logger.Infof("[param] Merge target branch: %t", c.Params.MergeTargetBranch)
	l.Logger.Infof("[param] Target branch: %s", c.Params.TargetBranch)
	l.Logger.Infof("[param] Merge source repo URL: %s", c.Params.MergeSourceRepoURL)
	l.Logger.Infof("[param] Merge source depth: %d", c.Params.MergeSourceDepth)
	l.Logger.Infof("[param] Basic auth directory: %s", c.Params.BasicAuthDirectory)
	l.Logger.Infof("[param] SSH directory: %s", c.Params.SSHDirectory)
	l.Logger.Infof("[param] HTTP proxy: %s", c.Params.HTTPProxy)
	l.Logger.Infof("[param] HTTPS proxy: %s", c.Params.HTTPSProxy)
	l.Logger.Infof("[param] No proxy: %s", c.Params.NoProxy)
	l.Logger.Infof("[param] Short commit length: %d", c.Params.ShortCommitLength)
	l.Logger.Infof("[param] CA bundle path: %s", c.Params.CaBundlePath)
	l.Logger.Infof("[param] Retry max attempts: %d", c.Params.RetryMaxAttempts)
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
	return nil
}

func (c *GitClone) getCheckoutDir() string {
	return filepath.Join(c.Params.OutputDir, c.Params.Subdirectory)
}

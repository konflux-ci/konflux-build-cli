package git_clone

import (
	"fmt"
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
		if err := c.checkSymlinks(c.getCheckoutDir()); err != nil {
			return err
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
	l.Logger.Infof("[param] URL: %s", c.Params.Url)
	if c.Params.Revision != "" {
		l.Logger.Infof("[param] Revision: %s", c.Params.Revision)
	}
	if c.Params.Refspec != "" {
		l.Logger.Infof("[param] Refspec: %s", c.Params.Refspec)
	}
	l.Logger.Infof("[param] Depth: %d", c.Params.Depth)
	l.Logger.Infof("[param] Submodules: %t", c.Params.Submodules)
	if c.Params.SubmodulePaths != "" {
		l.Logger.Infof("[param] Submodule paths: %s", c.Params.SubmodulePaths)
	}
	l.Logger.Infof("[param] SSL verify: %t", c.Params.SslVerify)
	l.Logger.Infof("[param] Output dir: %s", c.Params.OutputDir)
	l.Logger.Infof("[param] Subdirectory: %s", c.Params.Subdirectory)
	if c.Params.SparseCheckoutDirectories != "" {
		l.Logger.Infof("[param] Sparse checkout directories: %s", c.Params.SparseCheckoutDirectories)
	}
	l.Logger.Infof("[param] Delete existing: %t", c.Params.DeleteExisting)
	l.Logger.Infof("[param] Enable symlink check: %t", c.Params.EnableSymlinkCheck)
	l.Logger.Infof("[param] Fetch tags: %t", c.Params.FetchTags)
	l.Logger.Infof("[param] Merge target branch: %t", c.Params.MergeTargetBranch)
	if c.Params.MergeTargetBranch {
		l.Logger.Infof("[param] Target branch: %s", c.Params.TargetBranch)
		if c.Params.MergeSourceRepoUrl != "" {
			l.Logger.Infof("[param] Merge source repo URL: %s", c.Params.MergeSourceRepoUrl)
		}
	}
	if c.Params.BasicAuthDirectory != "" {
		l.Logger.Infof("[param] Basic auth directory: %s", c.Params.BasicAuthDirectory)
	}
	if c.Params.SshDirectory != "" {
		l.Logger.Infof("[param] SSH directory: %s", c.Params.SshDirectory)
	}
}

func (c *GitClone) validateParams() error {
	if c.Params.Url == "" {
		return fmt.Errorf("url parameter is required")
	}
	return nil
}

func (c *GitClone) getCheckoutDir() string {
	return filepath.Join(c.Params.OutputDir, c.Params.Subdirectory)
}

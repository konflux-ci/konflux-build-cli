package commands

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/spf13/cobra"
)

var GitCloneParamsConfig = map[string]common.Parameter{
	// TODO fill
}

type GitCloneParams struct {
	// TODO fill
}

type GitCloneCliWrappers struct {
	// TODO fill
}

type GitCloneResults struct {
	// TODO fill
}

type GitClone struct {
	Params        *GitCloneParams
	CliWrappers   GitCloneCliWrappers
	Results       GitCloneResults
	ResultsWriter common.ResultsWriterInterface

	// TODO fill
}

func NewGitClone(cmd *cobra.Command) (*GitClone, error) {
	gitClone := &GitClone{}

	params := &GitCloneParams{}
	if err := common.ParseParameters(cmd, GitCloneParamsConfig, params); err != nil {
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
	// TODO: create and assign CLI wrappers
	// executor := cliWrappers.NewCliExecutor()
	// someCli, err := cliWrappers.NewSomeCli(executor)
	// if err != nil {
	//     return err
	// }
	// c.CliWrappers.SomeCli = someCli

	return nil
}

func (c *GitClone) Run() error {
	// TODO: log parameters
	// l.Logger.Infof("[param] ParamName: %s", c.Params.ParamName)

	// TODO: validate parameters
	// if err := c.validateParams(); err != nil {
	//     return err
	// }

	// TODO: implement command logic

	return nil
}

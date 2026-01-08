package git_clone

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/spf13/cobra"
)

var ParamsConfig = map[string]common.Parameter{
	// TODO fill
}

type Params struct {
	// TODO fill
}

type Wrappers struct {
	// TODO fill
}

type Results struct {
	// TODO fill
}

type GitClone struct {
	Params        *Params
	CliWrappers   Wrappers
	Results       Results
	ResultsWriter common.ResultsWriterInterface

	// TODO fill
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

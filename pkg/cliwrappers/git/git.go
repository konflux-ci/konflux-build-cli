package git

import (
	"errors"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

type CliInterface interface {
	Init(workdir string) error
}

var _ CliInterface = &Cli{}

type Cli struct {
	Executor cliwrappers.CliExecutorInterface
}

func NewCli(executor cliwrappers.CliExecutorInterface) (*Cli, error) {
	gitCliAvailable, err := cliwrappers.CheckCliToolAvailable("git")
	if err != nil {
		return nil, err
	}
	if !gitCliAvailable {
		return nil, errors.New("git CLI is not available")
	}

	return &Cli{
		Executor: executor,
	}, nil
}

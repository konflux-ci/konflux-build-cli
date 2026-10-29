package cliwrappers

import (
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

type TknPushArgs struct {
	DestinationImage string
	Filenames        []string
	Annotations      map[string]string
	DockerConfigDir  string // Directory containing config.json.
}

var tknLog = l.Logger.WithField("logger", "TknCli")

type TknCliInterface interface {
	Push(args *TknPushArgs) (stdout, stderr string, err error)
}

type TknCli struct {
	Executor CliExecutorInterface
}

var _ TknCliInterface = &TknCli{}

func NewTknCli(executor CliExecutorInterface) (*TknCli, error) {
	cliAvailable, err := CheckCliToolAvailable("tkn")
	if err != nil || !cliAvailable {
		return nil, err
	}

	return &TknCli{
		Executor: executor,
	}, nil
}

func (b *TknCli) Push(args *TknPushArgs) (stdout, stderr string, err error) {
}

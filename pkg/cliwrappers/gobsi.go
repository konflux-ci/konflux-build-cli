// -> https://github.com/konflux-ci/gobsi <-

package cliwrappers

import (
	"errors"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var gobsiLog = l.Logger.WithField("logger", "GobsiCli")

type GobsiCliInterface interface {
	Build(args *GobsiBuildArgs) error
}

var _ GobsiCliInterface = &GobsiCli{}

type GobsiCli struct {
	Executor CliExecutorInterface
}

func NewGobsiCli(executor CliExecutorInterface) (*GobsiCli, error) {
	gobsiCliAvailable, err := CheckCliToolAvailable("gobsi")
	if err != nil {
		return nil, err
	}
	if !gobsiCliAvailable {
		return nil, errors.New("gobsi CLI is not available")
	}

	return &GobsiCli{
		Executor: executor,
	}, nil
}

type GobsiBuildArgs struct {
	OutputDir    string
	SrpmDir      string
	ExtraSrcDirs []string
	MergeDirs    []string
	Debug        bool
}

func (g *GobsiCli) Build(args *GobsiBuildArgs) error {
	gobsiArgs := []string{"--output", args.OutputDir}

	if args.SrpmDir != "" {
		gobsiArgs = append(gobsiArgs, "--srpm-dir", args.SrpmDir)
	}
	for _, dir := range args.ExtraSrcDirs {
		gobsiArgs = append(gobsiArgs, "--extra-src-dir", dir)
	}
	for _, dir := range args.MergeDirs {
		gobsiArgs = append(gobsiArgs, "--merge", dir)
	}
	if args.Debug {
		gobsiArgs = append(gobsiArgs, "--debug")
	}

	fullCmd := shellJoin("gobsi", gobsiArgs...)
	gobsiLog.Debugf("Running command:\n%s", fullCmd)

	_, _, _, err := g.Executor.Execute(Cmd{Name: "gobsi", Args: gobsiArgs, LogOutput: true})
	if err != nil {
		gobsiLog.Errorf("gobsi build failed: %s", err.Error())
		return err
	}

	gobsiLog.Debug("Source image build completed successfully")

	return nil
}

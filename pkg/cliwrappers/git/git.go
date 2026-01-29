package git

import (
	"errors"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

// CliInterface defines the methods for interacting with git via the CLI.
type CliInterface interface {
	// Init initializes a new git repository. Runs: git init
	Init(workdir string) error
}

var _ CliInterface = &GitCli{}

// GitCli provides methods for executing git commands via a CLI executor.
type GitCli struct {
	Executor cliwrappers.CliExecutorInterface
}

// NewGitCli creates a new GitCli instance after verifying git is available and meets the minimum version.
func NewGitCli(executor cliwrappers.CliExecutorInterface) (*GitCli, error) {
	gitCliAvailable, err := cliwrappers.CheckCliToolAvailable("git")
	if err != nil {
		return nil, err
	}
	if !gitCliAvailable {
		return nil, errors.New("git CLI is not available")
	}

	return &GitCli{
		Executor: executor,
	}, nil
}

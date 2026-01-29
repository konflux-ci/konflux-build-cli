package git

import (
	"errors"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

type CliInterface interface {
	Init(workdir string) error
	ConfigLocal(workdir, key, value string) error
	RevParse(workdir string, ref string, short bool, length int) (string, error)
	RemoteAdd(workdir, name, url string) (string, error)
	FetchWithRefspec(workdir, remote, refspec string, depth int) error
<<<<<<< Updated upstream
	Fetch(workdir, repository string, depth int) (string, error)
=======
	FetchTags(workdir string) (string, error)
>>>>>>> Stashed changes
	Checkout(workdir, ref string) error
	Commit(workdir, targetBranch, remote, resultSHA string) (string, error)
	Merge(workdir, fetchHead string) (string, error)
	SetSparseCheckout(workdir, sparseCheckoutDirectories string) error
	SubmoduleUpdate(workdir string, init bool, paths string) error
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

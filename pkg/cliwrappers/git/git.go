package git

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

type CliInterface interface {
	Init(workdir string) error
	RemoteAdd(workdir, name, url string) (string, error)
	FetchWithRefspec(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error
	Checkout(workdir, ref string) error
	SubmoduleUpdate(workdir string, init bool, depth int, paths []string) error
}

var _ CliInterface = &Cli{}

type Cli struct {
	Executor cliwrappers.CliExecutorInterface
}

var minGitVersion = [3]int{2, 25, 0}
var gitVersionRegex = regexp.MustCompile(`git version (\d+)\.(\d+)\.(\d+)`)

func NewCli(executor cliwrappers.CliExecutorInterface) (*Cli, error) {
	gitCliAvailable, err := cliwrappers.CheckCliToolAvailable("git")
	if err != nil {
		return nil, err
	}
	if !gitCliAvailable {
		return nil, errors.New("git CLI is not available")
	}

	stdout, _, _, err := executor.Execute("git", "--version")
	if err != nil {
		return nil, fmt.Errorf("failed to get git version: %w", err)
	}
	version, err := parseGitVersion(stdout)
	if err != nil {
		return nil, err
	}
	if !isVersionAtLeast(version, minGitVersion) {
		return nil, fmt.Errorf("git version %d.%d.%d is below minimum required %d.%d.%d",
			version[0], version[1], version[2],
			minGitVersion[0], minGitVersion[1], minGitVersion[2])
	}

	return &Cli{
		Executor: executor,
	}, nil
}

func parseGitVersion(output string) ([3]int, error) {
	m := gitVersionRegex.FindStringSubmatch(output)
	if m == nil {
		return [3]int{}, fmt.Errorf("failed to parse git version from output: %q", output)
	}
	var version [3]int
	for i := 0; i < 3; i++ {
		version[i], _ = strconv.Atoi(m[i+1])
	}
	return version, nil
}

func isVersionAtLeast(version, minimum [3]int) bool {
	for i := 0; i < 3; i++ {
		if version[i] > minimum[i] {
			return true
		}
		if version[i] < minimum[i] {
			return false
		}
	}
	return true
}

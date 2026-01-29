package git

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

// CliInterface defines the methods for interacting with git via the CLI.
type CliInterface interface {
	// Init initializes a new git repository. Runs: git init
	Init(workdir string) error
	// RemoteAdd adds a new remote. Runs: git remote add <name> <url>
	RemoteAdd(workdir, name, url string) (string, error)
	// FetchWithRefspec fetches a refspec from a remote with retry. Runs: git fetch [options] <remote> [<refspec>]
	FetchWithRefspec(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error
	// Checkout checks out a ref. Runs: git checkout <ref>
	Checkout(workdir, ref string) error
	// SubmoduleUpdate initializes and updates submodules. Runs: git submodule update --recursive [options]
	SubmoduleUpdate(workdir string, init bool, depth int, paths []string) error
	// SetSparseCheckout configures sparse checkout directories. Runs: git sparse-checkout set <dirs...>
	SetSparseCheckout(workdir string, directories []string) error
	// ConfigLocal sets a local git config value. Runs: git config --local <key> <value>
	ConfigLocal(workdir, key, value string) error
	// Commit creates a commit with the given message. Runs: git commit -m <message>
	Commit(workdir, message string) (string, error)
	// Merge merges a ref with a commit message. Runs: git merge -m <message> --no-ff <ref>
	Merge(workdir, ref, message string) (string, error)
	RevParse(workdir, ref string, short bool, length int) (string, error)
}

var _ CliInterface = &GitCli{}

// GitCli provides methods for executing git commands via a CLI executor.
type GitCli struct {
	Executor cliwrappers.CliExecutorInterface
}

var minGitVersion = [3]int{2, 25, 0}
var gitVersionRegex = regexp.MustCompile(`git version (\d+)\.(\d+)\.(\d+)`)

// NewGitCli creates a new GitCli instance after verifying git is available and meets the minimum version.
func NewGitCli(executor cliwrappers.CliExecutorInterface) (*GitCli, error) {
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

	return &GitCli{
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
		v, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, fmt.Errorf("failed to parse git version component %q: %w", m[i+1], err)
		}
		version[i] = v
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

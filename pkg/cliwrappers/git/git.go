package git

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var gitLog = l.Logger.WithField("logger", "GitCli")

// CliInterface defines the methods for interacting with git via the CLI.
type CliInterface interface {
	// Init initializes a new git repository. Runs: git init
	Init() error
	// ConfigLocal sets a local git config value. Runs: git config --local <key> <value>
	ConfigLocal(key, value string) error
	// RevParse resolves a git ref to its SHA. Runs: git rev-parse [--short[=N]] <ref>
	RevParse(ref string, short bool, length int) (string, error)
	// RemoteAdd adds a new remote. Runs: git remote add <name> <url>
	RemoteAdd(name, url string) (string, error)
	// FetchWithRefspec fetches a refspec from a remote with retry. Runs: git fetch [options] <remote> [<refspec>]
	FetchWithRefspec(opts FetchOptions) error
	// Checkout checks out a ref. Runs: git checkout <ref>
	Checkout(ref string) error
	// Commit creates a commit with the given message. Runs: git commit -m <message>
	Commit(message string) (string, error)
	// Merge merges a ref with a commit message. Runs: git merge -m <message> --no-ff <ref>
	Merge(ref, message string) (string, error)
	// SetSparseCheckout configures sparse checkout directories. Runs: git sparse-checkout set <dirs...>
	SetSparseCheckout(directories []string) error
	// SubmoduleUpdate initializes and updates submodules. Runs: git submodule update --recursive [options]
	SubmoduleUpdate(init bool, depth int, paths []string) error
	// FetchTags fetches all tags from the remote. Runs: git fetch --tags
	FetchTags() ([]string, error)
	// Log returns formatted git log output. Runs: git log [--pretty=<format>] [-N]
	Log(format string, count int) (string, error)
}

// FetchOptions contains the options for FetchWithRefspec.
type FetchOptions struct {
	Remote      string
	Refspec     string
	Depth       int
	Submodules  bool
	MaxAttempts int
}

var _ CliInterface = &GitCli{}

// GitCli provides methods for executing git commands via a CLI executor.
type GitCli struct {
	Executor cliwrappers.CliExecutorInterface
	Workdir  string
}

var minGitVersion = [3]int{2, 25, 0}
var gitVersionRegex = regexp.MustCompile(`git version (\d+)\.(\d+)\.(\d+)`)

// NewGitCli creates a new GitCli instance after verifying git is available and meets the minimum version.
func NewGitCli(executor cliwrappers.CliExecutorInterface, workdir string) (*GitCli, error) {
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
		Workdir:  workdir,
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
	return slices.Compare(version[:], minimum[:]) >= 0
}

// --- Repository operations ---

// Init initializes a new git repository in the working directory.
// Runs: git init
func (g *GitCli) Init() error {
	gitLog.Debugf("[command]: git init (in %s)", g.Workdir)

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", "init")
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git init stderr: %s", stderr)
		return fmt.Errorf("git init failed with exit code %d: %w", exitCode, err)
	}
	return nil
}

// SetSparseCheckout configures sparse checkout for the given directories.
// Runs: git config --local core.sparseCheckout true && git sparse-checkout set <directories...>
func (g *GitCli) SetSparseCheckout(directories []string) error {
	gitLog.Debugf("Configuring sparse checkout: %v", directories)
	if len(directories) == 0 {
		return fmt.Errorf("directories parameter empty")
	}

	if err := g.ConfigLocal("core.sparseCheckout", "true"); err != nil {
		return fmt.Errorf("failed to enable sparse checkout: %w", err)
	}

	args := append([]string{"sparse-checkout", "set", "--"}, directories...)
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", args...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git sparse-checkout stderr: %s", stderr)
		return fmt.Errorf("failed to set sparse checkout directories: %w", err)
	}
	return nil
}

// ConfigLocal sets a git config value locally in the repository.
// Runs: git config --local <key> <value>
func (g *GitCli) ConfigLocal(key, value string) error {
	if key == "" {
		return errors.New("config key must not be empty")
	}
	gitArgs := []string{"config", "--local", key, value}
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git config stderr: %s", stderr)
		return fmt.Errorf("git config failed with exit code %d: %w", exitCode, err)
	}
	return nil
}

// Commit creates a commit with the specified message.
// Runs: git commit -m <message>
func (g *GitCli) Commit(message string) (string, error) {
	gitArgs := []string{"commit", "-m", message}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git commit stderr: %s", stderr)
		return "", fmt.Errorf("git commit failed with exit code %d: %w", exitCode, err)
	}
	return strings.TrimSpace(stdout), nil
}

// Merge merges the specified ref into the current branch with the given commit message.
// Uses --no-ff to always create a merge commit. Returns the merge output.
// If the merge is already up-to-date, no commit is created and no error is returned.
// Runs: git merge -m <message> --no-ff --allow-unrelated-histories <ref>
func (g *GitCli) Merge(ref, message string) (string, error) {
	if ref == "" {
		return "", errors.New("ref must not be empty")
	}
	gitArgs := []string{"merge", "-m", message, "--no-ff", "--allow-unrelated-histories", ref}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git merge stderr: %s", stderr)
		return "", fmt.Errorf("git merge failed with exit code %d: %w", exitCode, err)
	}
	return strings.TrimSpace(stdout), nil
}

// --- Remote operations ---

// RemoteAdd adds a new remote with the given name and URL.
// Runs: git remote add <name> <url>
func (g *GitCli) RemoteAdd(name, url string) (string, error) {
	if name == "" {
		return "", errors.New("remote name must not be empty")
	}
	if url == "" {
		return "", errors.New("remote url must not be empty")
	}
	gitArgs := []string{"remote", "add", name, url}

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git remote add stderr: %s", stderr)
		return "", fmt.Errorf("git remote add failed with exit code %d: %w", exitCode, err)
	}
	return strings.TrimSpace(stdout), nil
}

// FetchTags fetches all tags from the remote and returns the list of tags.
// Runs: git fetch --tags && git tag -l
func (g *GitCli) FetchTags() ([]string, error) {
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", "fetch", "--tags")
	if err != nil {
		gitLog.Debugf("git fetch --tags stderr: %s", stderr)
		return nil, fmt.Errorf("git fetch --tags failed: %w", err)
	}
	if exitCode != 0 {
		gitLog.Debugf("git fetch --tags stderr: %s", stderr)
		return nil, fmt.Errorf("git fetch --tags failed with exit code %d", exitCode)
	}

	stdout, stderr2, exitCode2, err := g.Executor.ExecuteInDir(g.Workdir, "git", "tag", "-l")
	if err != nil {
		gitLog.Debugf("git tag -l stderr: %s", stderr2)
		return nil, fmt.Errorf("git tag -l failed: %w", err)
	}
	if exitCode2 != 0 {
		gitLog.Debugf("git tag -l stderr: %s", stderr2)
		return nil, fmt.Errorf("git tag -l failed with exit code %d", exitCode2)
	}

	tags := []string{}
	for _, tag := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// FetchWithRefspec fetches a specific refspec from a remote with optional depth and retry.
// Runs: git fetch [--recurse-submodules=yes] [--depth=N] <remote> --update-head-ok --force [<refspec>]
func (g *GitCli) FetchWithRefspec(opts FetchOptions) error {
	if opts.Remote == "" {
		return errors.New("remote must not be empty")
	}
	gitArgs := []string{"fetch"}

	if opts.Submodules {
		gitArgs = append(gitArgs, "--recurse-submodules=yes")
	}

	if opts.Depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", opts.Depth))
	}

	gitArgs = append(gitArgs, opts.Remote, "--update-head-ok", "--force")

	if opts.Refspec != "" {
		gitArgs = append(gitArgs, opts.Refspec)
	}

	retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
		return g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	}).WithMaxAttempts(opts.MaxAttempts).
		StopOnExitCode(128).
		StopIfOutputContains("Authentication failed").
		StopIfOutputContains("could not read Username").
		StopIfOutputContains("fatal: repository").
		StopIfOutputContains("Permission denied").
		StopIfOutputContains("Could not resolve hostname")

	_, stderr, exitCode, err := retryer.Run()
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git fetch stderr: %s", stderr)
		return fmt.Errorf("git fetch failed with exit code %d: %w", exitCode, err)
	}
	return nil
}

// Checkout checks out the specified ref (branch, tag, or commit SHA).
// Runs: git checkout <ref>
func (g *GitCli) Checkout(ref string) error {
	if ref == "" {
		return errors.New("ref must not be empty")
	}
	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", "checkout", ref)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git checkout stderr: %s", stderr)
		return fmt.Errorf("git checkout failed with exit code %d: %w", exitCode, err)
	}
	return nil
}

// SubmoduleUpdate initializes and/or updates submodules recursively.
// Runs: git submodule update --recursive [--init] [--force] [--depth=N] [-- paths...]
func (g *GitCli) SubmoduleUpdate(init bool, depth int, paths []string) error {
	gitArgs := []string{"submodule", "update", "--recursive"}

	if init {
		gitArgs = append(gitArgs, "--init")
	}

	gitArgs = append(gitArgs, "--force")

	if depth > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("--depth=%d", depth))
	}

	if len(paths) > 0 {
		gitArgs = append(gitArgs, "--")
		gitArgs = append(gitArgs, paths...)
	}

	_, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git submodule update stderr: %s", stderr)
		return fmt.Errorf("git submodule update failed with exit code %d: %w", exitCode, err)
	}
	return nil
}

// --- Info operations ---

// RevParse resolves a git ref to its SHA. If short is true, returns a shortened SHA.
// Runs: git rev-parse [--short[=N]] <ref>
func (g *GitCli) RevParse(ref string, short bool, length int) (string, error) {
	if ref == "" {
		return "", errors.New("ref must not be empty")
	}
	gitArgs := []string{"rev-parse"}

	if short {
		if length > 0 {
			gitArgs = append(gitArgs, fmt.Sprintf("--short=%d", length))
		} else {
			gitArgs = append(gitArgs, "--short")
		}
	}
	gitArgs = append(gitArgs, ref)

	gitLog.Debugf("[command]: git %s (in %s)", strings.Join(gitArgs, " "), g.Workdir)

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git rev-parse stderr: %s", stderr)
		return "", fmt.Errorf("git rev-parse failed with exit code %d: %w", exitCode, err)
	}
	return strings.TrimSpace(stdout), nil
}

// Log runs git log with the specified format and count, returning the output.
// Runs: git log [-N] [--pretty=<format>]
func (g *GitCli) Log(format string, count int) (string, error) {
	gitArgs := []string{"log"}

	if count > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("-%d", count))
	}
	if format != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--pretty=%s", format))
	}

	gitLog.Debugf("[command]: git %s (in %s)", strings.Join(gitArgs, " "), g.Workdir)

	stdout, stderr, exitCode, err := g.Executor.ExecuteInDir(g.Workdir, "git", gitArgs...)
	if err != nil || exitCode != 0 {
		gitLog.Debugf("git log stderr: %s", stderr)
		return "", fmt.Errorf("git log failed with exit code %d: %w", exitCode, err)
	}
	return strings.TrimSpace(stdout), nil
}

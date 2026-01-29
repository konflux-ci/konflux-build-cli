package git_clone

import (
	"fmt"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

const (
	mergeCommitEmail = "git-clone@konflux-ci.dev"
	mergeCommitName  = "Konflux CI Git Clone"
)

func (c *GitClone) mergeTargetBranch() error {
	if c.Params.Depth == 1 {
		l.Logger.Warning("Shallow clone with depth=1 may cause merge conflicts due to insufficient commit history.")
	}

	if c.Params.MergeSourceDepth == 1 {
		l.Logger.Warning("Shallow fetch with mergeSourceDepth=1 may cause merge conflicts due to insufficient commit history.")
	}

	mergeRemote := "origin"
	if c.Params.MergeSourceRepoURL != "" {
		normalizedOrigin := normalizeGitURL(c.Params.URL)
		normalizedMerge := normalizeGitURL(c.Params.MergeSourceRepoURL)

		if normalizedOrigin == normalizedMerge {
			l.Logger.Info("Merge source URL is the same as origin. Using existing 'origin' remote.")
		} else {
			l.Logger.Infof("Merging from different repository: '%s'", c.Params.MergeSourceRepoURL)
			l.Logger.Info("Adding remote 'merge-source'...")
			mergeRemote = "merge-source"
			add, err := c.CliWrappers.GitCli.RemoteAdd(c.getCheckoutDir(), mergeRemote, c.Params.MergeSourceRepoURL)
			if err != nil {
				return err
			}
			l.Logger.Infof("Remote add: %s", add)
		}
	}

	maxAttempts := c.Params.RetryMaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	err := c.CliWrappers.GitCli.FetchWithRefspec(c.getCheckoutDir(), mergeRemote, c.Params.TargetBranch, c.Params.MergeSourceDepth, false, maxAttempts)
	if err != nil {
		return err
	}

	err = c.CliWrappers.GitCli.ConfigLocal(c.getCheckoutDir(), "user.email", mergeCommitEmail)
	if err != nil {
		return err
	}
	err = c.CliWrappers.GitCli.ConfigLocal(c.getCheckoutDir(), "user.name", mergeCommitName)
	if err != nil {
		return err
	}

	mergeRef := fmt.Sprintf("%s/%s", mergeRemote, c.Params.TargetBranch)
	merge, err := c.CliWrappers.GitCli.Merge(c.getCheckoutDir(), mergeRef)
	if err != nil {
		return err
	}
	l.Logger.Infof("Merge: %s", merge)

	commit, err := c.CliWrappers.GitCli.Commit(c.getCheckoutDir(), c.Params.TargetBranch, mergeRemote, c.Results.Commit)
	if err != nil {
		return err
	}
	l.Logger.Infof("Commit: %s", commit)

	c.Results.MergedSha, err = c.CliWrappers.GitCli.RevParse(c.getCheckoutDir(), "HEAD", false, 0)
	if err != nil {
		return err
	}

	return nil
}

func normalizeGitURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	return url
}

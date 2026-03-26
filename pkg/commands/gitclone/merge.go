package gitclone

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers/git"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

func (c *GitClone) mergeTargetBranch() error {
	if c.Params.Depth == 1 {
		l.Logger.Warning("Shallow clone with depth=1 may cause merge conflicts due to insufficient commit history.")
	}

	if c.Params.MergeSourceDepth == 1 {
		l.Logger.Warning("Shallow fetch with merge-source-depth=1 may cause merge conflicts due to insufficient commit history.")
	}

	mergeRemote := "origin"
	if c.Params.MergeSourceRepoURL != "" {
		normalizedOrigin := normalizeGitURL(c.Params.URL)
		normalizedMerge := normalizeGitURL(c.Params.MergeSourceRepoURL)

		if normalizedOrigin == normalizedMerge {
			l.Logger.Debug("Merge source URL is the same as origin. Using existing 'origin' remote.")
		} else {
			l.Logger.Debugf("Merging from different repository: '%s'", c.Params.MergeSourceRepoURL)
			mergeRemote = "merge-source"
			if _, err := c.CliWrappers.GitCli.RemoteAdd(mergeRemote, c.Params.MergeSourceRepoURL); err != nil {
				return err
			}
		}
	}

	maxAttempts := c.Params.RetryMaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	err := c.CliWrappers.GitCli.FetchWithRefspec(git.FetchOptions{
		Remote:      mergeRemote,
		Refspec:     c.Params.TargetBranch,
		Depth:       c.Params.MergeSourceDepth,
		Submodules:  false,
		MaxAttempts: maxAttempts,
	})
	if err != nil {
		return err
	}

	err = c.CliWrappers.GitCli.ConfigLocal("user.email", c.Params.MergeCommitAuthorEmail)
	if err != nil {
		return err
	}
	err = c.CliWrappers.GitCli.ConfigLocal("user.name", c.Params.MergeCommitAuthorName)
	if err != nil {
		return err
	}

	// Get the current HEAD SHA before merging to use in the commit message
	currentSha, err := c.CliWrappers.GitCli.RevParse("HEAD", false, 0)
	if err != nil {
		return fmt.Errorf("failed to get pre-merge HEAD SHA: %w", err)
	}

	mergeRef := fmt.Sprintf("%s/%s", mergeRemote, c.Params.TargetBranch)
	message := fmt.Sprintf("Merge branch '%s' from %s into %s", c.Params.TargetBranch, mergeRemote, currentSha)
	merge, err := c.CliWrappers.GitCli.Merge(mergeRef, message)
	if err != nil {
		return err
	}
	l.Logger.Debugf("Merge: %s", merge)

	c.Results.MergedSha, err = c.CliWrappers.GitCli.RevParse("HEAD", false, 0)
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

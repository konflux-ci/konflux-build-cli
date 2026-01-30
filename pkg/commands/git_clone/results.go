package git_clone

import (
	"fmt"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

type Results struct {
	Commit          string `json:"commit"`
	ShortCommit     string `json:"shortCommit"`
	Url             string `json:"url"`
	CommitTimestamp string `json:"commitTimestamp"`
	MergedSha       string `json:"mergedSha,omitempty"`
	ChainsGitUrl    string `json:"CHAINS-GIT_URL"`
	ChainsGitCommit string `json:"CHAINS-GIT_COMMIT"`
}

func (c *GitClone) gatherCommitInfo() error {
	checkoutDir := c.getCheckoutDir()

	// Get full SHA
	sha, err := c.CliWrappers.GitCli.RevParse(checkoutDir, "HEAD", false, 0)
	if err != nil {
		return fmt.Errorf("failed to get commit SHA: %w", err)
	}
	c.Results.Commit = sha

	// Get short SHA
	shortSha, err := c.CliWrappers.GitCli.RevParse(checkoutDir, "HEAD", true, c.Params.ShortCommitLength)
	if err != nil {
		return fmt.Errorf("failed to get short commit SHA: %w", err)
	}
	c.Results.ShortCommit = shortSha

	// Get commit timestamp
	timestamp, err := c.CliWrappers.GitCli.Log(checkoutDir, "%ct", 1)
	if err != nil {
		return fmt.Errorf("failed to get commit timestamp: %w", err)
	}
	c.Results.CommitTimestamp = timestamp

	c.Results.Url = c.Params.Url

	// CHAINS results are duplicates for Tekton Chains provenance
	c.Results.ChainsGitUrl = c.Params.Url
	c.Results.ChainsGitCommit = sha

	l.Logger.Infof("Commit: %s", c.Results.Commit)
	l.Logger.Infof("Short commit: %s", c.Results.ShortCommit)
	l.Logger.Infof("Commit timestamp: %s", c.Results.CommitTimestamp)

	return nil
}

func (c *GitClone) outputResults() error {
	resultJson, err := c.ResultsWriter.CreateResultJson(c.Results)
	if err != nil {
		l.Logger.Errorf("failed to create results json: %s", err.Error())
		return err
	}
	fmt.Print(resultJson)
	return nil
}
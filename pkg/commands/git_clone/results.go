package git_clone

import (
	"fmt"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

type Results struct {
	Commit          string `json:"commit"`
	ShortCommit     string `json:"shortCommit"`
	URL             string `json:"url"`
	CommitTimestamp string `json:"commitTimestamp"`
	MergedSha       string `json:"mergedSha,omitempty"`
	ChainsGitURL    string `json:"CHAINS-GIT_URL"`
	ChainsGitCommit string `json:"CHAINS-GIT_COMMIT"`
}

func (c *GitClone) outputResults() error {
	resultJson, err := c.ResultsWriter.CreateResultJson(c.Results)
	if err != nil {
		l.Logger.Errorf("failed to create results json: %s", err.Error())
		return err
	}
	fmt.Println(resultJson)
	return nil
}

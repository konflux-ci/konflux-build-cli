package gitclone

// Results holds commit information gathered after a successful clone.
type Results struct {
	Commit          string `json:"commit"`
	ShortCommit     string `json:"shortCommit"`
	URL             string `json:"url"`
	CommitTimestamp string `json:"commitTimestamp"`
	MergedSha       string `json:"mergedSha,omitempty"`
	ChainsGitURL    string `json:"CHAINS-GIT_URL"`
	ChainsGitCommit string `json:"CHAINS-GIT_COMMIT"`
}

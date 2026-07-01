package integration_tests

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/gomega/types"
)

// buildahStepLine matches buildah's instruction echo lines, e.g.
// "STEP 1/3: RUN echo hello" or "STEP 2: FROM ubuntu".
// These appear in stderr because the CLI re-logs buildah's stdout.
var buildahStepLine = regexp.MustCompile(`(?m)^.*STEP \d+[^:]*:.*$`)

// filterBuildahSteps removes buildah STEP instruction echo lines from output,
// leaving only actual command output.
func filterBuildahSteps(output string) string {
	return buildahStepLine.ReplaceAllString(output, "")
}

// ContainCommandOutput succeeds if the actual string contains substr
// after filtering out buildah's STEP instruction echo lines.
//
// Buildah echoes every RUN instruction to stdout (e.g. "STEP 2: RUN echo hello"),
// which the CLI re-logs to stderr. A plain ContainSubstring("hello") would match
// even if the command never ran. This matcher filters those lines first.
func ContainCommandOutput(substr string) types.GomegaMatcher {
	return &commandOutputMatcher{substr: substr}
}

type commandOutputMatcher struct {
	substr string
}

func (m *commandOutputMatcher) Match(actual interface{}) (bool, error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ContainCommandOutput expects a string, got %T", actual)
	}
	filtered := filterBuildahSteps(s)
	return strings.Contains(filtered, m.substr), nil
}

func (m *commandOutputMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf(
		"Expected command output to contain %q, but it did not.\n"+
			"Note: buildah STEP lines were filtered out. Raw output:\n%s",
		m.substr, actual,
	)
}

func (m *commandOutputMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf(
		"Expected command output not to contain %q, but it did",
		m.substr,
	)
}

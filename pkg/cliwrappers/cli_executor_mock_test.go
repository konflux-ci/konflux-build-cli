package cliwrappers_test

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

var _ cliwrappers.CliExecutorInterface = &mockExecutor{}

type mockExecutor struct {
	executeFunc func(cmd cliwrappers.Cmd) (string, string, int, error)
}

func (m *mockExecutor) Execute(cmd cliwrappers.Cmd) (string, string, int, error) {
	if m.executeFunc != nil {
		return m.executeFunc(cmd)
	}
	return "", "", 0, nil
}

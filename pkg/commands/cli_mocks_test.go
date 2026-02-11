package commands

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

var _ cliwrappers.SkopeoCliInterface = &mockSkopeoCli{}

type mockSkopeoCli struct {
	CopyFunc    func(args *cliwrappers.SkopeoCopyArgs) error
	InspectFunc func(args *cliwrappers.SkopeoInspectArgs) (string, error)
}

func (m *mockSkopeoCli) Copy(args *cliwrappers.SkopeoCopyArgs) error {
	if m.CopyFunc != nil {
		return m.CopyFunc(args)
	}
	return nil
}

func (m *mockSkopeoCli) Inspect(args *cliwrappers.SkopeoInspectArgs) (string, error) {
	if m.InspectFunc != nil {
		return m.InspectFunc(args)
	}
	return "", nil
}

var _ cliwrappers.BuildahCliInterface = &mockBuildahCli{}

type mockBuildahCli struct {
	BuildFunc          func(args *cliwrappers.BuildahBuildArgs) error
	PushFunc           func(args *cliwrappers.BuildahPushArgs) (string, error)
	ManifestCreateFunc func(args *cliwrappers.BuildahManifestCreateArgs) error
	ManifestAddFunc    func(args *cliwrappers.BuildahManifestAddArgs) error
	ManifestInspectFunc func(args *cliwrappers.BuildahManifestInspectArgs) (string, error)
	ManifestPushFunc   func(args *cliwrappers.BuildahManifestPushArgs) (string, error)
}

func (m *mockBuildahCli) Build(args *cliwrappers.BuildahBuildArgs) error {
	if m.BuildFunc != nil {
		return m.BuildFunc(args)
	}
	return nil
}

func (m *mockBuildahCli) Push(args *cliwrappers.BuildahPushArgs) (string, error) {
	if m.PushFunc != nil {
		return m.PushFunc(args)
	}
	return "", nil
}

func (m *mockBuildahCli) ManifestCreate(args *cliwrappers.BuildahManifestCreateArgs) error {
	if m.ManifestCreateFunc != nil {
		return m.ManifestCreateFunc(args)
	}
	return nil
}

func (m *mockBuildahCli) ManifestAdd(args *cliwrappers.BuildahManifestAddArgs) error {
	if m.ManifestAddFunc != nil {
		return m.ManifestAddFunc(args)
	}
	return nil
}

func (m *mockBuildahCli) ManifestInspect(args *cliwrappers.BuildahManifestInspectArgs) (string, error) {
	if m.ManifestInspectFunc != nil {
		return m.ManifestInspectFunc(args)
	}
	return "", nil
}

func (m *mockBuildahCli) ManifestPush(args *cliwrappers.BuildahManifestPushArgs) (string, error) {
	if m.ManifestPushFunc != nil {
		return m.ManifestPushFunc(args)
	}
	return "", nil
}

var _ cliwrappers.OrasCliInterface = &mockOrasCli{}

type mockOrasCli struct {
	Executor cliwrappers.CliExecutorInterface
	PushFunc func(args *cliwrappers.OrasPushArgs) (string, string, error)
}

func (m *mockOrasCli) Push(args *cliwrappers.OrasPushArgs) (string, string, error) {
	if m.PushFunc != nil {
		return m.PushFunc(args)
	}
	return "", "", nil
}

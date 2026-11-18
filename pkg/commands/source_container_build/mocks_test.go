package source_container_build

import (
	"encoding/json"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

var _ cliwrappers.GitCliInterface = &mockGitCli{}

type mockGitCli struct {
	SetEnvFunc             func(key, value string)
	InitFunc               func() error
	SetSparseCheckoutFunc  func(directories []string) error
	RemoteAddFunc          func(name, url string) (string, error)
	FetchWithRefspecFunc   func(opts cliwrappers.GitFetchOptions) error
	CheckoutFunc           func(ref string) error
	SubmoduleUpdateFunc    func(init bool, depth int, paths []string) error
	SubmoduleFetchTagsFunc func() error
	RevParseFunc           func(ref string, short bool, length int) (string, error)
	LogFunc                func(format string, count int) (string, error)
	ConfigLocalFunc        func(key, value string) error
	ConfigGetFunc          func(key string) (string, error)
	CommitFunc             func(message string) (string, error)
	MergeFunc              func(ref, message string) (string, error)
	FetchTagsFunc          func() ([]string, error)
	StashFunc              func() error
	StashPopFunc           func() error
	ShowFunc               func(format string) (string, error)
	LsFilesFunc            func(recurseSubmodules bool) ([]string, error)
}

func (m *mockGitCli) SetEnv(key, value string) {
	if m.SetEnvFunc != nil {
		m.SetEnvFunc(key, value)
	}
}

func (m *mockGitCli) Init() error {
	if m.InitFunc != nil {
		return m.InitFunc()
	}
	return nil
}

func (m *mockGitCli) SetSparseCheckout(directories []string) error {
	if m.SetSparseCheckoutFunc != nil {
		return m.SetSparseCheckoutFunc(directories)
	}
	return nil
}

func (m *mockGitCli) RemoteAdd(name, url string) (string, error) {
	if m.RemoteAddFunc != nil {
		return m.RemoteAddFunc(name, url)
	}
	return "", nil
}

func (m *mockGitCli) FetchWithRefspec(opts cliwrappers.GitFetchOptions) error {
	if m.FetchWithRefspecFunc != nil {
		return m.FetchWithRefspecFunc(opts)
	}
	return nil
}

func (m *mockGitCli) Checkout(ref string) error {
	if m.CheckoutFunc != nil {
		return m.CheckoutFunc(ref)
	}
	return nil
}

func (m *mockGitCli) SubmoduleUpdate(init bool, depth int, paths []string) error {
	if m.SubmoduleUpdateFunc != nil {
		return m.SubmoduleUpdateFunc(init, depth, paths)
	}
	return nil
}

func (m *mockGitCli) SubmoduleFetchTags() error {
	if m.SubmoduleFetchTagsFunc != nil {
		return m.SubmoduleFetchTagsFunc()
	}
	return nil
}

func (m *mockGitCli) RevParse(ref string, short bool, length int) (string, error) {
	if m.RevParseFunc != nil {
		return m.RevParseFunc(ref, short, length)
	}
	return "", nil
}

func (m *mockGitCli) Log(format string, count int) (string, error) {
	if m.LogFunc != nil {
		return m.LogFunc(format, count)
	}
	return "", nil
}

func (m *mockGitCli) ConfigLocal(key, value string) error {
	if m.ConfigLocalFunc != nil {
		return m.ConfigLocalFunc(key, value)
	}
	return nil
}

func (m *mockGitCli) ConfigGet(key string) (string, error) {
	if m.ConfigGetFunc != nil {
		return m.ConfigGetFunc(key)
	}
	return "", nil
}

func (m *mockGitCli) Commit(message string) (string, error) {
	if m.CommitFunc != nil {
		return m.CommitFunc(message)
	}
	return "", nil
}

func (m *mockGitCli) Merge(ref, message string) (string, error) {
	if m.MergeFunc != nil {
		return m.MergeFunc(ref, message)
	}
	return "", nil
}

func (m *mockGitCli) FetchTags() ([]string, error) {
	if m.FetchTagsFunc != nil {
		return m.FetchTagsFunc()
	}
	return nil, nil
}

func (m *mockGitCli) Stash() error {
	if m.StashFunc != nil {
		return m.StashFunc()
	}
	return nil
}

func (m *mockGitCli) StashPop() error {
	if m.StashPopFunc != nil {
		return m.StashPopFunc()
	}
	return nil
}

func (m *mockGitCli) Show(format string) (string, error) {
	if m.ShowFunc != nil {
		return m.ShowFunc(format)
	}
	return "", nil
}

func (m *mockGitCli) LsFiles(recurseSubmodules bool) ([]string, error) {
	if m.LsFilesFunc != nil {
		return m.LsFilesFunc(recurseSubmodules)
	}
	return nil, nil
}

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

var _ cliwrappers.GobsiCliInterface = &mockGobsiCli{}

type mockGobsiCli struct {
	BuildFunc func(args *cliwrappers.GobsiBuildArgs) error
}

func (m *mockGobsiCli) Build(args *cliwrappers.GobsiBuildArgs) error {
	if m.BuildFunc != nil {
		return m.BuildFunc(args)
	}
	return nil
}

var _ common.ResultsWriterInterface = &mockResultsWriter{}

type mockResultsWriter struct {
	WriteResultStringFunc func(result, path string) error
	CreateResultJsonFunc  func(result any) (string, error)

	// Result file path => result data
	WrittenResults map[string]string
}

func (m *mockResultsWriter) CreateResultJson(result any) (string, error) {
	if m.CreateResultJsonFunc != nil {
		return m.CreateResultJsonFunc(result)
	}

	resultJson, err := json.Marshal(result)
	return string(resultJson), err
}

func (m *mockResultsWriter) WriteResultString(result, path string) error {
	if m.WriteResultStringFunc != nil {
		return m.WriteResultStringFunc(result, path)
	}

	if m.WrittenResults == nil {
		m.WrittenResults = make(map[string]string)
	}
	m.WrittenResults[path] = result
	return nil
}

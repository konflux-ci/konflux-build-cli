package git_clone

import (
	"encoding/json"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers/git"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

var _ git.CliInterface = &mockGitCli{}

type mockGitCli struct {
	InitFunc              func(workdir string) error
	SetSparseCheckoutFunc func(workdir string, directories []string) error
	RemoteAddFunc         func(workdir, name, url string) (string, error)
	FetchWithRefspecFunc  func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error
	CheckoutFunc          func(workdir, ref string) error
	SubmoduleUpdateFunc   func(workdir string, init bool, depth int, paths []string) error
	RevParseFunc          func(workdir string, ref string, short bool, length int) (string, error)
	LogFunc               func(workdir string, format string, count int) (string, error)
	ConfigLocalFunc       func(workdir, key, value string) error
	CommitFunc            func(workdir, targetBranch, remote, resultSHA string) (string, error)
	MergeFunc             func(workdir, fetchHead string) (string, error)
	FetchTagsFunc         func(workdir string) ([]string, error)
}

func (m *mockGitCli) FetchTags(workdir string) ([]string, error) {
	if m.FetchTagsFunc != nil {
		return m.FetchTagsFunc(workdir)
	}
	return nil, nil
}

func (m *mockGitCli) RemoteAdd(workdir, name, url string) (string, error) {
	if m.RemoteAddFunc != nil {
		return m.RemoteAddFunc(workdir, name, url)
	}
	return "", nil
}

func (m *mockGitCli) ConfigLocal(workdir, key, value string) error {
	if m.ConfigLocalFunc != nil {
		return m.ConfigLocalFunc(workdir, key, value)
	}
	return nil
}

func (m *mockGitCli) Commit(workdir, targetBranch, remote, resultSHA string) (string, error) {
	if m.CommitFunc != nil {
		return m.CommitFunc(workdir, targetBranch, remote, resultSHA)
	}
	return "", nil
}

func (m *mockGitCli) Merge(workdir, fetchHead string) (string, error) {
	if m.MergeFunc != nil {
		return m.MergeFunc(workdir, fetchHead)
	}
	return "", nil
}

func (m *mockGitCli) FetchWithRefspec(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
	if m.FetchWithRefspecFunc != nil {
		return m.FetchWithRefspecFunc(workdir, remote, refspec, depth, submodules, maxAttempts)
	}
	return nil
}

func (m *mockGitCli) Checkout(workdir, ref string) error {
	if m.CheckoutFunc != nil {
		return m.CheckoutFunc(workdir, ref)
	}
	return nil
}

func (m *mockGitCli) SubmoduleUpdate(workdir string, init bool, depth int, paths []string) error {
	if m.SubmoduleUpdateFunc != nil {
		return m.SubmoduleUpdateFunc(workdir, init, depth, paths)
	}
	return nil
}

func (m *mockGitCli) Init(workdir string) error {
	if m.InitFunc != nil {
		return m.InitFunc(workdir)
	}
	return nil
}

func (m *mockGitCli) SetSparseCheckout(workdir string, directories []string) error {
	if m.SetSparseCheckoutFunc != nil {
		return m.SetSparseCheckoutFunc(workdir, directories)
	}
	return nil
}

func (m *mockGitCli) RevParse(workdir string, ref string, short bool, length int) (string, error) {
	if m.RevParseFunc != nil {
		return m.RevParseFunc(workdir, ref, short, length)
	}
	return "", nil
}

func (m *mockGitCli) Log(workdir string, format string, count int) (string, error) {
	if m.LogFunc != nil {
		return m.LogFunc(workdir, format, count)
	}
	return "", nil
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

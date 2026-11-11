package cliwrappers_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

func setupGobsiCli() (*cliwrappers.GobsiCli, *mockExecutor) {
	executor := &mockExecutor{}
	gobsiCli := &cliwrappers.GobsiCli{Executor: executor}
	return gobsiCli, executor
}

func TestGobsiCliBuildArgs(t *testing.T) {
	g := NewWithT(t)

	gobsiCli, executor := setupGobsiCli()
	var capturedCmd cliwrappers.Cmd

	executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
		capturedCmd = cmd
		return "", "", 0, nil
	}

	args := &cliwrappers.GobsiBuildArgs{
		OutputDir:    "/work/source_image_output",
		SrpmDir:      "/work/rpm_dir",
		ExtraSrcDirs: []string{"/work/source_archive", "/work/prefetched_sources/src-0"},
		MergeDirs:    []string{"/work/parent_image_sources"},
		Debug:        true,
	}

	err := gobsiCli.Build(args)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(capturedCmd.Name).To(Equal("gobsi"))
	g.Expect(capturedCmd.LogOutput).To(BeTrue())
	g.Expect(capturedCmd.Args).To(Equal([]string{
		"--output", "/work/source_image_output",
		"--srpm-dir", "/work/rpm_dir",
		"--extra-src-dir", "/work/source_archive",
		"--extra-src-dir", "/work/prefetched_sources/src-0",
		"--merge", "/work/parent_image_sources",
		"--debug",
	}))
}

func TestGobsiCliBuildOnlyRequiredArgs(t *testing.T) {
	g := NewWithT(t)

	gobsiCli, executor := setupGobsiCli()
	var capturedArgs []string

	executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
		capturedArgs = cmd.Args
		return "", "", 0, nil
	}

	args := &cliwrappers.GobsiBuildArgs{
		OutputDir:    "/work/out",
		ExtraSrcDirs: []string{"/work/source_archive"},
	}

	err := gobsiCli.Build(args)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(capturedArgs).To(Equal([]string{
		"--output", "/work/out",
		"--extra-src-dir", "/work/source_archive",
	}))
}

func TestGobsiCliBuildPropagatesError(t *testing.T) {
	g := NewWithT(t)

	gobsiCli, executor := setupGobsiCli()
	expectedErr := errors.New("exit status 1")

	executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
		return "", "no SRPMs found", 1, expectedErr
	}

	err := gobsiCli.Build(&cliwrappers.GobsiBuildArgs{
		OutputDir: "/work/out",
		SrpmDir:   "/work/rpm_dir",
	})
	g.Expect(err).To(MatchError(expectedErr))
}

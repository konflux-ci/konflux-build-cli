package integration_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
)

const gitCloneRunnerImage = "quay.io/konflux-ci/task-runner:1.1.1"

type gitCloneResult struct {
	Commit    string `json:"commit"`
	MergedSha string `json:"mergedSha,omitempty"`
}

// parseGitCloneResult extracts the JSON result line from mixed stdout output.
func parseGitCloneResult(stdout string) (gitCloneResult, error) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			var r gitCloneResult
			if json.Unmarshal([]byte(line), &r) == nil && r.Commit != "" {
				return r, nil
			}
		}
	}
	return gitCloneResult{}, fmt.Errorf("no result found")
}

// startGitCloneContainer starts a container with root bind-mounted at /workspace.
func startGitCloneContainer(t *testing.T, root string) *TestRunnerContainer {
	t.Helper()
	Expect(os.WriteFile(filepath.Join(root, "git.config"), []byte("[safe]\n\tdirectory = *\n[protocol \"file\"]\n\tallow = always\n"), 0644)).To(Succeed())

	container := NewBuildCliRunnerContainer(GenerateUniqueTag(t), gitCloneRunnerImage)
	container.AddVolumeWithOptions(root, "/workspace", "z")
	container.SetWorkdir("/workspace")
	container.AddEnv("HOME", "/workspace")
	container.AddEnv("GIT_CONFIG_GLOBAL", "/workspace/git.config")
	Expect(container.Start()).To(Succeed())
	t.Cleanup(func() { container.DeleteIfExists() })
	return container
}

func TestGitClone(t *testing.T) {
	tests := []struct {
		name    string
		skip    func() bool
		setup   func(t *testing.T, root string)
		url     string
		args    []string
		wantErr bool
		check   func(t *testing.T, root, stdout, stderr string)
	}{
		{
			name: "basic clone",
			setup: func(t *testing.T, root string) {
				repo := createLocalTestRepo(t)
				bareCloneToPath(t, repo.Path, filepath.Join(root, "repo.git"))
			},
			url:  "file:///workspace/repo.git",
			args: []string{"--depth", "0", "--submodules=false"},
			check: func(t *testing.T, root, stdout, stderr string) {
				Expect(filepath.Join(root, "out", "README.md")).To(BeAnExistingFile())
				Expect(filepath.Join(root, "out", "second.txt")).To(BeAnExistingFile())
			},
		},
		{
			name: "shallow tag",
			setup: func(t *testing.T, root string) {
				repo := createLocalTestRepo(t)
				bareCloneToPath(t, repo.Path, filepath.Join(root, "repo.git"))
				Expect(os.WriteFile(filepath.Join(root, "expected-commit"), []byte(repo.TagCommit), 0644)).To(Succeed())
			},
			url:  "file:///workspace/repo.git",
			args: []string{"--depth", "1", "--revision", "v1.0.0", "--submodules=false"},
			check: func(t *testing.T, root, stdout, stderr string) {
				_, err := os.Stat(filepath.Join(root, "out", "second.txt"))
				Expect(err).To(MatchError(os.ErrNotExist))

				expected, err := os.ReadFile(filepath.Join(root, "expected-commit"))
				Expect(err).ToNot(HaveOccurred())
				head := runGit(t, filepath.Join(root, "out"), "rev-parse", "HEAD")
				Expect(head).To(Equal(string(expected)))
			},
		},
		{
			name: "sparse checkout",
			setup: func(t *testing.T, root string) {
				repo := createLocalTestRepo(t)
				bareCloneToPath(t, repo.Path, filepath.Join(root, "repo.git"))
			},
			url:  "file:///workspace/repo.git",
			args: []string{"--depth", "0", "--sparse-checkout-directories", "dir-a", "--submodules=false"},
			check: func(t *testing.T, root, stdout, stderr string) {
				Expect(filepath.Join(root, "out", "dir-a", "file.txt")).To(BeAnExistingFile())
				_, err := os.Stat(filepath.Join(root, "out", "dir-b"))
				Expect(err).To(MatchError(os.ErrNotExist))
			},
		},
		{
			name: "submodules",
			setup: func(t *testing.T, root string) {
				headCommit := prepareSubmoduleBareRepos(t, root)
				Expect(os.WriteFile(filepath.Join(root, "expected-commit"), []byte(headCommit), 0644)).To(Succeed())
			},
			url:  "file:///workspace/main-bare.git",
			args: []string{"--depth", "0", "--submodules=true"},
			check: func(t *testing.T, root, stdout, stderr string) {
				content, err := os.ReadFile(filepath.Join(root, "out", "my-submodule", "sub-file.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("submodule content\n"))

				expected, err := os.ReadFile(filepath.Join(root, "expected-commit"))
				Expect(err).ToNot(HaveOccurred())
				head := runGit(t, filepath.Join(root, "out"), "rev-parse", "HEAD")
				Expect(head).To(Equal(string(expected)))
			},
		},
		{
			name:  "merge target branch",
			setup: func(t *testing.T, root string) { prepareMergeBranchBareRepo(t, root) },
			url:   "file:///workspace/merge-bare.git",
			args:  []string{"--depth", "0", "--revision", "feature", "--merge-target-branch", "--target-branch", "main", "--merge-source-depth", "0", "--submodules=false"},
			check: func(t *testing.T, root, stdout, stderr string) {
				Expect(filepath.Join(root, "out", "feature-only.txt")).To(BeAnExistingFile())
				Expect(filepath.Join(root, "out", "main-only.txt")).To(BeAnExistingFile())

				result, err := parseGitCloneResult(stdout)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.MergedSha).ToNot(BeEmpty())
			},
		},
		{
			name: "delete existing",
			setup: func(t *testing.T, root string) {
				repo := createLocalTestRepo(t)
				bareCloneToPath(t, repo.Path, filepath.Join(root, "repo.git"))
				Expect(os.MkdirAll(filepath.Join(root, "out"), 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(root, "out", "stale.txt"), []byte("stale"), 0644)).To(Succeed())
			},
			url:  "file:///workspace/repo.git",
			args: []string{"--depth", "0", "--submodules=false", "--delete-existing"},
			check: func(t *testing.T, root, stdout, stderr string) {
				Expect(filepath.Join(root, "out", "README.md")).To(BeAnExistingFile())
				_, err := os.Stat(filepath.Join(root, "out", "stale.txt"))
				Expect(err).To(MatchError(os.ErrNotExist))
			},
		},
		{
			name:    "nonexistent repo",
			setup:   func(t *testing.T, root string) {},
			url:     "file:///workspace/nonexistent.git",
			args:    []string{"--depth", "0", "--submodules=false", "--retry-max-attempts", "1"},
			wantErr: true,
			check:   func(t *testing.T, root, stdout, stderr string) {},
		},
		{
			name:    "symlink rejection",
			skip:    func() bool { return runtime.GOOS == "windows" },
			setup:   func(t *testing.T, root string) { prepareBareRepoWithExternalSymlink(t, root) },
			url:     "file:///workspace/bad-symlink-bare.git",
			args:    []string{"--depth", "0", "--enable-symlink-check=true", "--submodules=false"},
			wantErr: true,
			check: func(t *testing.T, root, stdout, stderr string) {
				Expect(stderr).To(ContainSubstring("symlink"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != nil && tc.skip() {
				t.Skip()
			}
			SetupGomega(t)

			root, err := CreateTempDir("gitclone-test-")
			Expect(err).ToNot(HaveOccurred())
			t.Cleanup(func() { os.RemoveAll(root) })
			tc.setup(t, root)
			container := startGitCloneContainer(t, root)

			args := append([]string{"git-clone", "--url", tc.url, "--output-dir", "/workspace/out", "--ssl-verify=false"}, tc.args...)
			stdout, stderr, err := container.ExecuteCommandWithOutput(KonfluxBuildCli, args...)

			if tc.wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred(), "stderr: %s", stderr)
			}
			tc.check(t, root, stdout, stderr)
		})
	}
}

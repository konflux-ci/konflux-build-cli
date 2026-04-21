package integration_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

// runGit runs a git command in dir and returns trimmed stdout.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, code, err := cliwrappers.NewCliExecutor().Execute(cliwrappers.Cmd{
		Name: "git", Args: args, Dir: dir,
	})
	if err != nil || code != 0 {
		t.Fatalf("git %s failed (exit %d): %v\nstderr: %s", args[0], code, err, stderr)
	}
	return strings.TrimSpace(stdout)
}

// bareCloneToPath creates a bare clone of src at dest, simulating a remote repo.
func bareCloneToPath(t *testing.T, src, dest string) {
	t.Helper()
	_ = os.RemoveAll(dest)
	runGit(t, "", "clone", "--bare", src, dest)
}

// initGitRepo creates a directory and initializes a git repo with "main" branch.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
}

type testRepoInfo struct {
	Path      string
	TagCommit string
}

// createLocalTestRepo creates a repo with 2 commits, tag v1.0.0 on first, dir-a/ and dir-b/.
func createLocalTestRepo(t *testing.T) testRepoInfo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	initGitRepo(t, dir)

	for _, d := range []string{"dir-a", "dir-b"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, d, "file.txt"), []byte(d), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "first")
	runGit(t, dir, "tag", "-a", "v1.0.0", "-m", "v1.0.0")
	tagCommit := runGit(t, dir, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("second\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "second")

	return testRepoInfo{Path: dir, TagCommit: tagCommit}
}

// prepareSubmoduleBareRepos creates a main repo with one submodule as bare repos. Returns HEAD commit.
func prepareSubmoduleBareRepos(t *testing.T, root string) string {
	t.Helper()

	subDir := filepath.Join(root, "_sub")
	initGitRepo(t, subDir)
	if err := os.WriteFile(filepath.Join(subDir, "sub-file.txt"), []byte("submodule content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, subDir, "add", "-A")
	runGit(t, subDir, "commit", "-m", "sub commit")
	bareCloneToPath(t, subDir, filepath.Join(root, "sub-bare.git"))

	mainDir := filepath.Join(root, "_main")
	initGitRepo(t, mainDir)
	if err := os.WriteFile(filepath.Join(mainDir, "main.txt"), []byte("main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, mainDir, "add", "-A")
	runGit(t, mainDir, "commit", "-m", "main commit")
	runGit(t, mainDir, "-c", "protocol.file.allow=always", "submodule", "add", "../sub-bare.git", "my-submodule")
	runGit(t, mainDir, "commit", "-m", "add submodule")
	bareCloneToPath(t, mainDir, filepath.Join(root, "main-bare.git"))

	head := runGit(t, filepath.Join(root, "main-bare.git"), "rev-parse", "HEAD")

	_ = os.RemoveAll(subDir)
	_ = os.RemoveAll(mainDir)
	return head
}

// prepareMergeBranchBareRepo creates a bare repo with diverged main and feature branches.
func prepareMergeBranchBareRepo(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "_merge")
	initGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")

	runGit(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature-only.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "feature")

	runGit(t, dir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "main-only.txt"), []byte("main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "main")

	bareCloneToPath(t, dir, filepath.Join(root, "merge-bare.git"))
	_ = os.RemoveAll(dir)
}

// prepareBareRepoWithExternalSymlink creates a bare repo containing a symlink pointing outside the repo.
func prepareBareRepoWithExternalSymlink(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "_symlink")
	initGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/hosts", filepath.Join(dir, "bad.link")); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "external symlink")

	bareCloneToPath(t, dir, filepath.Join(root, "bad-symlink-bare.git"))
	_ = os.RemoveAll(dir)
}

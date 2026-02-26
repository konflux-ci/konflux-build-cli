package integration_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers/git"
	"github.com/konflux-ci/konflux-build-cli/pkg/commands/git_clone"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

func newGitClone(params *git_clone.Params) (*git_clone.GitClone, error) {
	executor := cliwrappers.NewCliExecutor()
	gitCli, err := git.NewCli(executor)
	if err != nil {
		return nil, err
	}

	return &git_clone.GitClone{
		Params: params,
		CliWrappers: git_clone.CliWrappers{
			GitCli: gitCli,
		},
		ResultsWriter: common.NewResultsWriter(),
	}, nil
}

// Test_GitClone_FailForWrongURL tests that git-clone fails when given a non-existent repository URL
func Test_GitClone_FailForWrongURL(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/user/repo-does-not-exist",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		RetryMaxAttempts:  0,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("git fetch failed"))
}

// Test_GitClone_RunWithTag tests that git-clone successfully clones a repository at a specific tag
func Test_GitClone_RunWithTag(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify files were cloned
	checkoutDir := filepath.Join(tempDir, "source")
	files, err := os.ReadDir(checkoutDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(files)).To(BeNumerically(">", 0))

	// Verify results contain commit info
	g.Expect(gitClone.Results.Commit).ToNot(BeEmpty())
	g.Expect(gitClone.Results.ShortCommit).ToNot(BeEmpty())
	g.Expect(gitClone.Results.CommitTimestamp).ToNot(BeEmpty())
	g.Expect(gitClone.Results.URL).To(Equal("https://github.com/kelseyhightower/nocode"))

	// Tag 1.0.0 is pinned to this specific commit
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
	g.Expect(gitClone.Results.ShortCommit).To(Equal("ed6c73f"))
}

// Test_GitClone_RunWithoutArgs tests that git-clone successfully clones a repository with just the URL
func Test_GitClone_RunWithoutArgs(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify files were cloned
	checkoutDir := filepath.Join(tempDir, "source")
	files, err := os.ReadDir(checkoutDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(files)).To(BeNumerically(">", 0))

	// Verify results
	g.Expect(gitClone.Results.Commit).To(Equal("6c073b08f7987018cbb2cb9a5747c84913b3608e"))
	g.Expect(gitClone.Results.ShortCommit).To(Equal("6c073b0"))
	g.Expect(gitClone.Results.CommitTimestamp).To(Equal("1579634710"))
	g.Expect(gitClone.Results.URL).To(Equal("https://github.com/kelseyhightower/nocode"))
}

// Test_GitClone_WithDepth tests cloning with specific history depth
func Test_GitClone_WithDepth(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             2,
		ShortCommitLength: 7,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify we have exactly 2 commits since depth is set to 2
	checkoutDir := filepath.Join(tempDir, "source")
	executor := cliwrappers.NewCliExecutor()
	stdout, _, exitCode, err := executor.ExecuteInDir(checkoutDir, "git", "rev-list", "--count", "HEAD")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	g.Expect(strings.TrimSpace(stdout)).To(Equal("2"))
}

// Test_GitClone_DeleteExisting tests that existing directory is cleaned before clone
func Test_GitClone_DeleteExisting(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create existing directory with a file that should be deleted
	checkoutDir := filepath.Join(tempDir, "source")
	err = os.MkdirAll(checkoutDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	existingFile := filepath.Join(checkoutDir, "should-be-deleted.txt")
	err = os.WriteFile(existingFile, []byte("this file should be deleted"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		DeleteExisting:    true,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the existing file was deleted
	_, statErr := os.Stat(existingFile)
	g.Expect(os.IsNotExist(statErr)).To(BeTrue(), "existing file should have been deleted")

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_FetchTags tests that all tags are fetched when enabled
func Test_GitClone_FetchTags(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		FetchTags:         true,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify tags were fetched (nocode has tag 1.0.0)
	checkoutDir := filepath.Join(tempDir, "source")
	executor := cliwrappers.NewCliExecutor()
	stdout, _, exitCode, err := executor.ExecuteInDir(checkoutDir, "git", "tag", "-l")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	g.Expect(stdout).To(ContainSubstring("1.0.0"))
}

// Test_GitClone_Submodules tests that submodules are initialized when enabled
func Test_GitClone_Submodules(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Using a repo with submodules
	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/konflux-ci/buildah-container",
		Revision:          "main",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		Submodules:        true,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify submodules were initialized (check .git in submodule dir or files exist)
	checkoutDir := filepath.Join(tempDir, "source")
	executor := cliwrappers.NewCliExecutor()
	stdout, _, exitCode, err := executor.ExecuteInDir(checkoutDir, "git", "submodule", "status")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	// If submodules were initialized, they won't have a '-' prefix at the start of the line
	// (uninitialized submodules show as "-SHA path", initialized show as " SHA path")
	for _, line := range strings.Split(stdout, "\n") {
		if len(line) > 0 {
			g.Expect(line[0]).ToNot(Equal(byte('-')), "submodule should be initialized (no leading '-'): %s", line)
		}
	}
}

// Test_GitClone_Refspec tests fetching a specific refspec before checkout
func Test_GitClone_Refspec(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Clone with a refspec to fetch a specific PR ref
	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Refspec:           "+refs/tags/*:refs/tags/*",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the refspec was fetched (tags should be available)
	checkoutDir := filepath.Join(tempDir, "source")
	executor := cliwrappers.NewCliExecutor()
	stdout, _, exitCode, err := executor.ExecuteInDir(checkoutDir, "git", "tag", "-l")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	g.Expect(stdout).To(ContainSubstring("1.0.0"), "refspec should have fetched tags")
}

// Test_GitClone_SparseCheckout tests cloning only specific directories
func Test_GitClone_SparseCheckout(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Clone only specific directories using sparse checkout
	gitClone, err := newGitClone(&git_clone.Params{
		URL:                       "https://github.com/konflux-ci/build-definitions",
		Revision:                  "main",
		OutputDir:                 tempDir,
		Subdirectory:              "source",
		Depth:                     1,
		ShortCommitLength:         7,
		SparseCheckoutDirectories: "task/git-clone",
		RetryMaxAttempts:          3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify only the specified directory was checked out
	checkoutDir := filepath.Join(tempDir, "source")

	// The sparse checkout directory should exist
	sparseDir := filepath.Join(checkoutDir, "task", "git-clone")
	_, err = os.Stat(sparseDir)
	g.Expect(err).ToNot(HaveOccurred(), "sparse checkout directory should exist")

	// Other directories should NOT exist
	otherDir := filepath.Join(checkoutDir, "task", "buildah")
	_, err = os.Stat(otherDir)
	g.Expect(os.IsNotExist(err)).To(BeTrue(), "other directories should not be checked out")
}

// Test_GitClone_SymlinkCheckPasses tests that symlink check passes for safe repos
func Test_GitClone_SymlinkCheckPasses(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		EnableSymlinkCheck: true,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred(), "symlink check should pass for safe repo")

	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_SymlinkCheckFails tests that symlinks pointing outside repo are detected
func Test_GitClone_SymlinkCheckFails(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a local git repository with a bad symlink
	repoDir := filepath.Join(tempDir, "bad-repo")
	err = os.MkdirAll(repoDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	executor := cliwrappers.NewCliExecutor()

	// Initialize git repo
	_, _, exitCode, err := executor.ExecuteInDir(repoDir, "git", "init")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))

	// Configure git user for commit
	_, _, exitCode, err = executor.ExecuteInDir(repoDir, "git", "config", "user.email", "test@test.com")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	_, _, exitCode, err = executor.ExecuteInDir(repoDir, "git", "config", "user.name", "Test")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))

	// Create a bad symlink pointing outside the repo
	badSymlink := filepath.Join(repoDir, "bad-symlink")
	err = os.Symlink("/etc/passwd", badSymlink)
	g.Expect(err).ToNot(HaveOccurred())

	// Add and commit
	_, _, exitCode, err = executor.ExecuteInDir(repoDir, "git", "add", "-A")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	_, _, exitCode, err = executor.ExecuteInDir(repoDir, "git", "commit", "-m", "Add bad symlink")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))

	// Now clone this local repo with symlink check enabled
	gitClone, err := newGitClone(&git_clone.Params{
		URL:                repoDir, // Clone from local path
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		EnableSymlinkCheck: true,
		RetryMaxAttempts:   0,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).To(HaveOccurred(), "symlink check should fail for repo with bad symlink")
	g.Expect(err.Error()).To(ContainSubstring("symlink"), "error should mention symlink")
}

// Test_GitClone_MergeTargetBranch tests merging target branch into checked-out revision
func Test_GitClone_MergeTargetBranch(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             0,
		ShortCommitLength: 7,
		MergeTargetBranch: true,
		TargetBranch:      "master",
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify merge happened
	g.Expect(gitClone.Results.Commit).ToNot(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"),
		"commit should differ after merge")

	// Verify MergedSha result is set
	g.Expect(gitClone.Results.MergedSha).ToNot(BeEmpty(), "merged SHA should be set")
}

// Test_GitClone_SSLVerifyDisabled tests that SSL verification can be disabled
func Test_GitClone_SSLVerifyDisabled(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		SSLVerify:         false,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))

	// Verify GIT_SSL_NO_VERIFY env var was set (we use env vars instead of git config)
	g.Expect(os.Getenv("GIT_SSL_NO_VERIFY")).To(Equal("true"))
}

// Test_GitClone_BasicAuthWithGitCredentials tests basic auth using .git-credentials and .gitconfig files
func Test_GitClone_BasicAuthWithGitCredentials(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create basic auth directory with .git-credentials and .gitconfig
	authDir := filepath.Join(tempDir, "basic-auth")
	err = os.MkdirAll(authDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Write .git-credentials file (fake credentials - won't be used for actual auth)
	gitCredentials := "https://testuser:testpass@github.com\n"
	err = os.WriteFile(filepath.Join(authDir, ".git-credentials"), []byte(gitCredentials), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	// Write .gitconfig file
	gitConfig := "[credential \"https://github.com\"]\n  helper = store\n"
	err = os.WriteFile(filepath.Join(authDir, ".gitconfig"), []byte(gitConfig), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		URL:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_BasicAuthWithUsernamePassword tests basic auth using username/password files (k8s secret format)
func Test_GitClone_BasicAuthWithUsernamePassword(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create basic auth directory with username and password files (kubernetes.io/basic-auth format)
	authDir := filepath.Join(tempDir, "basic-auth")
	err = os.MkdirAll(authDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Write username file
	err = os.WriteFile(filepath.Join(authDir, "username"), []byte("testuser"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	// Write password file
	err = os.WriteFile(filepath.Join(authDir, "password"), []byte("testpass"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		URL:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_SSHSetup tests that SSH keys are properly set up
func Test_GitClone_SSHSetup(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create SSH directory with test files
	sshDir := filepath.Join(tempDir, "ssh-keys")
	err = os.MkdirAll(sshDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Create fake SSH key files
	fakePrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDFakeKeyForTestingPurposesOnlyDoNotUseAAAA==
-----END OPENSSH PRIVATE KEY-----`
	err = os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte(fakePrivateKey), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	fakePublicKey := "ssh-ed25519 AAAAC3NzaFakeKeyForTestingPurposesOnly test@example.com"
	err = os.WriteFile(filepath.Join(sshDir, "id_rsa.pub"), []byte(fakePublicKey), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	knownHosts := "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"
	err = os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(knownHosts), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	sshConfig := "Host github.com\n  IdentityFile ~/.ssh/id_rsa\n  StrictHostKeyChecking no\n"
	err = os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Clone a public repo (SSH keys won't actually be used, but setup should work)
	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		SSHDirectory:      sshDir,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_ChainsResults tests that CHAINS results are properly set
func Test_GitClone_ChainsResults(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify CHAINS results are set correctly
	g.Expect(gitClone.Results.ChainsGitURL).To(Equal("https://github.com/kelseyhightower/nocode"))
	g.Expect(gitClone.Results.ChainsGitCommit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))

	// Verify they match the regular results
	g.Expect(gitClone.Results.ChainsGitURL).To(Equal(gitClone.Results.URL))
	g.Expect(gitClone.Results.ChainsGitCommit).To(Equal(gitClone.Results.Commit))
}

// Test_GitClone_BasicAuthInvalidFormat tests that invalid basic auth format is rejected
func Test_GitClone_BasicAuthInvalidFormat(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create basic auth directory with invalid format (only username, no password)
	authDir := filepath.Join(tempDir, "basic-auth")
	err = os.MkdirAll(authDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Write only username file (missing password)
	err = os.WriteFile(filepath.Join(authDir, "username"), []byte("testuser"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		URL:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unknown basic-auth workspace format"))
}

// Test_GitClone_DoesNotModifyGlobalGitConfig verifies that git-clone does not modify global git config
func Test_GitClone_DoesNotModifyGlobalGitConfig(t *testing.T) {
	g := NewWithT(t)

	// Get current global git config state
	executor := cliwrappers.NewCliExecutor()
	configBefore, _, _, _ := executor.Execute("git", "config", "--global", "--list")

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitClone, err := newGitClone(&git_clone.Params{
		URL:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		SSLVerify:         false, // This should NOT modify global config
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify global git config was not modified
	configAfter, _, _, _ := executor.Execute("git", "config", "--global", "--list")
	g.Expect(configAfter).To(Equal(configBefore), "Global git config should not be modified by git-clone")
}

// Test_GitClone_DoesNotWriteToHomeDirectory verifies that git-clone does not write
// .git-credentials, .gitconfig, or .ssh/ to the user's $HOME directory.
func Test_GitClone_DoesNotWriteToHomeDirectory(t *testing.T) {
	g := NewWithT(t)

	home, err := os.UserHomeDir()
	g.Expect(err).ToNot(HaveOccurred())

	// Snapshot $HOME state before the run
	homeGitCredsBefore := fileModTime(filepath.Join(home, ".git-credentials"))
	homeGitConfigBefore := fileModTime(filepath.Join(home, ".gitconfig"))
	homeSSHBefore := fileModTime(filepath.Join(home, ".ssh"))

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create basic auth directory
	authDir := filepath.Join(tempDir, "basic-auth")
	err = os.MkdirAll(authDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(filepath.Join(authDir, ".git-credentials"), []byte("https://u:p@github.com\n"), 0600)
	g.Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(filepath.Join(authDir, ".gitconfig"), []byte("[credential \"https://github.com\"]\n  helper = store\n"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		URL:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify nothing was written to $HOME
	homeGitCredsAfter := fileModTime(filepath.Join(home, ".git-credentials"))
	homeGitConfigAfter := fileModTime(filepath.Join(home, ".gitconfig"))
	homeSSHAfter := fileModTime(filepath.Join(home, ".ssh"))

	g.Expect(homeGitCredsAfter).To(Equal(homeGitCredsBefore), "$HOME/.git-credentials should not be modified")
	g.Expect(homeGitConfigAfter).To(Equal(homeGitConfigBefore), "$HOME/.gitconfig should not be modified")
	g.Expect(homeSSHAfter).To(Equal(homeSSHBefore), "$HOME/.ssh should not be modified")
}

// fileModTime returns the modification time of a file, or zero time if the file does not exist.
func fileModTime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

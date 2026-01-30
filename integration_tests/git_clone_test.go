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

// Test_GitClone_FailForWrongUrl tests that git-clone fails when given a non-existent repository URL
func Test_GitClone_FailForWrongUrl(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/user/repo-does-not-exist",
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
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	g.Expect(gitClone.Results.Url).To(Equal("https://github.com/kelseyhightower/nocode"))

	// Tag 1.0.0 is pinned to this specific commit
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
	g.Expect(gitClone.Results.ShortCommit).To(Equal("ed6c73f"))
}

// Test_GitClone_RunWithoutArgs tests that git-clone successfully clones a repository with just the URL
func Test_GitClone_RunWithoutArgs(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	g.Expect(gitClone.Results.Url).To(Equal("https://github.com/kelseyhightower/nocode"))
}

// Test_GitClone_WithDepth tests cloning with specific history depth
func Test_GitClone_WithDepth(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

	// Create existing directory with a file that should be deleted
	checkoutDir := filepath.Join(tempDir, "source")
	err = os.MkdirAll(checkoutDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	existingFile := filepath.Join(checkoutDir, "should-be-deleted.txt")
	err = os.WriteFile(existingFile, []byte("this file should be deleted"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

	// Using a repo with submodules
	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/konflux-ci/buildah-container",
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
	defer os.RemoveAll(tempDir)

	// Clone with a refspec to fetch a specific PR ref
	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

	// Clone only specific directories using sparse checkout
	gitClone, err := newGitClone(&git_clone.Params{
		Url:                       "https://github.com/konflux-ci/build-definitions",
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
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:                "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

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
		Url:                repoDir, // Clone from local path
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
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		SslVerify:         false,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))

	// Verify git config was set (check global config)
	executor := cliwrappers.NewCliExecutor()
	stdout, _, exitCode, err := executor.Execute("git", "config", "--global", "http.sslVerify")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exitCode).To(Equal(0))
	g.Expect(strings.TrimSpace(stdout)).To(Equal("false"))
}

// Test_GitClone_BasicAuthWithGitCredentials tests basic auth using .git-credentials and .gitconfig files
func Test_GitClone_BasicAuthWithGitCredentials(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	// Create a fake user home directory
	userHome := filepath.Join(tempDir, "home")
	err = os.MkdirAll(userHome, 0755)
	g.Expect(err).ToNot(HaveOccurred())

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
		Url:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		UserHome:           userHome,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the credentials files were copied to user home
	copiedCredentials := filepath.Join(userHome, ".git-credentials")
	copiedConfig := filepath.Join(userHome, ".gitconfig")

	_, err = os.Stat(copiedCredentials)
	g.Expect(err).ToNot(HaveOccurred(), ".git-credentials should be copied to user home")

	_, err = os.Stat(copiedConfig)
	g.Expect(err).ToNot(HaveOccurred(), ".gitconfig should be copied to user home")

	// Verify file permissions (should be 0400)
	credInfo, err := os.Stat(copiedCredentials)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(credInfo.Mode().Perm()).To(Equal(os.FileMode(0400)))

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_BasicAuthWithUsernamePassword tests basic auth using username/password files (k8s secret format)
func Test_GitClone_BasicAuthWithUsernamePassword(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	// Create a fake user home directory
	userHome := filepath.Join(tempDir, "home")
	err = os.MkdirAll(userHome, 0755)
	g.Expect(err).ToNot(HaveOccurred())

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
		Url:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		UserHome:           userHome,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify .git-credentials was created with correct content
	copiedCredentials := filepath.Join(userHome, ".git-credentials")
	credContent, err := os.ReadFile(copiedCredentials)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(credContent)).To(ContainSubstring("https://testuser:testpass@github.com"))

	// Verify .gitconfig was created
	copiedConfig := filepath.Join(userHome, ".gitconfig")
	configContent, err := os.ReadFile(copiedConfig)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(configContent)).To(ContainSubstring("[credential \"https://github.com\"]"))
	g.Expect(string(configContent)).To(ContainSubstring("helper = store"))

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_SSHSetup tests that SSH keys are properly set up
func Test_GitClone_SSHSetup(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	// Create a fake user home directory
	userHome := filepath.Join(tempDir, "home")
	err = os.MkdirAll(userHome, 0755)
	g.Expect(err).ToNot(HaveOccurred())

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
		Url:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		UserHome:          userHome,
		SshDirectory:      sshDir,
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify SSH directory was created in user home
	destSSHDir := filepath.Join(userHome, ".ssh")
	info, err := os.Stat(destSSHDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(info.IsDir()).To(BeTrue())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0700)))

	// Verify all SSH files were copied
	for _, fileName := range []string{"id_rsa", "id_rsa.pub", "known_hosts", "config"} {
		destPath := filepath.Join(destSSHDir, fileName)
		_, err := os.Stat(destPath)
		g.Expect(err).ToNot(HaveOccurred(), "SSH file %s should be copied", fileName)

		// Verify permissions are 0400
		info, err := os.Stat(destPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0400)))
	}

	// Verify private key content was preserved
	copiedKey, err := os.ReadFile(filepath.Join(destSSHDir, "id_rsa"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(copiedKey)).To(Equal(fakePrivateKey))

	// Verify clone succeeded
	g.Expect(gitClone.Results.Commit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))
}

// Test_GitClone_ProxySetup tests that proxy environment variables are set
func Test_GitClone_ProxySetup(t *testing.T) {
	g := NewWithT(t)

	// Save original env vars to restore later
	origHttpProxy := os.Getenv("HTTP_PROXY")
	origHttpsProxy := os.Getenv("HTTPS_PROXY")
	origNoProxy := os.Getenv("NO_PROXY")
	defer func() {
		os.Setenv("HTTP_PROXY", origHttpProxy)
		os.Setenv("HTTPS_PROXY", origHttpsProxy)
		os.Setenv("NO_PROXY", origNoProxy)
	}()

	// Clear proxy env vars before test
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("NO_PROXY")

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
		Revision:          "1.0.0",
		OutputDir:         tempDir,
		Subdirectory:      "source",
		Depth:             1,
		ShortCommitLength: 7,
		HttpProxy:         "http://proxy.example.com:8080",
		HttpsProxy:        "https://proxy.example.com:8443",
		NoProxy:           "localhost,127.0.0.1,.example.com",
		RetryMaxAttempts:  3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Note: The clone will still succeed because GitHub is likely in NO_PROXY
	// or the proxy values are fake. The important thing is that the env vars are set.
	err = gitClone.Run()
	// We expect this might fail since the proxy is fake, but let's check env vars were set
	// before the clone attempted

	// Check that environment variables were set (they persist after Run())
	g.Expect(os.Getenv("HTTP_PROXY")).To(Equal("http://proxy.example.com:8080"))
	g.Expect(os.Getenv("HTTPS_PROXY")).To(Equal("https://proxy.example.com:8443"))
	g.Expect(os.Getenv("NO_PROXY")).To(Equal("localhost,127.0.0.1,.example.com"))
}

// Test_GitClone_ChainsResults tests that CHAINS results are properly set
func Test_GitClone_ChainsResults(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	gitClone, err := newGitClone(&git_clone.Params{
		Url:               "https://github.com/kelseyhightower/nocode",
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
	g.Expect(gitClone.Results.ChainsGitUrl).To(Equal("https://github.com/kelseyhightower/nocode"))
	g.Expect(gitClone.Results.ChainsGitCommit).To(Equal("ed6c73fc16578ec53ea374585df2b965ce9f4a31"))

	// Verify they match the regular results
	g.Expect(gitClone.Results.ChainsGitUrl).To(Equal(gitClone.Results.Url))
	g.Expect(gitClone.Results.ChainsGitCommit).To(Equal(gitClone.Results.Commit))
}

// Test_GitClone_BasicAuthInvalidFormat tests that invalid basic auth format is rejected
func Test_GitClone_BasicAuthInvalidFormat(t *testing.T) {
	g := NewWithT(t)

	tempDir, err := os.MkdirTemp("", "git-clone-test-*")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)

	// Create a fake user home directory
	userHome := filepath.Join(tempDir, "home")
	err = os.MkdirAll(userHome, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Create basic auth directory with invalid format (only username, no password)
	authDir := filepath.Join(tempDir, "basic-auth")
	err = os.MkdirAll(authDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Write only username file (missing password)
	err = os.WriteFile(filepath.Join(authDir, "username"), []byte("testuser"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	gitClone, err := newGitClone(&git_clone.Params{
		Url:                "https://github.com/kelseyhightower/nocode",
		Revision:           "1.0.0",
		OutputDir:          tempDir,
		Subdirectory:       "source",
		Depth:              1,
		ShortCommitLength:  7,
		UserHome:           userHome,
		BasicAuthDirectory: authDir,
		RetryMaxAttempts:   3,
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = gitClone.Run()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unknown basic-auth workspace format"))
}

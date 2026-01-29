package git_clone

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_GitClone_validateParams(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		params      Params
		expectError bool
		errContains string
	}{
		{
			name: "should pass with valid URL",
			params: Params{
				URL: "https://github.com/user/repo.git",
			},
			expectError: false,
		},
		{
			name: "should pass with URL and revision",
			params: Params{
				URL:      "https://github.com/user/repo.git",
				Revision: "main",
			},
			expectError: false,
		},
		{
			name: "should pass with all parameters",
			params: Params{
				URL:               "https://github.com/user/repo.git",
				Revision:          "v1.0.0",
				Depth:             10,
				ShortCommitLength: 8,
				OutputDir:         "/tmp",
				Subdirectory:      "source",
			},
			expectError: false,
		},
		{
			name:        "should fail with empty URL",
			params:      Params{},
			expectError: true,
			errContains: "url parameter is required",
		},
		{
			name: "should fail with empty URL string",
			params: Params{
				URL: "",
			},
			expectError: true,
			errContains: "url parameter is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &GitClone{
				Params: &tc.params,
			}

			err := c.validateParams()

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errContains))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func Test_GitClone_getCheckoutDir(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		params   Params
		expected string
	}{
		{
			name: "should return default path",
			params: Params{
				OutputDir:    ".",
				Subdirectory: "source",
			},
			expected: "source",
		},
		{
			name: "should combine output dir and subdirectory",
			params: Params{
				OutputDir:    "/tmp/workspace",
				Subdirectory: "source",
			},
			expected: "/tmp/workspace/source",
		},
		{
			name: "should handle custom subdirectory",
			params: Params{
				OutputDir:    "/workspace",
				Subdirectory: "my-repo",
			},
			expected: "/workspace/my-repo",
		},
		{
			name: "should handle empty subdirectory",
			params: Params{
				OutputDir:    "/workspace",
				Subdirectory: "",
			},
			expected: "/workspace",
		},
		{
			name: "should handle nested subdirectory",
			params: Params{
				OutputDir:    "/workspace",
				Subdirectory: "repos/source",
			},
			expected: "/workspace/repos/source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &GitClone{
				Params: &tc.params,
			}

			result := c.getCheckoutDir()

			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func Test_GitClone_gatherCommitInfo(t *testing.T) {
	g := NewWithT(t)

	const checkoutDir = "/workspace/source"
	const fullSha = "abc123def456789012345678901234567890abcd"
	const shortSha = "abc123d"
	const timestamp = "1704067200"

	var _mockGitCli *mockGitCli
	var c *GitClone

	beforeEach := func() {
		_mockGitCli = &mockGitCli{}
		c = &GitClone{
			CliWrappers: CliWrappers{GitCli: _mockGitCli},
			Params: &Params{
				URL:               "https://github.com/user/repo.git",
				OutputDir:         "/workspace",
				Subdirectory:      "source",
				ShortCommitLength: 7,
			},
		}
	}

	t.Run("should gather all commit info successfully", func(t *testing.T) {
		beforeEach()

		revParseCallCount := 0
		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			g.Expect(workdir).To(Equal(checkoutDir))
			g.Expect(ref).To(Equal("HEAD"))

			revParseCallCount++
			if !short {
				return fullSha, nil
			}
			g.Expect(length).To(Equal(7))
			return shortSha, nil
		}

		isLogCalled := false
		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			isLogCalled = true
			g.Expect(workdir).To(Equal(checkoutDir))
			g.Expect(format).To(Equal("%ct"))
			g.Expect(count).To(Equal(1))
			return timestamp, nil
		}

		err := c.gatherCommitInfo()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(revParseCallCount).To(Equal(2))
		g.Expect(isLogCalled).To(BeTrue())
		g.Expect(c.Results.Commit).To(Equal(fullSha))
		g.Expect(c.Results.ShortCommit).To(Equal(shortSha))
		g.Expect(c.Results.CommitTimestamp).To(Equal(timestamp))
		g.Expect(c.Results.URL).To(Equal("https://github.com/user/repo.git"))
		g.Expect(c.Results.ChainsGitURL).To(Equal("https://github.com/user/repo.git"))
		g.Expect(c.Results.ChainsGitCommit).To(Equal(fullSha))
	})

	t.Run("should fail if getting full SHA fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if !short {
				return "", errors.New("rev-parse failed")
			}
			return shortSha, nil
		}

		err := c.gatherCommitInfo()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get commit SHA"))
	})

	t.Run("should fail if getting short SHA fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				return "", errors.New("rev-parse short failed")
			}
			return fullSha, nil
		}

		err := c.gatherCommitInfo()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get short commit SHA"))
	})

	t.Run("should fail if getting timestamp fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				return shortSha, nil
			}
			return fullSha, nil
		}

		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			return "", errors.New("git log failed")
		}

		err := c.gatherCommitInfo()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get commit timestamp"))
	})

	t.Run("should use custom short commit length", func(t *testing.T) {
		beforeEach()
		c.Params.ShortCommitLength = 12

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				g.Expect(length).To(Equal(12))
				return "abc123def456", nil
			}
			return fullSha, nil
		}

		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			return timestamp, nil
		}

		err := c.gatherCommitInfo()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.ShortCommit).To(Equal("abc123def456"))
	})
}

func Test_GitClone_performClone(t *testing.T) {
	g := NewWithT(t)

	var _mockGitCli *mockGitCli
	var c *GitClone
	var tmpDir string

	beforeEach := func() {
		tmpDir = t.TempDir()
		_mockGitCli = &mockGitCli{}
		c = &GitClone{
			CliWrappers: CliWrappers{GitCli: _mockGitCli},
			Params: &Params{
				URL:              "https://github.com/user/repo.git",
				Depth:            1,
				RetryMaxAttempts: 10,
				OutputDir:        tmpDir,
				Subdirectory:     "source",
			},
		}
	}

	t.Run("should clone with basic parameters using init+fetch+checkout", func(t *testing.T) {
		beforeEach()

		isInitCalled := false
		_mockGitCli.InitFunc = func(workdir string) error {
			isInitCalled = true
			g.Expect(workdir).To(ContainSubstring("source"))
			return nil
		}

		isRemoteAddCalled := false
		_mockGitCli.RemoteAddFunc = func(workdir, name, url string) (string, error) {
			isRemoteAddCalled = true
			g.Expect(name).To(Equal("origin"))
			g.Expect(url).To(Equal("https://github.com/user/repo.git"))
			return "", nil
		}

		isFetchCalled := false
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			isFetchCalled = true
			g.Expect(remote).To(Equal("origin"))
			g.Expect(depth).To(Equal(1))
			return nil
		}

		isCheckoutCalled := false
		_mockGitCli.CheckoutFunc = func(workdir, ref string) error {
			isCheckoutCalled = true
			return nil
		}

		err := c.performClone()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isInitCalled).To(BeTrue())
		g.Expect(isRemoteAddCalled).To(BeTrue())
		g.Expect(isFetchCalled).To(BeTrue())
		g.Expect(isCheckoutCalled).To(BeTrue())
	})

	t.Run("should fetch with revision as refspec", func(t *testing.T) {
		beforeEach()
		c.Params.Revision = "develop"

		isFetchCalled := false
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			isFetchCalled = true
			g.Expect(refspec).To(Equal("develop"))
			return nil
		}

		err := c.performClone()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isFetchCalled).To(BeTrue())
	})

	t.Run("should fetch with custom depth", func(t *testing.T) {
		beforeEach()
		c.Params.Depth = 50

		isFetchCalled := false
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			isFetchCalled = true
			g.Expect(depth).To(Equal(50))
			return nil
		}

		err := c.performClone()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isFetchCalled).To(BeTrue())
	})

	t.Run("should fail if init fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.InitFunc = func(workdir string) error {
			return errors.New("init failed")
		}

		err := c.performClone()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git init failed"))
	})

	t.Run("should pass maxAttempts to FetchWithRefspec", func(t *testing.T) {
		beforeEach()
		c.Params.RetryMaxAttempts = 5

		receivedMaxAttempts := 0
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			receivedMaxAttempts = maxAttempts
			return nil
		}

		err := c.performClone()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(receivedMaxAttempts).To(Equal(5))
	})

	t.Run("should fail if fetch fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			return errors.New("network error")
		}

		err := c.performClone()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git fetch failed"))
	})

	t.Run("should update submodules when enabled", func(t *testing.T) {
		beforeEach()
		c.Params.Submodules = true
		c.Params.SubmodulePaths = "lib,vendor"

		isSubmoduleUpdateCalled := false
		_mockGitCli.SubmoduleUpdateFunc = func(workdir string, init bool, depth int, paths []string) error {
			isSubmoduleUpdateCalled = true
			g.Expect(init).To(BeTrue())
			g.Expect(paths).To(Equal([]string{"lib", "vendor"}))
			return nil
		}

		err := c.performClone()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isSubmoduleUpdateCalled).To(BeTrue())
	})
}

func Test_GitClone_outputResults(t *testing.T) {
	g := NewWithT(t)

	var _mockResultsWriter *mockResultsWriter
	var c *GitClone

	beforeEach := func() {
		_mockResultsWriter = &mockResultsWriter{}
		c = &GitClone{
			ResultsWriter: _mockResultsWriter,
			Results: Results{
				Commit:          "abc123def456789012345678901234567890abcd",
				ShortCommit:     "abc123d",
				URL:             "https://github.com/user/repo.git",
				CommitTimestamp: "1704067200",
			},
		}
	}

	t.Run("should output results successfully", func(t *testing.T) {
		beforeEach()

		isCreateResultJsonCalled := false
		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			isCreateResultJsonCalled = true
			results, ok := result.(Results)
			g.Expect(ok).To(BeTrue())
			g.Expect(results.Commit).To(Equal("abc123def456789012345678901234567890abcd"))
			g.Expect(results.ShortCommit).To(Equal("abc123d"))
			g.Expect(results.URL).To(Equal("https://github.com/user/repo.git"))
			g.Expect(results.CommitTimestamp).To(Equal("1704067200"))
			return `{"commit":"abc123def456789012345678901234567890abcd"}`, nil
		}

		err := c.outputResults()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isCreateResultJsonCalled).To(BeTrue())
	})

	t.Run("should output results with merged SHA", func(t *testing.T) {
		beforeEach()
		c.Results.MergedSha = "def456abc789"

		isCreateResultJsonCalled := false
		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			isCreateResultJsonCalled = true
			results, ok := result.(Results)
			g.Expect(ok).To(BeTrue())
			g.Expect(results.MergedSha).To(Equal("def456abc789"))
			return `{"commit":"abc123","mergedSha":"def456abc789"}`, nil
		}

		err := c.outputResults()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isCreateResultJsonCalled).To(BeTrue())
	})

	t.Run("should fail if creating result json fails", func(t *testing.T) {
		beforeEach()

		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			return "", errors.New("failed to create json")
		}

		err := c.outputResults()

		g.Expect(err).To(HaveOccurred())
	})
}

func Test_GitClone_Run(t *testing.T) {
	g := NewWithT(t)

	const fullSha = "abc123def456789012345678901234567890abcd"
	const shortSha = "abc123d"
	const timestamp = "1704067200"

	var _mockGitCli *mockGitCli
	var _mockResultsWriter *mockResultsWriter
	var c *GitClone
	var tmpDir string

	beforeEach := func() {
		tmpDir = t.TempDir()
		_mockGitCli = &mockGitCli{}
		_mockResultsWriter = &mockResultsWriter{}
		c = &GitClone{
			CliWrappers:   CliWrappers{GitCli: _mockGitCli},
			ResultsWriter: _mockResultsWriter,
			Params: &Params{
				URL:               "https://github.com/user/repo.git",
				Depth:             1,
				ShortCommitLength: 7,
				OutputDir:         tmpDir,
				Subdirectory:      "source",
				RetryMaxAttempts:  10,
			},
		}
	}

	t.Run("should run successfully with basic parameters", func(t *testing.T) {
		beforeEach()

		isInitCalled := false
		_mockGitCli.InitFunc = func(workdir string) error {
			isInitCalled = true
			return nil
		}

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				return shortSha, nil
			}
			return fullSha, nil
		}

		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			return timestamp, nil
		}

		isCreateResultJsonCalled := false
		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			isCreateResultJsonCalled = true
			results, ok := result.(Results)
			g.Expect(ok).To(BeTrue())
			g.Expect(results.Commit).To(Equal(fullSha))
			g.Expect(results.ShortCommit).To(Equal(shortSha))
			g.Expect(results.CommitTimestamp).To(Equal(timestamp))
			return `{"commit":"abc123"}`, nil
		}

		err := c.Run()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isInitCalled).To(BeTrue())
		g.Expect(isCreateResultJsonCalled).To(BeTrue())
	})

	t.Run("should fail if URL is empty", func(t *testing.T) {
		beforeEach()
		c.Params.URL = ""

		err := c.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("url parameter is required"))
	})

	t.Run("should fail if init fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.InitFunc = func(workdir string) error {
			return errors.New("init failed")
		}

		err := c.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git init failed"))
	})

	t.Run("should fail if gathering commit info fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			return "", errors.New("rev-parse failed")
		}

		err := c.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get commit SHA"))
	})

	t.Run("should fail if outputting results fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				return shortSha, nil
			}
			return fullSha, nil
		}

		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			return timestamp, nil
		}

		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			return "", errors.New("json marshal failed")
		}

		err := c.Run()

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("should run with revision parameter", func(t *testing.T) {
		beforeEach()
		c.Params.Revision = "v1.0.0"

		isFetchCalled := false
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			isFetchCalled = true
			g.Expect(refspec).To(Equal("v1.0.0"))
			return nil
		}

		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			if short {
				return shortSha, nil
			}
			return fullSha, nil
		}

		_mockGitCli.LogFunc = func(workdir string, format string, count int) (string, error) {
			return timestamp, nil
		}

		_mockResultsWriter.CreateResultJsonFunc = func(result any) (string, error) {
			return `{}`, nil
		}

		err := c.Run()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isFetchCalled).To(BeTrue())
	})
}

func Test_normalizeGitURL(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "should strip trailing slash",
			input:    "https://github.com/user/repo/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "should strip .git suffix",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "should strip both trailing slash and .git",
			input:    "https://github.com/user/repo.git/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "should not modify clean URL",
			input:    "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeGitURL(tc.input)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func Test_GitClone_mergeTargetBranch(t *testing.T) {
	g := NewWithT(t)

	const mergedSha = "merged123456789"

	var _mockGitCli *mockGitCli
	var c *GitClone

	beforeEach := func() {
		_mockGitCli = &mockGitCli{}
		c = &GitClone{
			CliWrappers: CliWrappers{GitCli: _mockGitCli},
			Params: &Params{
				URL:              "https://github.com/user/repo.git",
				OutputDir:        "/workspace",
				Subdirectory:     "source",
				TargetBranch:     "main",
				Depth:            10,
				MergeSourceDepth: 0,
				RetryMaxAttempts: 3,
			},
			Results: Results{
				Commit: "abc123",
			},
		}
		_mockGitCli.RevParseFunc = func(workdir string, ref string, short bool, length int) (string, error) {
			return mergedSha, nil
		}
	}

	t.Run("should merge from origin when no merge source URL", func(t *testing.T) {
		beforeEach()

		fetchedRemote := ""
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			fetchedRemote = remote
			g.Expect(refspec).To(Equal("main"))
			g.Expect(submodules).To(BeFalse())
			return nil
		}

		mergedRef := ""
		_mockGitCli.MergeFunc = func(workdir, fetchHead string) (string, error) {
			mergedRef = fetchHead
			return "", nil
		}

		err := c.mergeTargetBranch()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fetchedRemote).To(Equal("origin"))
		g.Expect(mergedRef).To(Equal("origin/main"))
		g.Expect(c.Results.MergedSha).To(Equal(mergedSha))
	})

	t.Run("should use origin when merge source URL matches", func(t *testing.T) {
		beforeEach()
		c.Params.MergeSourceRepoURL = "https://github.com/user/repo.git"

		fetchedRemote := ""
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			fetchedRemote = remote
			return nil
		}

		_mockGitCli.MergeFunc = func(workdir, fetchHead string) (string, error) {
			return "", nil
		}

		err := c.mergeTargetBranch()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fetchedRemote).To(Equal("origin"))
	})

	t.Run("should add merge-source remote for different repo", func(t *testing.T) {
		beforeEach()
		c.Params.MergeSourceRepoURL = "https://github.com/other/repo"

		addedRemoteName := ""
		_mockGitCli.RemoteAddFunc = func(workdir, name, url string) (string, error) {
			addedRemoteName = name
			g.Expect(url).To(Equal("https://github.com/other/repo"))
			return "", nil
		}

		fetchedRemote := ""
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			fetchedRemote = remote
			return nil
		}

		mergedRef := ""
		_mockGitCli.MergeFunc = func(workdir, fetchHead string) (string, error) {
			mergedRef = fetchHead
			return "", nil
		}

		err := c.mergeTargetBranch()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(addedRemoteName).To(Equal("merge-source"))
		g.Expect(fetchedRemote).To(Equal("merge-source"))
		g.Expect(mergedRef).To(Equal("merge-source/main"))
	})

	t.Run("should fail if remote add fails", func(t *testing.T) {
		beforeEach()
		c.Params.MergeSourceRepoURL = "https://github.com/other/repo"

		_mockGitCli.RemoteAddFunc = func(workdir, name, url string) (string, error) {
			return "", errors.New("remote add failed")
		}

		err := c.mergeTargetBranch()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("remote add failed"))
	})

	t.Run("should fail if fetch fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int, submodules bool, maxAttempts int) error {
			return errors.New("fetch failed")
		}

		err := c.mergeTargetBranch()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("fetch failed"))
	})

	t.Run("should fail if merge fails", func(t *testing.T) {
		beforeEach()

		_mockGitCli.MergeFunc = func(workdir, fetchHead string) (string, error) {
			return "", errors.New("merge conflict")
		}

		err := c.mergeTargetBranch()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("merge conflict"))
	})

	t.Run("should set config with correct email and name", func(t *testing.T) {
		beforeEach()

		configValues := map[string]string{}
		_mockGitCli.ConfigLocalFunc = func(workdir, key, value string) error {
			configValues[key] = value
			return nil
		}
		_mockGitCli.MergeFunc = func(workdir, fetchHead string) (string, error) {
			return "", nil
		}

		err := c.mergeTargetBranch()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(configValues["user.email"]).To(Equal("git-clone@konflux-ci.dev"))
		g.Expect(configValues["user.name"]).To(Equal("Konflux CI Git Clone"))
	})
}

func Test_GitClone_cleanCheckoutDir(t *testing.T) {
	g := NewWithT(t)

	t.Run("should remove contents but preserve directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		checkoutDir := filepath.Join(tmpDir, "source")
		g.Expect(os.MkdirAll(checkoutDir, 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(checkoutDir, "file1.txt"), []byte("hello"), 0644)).To(Succeed())
		g.Expect(os.MkdirAll(filepath.Join(checkoutDir, "subdir"), 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(checkoutDir, "subdir", "file2.txt"), []byte("world"), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{OutputDir: tmpDir, Subdirectory: "source"},
		}

		err := c.cleanCheckoutDir()

		g.Expect(err).ToNot(HaveOccurred())
		entries, err := os.ReadDir(checkoutDir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entries).To(BeEmpty())
		// Directory itself still exists
		_, err = os.Stat(checkoutDir)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should succeed if directory does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		c := &GitClone{
			Params: &Params{OutputDir: tmpDir, Subdirectory: "nonexistent"},
		}

		err := c.cleanCheckoutDir()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should fail if path is a file not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "source")
		g.Expect(os.WriteFile(filePath, []byte("not a dir"), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{OutputDir: tmpDir, Subdirectory: "source"},
		}

		err := c.cleanCheckoutDir()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("not a directory"))
	})
}

func Test_GitClone_setupBasicAuth(t *testing.T) {
	g := NewWithT(t)

	t.Run("should skip when no basic auth directory", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{BasicAuthDirectory: ""},
		}

		err := c.setupBasicAuth()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should skip when auth directory does not exist", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{BasicAuthDirectory: "/nonexistent/path"},
		}

		err := c.setupBasicAuth()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should copy .git-credentials and rewrite .gitconfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		authDir := filepath.Join(tmpDir, "auth")
		internalDir := t.TempDir()
		g.Expect(os.MkdirAll(authDir, 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(authDir, ".git-credentials"), []byte("https://user:pass@github.com"), 0644)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(authDir, ".gitconfig"), []byte("[credential]\n  helper = store"), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{
				URL:                "https://github.com/user/repo",
				BasicAuthDirectory: authDir,
			},
			internalDir: internalDir,
		}

		defer func() { _ = os.Unsetenv("GIT_CONFIG_GLOBAL") }()
		err := c.setupBasicAuth()

		g.Expect(err).ToNot(HaveOccurred())
		creds, err := os.ReadFile(filepath.Join(internalDir, ".git-credentials"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(creds)).To(Equal("https://user:pass@github.com"))
		config, err := os.ReadFile(filepath.Join(internalDir, ".gitconfig"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(config)).To(ContainSubstring("helper = store --file=" + filepath.Join(internalDir, ".git-credentials")))
		g.Expect(os.Getenv("GIT_CONFIG_GLOBAL")).To(Equal(filepath.Join(internalDir, ".gitconfig")))
	})

	t.Run("should generate credentials from username/password", func(t *testing.T) {
		tmpDir := t.TempDir()
		authDir := filepath.Join(tmpDir, "auth")
		internalDir := t.TempDir()
		g.Expect(os.MkdirAll(authDir, 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(authDir, "username"), []byte("myuser"), 0644)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(authDir, "password"), []byte("mypass"), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{
				URL:                "https://github.com/user/repo",
				BasicAuthDirectory: authDir,
			},
			internalDir: internalDir,
		}

		defer func() { _ = os.Unsetenv("GIT_CONFIG_GLOBAL") }()
		err := c.setupBasicAuth()

		g.Expect(err).ToNot(HaveOccurred())
		creds, err := os.ReadFile(filepath.Join(internalDir, ".git-credentials"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(creds)).To(Equal("https://myuser:mypass@github.com\n"))
		config, err := os.ReadFile(filepath.Join(internalDir, ".gitconfig"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(config)).To(ContainSubstring("helper = store --file=" + filepath.Join(internalDir, ".git-credentials")))
		g.Expect(os.Getenv("GIT_CONFIG_GLOBAL")).To(Equal(filepath.Join(internalDir, ".gitconfig")))
	})

	t.Run("should fail with unknown auth format", func(t *testing.T) {
		tmpDir := t.TempDir()
		authDir := filepath.Join(tmpDir, "auth")
		g.Expect(os.MkdirAll(authDir, 0755)).To(Succeed())
		// Create an empty directory - neither format matches

		c := &GitClone{
			Params: &Params{
				URL:                "https://github.com/user/repo",
				BasicAuthDirectory: authDir,
			},
			internalDir: t.TempDir(),
		}

		err := c.setupBasicAuth()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unknown basic-auth workspace format"))
	})
}

func Test_GitClone_setupSSH(t *testing.T) {
	g := NewWithT(t)

	t.Run("should skip when no ssh directory", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{SSHDirectory: ""},
		}

		err := c.setupSSH()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should skip when ssh directory does not exist", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{SSHDirectory: "/nonexistent/path"},
		}

		err := c.setupSSH()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should copy SSH files and set GIT_SSH_COMMAND", func(t *testing.T) {
		tmpDir := t.TempDir()
		sshDir := filepath.Join(tmpDir, "ssh-keys")
		internalDir := t.TempDir()
		g.Expect(os.MkdirAll(sshDir, 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("private-key"), 0644)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte("github.com ssh-rsa AAAA..."), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{
				SSHDirectory: sshDir,
			},
			internalDir: internalDir,
		}

		defer func() { _ = os.Unsetenv("GIT_SSH_COMMAND") }()
		err := c.setupSSH()

		g.Expect(err).ToNot(HaveOccurred())
		key, err := os.ReadFile(filepath.Join(internalDir, ".ssh", "id_rsa"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(key)).To(Equal("private-key"))
		hosts, err := os.ReadFile(filepath.Join(internalDir, ".ssh", "known_hosts"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(hosts)).To(Equal("github.com ssh-rsa AAAA..."))

		sshCmd := os.Getenv("GIT_SSH_COMMAND")
		g.Expect(sshCmd).To(ContainSubstring("-i " + filepath.Join(internalDir, ".ssh", "id_rsa")))
		g.Expect(sshCmd).To(ContainSubstring("-o UserKnownHostsFile=" + filepath.Join(internalDir, ".ssh", "known_hosts")))
	})

	t.Run("should skip subdirectories", func(t *testing.T) {
		tmpDir := t.TempDir()
		sshDir := filepath.Join(tmpDir, "ssh-keys")
		internalDir := t.TempDir()
		g.Expect(os.MkdirAll(sshDir, 0755)).To(Succeed())
		g.Expect(os.MkdirAll(filepath.Join(sshDir, "subdir"), 0755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("key"), 0644)).To(Succeed())

		c := &GitClone{
			Params: &Params{
				SSHDirectory: sshDir,
			},
			internalDir: internalDir,
		}

		defer func() { _ = os.Unsetenv("GIT_SSH_COMMAND") }()
		err := c.setupSSH()

		g.Expect(err).ToNot(HaveOccurred())
		// Subdir should not be copied
		_, err = os.Stat(filepath.Join(internalDir, ".ssh", "subdir"))
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	})
}

func Test_GitClone_validateParams_depth(t *testing.T) {
	g := NewWithT(t)

	t.Run("should fail with negative depth", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{URL: "https://github.com/user/repo", Depth: -1},
		}

		err := c.validateParams()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("depth must be >= 0"))
	})

	t.Run("should pass with depth 0 (full history)", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{URL: "https://github.com/user/repo", Depth: 0},
		}

		err := c.validateParams()

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should fail with negative merge-source-depth", func(t *testing.T) {
		c := &GitClone{
			Params: &Params{URL: "https://github.com/user/repo", MergeSourceDepth: -5},
		}

		err := c.validateParams()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("merge-source-depth must be >= 0"))
	})
}

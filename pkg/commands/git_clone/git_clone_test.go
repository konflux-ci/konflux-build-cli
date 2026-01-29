package git_clone

import (
	"errors"
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
				Url: "https://github.com/user/repo.git",
			},
			expectError: false,
		},
		{
			name: "should pass with URL and revision",
			params: Params{
				Url:      "https://github.com/user/repo.git",
				Revision: "main",
			},
			expectError: false,
		},
		{
			name: "should pass with all parameters",
			params: Params{
				Url:               "https://github.com/user/repo.git",
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
				Url: "",
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
				Url:               "https://github.com/user/repo.git",
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
		g.Expect(c.Results.Url).To(Equal("https://github.com/user/repo.git"))
		g.Expect(c.Results.ChainsGitUrl).To(Equal("https://github.com/user/repo.git"))
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
				Url:              "https://github.com/user/repo.git",
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
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int) error {
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
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int) error {
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
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int) error {
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

	t.Run("should fail if fetch fails after retries", func(t *testing.T) {
		beforeEach()
		c.Params.RetryMaxAttempts = 2

		fetchAttempts := 0
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int) error {
			fetchAttempts++
			return errors.New("network error")
		}

		err := c.performClone()

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("git fetch failed after 2 attempts"))
		g.Expect(fetchAttempts).To(Equal(2))
	})

	t.Run("should update submodules when enabled", func(t *testing.T) {
		beforeEach()
		c.Params.Submodules = true
		c.Params.SubmodulePaths = "lib,vendor"

		isSubmoduleUpdateCalled := false
		_mockGitCli.SubmoduleUpdateFunc = func(workdir string, init bool, paths string) error {
			isSubmoduleUpdateCalled = true
			g.Expect(init).To(BeTrue())
			g.Expect(paths).To(Equal("lib,vendor"))
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
				Url:             "https://github.com/user/repo.git",
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
			g.Expect(results.Url).To(Equal("https://github.com/user/repo.git"))
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
				Url:               "https://github.com/user/repo.git",
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
		c.Params.Url = ""

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
		_mockGitCli.FetchWithRefspecFunc = func(workdir, remote, refspec string, depth int) error {
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
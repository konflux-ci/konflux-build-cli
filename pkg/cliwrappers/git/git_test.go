package git

import (
	"errors"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	. "github.com/onsi/gomega"
)

var _ cliwrappers.CliExecutorInterface = &mockExecutor{}

type mockExecutor struct {
	executeFunc            func(command string, args ...string) (string, string, int, error)
	executeInDirFunc       func(workdir, command string, args ...string) (string, string, int, error)
	executeWithOutput      func(command string, args ...string) (string, string, int, error)
	executeInDirWithOutput func(workdir, command string, args ...string) (string, string, int, error)
}

func (m *mockExecutor) Execute(command string, args ...string) (string, string, int, error) {
	if m.executeFunc != nil {
		return m.executeFunc(command, args...)
	}
	return "", "", 0, nil
}

func (m *mockExecutor) ExecuteInDir(workdir, command string, args ...string) (string, string, int, error) {
	if m.executeInDirFunc != nil {
		return m.executeInDirFunc(workdir, command, args...)
	}
	return "", "", 0, nil
}

func (m *mockExecutor) ExecuteWithOutput(command string, args ...string) (string, string, int, error) {
	if m.executeWithOutput != nil {
		return m.executeWithOutput(command, args...)
	}
	return "", "", 0, nil
}

func (m *mockExecutor) ExecuteInDirWithOutput(workdir, command string, args ...string) (string, string, int, error) {
	if m.executeInDirWithOutput != nil {
		return m.executeInDirWithOutput(workdir, command, args...)
	}
	return "", "", 0, nil
}

func Test_parseGitVersion(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		input       string
		expected    [3]int
		expectError bool
		errContains string
	}{
		{
			name:     "should parse standard version",
			input:    "git version 2.43.0",
			expected: [3]int{2, 43, 0},
		},
		{
			name:     "should parse version with trailing newline",
			input:    "git version 2.25.1\n",
			expected: [3]int{2, 25, 1},
		},
		{
			name:     "should parse version with extra components (e.g. Apple Git)",
			input:    "git version 2.39.5.1.3",
			expected: [3]int{2, 39, 5},
		},
		{
			name:        "should fail on missing prefix",
			input:       "2.43.0",
			expectError: true,
			errContains: "failed to parse git version",
		},
		{
			name:        "should fail on too few components",
			input:       "git version 2.43",
			expectError: true,
			errContains: "failed to parse git version",
		},
		{
			name:        "should fail on non-numeric component",
			input:       "git version 2.abc.0",
			expectError: true,
			errContains: "failed to parse git version",
		},
		{
			name:        "should fail on empty input",
			input:       "",
			expectError: true,
			errContains: "failed to parse git version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version, err := parseGitVersion(tc.input)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errContains))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(version).To(Equal(tc.expected))
			}
		})
	}
}

func Test_isVersionAtLeast(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		version  [3]int
		minimum  [3]int
		expected bool
	}{
		{
			name:     "should return true when equal",
			version:  [3]int{2, 25, 0},
			minimum:  [3]int{2, 25, 0},
			expected: true,
		},
		{
			name:     "should return true when major is greater",
			version:  [3]int{3, 0, 0},
			minimum:  [3]int{2, 25, 0},
			expected: true,
		},
		{
			name:     "should return true when minor is greater",
			version:  [3]int{2, 43, 0},
			minimum:  [3]int{2, 25, 0},
			expected: true,
		},
		{
			name:     "should return true when patch is greater",
			version:  [3]int{2, 25, 1},
			minimum:  [3]int{2, 25, 0},
			expected: true,
		},
		{
			name:     "should return false when major is less",
			version:  [3]int{1, 30, 0},
			minimum:  [3]int{2, 25, 0},
			expected: false,
		},
		{
			name:     "should return false when minor is less",
			version:  [3]int{2, 24, 0},
			minimum:  [3]int{2, 25, 0},
			expected: false,
		},
		{
			name:     "should return false when patch is less",
			version:  [3]int{2, 25, 0},
			minimum:  [3]int{2, 25, 1},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isVersionAtLeast(tc.version, tc.minimum)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func Test_NewCli_versionCheck(t *testing.T) {
	g := NewWithT(t)

	t.Run("should fail when git version is below minimum", func(t *testing.T) {
		executor := &mockExecutor{
			executeFunc: func(command string, args ...string) (string, string, int, error) {
				return "git version 2.24.0\n", "", 0, nil
			},
		}

		cli, err := NewCli(executor)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("below minimum required"))
		g.Expect(cli).To(BeNil())
	})

	t.Run("should fail when git --version command fails", func(t *testing.T) {
		executor := &mockExecutor{
			executeFunc: func(command string, args ...string) (string, string, int, error) {
				return "", "error", 1, errors.New("command failed")
			},
		}

		cli, err := NewCli(executor)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get git version"))
		g.Expect(cli).To(BeNil())
	})

	t.Run("should succeed when git version meets minimum", func(t *testing.T) {
		executor := &mockExecutor{
			executeFunc: func(command string, args ...string) (string, string, int, error) {
				return "git version 2.43.0\n", "", 0, nil
			},
		}

		cli, err := NewCli(executor)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cli).ToNot(BeNil())
	})

	t.Run("should succeed when git version equals minimum", func(t *testing.T) {
		executor := &mockExecutor{
			executeFunc: func(command string, args ...string) (string, string, int, error) {
				return "git version 2.25.0\n", "", 0, nil
			},
		}

		cli, err := NewCli(executor)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cli).ToNot(BeNil())
	})
}

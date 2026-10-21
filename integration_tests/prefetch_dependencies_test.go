package integration_tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"

	. "github.com/onsi/gomega"
)

const (
	hermetoLatestImage = "quay.io/konflux-ci/hermeto:latest"
	testFixtureOrigin  = "file:///workspace"
)

type prefetchDependenciesTestParams struct {
	SourceDir           string
	Input               string
	OutputDir           string
	OutputDirMountPoint string
	EnvFiles            []string
}

func parseSBOM(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sbom map[string]any
	if err := json.Unmarshal(content, &sbom); err != nil {
		return nil, err
	}

	return sbom, nil
}

// TODO: This could be shared with the integration tests framework.
func setupContext(t *testing.T) string {
	t.Helper()
	tempDir, err := CreateTempDir("prefetch-test-context-")
	if err != nil {
		t.Fatalf("Failed to create test context: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	return tempDir
}

func setupTestFixtureRepo(t *testing.T, fixtureName, fixturePath string) string {
	t.Helper()

	repoPath := filepath.Join(setupContext(t), "repo")
	sourcePath := filepath.Join(FindRepoRoot(), "integration_tests", "sample-projects", fixturePath)
	if err := os.MkdirAll(filepath.Join(repoPath, fixtureName), 0o755); err != nil {
		t.Fatalf("Failed to create fixture repository: %v", err)
	}
	if err := os.CopyFS(filepath.Join(repoPath, fixtureName), os.DirFS(sourcePath)); err != nil {
		t.Fatalf("Failed to copy %s fixture: %v", fixtureName, err)
	}

	executor := cliwrappers.NewCliExecutor()
	runGitCmd := func(args ...string) {
		gitArgs := append([]string{"-C", repoPath}, args...)
		if _, _, _, err := executor.Execute(cliwrappers.Command("git", gitArgs...)); err != nil {
			t.Fatalf("Failed to run git %v: %v", args, err)
		}
	}

	runGitCmd("init")
	runGitCmd("config", "user.name", "Integration Test")
	runGitCmd("config", "user.email", "integration-test@example.com")
	runGitCmd("remote", "add", "origin", testFixtureOrigin)
	runGitCmd("add", fixtureName)
	runGitCmd("commit", "-m", "Add integration test fixture")

	return repoPath
}

func runPrefetchDependencies(params prefetchDependenciesTestParams) error {
	container := NewBuildCliRunnerContainer("kbc-prefetch-dependencies", hermetoLatestImage)
	defer container.DeleteIfExists()

	container.AddVolumeWithOptions(params.SourceDir, "/workspace", "z")
	container.SetWorkdir("/workspace")
	if err := container.Start(); err != nil {
		return err
	}

	// Workaround to prevent Hermeto fail in container with:
	// GitInvalidRevisionError: Failed to access HEAD commit: SHA is empty, possible dubious ownership in the repository
	if err := container.ExecuteCommand("git", "config", "--global", "--add", "safe.directory", "/workspace"); err != nil {
		return err
	}

	args := []string{
		"prefetch-dependencies",
		"--input", params.Input,
	}
	if params.OutputDir != "" {
		args = append(args, "--output-dir", params.OutputDir)
	}
	if params.OutputDirMountPoint != "" {
		args = append(args, "--output-dir-mount-point", params.OutputDirMountPoint)
	}
	if len(params.EnvFiles) > 0 {
		args = append(args, "--env-files")
		args = append(args, params.EnvFiles...)
	}
	return container.ExecuteBuildCli(args...)
}

func TestPrefetchDependencies(t *testing.T) {
	SetupGomega(t)

	t.Run("should skip prefetch dependencies if no input is provided", func(t *testing.T) {
		tempDir := setupContext(t)

		params := prefetchDependenciesTestParams{
			SourceDir: tempDir,
			Input:     "",
		}
		Expect(runPrefetchDependencies(params)).To(Succeed())
	})

	t.Run("should prefetch dependencies with RPM input", func(t *testing.T) {
		repoPath := setupTestFixtureRepo(t, "rpm", "prefetch-rpm")

		params := prefetchDependenciesTestParams{
			SourceDir: repoPath,
			Input:     `{"packages": [{"type": "rpm", "path": "rpm"}]}`,
		}
		Expect(runPrefetchDependencies(params)).To(Succeed())

		// Check output directory layout.
		depsDir := filepath.Join(repoPath, "prefetch-output", "deps", "rpm")
		info, err := os.Stat(depsDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		// Check generated environment file.
		envFile := filepath.Join(repoPath, "prefetch.env")
		info, err = os.Stat(envFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().IsRegular()).To(BeTrue())

		// Check generated SBOM file.
		sbomFile := filepath.Join(repoPath, "prefetch-output", "bom.json")
		info, err = os.Stat(sbomFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().IsRegular()).To(BeTrue())

		// Check SPDX SBOM content.
		sbom, err := parseSBOM(sbomFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(sbom).To(HaveKey("packages"))
		Expect(sbom["packages"]).ToNot(BeEmpty())

		// Check repo file.
		var hermetoRepoCount int
		filepath.WalkDir(filepath.Join(repoPath, "prefetch-output"), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && entry.Name() == "hermeto.repo" {
				hermetoRepoCount++
			}
			return nil
		})
		Expect(hermetoRepoCount).To(BeNumerically(">", 0))
	})

	t.Run("should generate environment files with specified formats", func(t *testing.T) {
		repoPath := setupTestFixtureRepo(t, "gomod", "prefetch-gomod")

		params := prefetchDependenciesTestParams{
			SourceDir:           repoPath,
			Input:               `{"packages": [{"type": "gomod", "path": "gomod"}]}`,
			OutputDirMountPoint: "/tmp",
			EnvFiles:            []string{"hermeto.env", "prefetch.env", "prefetch-env.json"},
		}
		Expect(runPrefetchDependencies(params)).To(Succeed())

		// Check prefetch.env
		envContent, err := os.ReadFile(filepath.Join(repoPath, "prefetch.env"))
		Expect(err).ToNot(HaveOccurred())
		envLines := strings.Split(string(envContent), "\n")
		Expect(envLines).To(ContainElement("export GOMODCACHE=/tmp/deps/gomod/pkg/mod"))

		// Check that hermeto.env is the same as prefetch.env
		envContent2, err := os.ReadFile(filepath.Join(repoPath, "hermeto.env"))
		Expect(err).ToNot(HaveOccurred())
		Expect(envContent2).To(Equal(envContent))

		// Check prefetch-env.json
		jsonContent, err := os.ReadFile(filepath.Join(repoPath, "prefetch-env.json"))
		Expect(err).ToNot(HaveOccurred())

		type envVar struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		var jsonEnvData []envVar
		Expect(json.Unmarshal(jsonContent, &jsonEnvData)).To(Succeed())
		Expect(jsonEnvData).To(ContainElement(envVar{"GOMODCACHE", "/tmp/deps/gomod/pkg/mod"}))

		dependencyArchive := filepath.Join(repoPath, "prefetch-output", "deps", "gomod", "pkg", "mod", "cache", "download", "github.com", "pkg", "errors", "@v", "v0.9.1.zip")
		info, err := os.Stat(dependencyArchive)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().IsRegular()).To(BeTrue())
	})
}

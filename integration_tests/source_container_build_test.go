package integration_tests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
)

const (
	sourceDirInContainer    = "/tmp/scb/source"
	workspaceDirInContainer = "/tmp/scb/workspace"
	resultsDirInContainer   = "/tmp/scb/results"
)

// sourceContainerBuildResult mirrors the JSON the command reports.
type sourceContainerBuildResult struct {
	Status                  string `json:"status"`
	Message                 string `json:"message"`
	DependenciesIncluded    bool   `json:"dependencies_included"`
	BaseImageSourceIncluded bool   `json:"base_image_source_included"`
	ImageURL                string `json:"image_url"`
	ImageDigest             string `json:"image_digest"`
}

func setupSourceContainerBuildContainer(t *testing.T, imageRegistry ImageRegistry) *TestRunnerContainer {
	t.Helper()

	container := NewBuildCliRunnerContainer(
		"kbc-source-container-build",
		TaskRunnerImageRef,
		// The task-runner image has no gobsi, the released Build CLI image does.
		WithGobsi(),
		// The source build task runs as root, and git has to write to the
		// source repository to stash.
		WithUser("0"),
	)
	t.Cleanup(func() { container.DeleteIfExists() })

	Expect(container.StartWithRegistryIntegration(imageRegistry)).To(Succeed())

	return container
}

// createSourceRepo makes a git repository in the container for the build to
// take its sources from.
func createSourceRepo(t *testing.T, container *TestRunnerContainer, remoteUrl string, files map[string]string) {
	t.Helper()
	g := NewWithT(t)

	g.Expect(container.ExecuteCommand("mkdir", "-p", sourceDirInContainer)).To(Succeed())

	for name, content := range files {
		path := filepath.Join(sourceDirInContainer, name)
		script := fmt.Sprintf("mkdir -p %s && printf '%s' >%s", filepath.Dir(path), content, path)
		g.Expect(container.ExecuteCommand("bash", "-c", script)).To(Succeed())
	}

	script := strings.Join([]string{
		"cd " + sourceDirInContainer,
		"git init -q .",
		"git config user.email test@example.com",
		"git config user.name Test",
		"git remote add origin " + remoteUrl,
		"git add -A",
		"git commit -qm 'initial commit'",
	}, " && ")
	g.Expect(container.ExecuteCommand("bash", "-c", script)).To(Succeed())
}

// runSourceContainerBuild runs the command and returns the reported result
// together with the error, which is expected when the build fails.
func runSourceContainerBuild(
	t *testing.T, container *TestRunnerContainer, extraArgs ...string,
) (sourceContainerBuildResult, error) {
	t.Helper()
	g := NewWithT(t)

	g.Expect(container.ExecuteCommand("mkdir", "-p", resultsDirInContainer)).To(Succeed())

	args := append([]string{
		"image", "source-container-build",
		"--source-dir", sourceDirInContainer,
		"--workspace-dir", workspaceDirInContainer,
		"--result-path-build-result", filepath.Join(resultsDirInContainer, "build_result"),
		"--result-path-source-image-url", filepath.Join(resultsDirInContainer, "url"),
		"--result-path-source-image-digest", filepath.Join(resultsDirInContainer, "digest"),
		"--result-path-image-ref", filepath.Join(resultsDirInContainer, "image_ref"),
	}, extraArgs...)

	runErr := container.ExecuteBuildCli(args...)

	raw, _, err := container.ExecuteCommandWithOutput(
		"cat", filepath.Join(resultsDirInContainer, "build_result"))
	g.Expect(err).ToNot(HaveOccurred(), "the build result is written whatever the outcome")

	result := sourceContainerBuildResult{}
	g.Expect(json.Unmarshal([]byte(raw), &result)).To(Succeed())

	return result, runErr
}

// readResult returns the content of one of the written result files.
func readResult(t *testing.T, container *TestRunnerContainer, name string) string {
	t.Helper()

	stdout, _, err := container.ExecuteCommandWithOutput("cat", filepath.Join(resultsDirInContainer, name))
	NewWithT(t).Expect(err).ToNot(HaveOccurred())

	return strings.TrimSpace(stdout)
}

// pushBinaryImage creates and pushes an image to build the sources for.
func pushBinaryImage(t *testing.T, imageRepoUrl string) string {
	t.Helper()
	g := NewWithT(t)

	g.Expect(CreateTestImage(TestImageConfig{
		ImageRef:       imageRepoUrl,
		RandomDataSize: 1024,
	})).To(Succeed())
	t.Cleanup(func() { DeleteLocalImage(imageRepoUrl) }) //nolint:errcheck // best effort cleanup

	digest, err := PushImage(imageRepoUrl)
	g.Expect(err).ToNot(HaveOccurred())

	return digest
}

func TestSourceContainerBuild(t *testing.T) {
	SetupGomega(t)
	g := NewWithT(t)

	imageRegistry := SetupImageRegistry(t)
	imageRepoUrl := imageRegistry.GetTestNamespace() + "source-build-test"
	binaryImageDigest := pushBinaryImage(t, imageRepoUrl)

	container := setupSourceContainerBuildContainer(t, imageRegistry)
	createSourceRepo(t, container, "https://github.com/namespace/myapp.git", map[string]string{
		"main.go":        "package main\n",
		"pkg/helper.go":  "package pkg\n",
		"docs/README.md": "docs\n",
	})

	result, err := runSourceContainerBuild(t, container,
		"--binary-image-url", imageRepoUrl,
		"--binary-image-digest", binaryImageDigest,
	)
	g.Expect(err).ToNot(HaveOccurred())

	// The source image is published next to the binary image, named after its
	// digest.
	wantSourceImage := imageRepoUrl + ":" + strings.ReplaceAll(binaryImageDigest, ":", "-") + ".src"
	g.Expect(result.Status).To(Equal("success"))
	g.Expect(result.ImageURL).To(Equal(wantSourceImage))
	g.Expect(result.ImageDigest).To(HavePrefix("sha256:"))
	g.Expect(result.BaseImageSourceIncluded).To(BeFalse())
	g.Expect(result.DependenciesIncluded).To(BeFalse())

	// The results are reported for the task to pick up.
	g.Expect(readResult(t, container, "url")).To(Equal(result.ImageURL))
	g.Expect(readResult(t, container, "digest")).To(Equal(result.ImageDigest))
	g.Expect(readResult(t, container, "image_ref")).To(Equal(result.ImageURL + "@" + result.ImageDigest))

	// The source image really is in the registry, under the expected tag.
	sourceImageTag := strings.ReplaceAll(binaryImageDigest, ":", "-") + ".src"
	exists, err := CheckManifestExistence(imageRegistry, imageRepoUrl, sourceImageTag)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(exists).To(BeTrue(), "expected the source image to be pushed")

	// It carries the source archive of the repository at HEAD.
	sourceSha, _, err := container.ExecuteCommandWithOutput(
		"git", "-C", sourceDirInContainer, "rev-parse", "HEAD")
	g.Expect(err).ToNot(HaveOccurred())
	archive := fmt.Sprintf("myapp-%s.tar.gz", strings.TrimSpace(sourceSha))
	g.Expect(container.ExecuteCommand("test", "-f",
		filepath.Join(workspaceDirInContainer, "source-build", "source_archive", archive))).
		To(Succeed(), "expected the source archive to be named after the repository and commit")
}

func TestSourceContainerBuildWithPrefetchedDependencies(t *testing.T) {
	SetupGomega(t)
	g := NewWithT(t)

	imageRegistry := SetupImageRegistry(t)
	imageRepoUrl := imageRegistry.GetTestNamespace() + "source-build-prefetch-test"
	binaryImageDigest := pushBinaryImage(t, imageRepoUrl)

	container := setupSourceContainerBuildContainer(t, imageRegistry)
	createSourceRepo(t, container, "https://github.com/namespace/myapp.git",
		map[string]string{"main.go": "package main\n"})

	// A prefetch output holding one archived dependency.
	prefetchDir := "/tmp/scb/prefetch"
	depsDir := filepath.Join(prefetchDir, "output", "deps", "pip")
	script := strings.Join([]string{
		"mkdir -p " + depsDir,
		fmt.Sprintf("printf 'source' | gzip >%s/requests-2.0.tar.gz", depsDir),
		fmt.Sprintf("printf 'export PIP_INDEX_URL=off\\n' >%s/prefetch.env", prefetchDir),
	}, " && ")
	g.Expect(container.ExecuteCommand("bash", "-c", script)).To(Succeed())

	result, err := runSourceContainerBuild(t, container,
		"--binary-image-url", imageRepoUrl,
		"--binary-image-digest", binaryImageDigest,
		"--prefetch-artifacts-dir", prefetchDir,
	)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status).To(Equal("success"))
	g.Expect(result.DependenciesIncluded).To(BeTrue())

	// The archived dependency and the environment file are both collected.
	g.Expect(container.ExecuteCommand("test", "-f", filepath.Join(workspaceDirInContainer,
		"source-build", "prefetched_sources", "src-0", "deps", "pip", "requests-2.0.tar.gz"))).To(Succeed())
	g.Expect(container.ExecuteCommand("test", "-f", filepath.Join(workspaceDirInContainer,
		"source-build", "prefetch_env", "prefetch.env"))).To(Succeed())
}

func TestSourceContainerBuildDropsWhenBinaryImageIsGone(t *testing.T) {
	SetupGomega(t)
	g := NewWithT(t)

	imageRegistry := SetupImageRegistry(t)
	imageRepoUrl := imageRegistry.GetTestNamespace() + "source-build-drop-test"

	container := setupSourceContainerBuildContainer(t, imageRegistry)
	createSourceRepo(t, container, "https://github.com/namespace/myapp.git",
		map[string]string{"main.go": "package main\n"})

	// Nothing was ever pushed to this repository, so there is nothing left for
	// a source image to describe.
	digest := "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	result, err := runSourceContainerBuild(t, container,
		"--binary-image-url", imageRepoUrl,
		"--binary-image-digest", digest,
	)

	// A dropped build is not a failure.
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status).To(Equal("drop"))
	g.Expect(result.Message).To(ContainSubstring("no longer exists"))
	g.Expect(result.ImageURL).To(BeEmpty())
	g.Expect(result.ImageDigest).To(BeEmpty())
	g.Expect(readResult(t, container, "image_ref")).To(BeEmpty())
}

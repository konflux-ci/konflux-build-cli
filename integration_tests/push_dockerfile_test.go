package integration_tests

import (
	"fmt"
	"testing"
	"path/filepath"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	. "github.com/onsi/gomega"
)

const RunnerImage = "quay.io/konflux-ci/task-runner:1.1.1"

type PushDockerfileParams struct {
	ImageRepoUrl string
	ImageDigest  string
}

func setup(t *testing.T) (*TestRunnerContainer, ImageRegistry, error) {
	SetupGomega(t)

	imageRegistry, err := StartImageRegistry()
	if err != nil {
		return nil, nil, err
	}

	volumeOpts := []ContainerVolumeOption{}
	container, err := SetupBuildContainerWithCleanup(t, "kbc-push-dockerfile", RunnerImage, volumeOpts, imageRegistry)
	if err != nil {
		return nil, nil, err
	}

	return container, imageRegistry, nil
}

func TestPushDockerfile(t *testing.T) {
	// SetupGomega(t)

	// imageRegistry, err := StartImageRegistry()
	// Expect(err).ToNot(HaveOccurred())

	// volumeOpts := []ContainerVolumeOption{}
	// container, err := SetupBuildContainerWithCleanup(t, "kbc-push-dockerfile", RunnerImage, volumeOpts, imageRegistry)
	// Expect(err).ToNot(HaveOccurred())

	container, imageRegistry, err := setup(t)
	Expect(err).ToNot(HaveOccurred())

	homeDir, err := container.GetHomeDir()
	Expect(err).ToNot(HaveOccurred())

	err = container.ExecuteCommand("mkdir", filepath.Join(homeDir, "source"))
	Expect(err).ToNot(HaveOccurred())
	dockerfile := filepath.Join(homeDir, "source", "Dockerfile")
	err = container.ExecuteCommand("bash", "-c", fmt.Sprintf(`echo "FROM fedora" >%s`, dockerfile))
	Expect(err).ToNot(HaveOccurred())

	cmd := []string{
		"image", "push-dockerfile",
		"--image-url", imageRegistry.GetRegistryDomain() + "/app",
		"--digest", "sha256:cfc8226f8268c70848148f19c35b02788b272a5a7c0071906a9c6b654760e44a",
		"--source", "source",
	}
	err = container.ExecuteBuildCli(cmd...)
	Expect(err).ToNot(HaveOccurred())

	stdout, _, err := container.ExecuteCommand2("skopeo", "list-tags", "docker://localhost:5000/app")
	Expect(err).ToNot(HaveOccurred())

	Expect(stdout).Should(ContainSubstring("sha256-cfc8226f8268c70848148f19c35b02788b272a5a7c0071906a9c6b654760e44a.dockerfile"))
}

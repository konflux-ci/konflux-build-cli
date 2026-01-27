package integration_tests

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	. "github.com/onsi/gomega"
)

const RunnerImage = "quay.io/konflux-ci/task-runner:1.1.1"

type PushDockerfileParams struct {
	ImageRepoUrl string
	ImageDigest  string
}

var containerStoragePath string

func init() {
	fmt.Println("init gets called ...")

	containerStorageBase := filepath.Join(os.TempDir(), "kbc-image-build-tests")

	// Try to clean up the parent storage dir
	// 1. 'chmod -R' to ensure write permissions (container storage often includes read-only files)
	filepath.WalkDir(containerStorageBase, func(path string, d fs.DirEntry, err error) error {
		// Ignore errors, try to chmod everything if possible
		os.Chmod(path, 0777)
		return nil
	})
	// 2. 'rm -r'
	os.RemoveAll(containerStorageBase)

	// (Re-)create the parent storage dir
	err := os.Mkdir(containerStorageBase, 0755)
	if err != nil {
		panic(err)
	}
	// Create a subdirectory for this test run
	containerStoragePath, err = os.MkdirTemp(containerStorageBase, "container-storage-*")
	if err != nil {
		panic(err)
	}
	fmt.Println("Using container storage path:", containerStoragePath)
}

// Registers the Gomega failure handler for the test.
func setupGomega(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		fmt.Printf("Test Failure: %s\n", message)
		t.FailNow()
	})
}

func setupBuildContainer(runnerImage string, volumnOptions []ContainerVolumeOption, imageRegistry ImageRegistry) (*TestRunnerContainer, error) {
	container := NewBuildCliRunnerContainer("kbc-build", runnerImage, "/home/taskuser/.docker")
	for _, opt := range volumnOptions {
		container.AddVolumeWithOptions2(opt)
	}

	if imageRegistry != nil && imageRegistry.IsLocal() {
		container.AddVolumeWithOptions(imageRegistry.GetCaCertPath(), "/etc/pki/tls/certs/ca-custom-bundle.crt", "z")
	}

	err := container.Start()
	if err != nil {
		return nil, err
	}

	if imageRegistry != nil {
		login, password := imageRegistry.GetCredentials()
		err = container.InjectDockerAuth(imageRegistry.GetRegistryDomain(), login, password)
		if err != nil {
			return container, err
		}
	}

	return container, nil
}

func setupBuildContainerWithCleanup(t *testing.T, runnerImage string, volumeOptions []ContainerVolumeOption, imageRegistry ImageRegistry) *TestRunnerContainer {
	container, err := setupBuildContainer(runnerImage, volumeOptions, imageRegistry)
	t.Cleanup(func() {
		if container != nil {
			container.Delete()
		}
		if err := imageRegistry.Stop(); err != nil {
			fmt.Printf("Error on stopping image registry during test cleanup: %v\n", err)
		}
	})
	Expect(err).ToNot(HaveOccurred())
	return container
}

func startImageRegistry() (ImageRegistry, error) {
	imageRegistry := NewImageRegistry()
	if err := imageRegistry.Prepare(); err != nil {
		return nil, fmt.Errorf("Error on preparing image registry: %w", err)
	}
	if err := imageRegistry.Start(); err != nil {
		return nil, fmt.Errorf("Error on starting image registry: %w", err)
	}
	return imageRegistry, nil
}

func TestPushDockerfile(t *testing.T) {
	setupGomega(t)

	imageRegistry, err := startImageRegistry()
	Expect(err).ToNot(HaveOccurred())

	volumeOpts := []ContainerVolumeOption{
		{
			HostPath:      containerStoragePath,
			ContainerPath: "/var/lib/containers",
			MountOptions:  "z",
		},
	}
	container := setupBuildContainerWithCleanup(t, RunnerImage, volumeOpts, imageRegistry)
	err = container.ExecuteBuildCli("image", "push-dockerfile", "--help")
	Expect(err).ToNot(HaveOccurred())
}

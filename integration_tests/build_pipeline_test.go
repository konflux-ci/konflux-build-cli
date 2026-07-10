package integration_tests

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	"github.com/konflux-ci/konflux-build-cli/pkg/commands"
	gitcmd "github.com/konflux-ci/konflux-build-cli/pkg/commands/gitclone"
)

// TestBuildPipeline run whole build pipeline end to end.
// It uses PVC approach implemented via a shared folder on host.
// Included tasks:
// - init
// - clone-repository
// - prefetch-dependencies
// - build-container
// - build-image-index
// - build-source-image
// - apply-tags
// - push-dockerfile
func TestBuildPipeline(t *testing.T) {
	var err error
	SetupGomega(t)

	imageRegistry := SetupImageRegistry(t)

	// Shared volume between containers
	// workspaceDirHost, err := CreateTempDir("build-pipeline-")
	// Expect(err).ToNot(HaveOccurred())
	// t.Cleanup(func() { os.RemoveAll(workspaceDirHost) })
	workspaceDirHost := "/tmp/build-pipeline"

	const sourceDir = "source"
	sourceDirHost := path.Join(workspaceDirHost, sourceDir)

	Expect(os.MkdirAll(sourceDirHost, 0755)).To(Succeed())
	// Chmod to 0777 to allow the container user to write to the directory.
	// Use a separate Chmod rather than passing 0777 to MkdirAll,
	// because MkdirAll respects umask so the result may not actually be 0777.
	Expect(os.Chmod(sourceDirHost, 0777)).To(Succeed())
	/*
			initResults := struct {
				httpProxy string
				noProxy   string
			}{}
			t.Run("init", func(t *testing.T) {
				const konfluxConfigFilePath = "/etc/konflux-config"
				const httpProxyResultPath = "/tmp/http-proxy-result"
				const noProxyResultPath = "/tmp/no-proxy-result"

				container := NewBuildCliRunnerContainer("init", ApplyTagsImage)
				container.AddEnv("PLATFORM_CONFIG_FILE", konfluxConfigFilePath)

				err = container.Start()
				Expect(err).ToNot(HaveOccurred())
				t.Cleanup(func() { container.DeleteIfExists() })

				const konfluxConfigFile = `
		            [cache-proxy]
					allow-cache-proxy = false
					http-proxy = some.proxy.net
					no-proxy = localhost:1234
				`
				container.CreateFileInContainer(konfluxConfigFilePath, konfluxConfigFile)

				cacheProxyArgs := []string{"config", "cache-proxy", "--enable", "false"}
				cacheProxyArgs = append(cacheProxyArgs, "--http-proxy-result-path", httpProxyResultPath)
				cacheProxyArgs = append(cacheProxyArgs, "--no-proxy-result-path", noProxyResultPath)

				err = container.ExecuteBuildCli(cacheProxyArgs...)
				Expect(err).ToNot(HaveOccurred())

				initResults.httpProxy, err = container.GetFileContent(httpProxyResultPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(initResults.httpProxy).To(BeEmpty())
				initResults.noProxy, err = container.GetFileContent(noProxyResultPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(initResults.noProxy).To(BeEmpty())
			})
	*/

	const gitUrl = "https://github.com/konflux-ci/konflux-build-cli"
	outputImageUrl := imageRegistry.GetTestNamespace() + "image"
	outputImageTag := "result"
	newTag := time.Now().Format("2006-01-02_15-04-05")
	newTagFromLabel := "label-" + newTag

	cloneResults := gitcmd.Results{}
	t.Run("clone-repository", func(t *testing.T) {
		container := startGitCloneContainer(t, workspaceDirHost)

		args := []string{"git-clone", "--url", gitUrl, "--output-dir", path.Join("/workspace", sourceDir)}
		stdout, _, err := container.ExecuteBuildCliWithOutput(args...)
		Expect(err).ToNot(HaveOccurred(), "git clone failed")

		cloneResults, err = parseGitCloneResult(stdout)
		Expect(err).ToNot(HaveOccurred())
		Expect(cloneResults.Commit).ToNot(BeEmpty())
	})

	const prefetchDir = "hermeto"
	const prefetchOutputMountPoint = "/hermeto/output"
	prefetchOutputDir := path.Join(prefetchDir, "output")
	prefetchEnvFile := path.Join(prefetchDir, "prefetch.env")
	t.Run("prefetch-dependencies", func(t *testing.T) {
		err := runPrefetchDependencies(prefetchDependenciesTestParams{
			Input:               `{"packages": [{"type": "gomod"}]}`,
			Context:             sourceDirHost,
			OutputDir:           prefetchOutputDir,
			OutputDirMountPoint: prefetchOutputMountPoint,
			EnvFiles:            []string{prefetchEnvFile},
		})
		Expect(err).ToNot(HaveOccurred(), "prefetch failed")
	})

	buildResults := &commands.BuildResults{}
	t.Run("build-container", func(t *testing.T) {
		outputImageRef := imageRegistry.GetTestNamespace() + "image-" + runtime.GOARCH
		buildParams := BuildParams{
			Context:               sourceDirHost,
			OutputRef:             outputImageRef,
			Hermetic:              true,
			PrefetchDir:           path.Join("/workspace", prefetchDir),
			PrefetchOutputMount:   prefetchOutputMountPoint,
			Push:                  true,
			QuayImageExpiresAfter: "1h",
			Labels:                []string{newTagFromLabel},
		}

		buildResults, err = RunBuild(buildParams, imageRegistry)

		Expect(err).ToNot(HaveOccurred(), "build failed")
		Expect(buildResults.ImageUrl).To(Equal(outputImageRef))
		Expect(buildResults.Digest).To(MatchRegexp("^sha256:[a-f0-9]{64}$"))

		exists, err := imageRegistry.CheckManifestExistence(buildResults.ImageUrl, buildResults.Digest)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	buildImageIndexResults := &BuildImageIndexResults{}
	t.Run("build-image-index", func(t *testing.T) {
		params := BuildImageIndexParams{
			Image: outputImageUrl + ":" + outputImageTag,
			Images: []string{
				buildResults.ImageUrl + "@" + buildResults.Digest,
			},
			BuildahFormat:    "oci",
			AdditionalTags:   []string{"latest"},
			AlwaysBuildIndex: boolptr(true),
		}
		indexOutput, _, err := RunBuildImageIndex(params, imageRegistry, true)
		buildImageIndexResults = indexOutput.Results
		Expect(err).ToNot(HaveOccurred(), "build image index failed")
		Expect(buildImageIndexResults.ImageURL).To(Equal(outputImageUrl + ":" + outputImageTag))
		Expect(buildImageIndexResults.ImageRef).To(Equal(outputImageUrl + "@" + buildImageIndexResults.ImageDigest))
		Expect(buildImageIndexResults.ImageDigest).To(MatchRegexp("^sha256:[a-f0-9]{64}$"))
		Expect(buildImageIndexResults.Images).To(HaveLen(1))
		Expect(buildImageIndexResults.Images[0]).To(Equal(buildResults.ImageUrl + "@" + buildResults.Digest))

		imageIndexTags := []string{outputImageTag, "latest"}
		for _, tag := range imageIndexTags {
			exists, err := imageRegistry.CheckTagExistence(outputImageUrl, tag)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		}

		imageIndexInfo, err := imageRegistry.GetImageIndexInfo(outputImageUrl, outputImageTag)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to get image index %s:%s", outputImageUrl, outputImageTag))
		Expect(imageIndexInfo.MediaType).To(BeElementOf([]string{
			"application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json"}))
		Expect(imageIndexInfo.Manifests).To(HaveLen(1))

		manifestInfo := imageIndexInfo.Manifests[0]
		Expect(manifestInfo.MediaType).To(BeElementOf([]string{
			"application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json"}))
		Expect(manifestInfo.Digest).To(Equal(buildImageIndexResults.ImageDigest))
	})

	t.Run("build-source-image", func(t *testing.T) {
		// TODO implement when build-source-image is ported to the CLI
	})

	t.Run("apply-tags", func(t *testing.T) {
		applyTagsParams := ApplyTagsParams{
			ImageRepoUrl: outputImageUrl,
			ImageDigest:  buildImageIndexResults.ImageDigest,
			Tags:         []string{newTag},
		}

		err := RunApplyTags(applyTagsParams, imageRegistry)
		Expect(err).ToNot(HaveOccurred(), "apply tags failed")

		for _, tag := range []string{newTag, newTagFromLabel} {
			tagExists, err := imageRegistry.CheckTagExistence(outputImageUrl, tag)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to check for %s tag existence", tag))
			Expect(tagExists).To(BeTrue(), fmt.Sprintf("expected %s:%s to exist", outputImageUrl, tag))

			imageIndexInfo, err := imageRegistry.GetImageIndexInfo(outputImageUrl, tag)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to get image index %s:%s", outputImageUrl, tag))
			Expect(imageIndexInfo.MediaType).To(BeElementOf([]string{
				"application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json"}))
		}
	})

	pushContainerfileResults := &commands.PushContainerfileResults{}
	t.Run("push-dockerfile", func(t *testing.T) {
		expectedContainerfileTag := buildImageIndexResults.ImageDigest + ".containerfile"
		pushContainerfileParams := PushContainerfileParams{
			imageUrl: outputImageUrl,
			digest:   buildImageIndexResults.ImageDigest,
			source:   sourceDir,
		}
		pushContainerfileResults, err = RunPushContainerfile(pushContainerfileParams, imageRegistry)
		Expect(err).ToNot(HaveOccurred(), "push containerfile failed")
		Expect(pushContainerfileResults.ImageRef).To(Equal(outputImageUrl + ":" + expectedContainerfileTag))

		tagExists, err := imageRegistry.CheckTagExistence(outputImageUrl, expectedContainerfileTag)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to check for %s tag existence", expectedContainerfileTag))
		Expect(tagExists).To(BeTrue(), fmt.Sprintf("expected %s:%s to exist", outputImageUrl, expectedContainerfileTag))
	})
}

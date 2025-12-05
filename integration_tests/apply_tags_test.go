package integration_tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

const ApplyTagsImage = "registry.access.redhat.com/ubi9/skopeo:9.6-1754871306@sha256:e59e2cb3fd8d7613798738fb06aad5aab61f32c18aed595df16a46a8e078dfa6"

const KonfluxAdditionalTagsLabelName = "konflux.additional-tags"

type ApplyTagsParams struct {
	ImageRepoUrl string
	ImageDigest  string
	Tags         []string
}

func RunApplyTags(applyTagsParams ApplyTagsParams, imageRegistry ImageRegistry) error {
	var err error

	container := NewTestRunnerContainer("apply-tags", ApplyTagsImage)

	// Params
	container.AddEnv("KBC_APPLY_TAGS_IMAGE_URL", applyTagsParams.ImageRepoUrl)
	container.AddEnv("KBC_APPLY_TAGS_IMAGE_DIGEST", applyTagsParams.ImageDigest)
	container.AddEnv("KBC_APPLY_TAGS_FROM_IMAGE_LABEL", KonfluxAdditionalTagsLabelName)

	if imageRegistry.IsLocal() {
		container.AddNetwork(zotRegistryNetworkName)
	}
	if Debug {
		container.AddPort("2345", "2345")
	}
	err = container.Start()
	Expect(err).ToNot(HaveOccurred())
	defer container.Delete()

	err = container.CopyFileIntoContainer("../"+KonfluxBuildCli, "/usr/bin/")
	Expect(err).ToNot(HaveOccurred())

	login, password := imageRegistry.GetCredentials()
	err = container.InjectDockerAuth(imageRegistry.GetRegistryDomain(), login, password)
	Expect(err).ToNot(HaveOccurred())

	args := []string{"image", "apply-tags"}
	if len(applyTagsParams.Tags) > 0 {
		args = append(args, "--tags")
		args = append(args, applyTagsParams.Tags...)
	}

	if Debug {
		err = container.DebugCli(args...)
	} else {
		err = container.ExecuteAndWait(KonfluxBuildCli, args...)
	}
	Expect(err).ToNot(HaveOccurred())

	return nil
}

func TestApplyTags(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		fmt.Printf("Test Failure: %s\n", message)
		t.FailNow() // Terminate the test immediately
	})
	Expect(IsKonfluxCliCompiled()).To(BeTrue(), "CLI is not compiled. Compile it before running the test.")

	// Setup registry
	var err error
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Test input data
	imageRepoUrl := imageRegistry.GetTestNamespace() + "test-image"
	newTagsFromLabel := []string{"label-tag-1", "label-tag-2"}
	newTag := time.Now().Format("2006-01-02_15-04-05")
	newTagsFromArg := []string{newTag, "test"}

	// Create base image for the test
	err = CreateTestImage(TestImageConfig{
		ImageRef: imageRepoUrl,
		Labels: map[string]string{
			KonfluxAdditionalTagsLabelName: strings.Join(newTagsFromLabel, " "),
			QuayExpiresAfterLabelName:      "1h",
		},
		Size: 10 * 1024,
	})
	Expect(err).ToNot(HaveOccurred())
	imageDigest, err := PushImage(imageRepoUrl)
	Expect(err).ToNot(HaveOccurred())

	// Run the command
	applyTagsParams := ApplyTagsParams{
		ImageRepoUrl: imageRepoUrl,
		ImageDigest:  imageDigest,
		Tags:         newTagsFromArg,
	}
	err = RunApplyTags(applyTagsParams, imageRegistry)
	Expect(err).ToNot(HaveOccurred())

	// Check the result
	for _, tag := range append(newTagsFromArg, newTagsFromLabel...) {
		tagExists, err := imageRegistry.CheckTagExistance(imageRepoUrl, tag)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to check for %s tag existance", tag))
		Expect(tagExists).To(BeTrue(), fmt.Sprintf("Expected %s:%s to exist", imageRepoUrl, tag))
	}
}

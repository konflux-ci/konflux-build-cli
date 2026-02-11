package integration_tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	cliWrappers "github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

const BuildImageIndexImage = "quay.io/konflux-ci/task-runner:1.4.1"

type BuildImageIndexParams struct {
	Image            string
	Images           []string
	TLSVerify        bool
	BuildahFormat    string
	AlwaysBuildIndex bool
	AdditionalTags   []string
}

type BuildImageIndexResults struct {
	ImageDigest string `json:"image_digest"`
	ImageURL    string `json:"image_url"`
	ImageRef    string `json:"image_ref"`
	Images      string `json:"images"`
}

func RunBuildImageIndex(params BuildImageIndexParams, imageRegistry ImageRegistry) (*BuildImageIndexResults, error) {
	var err error

	container := NewBuildCliRunnerContainer("build-image-index", BuildImageIndexImage)
	defer container.DeleteIfExists()

	err = container.StartWithRegistryIntegration(imageRegistry)
	if err != nil {
		return nil, err
	}

	// Construct the build-image-index arguments
	args := []string{"image", "build-image-index"}
	args = append(args, "--image", params.Image)
	args = append(args, fmt.Sprintf("--tls-verify=%t", params.TLSVerify))
	args = append(args, "--buildah-format", params.BuildahFormat)
	args = append(args, fmt.Sprintf("--always-build-index=%t", params.AlwaysBuildIndex))

	for _, image := range params.Images {
		args = append(args, "--images", image)
	}

	for _, tag := range params.AdditionalTags {
		args = append(args, "--additional-tags", tag)
	}

	// Run the CLI and redirect JSON output to a file
	// This avoids issues with podman exec mixing stdout/stderr
	shellCmd := fmt.Sprintf("%s", KonfluxBuildCli)
	for _, arg := range args {
		escaped := strings.ReplaceAll(arg, "'", "'\"'\"'")
		shellCmd += fmt.Sprintf(" '%s'", escaped)
	}
	shellCmd += " > /tmp/cli-output.json"

	_, _, err = container.ExecuteCommandWithOutput("sh", "-c", shellCmd)
	if err != nil {
		return nil, err
	}

	jsonOutput, _, err := container.ExecuteCommandWithOutput("cat", "/tmp/cli-output.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read CLI output: %w", err)
	}

	var results BuildImageIndexResults
	if err := json.Unmarshal([]byte(jsonOutput), &results); err != nil {
		return nil, fmt.Errorf("failed to parse results JSON: %w", err)
	}

	return &results, nil
}

func TestBuildImageIndex_MultipleImages(t *testing.T) {
	SetupGomega(t)
	var err error

	// Setup registry
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Create input data
	baseImageRepo := imageRegistry.GetTestNamespace() + "test-image-index"
	tag := GenerateUniqueTag(t)
	indexImage := baseImageRepo + ":" + tag

	// Create and push two platform images (simulating amd64 and arm64)
	image1Ref := baseImageRepo + "-platform1:" + tag
	image2Ref := baseImageRepo + "-platform2:" + tag

	err = CreateTestImage(TestImageConfig{
		ImageRef:       image1Ref,
		RandomDataSize: 1024,
		Labels: map[string]string{
			"platform": "amd64",
		},
	})
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(image1Ref)

	err = CreateTestImage(TestImageConfig{
		ImageRef:       image2Ref,
		RandomDataSize: 2048,
		Labels: map[string]string{
			"platform": "arm64",
		},
	})
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(image2Ref)

	digest1, err := PushImage(image1Ref)
	Expect(err).ToNot(HaveOccurred())

	digest2, err := PushImage(image2Ref)
	Expect(err).ToNot(HaveOccurred())

	// Build the image references with digests
	imageRepo1 := common.GetImageName(image1Ref)
	imageRepo2 := common.GetImageName(image2Ref)
	image1WithDigest := imageRepo1 + "@" + digest1
	image2WithDigest := imageRepo2 + "@" + digest2

	// Run the command
	params := BuildImageIndexParams{
		Image:            indexImage,
		Images:           []string{image1WithDigest, image2WithDigest},
		TLSVerify:        !imageRegistry.IsLocal(),
		BuildahFormat:    "oci",
		AlwaysBuildIndex: true,
		AdditionalTags:   []string{"test-tag-1"},
	}

	results, err := RunBuildImageIndex(params, imageRegistry)
	Expect(err).ToNot(HaveOccurred())

	// Verify results
	Expect(results.ImageURL).To(Equal(indexImage))
	Expect(results.ImageDigest).ToNot(BeEmpty())
	Expect(results.ImageDigest).To(HavePrefix("sha256:"))
	Expect(results.ImageRef).To(ContainSubstring(results.ImageDigest))
	Expect(results.Images).To(ContainSubstring(digest1))
	Expect(results.Images).To(ContainSubstring(digest2))

	// Verify the index was pushed to registry
	tagExists, err := imageRegistry.CheckTagExistance(baseImageRepo, tag)
	Expect(err).ToNot(HaveOccurred())
	Expect(tagExists).To(BeTrue(), fmt.Sprintf("Expected %s to exist", indexImage))

	// Verify additional tag was created
	tagExists, err = imageRegistry.CheckTagExistance(baseImageRepo, "test-tag-1")
	Expect(err).ToNot(HaveOccurred())
	Expect(tagExists).To(BeTrue(), fmt.Sprintf("Expected %s:test-tag-1 to exist", baseImageRepo))

	// Verify the manifest is actually an index (multi-arch)
	executor := cliWrappers.NewCliExecutor()
	manifestRaw, _, _, err := executor.Execute("skopeo", "inspect", "--raw", "--tls-verify=false", "docker://"+indexImage)
	Expect(err).ToNot(HaveOccurred())

	var manifest map[string]interface{}
	err = json.Unmarshal([]byte(manifestRaw), &manifest)
	Expect(err).ToNot(HaveOccurred())

	// Check it's a manifest list/index
	mediaType := manifest["mediaType"].(string)
	Expect(mediaType).To(Or(
		Equal("application/vnd.oci.image.index.v1+json"),
		Equal("application/vnd.docker.distribution.manifest.list.v2+json"),
	))

	// Check it has manifests
	manifests := manifest["manifests"].([]interface{})
	Expect(len(manifests)).To(Equal(2), "Expected 2 platform manifests in the index")
}

func TestBuildImageIndex_SingleImageSkipIndex(t *testing.T) {
	SetupGomega(t)
	var err error

	// Setup registry
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Create input data
	baseImageRepo := imageRegistry.GetTestNamespace() + "test-single-image"
	tag := GenerateUniqueTag(t)
	targetImage := baseImageRepo + ":" + tag

	// Create and push a single image
	sourceImageRef := baseImageRepo + "-source:" + tag
	err = CreateTestImage(TestImageConfig{
		ImageRef:       sourceImageRef,
		RandomDataSize: 1024,
	})
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(sourceImageRef)

	digest, err := PushImage(sourceImageRef)
	Expect(err).ToNot(HaveOccurred())

	// Build the image reference with digest
	imageRepo := common.GetImageName(sourceImageRef)
	imageWithDigest := imageRepo + "@" + digest

	// Run the command with always-build-index=false
	params := BuildImageIndexParams{
		Image:            targetImage,
		Images:           []string{imageWithDigest},
		TLSVerify:        !imageRegistry.IsLocal(),
		BuildahFormat:    "oci",
		AlwaysBuildIndex: false,
	}

	results, err := RunBuildImageIndex(params, imageRegistry)
	Expect(err).ToNot(HaveOccurred())

	// Verify results - should just return info about the single image
	Expect(results.ImageURL).To(Equal(targetImage))
	Expect(results.ImageDigest).To(Equal(digest))
	Expect(results.ImageRef).To(ContainSubstring(digest))
	Expect(results.Images).To(Equal(imageWithDigest))
}

func TestBuildImageIndex_StripTagFromDigest(t *testing.T) {
	SetupGomega(t)
	var err error

	// Setup registry
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Create input data
	baseImageRepo := imageRegistry.GetTestNamespace() + "test-tag-digest"
	tag := GenerateUniqueTag(t)
	indexImage := baseImageRepo + ":" + tag

	// Create and push an image
	sourceImageRef := baseImageRepo + "-source:" + tag
	err = CreateTestImage(TestImageConfig{
		ImageRef:       sourceImageRef,
		RandomDataSize: 1024,
	})
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(sourceImageRef)

	digest, err := PushImage(sourceImageRef)
	Expect(err).ToNot(HaveOccurred())

	// Build the image reference with BOTH tag and digest (repository:tag@digest)
	// This tests the normalization logic
	imageWithTagAndDigest := sourceImageRef + "@" + digest

	// Run the command
	params := BuildImageIndexParams{
		Image:            indexImage,
		Images:           []string{imageWithTagAndDigest},
		TLSVerify:        !imageRegistry.IsLocal(),
		BuildahFormat:    "oci",
		AlwaysBuildIndex: true,
	}

	// This should succeed even though the input has tag+digest
	// The code should normalize it to just repository@digest
	results, err := RunBuildImageIndex(params, imageRegistry)
	Expect(err).ToNot(HaveOccurred())

	// Verify results
	Expect(results.ImageDigest).ToNot(BeEmpty())
	Expect(results.Images).To(ContainSubstring(digest))
}

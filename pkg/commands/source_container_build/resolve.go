package source_container_build

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

const (
	skopeoRetryTimes = 5

	// sourceImageTag is the reference gobsi writes its image under in the OCI
	// layout it produces.
	sourceImageTag = "latest-source"

	// localImagePrefix marks an image that was never pushed to a registry.
	localImagePrefix = "localhost/"
)

// generateKonfluxSourceImage names the source image belonging to a binary
// image. The name is derived from the binary image alone, so that a later build
// can find the source of its base image by computing the same name.
func generateKonfluxSourceImage(repo, digest string) string {
	return fmt.Sprintf("%s:%s.src", repo, strings.ReplaceAll(digest, ":", "-"))
}

// handleBaseImageSources resolves the source image of the base image and
// downloads it, returning the directory holding it. An empty directory means
// there is nothing to merge into the built image.
func (s *SourceContainerBuild) handleBaseImageSources(workDir string) (string, error) {
	baseImages := s.baseImages()
	if len(baseImages) == 0 {
		log.Info("No base image is specified. Skip handling sources of base image")
		return "", nil
	}
	if len(baseImages) > 1 {
		log.Infof("Multiple base images are specified: %v", baseImages)
	}
	// Only the last one is handled: the single image, or the base image of the
	// last stage of a multistage Containerfile, which is the one the binary
	// image is actually built on.
	baseImage := baseImages[len(baseImages)-1]

	sourceImage, err := s.resolveSourceImage(baseImage)
	if err != nil {
		return "", err
	}
	if sourceImage == "" {
		log.Infof("Source image is not resolved for image %s", baseImage)
		return "", nil
	}

	return s.downloadParentImageSources(sourceImage, workDir)
}

// baseImages returns the base images to consider, discovering them from the
// SBOM of the binary image when none were given.
func (s *SourceContainerBuild) baseImages() []string {
	if given := nonEmptyLines(s.Params.BaseImages); len(given) > 0 {
		log.Infof("Base images were given: %v", given)
		return given
	}

	log.Info("No base image was given, reading the SBOM of the binary image instead")

	if discovered := s.discoverBaseImage(); discovered != "" {
		return []string{discovered}
	}

	return nil
}

// resolveSourceImage finds the source image of a binary image, or reports an
// empty name when there is none to find.
func (s *SourceContainerBuild) resolveSourceImage(binaryImage string) (string, error) {
	if strings.HasPrefix(binaryImage, localImagePrefix) {
		log.Infof("Skip handling local image %s", binaryImage)
		return "", nil
	}

	if slices.Contains(s.Params.RegistryAllowlist, registryOf(binaryImage)) {
		return s.resolveSourceImageByVersionRelease(binaryImage)
	}
	log.Infof("Image %s does not come from supported allowed registry", binaryImage)

	return s.resolveSourceImageByManifest(binaryImage)
}

// resolveSourceImageByVersionRelease finds a source image published under the
// version and release the binary image is labelled with.
func (s *SourceContainerBuild) resolveSourceImageByVersionRelease(binaryImage string) (string, error) {
	repo := common.GetImageName(binaryImage)

	rawConfig, err := s.SkopeoCli.Inspect(&cliwrappers.SkopeoInspectArgs{
		ImageRef:   repo + "@" + common.GetImageDigest(binaryImage),
		Config:     true,
		RetryTimes: skopeoRetryTimes,
	})
	if err != nil {
		return "", err
	}

	var imageConfig struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(rawConfig), &imageConfig); err != nil {
		return "", fmt.Errorf("failed to parse the config of image %s: %w", binaryImage, err)
	}

	version := imageConfig.Config.Labels["version"]
	release := imageConfig.Config.Labels["release"]
	if version == "" || release == "" {
		log.Warnf("Image %s is not labelled with version and release", binaryImage)
		return "", nil
	}

	return s.sourceImageIfExists(fmt.Sprintf("%s:%s-%s-source", repo, version, release))
}

// resolveSourceImageByManifest finds a source image published under the Konflux
// naming scheme, which derives it from the digest of the binary image.
func (s *SourceContainerBuild) resolveSourceImageByManifest(binaryImage string) (string, error) {
	digest := common.GetImageDigest(binaryImage)
	if digest == "" {
		return "", fmt.Errorf("image %s is missing a digest", binaryImage)
	}

	return s.sourceImageIfExists(generateKonfluxSourceImage(common.GetImageName(binaryImage), digest))
}

// sourceImageIfExists returns the image if the registry has it, and an empty
// name if it does not.
func (s *SourceContainerBuild) sourceImageIfExists(sourceImage string) (string, error) {
	exists, err := s.registryHasImage(sourceImage)
	if err != nil {
		return "", err
	}
	if !exists {
		log.Infof("Source container image %s does not exist", sourceImage)
		return "", nil
	}

	return sourceImage, nil
}

// registryHasImage reports whether an image is present in its registry. Only an
// image the registry says it does not have counts as absent: anything else is
// an error, so that a registry problem is not mistaken for a missing image.
func (s *SourceContainerBuild) registryHasImage(image string) (bool, error) {
	_, err := s.SkopeoCli.Inspect(&cliwrappers.SkopeoInspectArgs{
		ImageRef:   image,
		Raw:        true,
		RetryTimes: skopeoRetryTimes,
	})
	if err == nil {
		return true, nil
	}
	for _, absent := range []string{cliwrappers.ManifestUnknownMessage, cliwrappers.RepositoryUnknownMessage} {
		if strings.Contains(err.Error(), absent) {
			return false, nil
		}
	}

	return false, err
}

// downloadParentImageSources copies a source image into an OCI layout below
// workDir, and returns the directory holding it.
func (s *SourceContainerBuild) downloadParentImageSources(sourceImage, workDir string) (string, error) {
	sourcesDir, err := createDir(workDir, "parent_image_sources")
	if err != nil {
		return "", err
	}
	log.Infof("Copy source image %s into directory %s", sourceImage, sourcesDir)

	err = s.SkopeoCli.Copy(&cliwrappers.SkopeoCopyArgs{
		SourceImage:          sourceImage,
		DestinationImage:     sourcesDir,
		DestinationTransport: cliwrappers.SkopeoTransportOCI,
		// An OCI image layout cannot hold signatures.
		RemoveSignatures: true,
		RetryTimes:       skopeoRetryTimes,
		// Only the parent source image is verified against these. The built
		// image is pushed to the registry of the binary image, which the
		// policy of the system already covers.
		PolicyPath:        s.Params.SignaturePolicy,
		RegistriesDirPath: s.Params.SignatureRegistriesDir,
	})
	if err == nil {
		return sourcesDir, nil
	}
	if !strings.Contains(err.Error(), cliwrappers.MissingSignatureMessage) {
		return "", err
	}

	// A half written layout must not be left behind for gobsi to merge.
	log.Infof("Source image %s has no signature, deleting %s", sourceImage, sourcesDir)
	if removeErr := os.RemoveAll(sourcesDir); removeErr != nil {
		return "", removeErr
	}

	if s.Params.IgnoreUnsignedImage {
		log.Info("Ignored missing source image signatures")
		return "", nil
	}

	return "", err
}

// pushToRegistry pushes the built source image and returns the digest it was
// published under.
func (s *SourceContainerBuild) pushToRegistry(imageOutputDir, destImage string) (string, error) {
	digestFile, err := os.CreateTemp("", "source-image-digest-*")
	if err != nil {
		return "", fmt.Errorf("failed to create a digest file: %w", err)
	}
	defer os.Remove(digestFile.Name()) //nolint:errcheck // a leftover temporary file is not worth failing the build
	if err := digestFile.Close(); err != nil {
		return "", err
	}

	log.Infof("Push source image %s", destImage)

	err = s.SkopeoCli.Copy(&cliwrappers.SkopeoCopyArgs{
		SourceImage:      imageOutputDir + ":" + sourceImageTag,
		SourceTransport:  cliwrappers.SkopeoTransportOCI,
		DestinationImage: destImage,
		DigestFile:       digestFile.Name(),
		RetryTimes:       skopeoRetryTimes,
	})
	if err != nil {
		return "", err
	}

	digest, err := os.ReadFile(digestFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read the digest of the pushed image: %w", err)
	}

	return strings.TrimSpace(string(digest)), nil
}

// registryOf returns the registry part of an image reference.
func registryOf(image string) string {
	registry, _, _ := strings.Cut(image, "/")

	return registry
}

// nonEmptyLines splits a value into lines, dropping the blank ones.
func nonEmptyLines(value string) []string {
	lines := []string{}
	for line := range strings.SplitSeq(value, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return lines
}

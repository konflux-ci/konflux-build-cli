package source_container_build

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/package-url/packageurl-go"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

const (
	sbomDownloadAttempts = 5
	// baseImageProperty marks the component of the SBOM that describes the base
	// image of the last build stage. The SBOM holds at most one of them.
	baseImageProperty = "konflux:container:is_base_image"
)

// discoverBaseImage finds the base image the binary image was built on by
// reading the SBOM published beside it.
//
// An SBOM that cannot be downloaded or holds no base image is not an error: the
// source image is then built without the sources of the base image, which is
// worth a warning but not a failed build.
func (s *SourceContainerBuild) discoverBaseImage() string {
	image, err := s.imageToInspect()
	if err != nil {
		log.Warnf("Cannot tell which image to read the SBOM of: %s", err.Error())
		return ""
	}

	sbom, err := s.downloadSBOM(image)
	if err != nil {
		log.Warnf("Failed to download the SBOM of %s: %s", image, err.Error())
		log.Warn("The source image will not include the sources of the base image")
		return ""
	}

	baseImage, err := baseImageFromSBOM(sbom)
	if err != nil {
		log.Warnf("Cannot read the base image out of the SBOM of %s: %s", image, err.Error())
		return ""
	}
	if baseImage == "" {
		log.Infof("The SBOM of %s names no base image", image)
		return ""
	}

	log.Infof("Found base image %s in the SBOM of %s", baseImage, image)

	return baseImage
}

// imageToInspect returns the reference of the image whose SBOM describes the
// build. Every manifest of an image index carries its own SBOM, so one of them
// is picked: they are built from the same sources and name the same base image.
func (s *SourceContainerBuild) imageToInspect() (string, error) {
	image := common.GetImageName(s.Params.BinaryImageURL) + "@" + s.Params.BinaryImageDigest

	rawManifest, err := s.SkopeoCli.Inspect(&cliwrappers.SkopeoInspectArgs{
		ImageRef:   image,
		Raw:        true,
		RetryTimes: skopeoRetryTimes,
	})
	if err != nil {
		return "", err
	}

	var manifest struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal([]byte(rawManifest), &manifest); err != nil {
		return "", fmt.Errorf("failed to parse the manifest of %s: %w", image, err)
	}

	if len(manifest.Manifests) == 0 {
		return image, nil
	}

	digest := manifest.Manifests[0].Digest
	log.Infof("Image %s is a manifest list, reading the SBOM of %s", image, digest)

	return common.GetImageName(s.Params.BinaryImageURL) + "@" + digest, nil
}

// downloadSBOM fetches the SBOM attached to an image.
func (s *SourceContainerBuild) downloadSBOM(image string) (string, error) {
	// Cosign looks up the exact repository and the registry host only, so it
	// misses a credential stored for a path between the two. Hand it an
	// authentication file holding the one credential that covers this image.
	registryAuth, err := common.SelectRegistryAuth(image, s.AuthFile)
	if err != nil {
		return "", err
	}

	dockerConfigDir, err := common.CreateDockerConfigDir(registryAuth)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dockerConfigDir) //nolint:errcheck // a leftover temporary directory is not worth failing the build

	return s.CosignCli.DownloadSBOM(&cliwrappers.CosignDownloadSBOMArgs{
		Image:       image,
		MaxAttempts: sbomDownloadAttempts,
	}, dockerConfigDir)
}

// baseImageFromSBOM takes the base image out of an SBOM in either of the two
// formats Konflux publishes, returning an empty reference when none is marked.
func baseImageFromSBOM(sbom string) (string, error) {
	var format struct {
		BomFormat string `json:"bomFormat"`
	}
	if err := json.Unmarshal([]byte(sbom), &format); err != nil {
		return "", fmt.Errorf("failed to parse the SBOM: %w", err)
	}

	if format.BomFormat == "CycloneDX" {
		return baseImageFromCycloneDX(sbom)
	}

	return baseImageFromSPDX(sbom)
}

// baseImageFromCycloneDX reads the base image out of a CycloneDX SBOM, where it
// is a component of the formulation carrying the base image property.
func baseImageFromCycloneDX(sbom string) (string, error) {
	var doc struct {
		Formulation []struct {
			Components []struct {
				Purl       string `json:"purl"`
				Properties []struct {
					Name string `json:"name"`
				} `json:"properties"`
			} `json:"components"`
		} `json:"formulation"`
	}
	if err := json.Unmarshal([]byte(sbom), &doc); err != nil {
		return "", fmt.Errorf("failed to parse the CycloneDX SBOM: %w", err)
	}

	for _, formulation := range doc.Formulation {
		for _, component := range formulation.Components {
			for _, property := range component.Properties {
				if property.Name == baseImageProperty {
					return imageFromPurl(component.Purl)
				}
			}
		}
	}

	return "", nil
}

type spdxAnnotation struct {
	Comment string `json:"comment"`
}

type spdxExternalRef struct {
	ReferenceType    string `json:"referenceType"`
	ReferenceLocator string `json:"referenceLocator"`
}

type spdxPackage struct {
	Annotations  []spdxAnnotation  `json:"annotations"`
	ExternalRefs []spdxExternalRef `json:"externalRefs"`
}

// baseImageFromSPDX reads the base image out of an SPDX SBOM, where the base
// image property is a JSON encoded annotation of the package.
func baseImageFromSPDX(sbom string) (string, error) {
	var doc struct {
		Packages []spdxPackage `json:"packages"`
	}
	if err := json.Unmarshal([]byte(sbom), &doc); err != nil {
		return "", fmt.Errorf("failed to parse the SPDX SBOM: %w", err)
	}

	for _, pkg := range doc.Packages {
		if !isBaseImagePackage(pkg) {
			continue
		}
		for _, ref := range pkg.ExternalRefs {
			if ref.ReferenceType == "purl" {
				return imageFromPurl(ref.ReferenceLocator)
			}
		}
	}

	return "", nil
}

// isBaseImagePackage reports whether any annotation of a package marks it as
// the base image. An annotation that does not hold JSON is one of the many that
// describe something else.
func isBaseImagePackage(pkg spdxPackage) bool {
	for _, annotation := range pkg.Annotations {
		var comment struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(annotation.Comment), &comment); err != nil {
			continue
		}
		if comment.Name == baseImageProperty {
			return true
		}
	}

	return false
}

// imageFromPurl turns the package url of an image into the reference to pull
// it by, so that pkg:oci/ubi@sha256:abc?repository_url=registry.io/ubi gives
// registry.io/ubi@sha256:abc.
func imageFromPurl(purl string) (string, error) {
	if purl == "" {
		return "", nil
	}

	parsed, err := packageurl.FromString(purl)
	if err != nil {
		return "", fmt.Errorf("failed to parse the package url '%s': %w", purl, err)
	}

	repository := parsed.Qualifiers.Map()["repository_url"]
	if repository == "" {
		return "", fmt.Errorf("package url '%s' has no repository_url", purl)
	}
	if parsed.Version == "" {
		return "", fmt.Errorf("package url '%s' has no digest", purl)
	}

	return repository + "@" + parsed.Version, nil
}

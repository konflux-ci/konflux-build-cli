package common

import (
	"github.com/containers/image/v5/docker/reference"
	go_digest "github.com/opencontainers/go-digest"
)

// GetImageName trims tag and/or digest from given image reference using containers/image library.
func GetImageName(imageURL string) string {
	// Use ParseNamed instead of ParseNormalizedNamed to preserve original image names without auto-normalization.
	ref, err := reference.ParseNamed(imageURL)
	if err != nil {
		// If parsing fails, return empty string to maintain backwards compatibility.
		return ""
	}
	// Use TrimNamed to remove tag and digest.
	base := reference.TrimNamed(ref)
	return base.Name()
}

// IsImageNameValid validates image name using containers/image library.
func IsImageNameValid(imageName string) bool {
	// Try to parse the image name as a named reference
	// This will validate the format according to Docker/OCI standards
	_, err := reference.ParseNamed(imageName)
	return err == nil
}

func IsImageTagValid(tagName string) bool {
	// Create a minimal named reference to test tag validation against
	namedRef, _ := reference.ParseNamed("registry.io/test")
	// Try to create a tagged reference - if it succeeds, the tag is valid
	_, err := reference.WithTag(namedRef, tagName)
	return err == nil
}

func IsImageDigestValid(digest string) bool {
	// Use the go-digest library (which is used by containers/image) to parse and validate.
	_, err := go_digest.Parse(digest)
	return err == nil
}

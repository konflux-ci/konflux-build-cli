package source_container_build

import (
	"reflect"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

var ParamsConfig = map[string]common.Parameter{
	"binary-image-url": {
		Name:       "binary-image-url",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_BINARY_IMAGE_URL",
		Usage:      "URL of the binary image to build the sources for",
		Required:   true,
	},
	"binary-image-digest": {
		Name:       "binary-image-digest",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_BINARY_IMAGE_DIGEST",
		Usage:      "Digest of the binary image represented by argument --binary-image-url",
		Required:   true,
	},
	"source-dir": {
		Name:       "source-dir",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_SOURCE_DIR",
		Usage:      "Path to the directory holding the source code the binary image was built from",
		Required:   true,
	},
	"workspace-dir": {
		Name:       "workspace-dir",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_WORKSPACE_DIR",
		Usage:      "Directory to keep intermediate build artifacts in. A temporary directory is used when unset",
	},
	"base-images": {
		Name:       "base-images",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_BASE_IMAGES",
		Usage: "Base images the binary image was built from, one per line. Only the last one is handled, " +
			"which is the single image or the last stage of a multistage Containerfile. " +
			"Discovered from the SBOM of the binary image when unset",
	},
	"prefetch-artifacts-dir": {
		Name:       "prefetch-artifacts-dir",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_PREFETCH_ARTIFACTS_DIR",
		Usage:      "Path to the output directory populated by the prefetch-dependencies command",
	},
	"registry-allowlist": {
		Name:       "registry-allowlist",
		TypeKind:   reflect.Slice,
		EnvVarName: "KBC_SCB_REGISTRY_ALLOWLIST",
		Usage: "Registries whose parent source images are resolved from the version and release labels " +
			"of the base image. Source images for other registries are resolved by digest",
	},
	"signature-policy": {
		Name:       "signature-policy",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_SIGNATURE_POLICY",
		Usage: "Path to a trust policy to verify the signature of the parent source image against. " +
			"The policy of the system is used when unset",
	},
	"signature-registries-dir": {
		Name:       "signature-registries-dir",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_SIGNATURE_REGISTRIES_DIR",
		Usage: "Path to a directory of per-registry configuration saying where the signatures of an " +
			"image are published. The configuration of the system is used when unset",
	},
	"ignore-unsigned-image": {
		Name:         "ignore-unsigned-image",
		TypeKind:     reflect.Bool,
		EnvVarName:   "KBC_SCB_IGNORE_UNSIGNED_IMAGE",
		DefaultValue: "false",
		Usage:        "Continue without the base image sources when the parent source image has no signature",
	},
	"result-path-build-result": {
		Name:       "result-path-build-result",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_RESULT_PATH_BUILD_RESULT",
		Usage:      "Build result (JSON) result file path",
	},
	"result-path-source-image-url": {
		Name:       "result-path-source-image-url",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_RESULT_PATH_SOURCE_IMAGE_URL",
		Usage:      "Source image URL result file path",
	},
	"result-path-source-image-digest": {
		Name:       "result-path-source-image-digest",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_RESULT_PATH_SOURCE_IMAGE_DIGEST",
		Usage:      "Source image digest result file path",
	},
	"result-path-image-ref": {
		Name:       "result-path-image-ref",
		TypeKind:   reflect.String,
		EnvVarName: "KBC_SCB_RESULT_PATH_IMAGE_REF",
		Usage:      "Source image reference (URL and digest) result file path",
	},
}

type Params struct {
	BinaryImageURL       string   `paramName:"binary-image-url"`
	BinaryImageDigest    string   `paramName:"binary-image-digest"`
	SourceDir            string   `paramName:"source-dir"`
	WorkspaceDir         string   `paramName:"workspace-dir"`
	BaseImages           string   `paramName:"base-images"`
	PrefetchArtifactsDir string   `paramName:"prefetch-artifacts-dir"`
	RegistryAllowlist    []string `paramName:"registry-allowlist"`

	SignaturePolicy        string `paramName:"signature-policy"`
	SignatureRegistriesDir string `paramName:"signature-registries-dir"`
	IgnoreUnsignedImage    bool   `paramName:"ignore-unsigned-image"`

	ResultPathBuildResult       string `paramName:"result-path-build-result"`
	ResultPathSourceImageURL    string `paramName:"result-path-source-image-url"`
	ResultPathSourceImageDigest string `paramName:"result-path-source-image-digest"`
	ResultPathImageRef          string `paramName:"result-path-image-ref"`
}

package tektonbundle

import (
	"reflect"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
)

var TektonBundlePushParamsConfig = map[string]common.Parameter{
	"image": {
		Name:       "image",
		ShortName:  "i",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_IMAGE",
		TypeKind:   reflect.String,
		Usage:      "Destination bundle image reference, including its tag.",
		Required:   true,
	},
	"source": {
		Name:       "source",
		ShortName:  "s",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_SOURCE",
		TypeKind:   reflect.String,
		Usage:      "Path to the directory containing the source files.",
		Required:   true,
	},
	"context": {
		Name:         "context",
		ShortName:    "c",
		EnvVarName:   "KBC_TEKTON_BUNDLE_PUSH_CONTEXT",
		TypeKind:     reflect.String,
		DefaultValue: ".",
		Usage:        "Files or directories to include, relative to source.",
	},
	"url": {
		Name:       "url",
		ShortName:  "u",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_URL",
		TypeKind:   reflect.String,
		Usage:      "Source code Git URL.",
		Required:   true,
	},
	"revision": {
		Name:       "revision",
		ShortName:  "r",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_REVISION",
		TypeKind:   reflect.String,
		Usage:      "Source code Git revision.",
		Required:   true,
	},
	"steps-image": {
		Name:       "steps-image",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_STEPS_IMAGE",
		TypeKind:   reflect.String,
		Usage:      "Optional image to configure Task steps with in the bundle.",
	},
	"steps-image-step-names": {
		Name:       "steps-image-step-names",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_STEPS_IMAGE_STEP_NAMES",
		TypeKind:   reflect.String,
		Usage: "Comma- or space-separated step names to update with steps-image. " +
			"Prefix names with ! to update all other steps. If empty, update all steps.",
	},
	"result-path-image-digest": {
		Name:       "result-path-image-digest",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_RESULT_PATH_IMAGE_DIGEST",
		TypeKind:   reflect.String,
		Usage:      "Write the pushed bundle image digest into this file.",
	},
	"result-path-image-url": {
		Name:       "result-path-image-url",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_RESULT_PATH_IMAGE_URL",
		TypeKind:   reflect.String,
		Usage:      "Write the pushed bundle image repository and tag into this file.",
	},
	"result-path-image-ref": {
		Name:       "result-path-image-ref",
		EnvVarName: "KBC_TEKTON_BUNDLE_PUSH_RESULT_PATH_IMAGE_REF",
		TypeKind:   reflect.String,
		Usage:      "Write the pushed bundle image reference with its digest into this file.",
	},
}

type TektonBundlePushParams struct {
	Image                 string `paramName:"image"`
	Source                string `paramName:"source"`
	Context               string `paramName:"context"`
	URL                   string `paramName:"url"`
	Revision              string `paramName:"revision"`
	StepsImage            string `paramName:"steps-image"`
	StepsImageStepNames   string `paramName:"steps-image-step-names"`
	ResultPathImageDigest string `paramName:"result-path-image-digest"`
	ResultPathImageURL    string `paramName:"result-path-image-url"`
	ResultPathImageRef    string `paramName:"result-path-image-ref"`
}

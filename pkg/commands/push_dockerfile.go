package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

const (
	dockerfileAritfactTagSuffix = ".dockerfile"
	dockerfileArtifactType      = "application/vnd.konflux.dockerfile"
	dockerfileContext           = "."
	dockerfileFilePath          = "./Dockerfile"
	artifactTypeStrLength       = 100
)

type PushDockerfileParams struct {
	ImageUrl           string `paramName:"image-url"`
	Digest             string `paramName:"digest"`
	Dockerfile         string `paramName:"dockerfile"`
	Context            string `paramName:"context"`
	TagSuffix          string `paramName:"tag-suffix"`
	ArtifactType       string `paramName:"artifact-type"`
	Source             string `paramName:"source"`
	ImageRefResultFile string `paramName:"image-ref-result-file"`
}

var PushDockerfileParamsConfig = map[string]common.Parameter{
	"image-url": {
		Name:       "image-url",
		ShortName:  "i",
		EnvVarName: "KBC_PUSH_DOCKERFILE_IMAGE_URL",
		TypeKind:   reflect.String,
		Usage:      "Required. Binary image URL. Dockerfile is pushed to the image repository where this binary image is.",
		Required:   true,
	},
	"digest": {
		Name:       "digest",
		ShortName:  "d",
		EnvVarName: "KBC_PUSH_DOCKERFILE_IMAGE_DIGEST",
		TypeKind:   reflect.String,
		Usage:      "Required. Binary image digest, which is used to construct the tag of Dockerfile image.",
		Required:   true,
	},
	"dockerfile": {
		Name:         "dockerfile",
		ShortName:    "f",
		EnvVarName:   "KBC_PUSH_DOCKERFILE_DOCKERFILE_PATH",
		TypeKind:     reflect.String,
		DefaultValue: dockerfileFilePath,
		Usage:        fmt.Sprintf("Optional. Path to Dockerfile relative to source repository root. Defaults to '%s'.", dockerfileFilePath),
		Required:     false,
	},
	"context": {
		Name:         "context",
		ShortName:    "c",
		EnvVarName:   "KBC_PUSH_DOCKERFILE_CONTEXT",
		TypeKind:     reflect.String,
		DefaultValue: dockerfileContext,
		Usage:        fmt.Sprintf("Optional. Build context used to search Dockerfile. Defaults to '%s'.", dockerfileContext),
		Required:     false,
	},
	"tag-suffix": {
		Name:         "tag-suffix",
		ShortName:    "t",
		EnvVarName:   "KBC_PUSH_DOCKERFILE_TAG_SUFFIX",
		TypeKind:     reflect.String,
		DefaultValue: ".dockerfile",
		Usage:        "Optional. Suffix to construct artifact image tag. Defaults to '.dockerfile'.",
		Required:     false,
	},
	"artifact-type": {
		Name:         "artifact-type",
		ShortName:    "a",
		EnvVarName:   "KBC_PUSH_DOCKERFILE_ARTIFACT_TYPE",
		TypeKind:     reflect.String,
		DefaultValue: dockerfileArtifactType,
		Usage:        fmt.Sprintf("Optional. Artifact type of the dockerfile artifact image. Defaults to '%s'.", dockerfileArtifactType),
		Required:     false,
	},
	"source": {
		Name:       "source",
		ShortName:  "s",
		EnvVarName: "KBC_PUSH_DOCKERFILE_SOURCE",
		TypeKind:   reflect.String,
		Usage:      "Directory containing the source code. It is a relative path to the root of current working directory.",
		Required:   true,
	},
	"image-ref-result-file": {
		Name:       "image-ref-result-file",
		ShortName:  "r",
		EnvVarName: "KBC_PUSH_DOCKERFILE_RESULT_IMAGE_REF",
		TypeKind:   reflect.String,
		Usage:      "Optional. Write digested image reference of the pushed Dockerfile image into this file.",
		Required:   false,
	},
}

type PushDockerfileResults struct {
	ImageRef string `json:"image_ref"`
}

type PushDockerfile struct {
	Params        *PushDockerfileParams
	Results       PushDockerfileResults
	ResultsWriter common.ResultsWriterInterface

	imageName string
}

func NewPushDockerfile(cmd *cobra.Command) (*PushDockerfile, error) {
	params := &PushDockerfileParams{}
	if err := common.ParseParameters(cmd, PushDockerfileParamsConfig, params); err != nil {
		return nil, err
	}
	pushDockerfile := &PushDockerfile{
		Params:        params,
		ResultsWriter: common.NewResultsWriter(),
	}
	return pushDockerfile, nil
}

type SearchOpts struct {
	SourceDir  string
	ContextDir string
	Dockerfile string
}

// FIXME: User SearchDockerfile function
func (c *PushDockerfile) SearchDockerfile(opts SearchOpts) (string, error) {
	return filepath.Join(opts.SourceDir, opts.ContextDir, opts.Dockerfile), nil
}

func (c *PushDockerfile) Run() error {
	l.Logger.Infoln("Push Dockerfile")
	c.logParams()

	imageUrl := c.Params.ImageUrl
	c.imageName = common.GetImageName(imageUrl)

	if err := c.validateParams(); err != nil {
		return err
	}

	curDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting current directory: %w", err)
	}
	l.Logger.Infof("Using current directory: %s\n", curDir)

	// sourceDir := filepath.Join(curDir, c.Params.Source)
	// contextDir := filepath.Join(sourceDir, c.Params.Context)
	// l.Logger.Infof("Using context directory: %s\n", contextDir)
	// realContextDir, err := filepath.EvalSymlinks(contextDir)
	// if err != nil {
	// 	return fmt.Errorf("Failed to eval symlinks for path %s: %w", contextDir, err)
	// }
	// l.Logger.Infof("Using real context directory: %s\n", realContextDir)
	// if strings.HasPrefix(realContextDir, sourceDir) {
	// 	return fmt.Errorf("Context '%s' is invalid as it escapes the source directory '%s'.", c.Params.Context, sourceDir)
	// }

	dockerfilePath, err := c.SearchDockerfile(SearchOpts{
		SourceDir:  c.Params.Source,
		ContextDir: c.Params.Context,
		Dockerfile: c.Params.Dockerfile,
	})
	if err != nil {
		return fmt.Errorf("Cannot find Dockerfile: %w", err)
	}

	l.Logger.Infof("Select registry authentication for %s\n", imageUrl)
	registryAuth, err := common.SelectRegistryAuthFromDefaultAuthFile(imageUrl)
	if err != nil {
		return fmt.Errorf("Cannot select registry authentication for image %s: %w", imageUrl, err)
	}

	username, password, err := common.ExtractCredential(registryAuth.Token)
	if err != nil {
		return fmt.Errorf("Error on extracting authentication credential: %w", err)
	}

	dockerfileImageRef, _ := registry.ParseReference(c.imageName)
	tag := c.dockerfileImageTag()

	l.Logger.Infof("Pushing Dockerfile to registry. File: %s, tag: %s\n", dockerfilePath, tag)

	absDockerfilePath, err := filepath.Abs(dockerfilePath)
	if err != nil {
		return fmt.Errorf("Error on getting absolute path of %s: %w", dockerfilePath, err)
	}
	digest, err := common.OrasPush(username, password, dockerfileImageRef, tag, absDockerfilePath, c.Params.ArtifactType)
	if err != nil {
		return fmt.Errorf("Failed to push Dockerfile: %w", err)
	}

	artifactImageRef := fmt.Sprintf("%s@%s", c.imageName, digest)

	c.Results.ImageRef = artifactImageRef
	if resultsJson, err := c.ResultsWriter.CreateResultJson(c.Results); err != nil {
		return fmt.Errorf("Error on creating results JSON: %w", err)
	} else {
		l.Logger.Infof("%s\n", resultsJson)
	}

	if c.Params.ImageRefResultFile != "" {
		if err = c.ResultsWriter.WriteResultString(artifactImageRef, c.Params.Digest); err != nil {
			return fmt.Errorf("Error on writing result image digest: %w", err)
		}
	}

	return nil
}

func (c *PushDockerfile) dockerfileImageTag() string {
	digest := strings.Replace(c.Params.Digest, ":", "-", 1)
	return digest + c.Params.TagSuffix
}

func (c *PushDockerfile) validateParams() error {
	if !common.IsImageNameValid(c.imageName) {
		return fmt.Errorf("image name '%s' is invalid", c.imageName)
	}

	if !common.IsImageDigestValid(c.Params.Digest) {
		return fmt.Errorf("image digest '%s' is invalid", c.Params.Digest)
	}

	if !common.IsImageTagValid(c.dockerfileImageTag()) {
		return fmt.Errorf("Tag suffix '%s' is invalid as part of image tag.", c.Params.TagSuffix)
	}

	tagSuffix := c.Params.TagSuffix
	if len(tagSuffix) > artifactTypeStrLength {
		return fmt.Errorf("Artifact type '%s' is too long. Keep it in %d characters.", tagSuffix, artifactTypeStrLength)
	}

	return nil
}

func (c *PushDockerfile) logParams() {
	l.Logger.Infof("[param] Image URL: %s", c.Params.ImageUrl)
	l.Logger.Infof("[param] Image digest: %s", c.Params.Digest)
	l.Logger.Infof("[param] Tag suffix: %s", c.Params.TagSuffix)
	l.Logger.Infof("[param] Dockerfile: %s", c.Params.Dockerfile)
	l.Logger.Infof("[param] Context: %s", c.Params.Context)
	l.Logger.Infof("[param] Artifact type: %s", c.Params.ArtifactType)
	l.Logger.Infof("[param] Source directory: %s", c.Params.Source)
	l.Logger.Infof("[param] Image Reference result file: %s", c.Params.ImageRefResultFile)
}

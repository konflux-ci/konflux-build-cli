package commands

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

type PushDockerfileParams struct {
	ImageUrl      string   `paramName:"image-url"`
	Digest        string   `paramName:"digest"`
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
		Params: params,
		ResultsWriter: common.NewResultsWriter(),
	}
	return pushDockerfile, nil
}

func (c *PushDockerfile) Run() error {
	l.Logger.Infoln("Push Dockerfile")
	c.logParams()

	c.imageName = common.GetImageName(c.Params.ImageUrl)

	if err := c.validateParams(); err != nil {
		return err
	}

	// TODO: push

	return nil
}

func (c *PushDockerfile) validateParams() error {
	if !common.IsImageNameValid(c.imageName) {
		return fmt.Errorf("image name '%s' is invalid", c.imageName)
	}

	if !common.IsImageDigestValid(c.Params.Digest) {
		return fmt.Errorf("image digest '%s' is invalid", c.Params.Digest)
	}

	return nil
}

func (c *PushDockerfile) logParams() {
	l.Logger.Infof("[param] Image URL: %s", c.Params.ImageUrl)
	l.Logger.Infof("[param] Image digest: %s", c.Params.Digest)
}

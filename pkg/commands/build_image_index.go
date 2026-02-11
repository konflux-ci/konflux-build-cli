package commands

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var BuildImageIndexParamsConfig = map[string]common.Parameter{
	"image": {
		Name:       "image",
		ShortName:  "i",
		EnvVarName: "KBC_BUILD_IMAGE_INDEX_IMAGE",
		TypeKind:   reflect.String,
		Usage:      "The target image and tag where the image will be pushed to",
		Required:   true,
	},
	"images": {
		Name:       "images",
		ShortName:  "",
		EnvVarName: "KBC_BUILD_IMAGE_INDEX_IMAGES",
		TypeKind:   reflect.Slice,
		Usage:      "List of Image Manifests to be referenced by the Image Index",
		Required:   true,
	},
}

type BuildImageIndexParams struct {
	Image  string   `paramName:"image"`
	Images []string `paramName:"images"`
}

type BuildImageIndexResults struct {
	ImageDigest string `json:"image_digest"`
	ImageURL    string `json:"image_url"`
	ImageRef    string `json:"image_ref"`
}

type BuildImageIndex struct {
	Params        *BuildImageIndexParams
	Results       BuildImageIndexResults
	ResultsWriter common.ResultsWriterInterface
}

func NewBuildImageIndex(cmd *cobra.Command) (*BuildImageIndex, error) {
	params := &BuildImageIndexParams{}
	if err := common.ParseParameters(cmd, BuildImageIndexParamsConfig, params); err != nil {
		return nil, err
	}

	buildImageIndex := &BuildImageIndex{
		Params:        params,
		ResultsWriter: common.NewResultsWriter(),
	}

	return buildImageIndex, nil
}

func (c *BuildImageIndex) Run() error {
	c.logParams()

	if err := c.validateParams(); err != nil {
		return err
	}

	l.Logger.Info("Building image index (not yet implemented)")

	if resultsJson, err := c.ResultsWriter.CreateResultJson(c.Results); err == nil {
		fmt.Print(resultsJson)
	} else {
		return fmt.Errorf("failed to create results JSON: %w", err)
	}

	return nil
}

func (c *BuildImageIndex) validateParams() error {
	if len(c.Params.Images) == 0 {
		return fmt.Errorf("at least one image must be provided via --images")
	}

	return nil
}

func (c *BuildImageIndex) logParams() {
	l.Logger.Infof("[param] Image: %s", c.Params.Image)
	l.Logger.Infof("[param] Images: %v", c.Params.Images)
}

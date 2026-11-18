package image

import (
	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/commands/source_container_build"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var SourceContainerBuildCmd = &cobra.Command{
	Use:   "source-container-build",
	Short: "Build and push the source container image of a binary image.",
	Long: `Build and push the source container image of a binary image.

The source image carries the sources the binary image was built from:

  - an archive of the repository at HEAD
  - the prefetched dependencies, as archives and as SRPMs
  - the sources of the base image of the last build stage, largely the SRPMs
    of the packages it ships

It is pushed beside the binary image, tagged with the binary image digest and
a .src suffix.

Nothing is pushed when the binary image is no longer in the registry. The
result reports the status "drop" and the command succeeds, since there is
nothing left for the source image to describe.`,
	Example: `
  konflux-build-cli image source-container-build \
    --binary-image-url quay.io/myorg/myimage:latest \
    --binary-image-digest sha256:abc123 \
    --source-dir ./source \
    --base-images-file ./base-images.txt \
    --prefetch-artifacts-dir ./prefetch \
    --registry-allowlist registry.access.redhat.com \
    --registry-allowlist registry.redhat.io
	`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Logger.Debug("Starting the `image source-container-build` command")
		sourceContainerBuild, err := source_container_build.New(cmd)
		if err != nil {
			logger.Logger.Fatal(err)
		}
		if err := sourceContainerBuild.Run(); err != nil {
			logger.Logger.Fatal(err)
		}
		logger.Logger.Debug("Finished the `image source-container-build` command")
	},
}

func init() {
	common.RegisterParameters(SourceContainerBuildCmd, source_container_build.ParamsConfig)
}

package image

import (
	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/commands"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

const commandName = "push-containerfile"

var PushContainerfileCmd = &cobra.Command{
	Use:   commandName,
	Short: "Discover Containerfile from source code and push it to registry as an OCI artifact.",
	Long:  "Discover Containerfile from source code and push it to registry as an OCI artifact.",
	Run: func(cmd *cobra.Command, args []string) {
		l.Logger.Debugf("Starting %s", commandName)
		pushContainerfile, err := commands.NewPushContainerfile(cmd)
		if err != nil {
			l.Logger.Fatal(err)
		}
		if err := pushContainerfile.Run(); err != nil {
			l.Logger.Fatal(err)
		}
		l.Logger.Debugf("Finished %s", commandName)
	},
}

func init() {
	common.RegisterParameters(PushContainerfileCmd, commands.PushContainerfileParamsConfig)
}

package tektonbundle

import (
	tektonbundlecmd "github.com/konflux-ci/konflux-build-cli/pkg/commands/tektonbundle"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var TektonBundlePushCmd = &cobra.Command{
	Use:     "push",
	Short:   "Push a Tekton bundle to an OCI registry",
	Long:    `TODO`,
	Example: `TODO`,
	Run: func(cmd *cobra.Command, arg []string) {
		pushTektonBundle, err := tektonbundlecmd.NewTektonBundlePush(cmd)
		if err != nil {
			l.Logger.Fatal(err)
		}
		if err := pushTektonBundle.Run(); err != nil {
			l.Logger.Fatal(err)
		}
	},
}

func init() {
	common.RegisterParameters(TektonBundlePushCmd, tektonbundlecmd.TektonBundlePushParamsConfig)
}

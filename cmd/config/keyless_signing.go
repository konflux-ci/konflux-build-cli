package config

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/commands"
	"github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var KeylessSigningCmd = &cobra.Command{
	Use:   "keyless-signing",
	Short: "Initialize keyless signing",
	Long:  "Initialize keyless signing configuration with cosign. See https://www.sigstore.dev",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Logger.Debug("Starting the `config keyless-signing` command")
		keylessSigning, err := commands.NewKeylessSigning(cmd)
		if err != nil {
			logger.Logger.Fatal(err)
		}
		if err := keylessSigning.Run(); err != nil {
			logger.Logger.Fatal(err)
		}
		logger.Logger.Debug("Finished the `config keyless-signing` command")
	},
}

package cmd

import (
	"github.com/konflux-ci/konflux-build-cli/cmd/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "A sub command group to work with configurations",
}

func init() {
	configCmd.AddCommand(config.ConfigCacheProxyCmd)
}

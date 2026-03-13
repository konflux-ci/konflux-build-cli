package cmd

import (
	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/cmd/internal"
)

var internalCmd = &cobra.Command{
	Use:    "internal",
	Short:  "Internal commands, not intended for direct use",
	Hidden: true,
}

func init() {
	internalCmd.AddCommand(internal.InUserNamespaceCmd)
}

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/cmd/tektonbundle"
)

var tektonBundleGroup = &cobra.Command{
	Use:    "tkn-bundle",
	Short:  "Creates and pushes a Tekton bundle containing the specified Tekton YAML files.",
	Hidden: false,
}

func init() {
	tektonBundleGroup.AddCommand(tektonbundle.TektonBundlePushCmd)
}

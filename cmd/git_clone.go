package cmd

import (
	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/commands/git_clone"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var GitCloneCmd = &cobra.Command{
	Use:   "git-clone",
	Short: "Clone a git repository",
	Long: `Clone a git repository with support for submodules, sparse checkout,
    authentication, and optional merge with a target branch.`,
	Run: func(cmd *cobra.Command, args []string) {
		l.Logger.Debug("Starting git-clone")
		gitClone, err := git_clone.New(cmd)
		if err != nil {
			l.Logger.Fatal(err)
		}
		if err := gitClone.Run(); err != nil {
			l.Logger.Fatal(err)
		}
		l.Logger.Debug("Finished git-clone")
	},
}

func init() {
	common.RegisterParameters(GitCloneCmd, git_clone.ParamsConfig)
}

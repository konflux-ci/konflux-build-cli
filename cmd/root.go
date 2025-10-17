/*
Copyright © 2025 Mykola Morhun

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "konflux-build-cli",
	Short: "A helper CLI tool for Konflux build pipelines",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	processedArgs := common.ExpandArrayParameters(os.Args[1:])
	rootCmd.SetArgs(processedArgs)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Common flags for all subcommands
	var logLevel string
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Set the logging level (debug, info, warn, error, fatal)")

	var workdir string
	rootCmd.PersistentFlags().StringVar(&workdir, "workdir", "", "Set working directory")

	cobra.OnInitialize(func() {
		if err := l.InitLogger(logLevel); err != nil {
			fmt.Printf("failed to init logger: %s", err.Error())
			os.Exit(2)
		}

		if workdir == "" {
			// Workdir parameter was not set, try env var
			workdir = os.Getenv("WORKDIR")
		}
		if workdir != "" {
			if err := os.Chdir(workdir); err != nil {
				l.Logger.Fatalf("failed to apply workdir '%s': %s", workdir, err.Error())
				os.Exit(3)
			}
		}
	})
}

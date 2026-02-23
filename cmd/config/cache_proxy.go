package config

import (
	"github.com/konflux-ci/konflux-build-cli/pkg/commands"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var ConfigCacheProxyCmd = &cobra.Command{
	Use:   "cache-proxy",
	Short: "enable or disable cache proxy",
	Long: `Helps enable or disable cache proxy in the build pipeline
based on the values set in "cluster-config" config map in "konflux-info" namespace `,
	Example: `To enable the cache proxy:
konflux-build-cli config cache-proxy --enable "true"

To disable the cache proxy:
konflux-build-cli config cache-proxy --enable "false"

To set the default http proxy and default no proxy values:
konflux-build-cli config cache-proxy --enable true --default-http-proxy "svc.local:3128" --default-no-proxy "docker.io,gcr.io"

To change the default result path for http-proxy and no-proxy:
konflux-build-cli config cache-proxy --enable true --http-proxy-result-path "/tmp/http-proxy" --no-proxy-result-path "/tmp/no-proxy"
	`,

	Run: func(cmd *cobra.Command, args []string) {
		l.Logger.Debug("Configuring cache-proxy...")
		enableCacheProxy, err := commands.NewCacheProxy(cmd)
		if err != nil {
			l.Logger.Fatal(err)
		}
		if err := enableCacheProxy.Run(); err != nil {
			l.Logger.Fatal(err)
		}
		l.Logger.Debug("cache-proxy configured")
	},
}

func init() {
	common.RegisterParameters(ConfigCacheProxyCmd, commands.CacheProxyParamsConfig)
}

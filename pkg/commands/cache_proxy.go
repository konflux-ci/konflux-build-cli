package commands

import (
	"reflect"

	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/konflux-ci/konflux-build-cli/pkg/config"
	"github.com/spf13/cobra"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var CacheProxyParamsConfig = map[string]common.Parameter{
	"enable": {
		Name:       "enable",
		ShortName:  "e",
		EnvVarName: "ENABLE",
		TypeKind:   reflect.String,
		Usage:      "Whether to enable cache proxy or not. Required.",
		Required:   true,
	},
	"default-http-proxy": {
		Name:         "default-http-proxy",
		ShortName:    "p",
		EnvVarName:   "DEFAULT_HTTP_PROXY",
		TypeKind:     reflect.String,
		Usage:        "change the default http proxy value. Optional.",
		Required:     false,
		DefaultValue: "",
	},
	"default-no-proxy": {
		Name:         "default-no-proxy",
		ShortName:    "n",
		EnvVarName:   "DEFAULT_NO_PROXY",
		TypeKind:     reflect.String,
		Usage:        "change the default no proxy value. Optional.",
		Required:     false,
		DefaultValue: "",
	},
	"http-proxy-result-path": {
		Name:         "http-proxy-result-path",
		ShortName:    "r",
		EnvVarName:   "HTTP_PROXY_RESULTS_PATH",
		TypeKind:     reflect.String,
		Usage:        "set the result path for http proxy. Optional.",
		Required:     false,
		DefaultValue: "http-proxy",
	},
	"no-proxy-result-path": {
		Name:         "no-proxy-result-path",
		ShortName:    "s",
		EnvVarName:   "NO_PROXY_RESULTS_PATH",
		TypeKind:     reflect.String,
		Usage:        "set the result path for no proxy. Optional.",
		Required:     false,
		DefaultValue: "no-proxy",
	},
}

type CacheProxyParams struct {
	Enable              string `paramName:"enable"`
	DefaultHttpProxy    string `paramName:"default-http-proxy"`
	DefaultNoProxy      string `paramName:"default-no-proxy"`
	HttpProxyResultPath string `paramName:"http-proxy-result-path"`
	NoProxyResultPath   string `paramName:"no-proxy-result-path"`
}

type CacheProxyResults struct {
	HttpProxy string `json:"http-proxy"`
	NoProxy   string `json:"no-proxy"`
}

type CacheProxy struct {
	Params        *CacheProxyParams
	Results       CacheProxyResults
	ResultsWriter common.ResultsWriterInterface
}

func NewCacheProxy(cmd *cobra.Command) (*CacheProxy, error) {
	var err error
	cacheProxy := &CacheProxy{}

	params := &CacheProxyParams{}
	if err = common.ParseParameters(cmd, CacheProxyParamsConfig, params); err != nil {
		return nil, err
	}
	cacheProxy.Params = params

	cacheProxy.ResultsWriter = common.NewResultsWriter()

	return cacheProxy, nil
}

// Run executes the command logic.
func (c *CacheProxy) Run() error {
	var cacheProxyConfig *config.CacheProxyConfig

	common.LogParameters(CacheProxyParamsConfig, c.Params)

	l.Logger.Debug("Reading config data")
	konfluxConfig, err := config.GetKonfluxConfig()
	if err != nil {
		// failed to read config data, use defaults
		l.Logger.Warnf("Error while reading config data: %s", err.Error())
		cacheProxyConfig = &config.CacheProxyConfig{
			Allowed:   true,
			HttpProxy: c.Params.DefaultHttpProxy,
			NoProxy:   c.Params.DefaultNoProxy,
		}
	} else {
		cacheProxyConfig = konfluxConfig.CacheProxy
		if cacheProxyConfig == nil {
			cacheProxyConfig = &config.CacheProxyConfig{
				Allowed: true, // If unset, allow using the proxy
			}
		}

		if cacheProxyConfig.HttpProxy == "" && cacheProxyConfig.NoProxy == "" {
			cacheProxyConfig.HttpProxy = c.Params.DefaultHttpProxy
			cacheProxyConfig.NoProxy = c.Params.DefaultNoProxy

			l.Logger.Debug("Falling back to default proxy config")
		}
	}
	l.Logger.Debugf("Using cache proxy config: %s", cacheProxyConfig.ToString())

	// Use proxy only if both backend and the parameter allow it
	if cacheProxyConfig.Allowed && c.Params.Enable == "true" {
		c.Results.HttpProxy = cacheProxyConfig.HttpProxy
		c.Results.NoProxy = cacheProxyConfig.NoProxy
		l.Logger.Info("Cache proxy enabled in both backend and param")
	} else {
		c.Results.HttpProxy = ""
		c.Results.NoProxy = ""
		if !cacheProxyConfig.Allowed {
			l.Logger.Info("Cache proxy is disabled via cluster config")
		} else {
			l.Logger.Info("Cache proxy is disabled via param")
		}
	}

	c.logResults()

	err = c.ResultsWriter.WriteResultString(c.Results.HttpProxy, c.Params.HttpProxyResultPath)
	if err != nil {
		l.Logger.Errorf("failed to write result for http-proxy with error: %s", err.Error())
	}
	err = c.ResultsWriter.WriteResultString(c.Results.NoProxy, c.Params.NoProxyResultPath)
	if err != nil {
		l.Logger.Errorf("failed to write result for no-proxy with error: %s", err.Error())
	}

	return nil
}

func (c *CacheProxy) logResults() {
	l.Logger.Infof("[result] HTTP PROXY: %s", c.Results.HttpProxy)
	l.Logger.Infof("[result] NO PROXY: %s", c.Results.NoProxy)
}

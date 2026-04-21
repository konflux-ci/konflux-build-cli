package config

import (
	"fmt"
	"strconv"
)

type CacheProxyConfig struct {
	Allowed   bool
	HttpProxy string
	NoProxy   string
}

var _ KonfluxConfigPart[*CacheProxyConfig] = (*CacheProxyConfig)(nil)

func NewCacheProxyConfig(rawConfig KonfluxRawConfig) (*CacheProxyConfig, error) {
	cacheProxyConfig := &CacheProxyConfig{
		HttpProxy: rawConfig.HttpProxy,
		NoProxy:   rawConfig.NoProxy,
	}

	if rawConfig.AllowCacheProxy == "" {
		// If unset, allow using the proxy.
		cacheProxyConfig.Allowed = true
		return cacheProxyConfig, nil
	}

	isCacheProxyEnabled, err := strconv.ParseBool(rawConfig.AllowCacheProxy)
	cacheProxyConfig.Allowed = isCacheProxyEnabled
	return cacheProxyConfig, err
}

func (c *CacheProxyConfig) DeepCopy() *CacheProxyConfig {
	return &CacheProxyConfig{
		Allowed:   c.Allowed,
		HttpProxy: c.HttpProxy,
		NoProxy:   c.NoProxy,
	}
}

func (c *CacheProxyConfig) ToString() string {
	return fmt.Sprintf("%+v", c)
}

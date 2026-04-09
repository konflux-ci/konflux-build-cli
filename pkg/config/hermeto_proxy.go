package config

import (
	"fmt"
	"strconv"
)

type HermetoProxyConfig struct {
	PackageRegistryProxyAllowed bool
	NpmProxy                    string
	YarnProxy                   string
}

var _ KonfluxConfigPart[*HermetoProxyConfig] = (*HermetoProxyConfig)(nil)

func NewHermetoProxyConfig(rawConfig KonfluxRawConfig) (*HermetoProxyConfig, error) {
	hermetoProxyConfig := &HermetoProxyConfig{
		NpmProxy:  rawConfig.HermetoNpmProxy,
		YarnProxy: rawConfig.HermetoYarnProxy,
	}

	if rawConfig.HermetoPackageRegistryProxyAllowed == "" {
		hermetoProxyConfig.PackageRegistryProxyAllowed = false
		return hermetoProxyConfig, nil
	}

	isPackageRegistryProxyAllowed, err := strconv.ParseBool(rawConfig.HermetoPackageRegistryProxyAllowed)
	hermetoProxyConfig.PackageRegistryProxyAllowed = isPackageRegistryProxyAllowed
	return hermetoProxyConfig, err
}

func (c *HermetoProxyConfig) DeepCopy() *HermetoProxyConfig {
	return &HermetoProxyConfig{
		PackageRegistryProxyAllowed: c.PackageRegistryProxyAllowed,
		NpmProxy:                    c.NpmProxy,
		YarnProxy:                   c.YarnProxy,
	}
}

func (c *HermetoProxyConfig) ToString() string {
	return fmt.Sprintf("%+v", c)
}

package config

import (
	"fmt"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/sirupsen/logrus"
)

var configLog = l.Logger.WithField("logger", "KonfluxConfig")

// Global interface to get Konflux config.
// Allow reassigning for mocking in tests.
var GetKonfluxConfig func() (*KonfluxConfig, error) = getKonfluxConfig

type KonfluxConfig struct {
	CacheProxy   *CacheProxyConfig
	HermetoProxy *HermetoProxyConfig
	// Raw config as it was read from config source
	RawConfig KonfluxRawConfig
}

// Interface for all sub structs included into KonfluxConfig (except RawConfig)
type KonfluxConfigPart[T any] interface {
	DeepCopy() T
	ToString() string
}

// Cache Konflux config to avoid unneeded expensive requests
var konfluxConfig *KonfluxConfig

func ResetConfigCache() {
	konfluxConfig = nil
}

func getKonfluxConfig() (*KonfluxConfig, error) {
	if konfluxConfig != nil {
		return konfluxConfig.DeepCopy(), nil
	}

	configReader, err := NewConfigReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create config reader: %w", err)
	}
	rawConfig, err := configReader.ReadConfigData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Konflux config: %w", err)
	}
	konfluxConfig = newKonfluxConfig(rawConfig)
	return getKonfluxConfig()
}

// newKonfluxConfig parses raw Konflux config into structured representations.
// If some parts of the config cannot be parsed, do not break whole config parse.
// It's up to specific portion of configuration to return nil or partial config.
func newKonfluxConfig(rawConfig *KonfluxRawConfig) *KonfluxConfig {
	konfluxConfig := &KonfluxConfig{RawConfig: *rawConfig}

	cacheProxy, err := NewCacheProxyConfig(*rawConfig)
	if err != nil {
		configLog.Errorf("failed to parse cache proxy config: %s", err.Error())
	}
	konfluxConfig.CacheProxy = cacheProxy

	hermetoProxy, err := NewHermetoProxyConfig(*rawConfig)
	if err != nil {
		configLog.Errorf("failed to parse hermeto proxy config: %s", err.Error())
	}
	konfluxConfig.HermetoProxy = hermetoProxy

	return konfluxConfig
}

func (c *KonfluxConfig) DeepCopy() *KonfluxConfig {
	copy := &KonfluxConfig{RawConfig: c.RawConfig}

	if c.CacheProxy != nil {
		copy.CacheProxy = c.CacheProxy.DeepCopy()
	}
	if c.HermetoProxy != nil {
		copy.HermetoProxy = c.HermetoProxy.DeepCopy()
	}

	return copy
}

func (c *KonfluxConfig) LogConfig(logLevel logrus.Level) {
	configLog.Logf(logLevel, "cache proxy config: %s", c.CacheProxy.ToString())
	configLog.Logf(logLevel, "hermeto proxy config: %s", c.HermetoProxy.ToString())

	configLog.Logf(logLevel, "Raw config: %+v", c.RawConfig)
}

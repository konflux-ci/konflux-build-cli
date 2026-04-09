package config_test

import (
	"errors"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/gomega"
)

var ReadConfigDataFunc func() (*config.KonfluxRawConfig, error)

type MockConfigReader struct{}

var _ config.ConfigReader = (*MockConfigReader)(nil)

func (mcr *MockConfigReader) ReadConfigData() (*config.KonfluxRawConfig, error) {
	if ReadConfigDataFunc != nil {
		return ReadConfigDataFunc()
	}
	return &config.KonfluxRawConfig{}, nil
}

var NewMockConfigReader = func() (config.ConfigReader, error) {
	return &MockConfigReader{}, nil
}

func TestKonfluxConfig(t *testing.T) {
	g := NewWithT(t)

	const testHttpProxy = "test-proxy.io"
	const testNoProxy = "no-proxy.io"
	const testNpmProxy = "npm-proxy.io"
	const testYarnProxy = "yarn-proxy.io"

	t.Run("should fail if failed to create config reader", func(t *testing.T) {
		config.NewConfigReader = func() (config.ConfigReader, error) {
			return nil, errors.New("failed to create config reader")
		}
		config.ResetConfigCache()

		kofluxConfig, err := config.GetKonfluxConfig()
		g.Expect(kofluxConfig).To(BeNil())
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("should fail if failed to read the config", func(t *testing.T) {
		ReadConfigDataFunc = func() (*config.KonfluxRawConfig, error) {
			return &config.KonfluxRawConfig{}, errors.New("failed to read config")
		}
		config.NewConfigReader = NewMockConfigReader
		config.ResetConfigCache()

		kofluxConfig, err := config.GetKonfluxConfig()
		g.Expect(kofluxConfig).To(BeNil())
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("should create and cache config reader", func(t *testing.T) {
		ReadConfigDataFunc = func() (*config.KonfluxRawConfig, error) {
			return &config.KonfluxRawConfig{
				AllowCacheProxy: "true",
				HttpProxy:       testHttpProxy,
				NoProxy:         testNoProxy,

				HermetoPackageRegistryProxyAllowed: "true",
				HermetoNpmProxy:                    testNpmProxy,
				HermetoYarnProxy:                   testYarnProxy,
			}, nil
		}
		config.NewConfigReader = NewMockConfigReader
		config.ResetConfigCache()

		kofluxConfig, err := config.GetKonfluxConfig()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(kofluxConfig.CacheProxy).ToNot(BeNil())
		g.Expect(kofluxConfig.CacheProxy.Allowed).To(BeTrue())
		g.Expect(kofluxConfig.CacheProxy.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(kofluxConfig.CacheProxy.NoProxy).To(Equal(testNoProxy))

		g.Expect(kofluxConfig.HermetoProxy).ToNot(BeNil())
		g.Expect(kofluxConfig.HermetoProxy.PackageRegistryProxyAllowed).To(BeTrue())
		g.Expect(kofluxConfig.HermetoProxy.NpmProxy).To(Equal(testNpmProxy))
		g.Expect(kofluxConfig.HermetoProxy.YarnProxy).To(Equal(testYarnProxy))

		// Test that it can log config without errors
		kofluxConfig.LogConfig(logrus.InfoLevel)

		// Read config second time, should return cached value
		ReadConfigDataFunc = func() (*config.KonfluxRawConfig, error) {
			// Should not attempt to read the config second time
			t.Fail()
			return nil, nil
		}
		kofluxConfig, err = config.GetKonfluxConfig()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(kofluxConfig.CacheProxy).ToNot(BeNil())
		g.Expect(kofluxConfig.HermetoProxy).ToNot(BeNil())
	})

	t.Run("should create config reader even if sub config errors", func(t *testing.T) {
		ReadConfigDataFunc = func() (*config.KonfluxRawConfig, error) {
			return &config.KonfluxRawConfig{
				AllowCacheProxy: "invalid",
				HttpProxy:       testHttpProxy,
				NoProxy:         testNoProxy,

				HermetoPackageRegistryProxyAllowed: "invalid",
				HermetoNpmProxy:                    testNpmProxy,
				HermetoYarnProxy:                   testYarnProxy,
			}, nil
		}
		config.NewConfigReader = NewMockConfigReader
		config.ResetConfigCache()

		kofluxConfig, err := config.GetKonfluxConfig()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(kofluxConfig.CacheProxy).ToNot(BeNil())
	})

	config.NewConfigReader = config.ConfigReaderFactory
}

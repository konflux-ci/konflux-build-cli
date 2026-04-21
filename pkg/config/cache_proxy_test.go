package config_test

import (
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"

	. "github.com/onsi/gomega"
)

func Test_NewCacheProxyConfig(t *testing.T) {
	g := NewWithT(t)

	const testHttpProxy = "test-proxy.io"
	const testNoProxy = "no-proxy.io"

	t.Run("should create cache proxy config", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{
			AllowCacheProxy: "true",
			HttpProxy:       testHttpProxy,
			NoProxy:         testNoProxy,
		}

		cacheProxyConfig, err := config.NewCacheProxyConfig(rawConfig)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cacheProxyConfig.Allowed).To(BeTrue())
		g.Expect(cacheProxyConfig.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(cacheProxyConfig.NoProxy).To(Equal(testNoProxy))
	})

	t.Run("should create cache proxy config if parse error happens", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{
			AllowCacheProxy: "abcd",
			HttpProxy:       testHttpProxy,
			NoProxy:         testNoProxy,
		}

		cacheProxyConfig, err := config.NewCacheProxyConfig(rawConfig)
		g.Expect(err).To(HaveOccurred())
		g.Expect(cacheProxyConfig.Allowed).To(BeFalse())
		g.Expect(cacheProxyConfig.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(cacheProxyConfig.NoProxy).To(Equal(testNoProxy))
	})

	t.Run("should create cache proxy config from empty values", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{}

		cacheProxyConfig, err := config.NewCacheProxyConfig(rawConfig)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cacheProxyConfig.Allowed).To(BeTrue())
		g.Expect(cacheProxyConfig.HttpProxy).To(BeEmpty())
		g.Expect(cacheProxyConfig.NoProxy).To(BeEmpty())
	})
}

func Test_CacheProxyConfig_DeepCopy(t *testing.T) {
	g := NewWithT(t)

	const testHttpProxy = "test-proxy.io"
	const testNoProxy = "no-proxy.io"

	t.Run("should deep copy cache proxy config", func(t *testing.T) {
		cacheProxyConfig := &config.CacheProxyConfig{
			Allowed:   true,
			HttpProxy: testHttpProxy,
			NoProxy:   testNoProxy,
		}

		cacheProxyConfigCopy := cacheProxyConfig.DeepCopy()

		cacheProxyConfig.Allowed = false
		cacheProxyConfig.HttpProxy = "proxy"
		cacheProxyConfig.NoProxy = "no-proxy"

		g.Expect(cacheProxyConfigCopy.Allowed).To(BeTrue())
		g.Expect(cacheProxyConfigCopy.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(cacheProxyConfigCopy.NoProxy).To(Equal(testNoProxy))
	})
}

func Test_CacheProxyConfig_ToString(t *testing.T) {
	g := NewWithT(t)

	cacheProxyConfig := &config.CacheProxyConfig{
		Allowed:   true,
		HttpProxy: "test-proxy.io",
		NoProxy:   "no-proxy.io",
	}

	str := cacheProxyConfig.ToString()

	g.Expect(str).ToNot(BeEmpty())
}

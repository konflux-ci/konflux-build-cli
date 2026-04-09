package config_test

import (
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"

	. "github.com/onsi/gomega"
)

func Test_NewHermetoProxyConfig(t *testing.T) {
	g := NewWithT(t)

	const testNpmProxy = "test-npm-proxy.io"
	const testYarnProxy = "test-yarn-proxy.io"

	t.Run("should create hermeto proxy config", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{
			HermetoPackageRegistryProxyAllowed: "true",
			HermetoNpmProxy:                    testNpmProxy,
			HermetoYarnProxy:                   testYarnProxy,
		}

		hermetoProxyConfig, err := config.NewHermetoProxyConfig(rawConfig)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hermetoProxyConfig.PackageRegistryProxyAllowed).To(BeTrue())
		g.Expect(hermetoProxyConfig.NpmProxy).To(Equal(testNpmProxy))
		g.Expect(hermetoProxyConfig.YarnProxy).To(Equal(testYarnProxy))
	})

	t.Run("should create hermeto proxy config if parse error happens", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{
			HermetoPackageRegistryProxyAllowed: "abcd",
			HermetoNpmProxy:                    testNpmProxy,
			HermetoYarnProxy:                   testYarnProxy,
		}

		hermetoProxyConfig, err := config.NewHermetoProxyConfig(rawConfig)
		g.Expect(err).To(HaveOccurred())
		g.Expect(hermetoProxyConfig.PackageRegistryProxyAllowed).To(BeFalse())
		g.Expect(hermetoProxyConfig.NpmProxy).To(Equal(testNpmProxy))
		g.Expect(hermetoProxyConfig.YarnProxy).To(Equal(testYarnProxy))
	})

	t.Run("should create hermeto proxy config from empty values", func(t *testing.T) {
		rawConfig := config.KonfluxRawConfig{}

		hermetoProxyConfig, err := config.NewHermetoProxyConfig(rawConfig)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hermetoProxyConfig.PackageRegistryProxyAllowed).To(BeFalse())
		g.Expect(hermetoProxyConfig.NpmProxy).To(BeEmpty())
		g.Expect(hermetoProxyConfig.YarnProxy).To(BeEmpty())
	})
}

func Test_HermetoProxyConfig_DeepCopy(t *testing.T) {
	g := NewWithT(t)

	const testNpmProxy = "test-npm-proxy.io"
	const testYarnProxy = "test-yarn-proxy.io"

	t.Run("should deep copy hermeto proxy config", func(t *testing.T) {
		hermetoProxyConfig := &config.HermetoProxyConfig{
			PackageRegistryProxyAllowed: true,
			NpmProxy:                    testNpmProxy,
			YarnProxy:                   testYarnProxy,
		}

		HermetoProxyConfigCopy := hermetoProxyConfig.DeepCopy()

		hermetoProxyConfig.PackageRegistryProxyAllowed = false
		hermetoProxyConfig.NpmProxy = "npm-proxy"
		hermetoProxyConfig.YarnProxy = "yarn-proxy"

		g.Expect(HermetoProxyConfigCopy.PackageRegistryProxyAllowed).To(BeTrue())
		g.Expect(HermetoProxyConfigCopy.NpmProxy).To(Equal(testNpmProxy))
		g.Expect(HermetoProxyConfigCopy.YarnProxy).To(Equal(testYarnProxy))
	})
}

func Test_HermetoProxyConfig_ToString(t *testing.T) {
	g := NewWithT(t)

	hermetoProxyConfig := &config.HermetoProxyConfig{
		PackageRegistryProxyAllowed: true,
		NpmProxy:                    "test-npm-proxy.io",
		YarnProxy:                   "test-yarn-proxy.io",
	}

	str := hermetoProxyConfig.ToString()

	g.Expect(str).ToNot(BeEmpty())
}

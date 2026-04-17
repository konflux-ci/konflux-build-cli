package prefetch_dependencies

import (
	"fmt"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"

	. "github.com/onsi/gomega"
)

func Test_getHermetoEnvFromConfigMap(t *testing.T) {
	g := NewWithT(t)

	t.Run("can successfully read hermeto config", func(t *testing.T) {
		const fakeProxyUrl = "https://www.example.com"

		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				HermetoProxy: &config.HermetoProxyConfig{
					PackageRegistryProxyAllowed: true,
					NpmProxy:                    fakeProxyUrl,
				},
			}, nil
		}

		expectedEnv := fmt.Sprintf("HERMETO_NPM__PROXY_URL=%s", fakeProxyUrl)

		parsedConfig, err := getPackageProxyConfiguration()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedConfig).ToNot(BeNil())
		g.Expect(parsedConfig[0]).To(Equal(expectedEnv))
	})

	t.Run("returns empty env when there is an error creating config map reader", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return nil, fmt.Errorf("Fake error")
		}

		parsedConfig, err := getPackageProxyConfiguration()

		g.Expect(err).To(HaveOccurred())
		g.Expect(parsedConfig).To(BeEmpty())
	})

	t.Run("returns empty env when config map is empty", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{}, nil
		}
		parsedConfig, err := getPackageProxyConfiguration()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedConfig).To(BeEmpty())
	})

	t.Run("returns empty env when no Hermeto fields are defined", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				HermetoProxy: &config.HermetoProxyConfig{},
			}, nil
		}
		parsedConfig, err := getPackageProxyConfiguration()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedConfig).To(BeEmpty())
	})
}

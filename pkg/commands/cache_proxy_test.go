package commands

import (
	"errors"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"
	"github.com/spf13/cobra"

	. "github.com/onsi/gomega"
)

func Test_CacheProxy_Run(t *testing.T) {
	g := NewWithT(t)

	const testHttpProxy = "test.caching:1234"
	const testNoProxy = "test.no-proxy:1234"
	const defaultHttpProxy = "test-proxy.io"
	const defaultNoProxy = "no-proxy.io"

	_mockResultsWriter := &mockResultsWriter{}
	c := &CacheProxy{
		Params:        &CacheProxyParams{},
		ResultsWriter: _mockResultsWriter,
	}

	t.Run("should enable cache-proxy when allowed in config and parameter", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				CacheProxy: &config.CacheProxyConfig{
					Allowed:   true,
					HttpProxy: testHttpProxy,
					NoProxy:   testNoProxy,
				},
			}, nil
		}
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(c.Results.NoProxy).Should(Equal(testNoProxy))
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("should disable cache-proxy when allowed in config but disallowed in parameter", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				CacheProxy: &config.CacheProxyConfig{
					Allowed:   true,
					HttpProxy: testHttpProxy,
					NoProxy:   testNoProxy,
				},
			}, nil
		}
		c.Params.Enable = "false"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("should disable cache-proxy when disallowed in config but allowed in parameter", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				CacheProxy: &config.CacheProxyConfig{
					Allowed:   false,
					HttpProxy: testHttpProxy,
					NoProxy:   testNoProxy,
				},
			}, nil
		}
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("should enable cache-proxy with defaults when failed to read config but allowed in parameter", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return nil, errors.New("failed to read config")
		}
		c.Params = &CacheProxyParams{
			Enable:           "true",
			DefaultHttpProxy: defaultHttpProxy,
			DefaultNoProxy:   defaultNoProxy,
		}

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(Equal(defaultHttpProxy))
		g.Expect(c.Results.NoProxy).Should(Equal(defaultNoProxy))
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("should disable cache-proxy when failed to read config and disallowed in parameter", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return nil, errors.New("failed to read config")
		}
		c.Params = &CacheProxyParams{
			Enable:           "false",
			DefaultHttpProxy: defaultHttpProxy,
			DefaultNoProxy:   defaultNoProxy,
		}

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})
}

func Test_NewCacheProxy(t *testing.T) {
	g := NewWithT(t)
	t.Run("should create CacheProxy instance", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("enable", "", "enable or disable cache-proxy")
		cmd.Flags().String("config-file", "", "config map file path")
		parseErr := cmd.Flags().Parse([]string{
			"--enable", "true",
		})
		g.Expect(parseErr).ToNot(HaveOccurred())

		cacheProxyInstance, err := NewCacheProxy(cmd)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cacheProxyInstance.Params).ToNot(BeNil())
		g.Expect(cacheProxyInstance.ResultsWriter).ToNot(BeNil())
	})
}

package commands

import (
	"context"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func Test_CacheProxy_Run(t *testing.T) {
	g := NewWithT(t)

	testPlatformConfigIniFilePath := "../../testdata/sample-platform-config.ini"
	testHttpProxy := "test.caching:3323"
	testNamespace := "test_namespace"
	testConfigMapName := "test_name"
	defaultHttpProxy := "test-proxy.io"
	defaultNoProxy := "no-proxy.io"

	var _mockResultsWriter *mockResultsWriter
	var c *CacheProxy
	ctx := context.Background()

	clusterConfigCMWithAllowCacheTrue := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testConfigMapName,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"allow-cache-proxy": "true",
			"http-proxy":        testHttpProxy,
			"no-proxy":          "",
		},
	}

	clusterConfigCMWithAllowCacheFalse := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testConfigMapName,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"allow-cache-proxy": "false",
			"http-proxy":        "",
			"no-proxy":          "",
		},
	}

	beforeEachWithTrueInConfigMap := func() {
		_mockResultsWriter = &mockResultsWriter{}
		fakeClient := fakeclient.NewClientset()
		fakeK8sConfigMapReader := &config.K8sConfigMapReader{Name: testConfigMapName, Namespace: testNamespace, Clientset: fakeClient}

		fakeClient.CoreV1().ConfigMaps(testNamespace).Create(ctx, clusterConfigCMWithAllowCacheTrue, metav1.CreateOptions{})

		c = &CacheProxy{
			Params:        &CacheProxyParams{},
			Configs:       CacheProxyConfigs{ConfigReader: fakeK8sConfigMapReader},
			ResultsWriter: _mockResultsWriter,
		}
	}

	beforeEachWithFalseInConfigMap := func() {
		_mockResultsWriter = &mockResultsWriter{}
		fakeClient := fakeclient.NewClientset()
		fakeK8sConfigMapReader := &config.K8sConfigMapReader{Name: testConfigMapName, Namespace: testNamespace, Clientset: fakeClient}

		fakeClient.CoreV1().ConfigMaps(testNamespace).Create(ctx, clusterConfigCMWithAllowCacheFalse, metav1.CreateOptions{})

		c = &CacheProxy{
			Params:        &CacheProxyParams{},
			Configs:       CacheProxyConfigs{ConfigReader: fakeK8sConfigMapReader},
			ResultsWriter: _mockResultsWriter,
		}
	}

	beforeEachWithoutConfigMap := func() {
		_mockResultsWriter = &mockResultsWriter{}
		fakeClient := fakeclient.NewClientset()
		fakeK8sConfigMapReader := &config.K8sConfigMapReader{Name: testConfigMapName, Namespace: testNamespace, Clientset: fakeClient}

		c = &CacheProxy{
			Params: &CacheProxyParams{
				DefaultHttpProxy: defaultHttpProxy,
				DefaultNoProxy:   defaultNoProxy,
			},
			Configs:       CacheProxyConfigs{ConfigReader: fakeK8sConfigMapReader},
			ResultsWriter: _mockResultsWriter,
		}
	}

	beforeEachWithPlatformConfigFile := func() {
		_mockResultsWriter = &mockResultsWriter{}

		c = &CacheProxy{
			Params: &CacheProxyParams{},
			Configs: CacheProxyConfigs{
				ConfigReader: &config.IniFileReader{FilePath: testPlatformConfigIniFilePath},
			},
			ResultsWriter: _mockResultsWriter,
		}
	}

	t.Run("enable cache-proxy when allow-cache-proxy in the cluster ConfigMap is true", func(t *testing.T) {
		beforeEachWithTrueInConfigMap()
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(Equal(testHttpProxy))
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("enable cache-proxy when allow-cache-proxy in the cluster ConfigMap is false", func(t *testing.T) {
		beforeEachWithFalseInConfigMap()
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).Should(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("disable cache-proxy when allow-cache-proxy in the cluster ConfigMap is true", func(t *testing.T) {
		beforeEachWithTrueInConfigMap()
		c.Params.Enable = "false"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).Should(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("enable cache-proxy when cluster ConfigMap does not exists", func(t *testing.T) {
		beforeEachWithoutConfigMap()
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(Equal(defaultHttpProxy))
		g.Expect(c.Results.NoProxy).To(Equal(defaultNoProxy))
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("disable cache-proxy when cluster ConfigMap does not exists", func(t *testing.T) {
		beforeEachWithoutConfigMap()
		c.Params.Enable = "false"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).Should(BeEmpty())
		g.Expect(c.Results.NoProxy).Should(BeEmpty())
		g.Expect(isWriteResultsStringCalled).To(BeTrue())
	})

	t.Run("reading config from the platform config ini is successful", func(t *testing.T) {
		beforeEachWithPlatformConfigFile()
		c.Params.Enable = "true"

		isWriteResultsStringCalled := false
		_mockResultsWriter.WriteResultStringFunc = func(result, path string) error {
			isWriteResultsStringCalled = true
			return nil
		}

		err := c.Run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(c.Results.HttpProxy).To(Equal("testproxy.local:3128"))
		g.Expect(c.Results.NoProxy).To(Equal("test.io"))
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
		g.Expect(cacheProxyInstance.Configs).ToNot(BeNil())
		g.Expect(cacheProxyInstance.ResultsWriter).ToNot(BeNil())
	})
}

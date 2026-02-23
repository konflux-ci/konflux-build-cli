package config

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func Test_GetConfigData(t *testing.T) {
	g := NewWithT(t)

	testName := "name"
	testNamespace := "namespace"

	configMap1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"allow-cache-proxy": "true",
			"http-proxy":        "test-proxy.io",
			"no-proxy":          "test.io",
		},
	}

	t.Run("successfully retrieves config data from cluster", func(t *testing.T) {

		fakeClient := fakeclient.NewClientset()
		newK8sConfigMapReader := K8sConfigMapReader{Name: testName, Namespace: testNamespace, Clientset: fakeClient}

		ctx := context.Background()
		_, err := fakeClient.CoreV1().ConfigMaps(testNamespace).Create(ctx, configMap1, metav1.CreateOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		cacheProxyConfig, err := newK8sConfigMapReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(cacheProxyConfig.AllowCacheProxy).Should(Equal("true"))
		g.Expect(cacheProxyConfig.HttpProxy).Should(Equal("test-proxy.io"))
		g.Expect(cacheProxyConfig.NoProxy).Should(Equal("test.io"))

	})

	t.Run("successfully retrieves config data from platform config ini file", func(t *testing.T) {

		newIniFileReader := IniFileReader{FilePath: "../../testdata/sample-platform-config.ini"}

		cacheProxyConfig, err := newIniFileReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(cacheProxyConfig.AllowCacheProxy).Should(Equal("true"))
		g.Expect(cacheProxyConfig.HttpProxy).Should(Equal("testproxy.local:3128"))
		g.Expect(cacheProxyConfig.NoProxy).Should(Equal("test.io"))
	})
}

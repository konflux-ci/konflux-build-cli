package config_test

import (
	"context"
	"os"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func TestConfigReaderFactory(t *testing.T) {
	g := NewWithT(t)

	t.Run("should create ini file reader", func(t *testing.T) {
		os.Setenv("PLATFORM_CONFIG_FILE", "/path/to/file.ini")
		defer os.Unsetenv("PLATFORM_CONFIG_FILE")

		configReader, err := config.ConfigReaderFactory()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(configReader).ToNot(BeNil())
		_, isIniConfigReader := configReader.(*config.IniFileReader)
		g.Expect(isIniConfigReader).To(BeTrue())
	})
}

func TestReadConfigData(t *testing.T) {
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
		newK8sConfigMapReader := config.K8sConfigMapReader{Name: testName, Namespace: testNamespace, Clientset: fakeClient}

		ctx := context.Background()
		_, err := fakeClient.CoreV1().ConfigMaps(testNamespace).Create(ctx, configMap1, metav1.CreateOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		konfluxInfo, err := newK8sConfigMapReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(konfluxInfo.AllowCacheProxy).Should(Equal("true"))
		g.Expect(konfluxInfo.HttpProxy).Should(Equal("test-proxy.io"))
		g.Expect(konfluxInfo.NoProxy).Should(Equal("test.io"))
	})

	t.Run("should fail to retrieve config data from cluster if config map doesn't exist", func(t *testing.T) {
		fakeClient := fakeclient.NewClientset()
		newK8sConfigMapReader := config.K8sConfigMapReader{Name: testName, Namespace: testNamespace, Clientset: fakeClient}

		konfluxInfo, err := newK8sConfigMapReader.ReadConfigData()

		g.Expect(err).To(HaveOccurred())
		g.Expect(konfluxInfo).To(BeNil())
	})

	t.Run("successfully retrieves config data from platform config ini file", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "*-platform-config.ini")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		_, err = tempFile.Write([]byte("[cache-proxy]\nallow-cache-proxy=true\nhttp-proxy=testproxy.local:3128\nno-proxy=test.io"))
		g.Expect(err).ShouldNot(HaveOccurred())

		newIniFileReader := config.IniFileReader{FilePath: tempFile.Name()}
		konfluxInfo, err := newIniFileReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(konfluxInfo.AllowCacheProxy).Should(Equal("true"))
		g.Expect(konfluxInfo.HttpProxy).Should(Equal("testproxy.local:3128"))
		g.Expect(konfluxInfo.NoProxy).Should(Equal("test.io"))
	})

	t.Run("should fail if specified platform config ini file doesn't exist", func(t *testing.T) {
		newIniFileReader := config.IniFileReader{FilePath: "/path/non-existent.ini"}
		konfluxInfo, err := newIniFileReader.ReadConfigData()
		g.Expect(err).To(HaveOccurred())
		g.Expect(konfluxInfo).To(BeNil())
	})
}

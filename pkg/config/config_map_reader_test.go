package config

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func Test_GetConfigMapData(t *testing.T) {
	g := NewWithT(t)

	testName := "name"
	testNamespace := "namespace"

	configMap1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"one": "1",
			"two": "2",
		},
	}

	t.Run("successfully retrieves config map data from cluster", func(t *testing.T) {

		fakeClient := fakeclient.NewClientset()
		newK8sConfigMapReader := K8sConfigMapReader{Name: testName, Namespace: testNamespace, Clientset: fakeClient}

		ctx := context.Background()
		_, err := fakeClient.CoreV1().ConfigMaps(testNamespace).Create(ctx, configMap1, metav1.CreateOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		data, err := newK8sConfigMapReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())
	})

	t.Run("successfully retrieves config map data from YAML file", func(t *testing.T) {

		newYamlFileReader := YAMLFileReader{FilePath: "../../testdata/cluster_config_config_map.yaml"}

		data, err := newYamlFileReader.ReadConfigData()

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())
	})
}

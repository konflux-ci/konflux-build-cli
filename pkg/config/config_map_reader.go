package config

import (
	"context"
	"fmt"
	"os"

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigReader defines the interface for reading config data.
type ConfigReader interface {
	ReadConfigData() (map[string]string, error)
}

// K8sConfigMapReader reads configuration from a Kubernetes cluster.
type K8sConfigMapReader struct {
	Name      string
	Namespace string
	Clientset kubernetes.Interface
}

// YAMLFileReader reads configuration from a local YAML file.
type YAMLFileReader struct {
	FilePath string
}

// ReadConfigData reads the YAML file, unmarshals it and returns the config data from the configmap.
func (y *YAMLFileReader) ReadConfigData() (map[string]string, error) {

	data, err := os.ReadFile(y.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", y.FilePath, err)
	}

	configMap := corev1.ConfigMap{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml data: %w", err)
	}

	return configMap.Data, nil
}

// ReadConfigData fetches the ConfigMap data from the Kubernetes cluster.
func (k *K8sConfigMapReader) ReadConfigData() (map[string]string, error) {
	ctx := context.Background()
	configMap, err := k.Clientset.CoreV1().ConfigMaps(k.Namespace).Get(ctx, k.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s: %w", k.Namespace, k.Name, err)
	}
	return configMap.Data, nil
}

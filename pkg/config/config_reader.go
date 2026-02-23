package config

import (
	"context"
	"fmt"
	"os"

	"github.com/konflux-ci/konflux-build-cli/pkg/clients"
	"gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CacheProxyConfig struct {
	AllowCacheProxy string
	HttpProxy       string
	NoProxy         string
}

// ConfigReader defines the interface for reading config data.
type ConfigReader interface {
	ReadConfigData() (*CacheProxyConfig, error)
}

// K8sConfigMapReader reads configuration from a Kubernetes cluster.
type K8sConfigMapReader struct {
	Name      string
	Namespace string
	Clientset kubernetes.Interface
}

// INIFileReader reads configuration from a local INI file.
type IniFileReader struct {
	FilePath string
}

func NewConfigReader() (ConfigReader, error) {
	platformConfigFile := os.Getenv("PLATFORM_CONFIG_FILE")
	if platformConfigFile != "" {
		return &IniFileReader{FilePath: platformConfigFile}, nil
	} else {
		clientset, err := clients.NewKubeClientSet()
		if err != nil {
			return nil, err
		}
		return &K8sConfigMapReader{Name: "cluster-config", Namespace: "konflux-info", Clientset: clientset}, nil
	}
}

// ReadConfigData reads platform config from the INI file
func (y *IniFileReader) ReadConfigData() (*CacheProxyConfig, error) {
	cfg, err := ini.Load(y.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load ini file: %v", err)
	}

	newCacheProxy := &CacheProxyConfig{
		AllowCacheProxy: cfg.Section("cache-proxy").Key("allow-cache-proxy").String(),
		HttpProxy:       cfg.Section("cache-proxy").Key("http-proxy").String(),
		NoProxy:         cfg.Section("cache-proxy").Key("no-proxy").String(),
	}

	return newCacheProxy, nil
}

// ReadConfigData reads the config from the ConfigMap data of a Kubernetes cluster.
func (k *K8sConfigMapReader) ReadConfigData() (*CacheProxyConfig, error) {
	ctx := context.Background()
	configMap, err := k.Clientset.CoreV1().ConfigMaps(k.Namespace).Get(ctx, k.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s: %w", k.Namespace, k.Name, err)
	}
	newCacheProxy := &CacheProxyConfig{
		AllowCacheProxy: configMap.Data["allow-cache-proxy"],
		HttpProxy:       configMap.Data["http-proxy"],
		NoProxy:         configMap.Data["no-proxy"],
	}
	return newCacheProxy, nil
}

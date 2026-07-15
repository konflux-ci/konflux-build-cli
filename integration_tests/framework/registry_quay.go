package integration_tests_framework

import (
	"fmt"
	"net/http"
	"os"
	"path"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
	"github.com/sirupsen/logrus"
)

const (
	QuayExpiresAfterLabelName = "quay.expires-after"

	quayDockerConfigDir = "/tmp/kbc-docker-config"
)

var _ ImageRegistry = &QuayRegistry{}

type QuayRegistry struct {
	namespace string
	login     string
	password  string

	logger *logrus.Entry
}

func NewQuayRegistry() ImageRegistry {
	return &QuayRegistry{}
}

func (q *QuayRegistry) Prepare() error {
	q.logger = l.Logger.WithField("logger", "quay-registry")

	if login, err := q.getEnvVar("QUAY_ROBOT_NAME"); err != nil {
		return err
	} else {
		q.login = login
	}
	if password, err := q.getEnvVar("QUAY_ROBOT_TOKEN"); err != nil {
		return err
	} else {
		q.password = password
	}
	if namespace, err := q.getEnvVar("QUAY_NAMESPACE"); err != nil {
		return err
	} else {
		q.namespace = namespace
	}

	if err := q.configureDockerAuth(); err != nil {
		return err
	}

	return nil
}

func (q *QuayRegistry) getEnvVar(envVarName string) (string, error) {
	value := os.Getenv(envVarName)
	if value == "" {
		return "", fmt.Errorf("%s env var is not set", envVarName)
	}
	return value, nil
}

func (q *QuayRegistry) configureDockerAuth() error {
	if err := EnsureDirectory(quayDockerConfigDir); err != nil {
		return err
	}

	dockerConfigJsonPath := path.Join(quayDockerConfigDir, "config.json")
	if !FileExists(dockerConfigJsonPath) {
		// Generate docker config json
		dockerConfigJson, err := GenerateDockerAuthContent(q.GetRegistryDomain(), q.login, q.password)
		if err != nil {
			q.logger.Errorf("failed to generate dockerconfigjson: %s", err.Error())
			return err
		}
		if err := os.WriteFile(dockerConfigJsonPath, dockerConfigJson, 0644); err != nil {
			q.logger.Errorf("failed to save dockerconfigjson: %s", err.Error())
			return err
		}
	}

	os.Setenv("DOCKER_CONFIG", quayDockerConfigDir)
	return nil
}

func (q *QuayRegistry) Start() error {
	return nil
}

func (q *QuayRegistry) Stop() error {
	os.RemoveAll(quayDockerConfigDir)
	return nil
}

func (q *QuayRegistry) IsLocal() bool {
	return false
}

func (q *QuayRegistry) GetRegistryDomain() string {
	return "quay.io"
}

func (q *QuayRegistry) GetTestNamespace() string {
	return q.GetRegistryDomain() + "/" + q.namespace + "/"
}

func (q *QuayRegistry) GetCredentials() (string, string) {
	return q.login, q.password
}

func (q *QuayRegistry) GetDockerConfigJsonContent() []byte {
	content, err := GenerateDockerAuthContent(q.GetRegistryDomain(), q.login, q.password)
	if err != nil {
		panic(fmt.Sprintf("failed to create docker config json data: %s", err.Error()))
	}
	return content
}

func (q *QuayRegistry) GetCaCertPath() string {
	return ""
}

func (q *QuayRegistry) DoRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{}
	return client.Do(req)
}

package integration_tests_framework

import (
	"encoding/base64"
	"encoding/json"
)

// https://github.com/podman-container-tools/container-libs/blob/b17b0a4c55bb953de8d783ea26534df22fdc9f2b/image/manifest/manifest.go#L56
var allContainerMediaTypes = []string{
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.v1+prettyjws",
	"application/vnd.docker.distribution.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.index.v1+json",
}

type ImageRegistry interface {
	// Returns true for locally hosted registries.
	IsLocal() bool
	// Ensures all resources are ready to use the registry.
	Prepare() error
	// Starts local registry.
	Start() error
	// Stops local registry.
	Stop() error
	// Returns usename and password / token to access test namespace, see GetTestNamespace.
	GetCredentials() (string, string)
	// Returns base registry url, e.g. registry.io:1234
	GetRegistryDomain() string
	// Returns first part of the image name to which user can push test data.
	// Example: quay.io/my-org/
	GetTestNamespace() string
	// Returns config.josn content to authorize clients.
	GetDockerConfigJsonContent() []byte
	// Returns path to the root CA certificate the registry is using (in case of self-signed certificate),
	// empty string otherwise.
	GetCaCertPath() string
	// Returns true if given image exists in the test namespace of the registry.
	CheckTagExistence(imageName, tag string) (bool, error)
	// Return image index information, primarily the list of included manifests.
	GetImageIndexInfo(imageName, tag string) (*ImageIndexManifest, error)
}

type ImageIndexManifest struct {
	MediaType   string          `json:"mediaType,omitempty"`
	Manifests   []ImageManifest `json:"manifests,omitempty"`
	RawManifest []byte          `json:"-"`
}
type ImageManifest struct {
	MediaType string `json:"mediaType,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

func GenerateDockerAuthContent(registry, login, password string) ([]byte, error) {
	return GenerateDockerAuthContentWithAliases([]string{registry}, login, password)
}

func GenerateDockerAuthContentWithAliases(registries []string, login, password string) ([]byte, error) {
	type dockerConfigAuth struct {
		Auth string `json:"auth"`
	}
	type dockerConfigJson struct {
		Auths map[string]dockerConfigAuth `json:"auths"`
	}

	auth := dockerConfigAuth{Auth: base64.StdEncoding.EncodeToString([]byte(login + ":" + password))}
	auths := map[string]dockerConfigAuth{}
	for _, registry := range registries {
		auths[registry] = auth
	}
	dockerconfig := dockerConfigJson{Auths: auths}
	return json.Marshal(dockerconfig)
}

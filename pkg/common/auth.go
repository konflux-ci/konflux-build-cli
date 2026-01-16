package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	REGISTRY_DOCKER_IO       = "docker.io"
	REGISTRY_INDEX_DOCKER_IO = "https://index.docker.io/v1/"
)

type RegistryAuth struct {
	Registry string
	Token    string
}

type RegistryAuths struct {
	Auths map[string]AuthEntry `json:"auths"`
}

type AuthEntry struct {
	Auth string `json:"auth"`
}

// SelectRegistryAuth selects registry authentication credential from authentication file.
// It takes an imageRef (container image reference like "registry.io/namespace/image:tag")
// and an optional authFilePath. If authFilePath is provided and non-empty, it uses that
// authentication file; otherwise, it defaults to ~/.docker/config.json.
// Returns an object of RegistryAuth and an error.
func SelectRegistryAuth(imageRef string, authFilePath ...string) (*RegistryAuth, error) {
	var authFile string
	if len(authFilePath) > 0 && authFilePath[0] != "" {
		authFile = authFilePath[0]
	} else {
		authFile = GetDefaultAuthFile()
	}

	imageRepo := GetImageName(imageRef)
	if imageRepo == "" {
		return nil, fmt.Errorf("Invalid image reference '%s'", imageRef)
	}

	registryAuths, err := readAuthFile(authFile)
	if err != nil {
		return nil, err
	}

	token := findAuth(registryAuths, imageRepo)
	if token == "" {
		return nil, fmt.Errorf("Registry authentication is not configured for %s.", imageRepo)
	}

	return &RegistryAuth{
		Registry: strings.Split(imageRepo, "/")[0],
		Token:    token,
	}, nil
}

// findAuth finds out authentication credential and return the raw string.
// If nothing is found, returns an empty string.
func findAuth(registryAuths *RegistryAuths, imageRepo string) string {
	authKey := imageRepo
	for {
		fmt.Println(":))))", authKey)
		if authEntry, exists := registryAuths.Auths[authKey]; exists {
			return authEntry.Auth
		}
		index := strings.LastIndex(authKey, "/")
		if index < 0 {
			break
		}
		authKey = authKey[:index]
	}
	registry := strings.Split(imageRepo, "/")[0]
	if registry == REGISTRY_DOCKER_IO {
		if authEntry, exists := registryAuths.Auths[REGISTRY_INDEX_DOCKER_IO]; exists {
			return authEntry.Auth
		}
	}
	return ""
}

func GetDefaultAuthFile() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".docker", "config.json")
}

func readAuthFile(authFilePath string) (*RegistryAuths, error) {
	data, err := os.ReadFile(authFilePath)
	if err != nil {
		return nil, err
	}

	var registryAuths RegistryAuths
	err = json.Unmarshal(data, &registryAuths)
	if err != nil {
		return nil, err
	}

	return &registryAuths, nil
}

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// OrasPush pushes a file to remote registry as an OCI artifact.
func OrasPush(username, password string, imageRef registry.Reference, tag, filePath, artifactType string) (string, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("Error on getting file stat from file %s: %w", filePath, err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("Pushing a directory is not supported: %s", filePath)
	}

	fileStorePath := os.TempDir()
	fs, err := file.New(fileStorePath)
	if err != nil {
		return "", fmt.Errorf("Error on creating a file store for oras-push: %w", err)
	}
	defer fs.Close()

	ctx := context.Background()
	fileDescriptor, err := fs.Add(ctx, filepath.Base(filePath), "", filePath)
	if err != nil {
		panic(err)
	}
	fileDescriptors := []v1.Descriptor{fileDescriptor}

	opts := oras.PackManifestOptions{Layers: fileDescriptors}
	manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifactType, opts)
	if err != nil {
		return "", fmt.Errorf("Error on creating manifest: %w", err)
	}
	fmt.Println("manifest descriptor:", manifestDescriptor)

	if err = fs.Tag(ctx, manifestDescriptor, tag); err != nil {
		return "", fmt.Errorf("Error on tagging manifest: %w", err)
	}

	repo := &remote.Repository{
		Reference: imageRef,
		Client: &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(imageRef.Registry, auth.Credential{
				Username: username,
				Password: password,
			}),
		},
	}
	descriptor, err := oras.Copy(ctx, fs, tag, repo, tag, oras.DefaultCopyOptions)
	if err != nil {
		return "", fmt.Errorf("Error on copying image to registry: %w", err)
	}

	return string(descriptor.Digest), nil
}

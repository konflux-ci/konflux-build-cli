package common

import (
	"fmt"
	"os"
	"path/filepath"
)

type DockerfileSearchOpts struct {
	// Source directory path containing application source code.
	SourceDir string
	// Build context directory within the source. It defaults to ".".
	ContextDir string
	// Dockerfile within the source. If not specified, it is searched in order
	// of ./Containerfile and ./Dockerfile. Containerfile takes precedence.
	Dockerfile string
}

// SearchDockerfile searches for a Dockerfile under the given source directory.
//
// Search for the Dockerfile under source/context/ first, then under source/.
// If Dockerfile is not specified, search ./Containerfile then ./Dockerfile.
//
// Note that the result path is not guaranteed to be a subpath of the source directory.
// If that is important, check with [RealPath.IsRelativeTo].
//
// Return an empty string if nothing is found.
func SearchDockerfile(opts DockerfileSearchOpts) (string, error) {
	if opts.SourceDir == "" {
		return "", fmt.Errorf("missing source directory")
	}
	contextDir := opts.ContextDir
	if contextDir == "" {
		contextDir = "."
	}

	var _search = func(dockerfile string) (string, error) {
		possibleDockerfiles := []string{
			filepath.Join(opts.SourceDir, contextDir, dockerfile),
			filepath.Join(opts.SourceDir, dockerfile),
		}
		for _, dockerfilePath := range possibleDockerfiles {
			if _, err := os.Stat(dockerfilePath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return "", fmt.Errorf("checking dockerfile existence: %w", err)
			}
			return dockerfilePath, nil
		}
		// No Dockerfile is found.
		return "", nil
	}

	if opts.Dockerfile == "" {
		for _, dockerfile := range []string{"./Containerfile", "./Dockerfile"} {
			dockerfilePath, err := _search(dockerfile)
			if err != nil {
				return "", err
			}
			if dockerfilePath != "" {
				return dockerfilePath, nil
			}
		}
		// Tried all. Nothing is found.
		return "", nil
	}

	return _search(opts.Dockerfile)
}

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

// SearchDockerfile searches Dockerfile from given source directory.
//
// Dockerfile must be present under the source and possibly the specified build context directory.
// Caller is responsible for determining the source directory is a relative or absolute path.
// SearchDockerfile does not make assumption on it and search just happens under the passed source directory.
//
// Escape from the source directory is checked. If the source itself is a symbolic link,
// SearchDockerfile does not treat it as an error.
//
// If Dockerfile option is not specified, SearchDockerfile searches ./Containerfile by default,
// then the ./Dockerfile if Containerfile is not found.
//
// Returning empty string to indicate neither is found.
func SearchDockerfile(opts DockerfileSearchOpts) (string, error) {
	if opts.SourceDir == "" {
		return "", fmt.Errorf("missing source directory")
	}
	contextDir := opts.ContextDir
	if contextDir == "" {
		contextDir = "."
	}

	sourceDir, err := ResolvePath(opts.SourceDir)
	if err != nil {
		return "", fmt.Errorf("resolving source dir: %w", err)
	}

	var _search = func(dockerfile string) (string, error) {
		possibleDockerfiles := []string{
			filepath.Join(opts.SourceDir, contextDir, dockerfile),
			filepath.Join(opts.SourceDir, dockerfile),
		}
		for _, dockerfilePath := range possibleDockerfiles {
			resolvedPath, err := ResolvePath(dockerfilePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return "", fmt.Errorf("resolving dockerfile path: %w", err)
			}
			if !resolvedPath.IsRelativeTo(sourceDir) {
				return "", fmt.Errorf("Dockerfile %s is not present under source '%s'", dockerfile, sourceDir) //nolint:staticcheck // ST1005: "Dockerfile" is a proper name
			}
			return resolvedPath.String(), nil
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

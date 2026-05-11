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

// Like filepath.Join, but absolute path elements replace the preceding elements.
//
// Example:
// joinOrReplace("/abs1", "rel1", "/abs2", "rel2") => /abs2/rel2
func joinOrReplace(pathElem ...string) string {
	var actualPathElems []string
	for _, elem := range pathElem {
		if filepath.IsAbs(elem) {
			actualPathElems = actualPathElems[:0]
		}
		actualPathElems = append(actualPathElems, elem)
	}
	return filepath.Join(actualPathElems...)
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
			joinOrReplace(opts.SourceDir, contextDir, dockerfile),
			joinOrReplace(opts.SourceDir, dockerfile),
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

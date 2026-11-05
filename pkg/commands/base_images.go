package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/opencontainers/go-digest"
)

// These structures match the Tekton artifacts file format without a Tekton dependency.
type baseImageArtifact struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type baseImageArtifacts struct {
	Inputs []baseImageArtifactGroup `json:"inputs"`
}

type baseImageArtifactGroup struct {
	Name   string              `json:"name"`
	Values []baseImageArtifact `json:"values"`
}

func (c *Build) validateBaseImageOutputPaths() error {
	paths := []string{c.Params.ResolvedBaseImagesOutput, c.Params.BaseImagesJSONOutput}
	seen := make(map[string]bool)
	var files []os.FileInfo
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("base image output path: %w", err)
		}
		directory, err := filepath.EvalSymlinks(filepath.Dir(absolute))
		if err != nil {
			return fmt.Errorf("base image output directory for %s: %w", path, err)
		}
		absolute = filepath.Join(directory, filepath.Base(absolute))
		if seen[absolute] {
			return fmt.Errorf("duplicate base image output path: %s", path)
		}
		seen[absolute] = true
		info, err := os.Lstat(absolute)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("checking base image output %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("base image output %s must be a regular file", path)
		}
		for _, previous := range files {
			if os.SameFile(info, previous) {
				return fmt.Errorf("duplicate base image output file: %s", path)
			}
		}
		files = append(files, info)
	}
	return nil
}

// writeBaseImageOutputs resolves the collected list once for all output formats.
// The inventory lookup does not record Buildah's executed inputs.
func (c *Build) writeBaseImageOutputs(pulledImages []BaseImage) error {
	if err := c.validateBaseImageOutputPaths(); err != nil {
		return err
	}
	p := c.Params
	if p.ResolvedBaseImagesOutput == "" && p.BaseImagesJSONOutput == "" {
		return nil
	}
	resolved, err := c.resolveBaseImages(pulledImages)
	if err != nil {
		return fmt.Errorf("determining resolved base images: %w", err)
	}

	if len(resolved) != len(pulledImages) {
		return fmt.Errorf("resolved base image count does not match input count")
	}

	type output struct{ path, value string }
	var outputs []output
	var legacy strings.Builder
	for i := 0; i < len(resolved) && i < len(pulledImages); i++ {
		fmt.Fprintf(&legacy, "%s %s\n", pulledImages[i].Ref, resolved[i].Ref)
	}
	outputs = append(outputs, output{p.ResolvedBaseImagesOutput, legacy.String()})
	if p.BaseImagesJSONOutput != "" {
		values := make([]baseImageArtifact, 0, len(resolved))
		for _, image := range resolved {
			artifact, err := baseImageArtifactFromReference(image.Ref)
			if err != nil {
				return err
			}
			values = append(values, artifact)
		}
		data, err := json.Marshal(baseImageArtifacts{Inputs: []baseImageArtifactGroup{{Name: "base-images", Values: values}}})
		if err != nil {
			return fmt.Errorf("encoding base image artifacts: %w", err)
		}
		outputs = append(outputs, output{p.BaseImagesJSONOutput, string(data)})
	}
	// Validate and encode all inputs before writing either output.
	for _, out := range outputs {
		if out.path != "" {
			if err := os.WriteFile(out.path, []byte(out.value), 0644); err != nil {
				return fmt.Errorf("writing resolved base images to %s: %w", out.path, err)
			}
		}
	}
	return nil
}

func baseImageArtifactFromReference(ref string) (baseImageArtifact, error) {
	named, err := reference.ParseNamed(ref)
	if err != nil {
		return baseImageArtifact{}, fmt.Errorf("parsing resolved base image %q: %w", ref, err)
	}
	canonical, ok := named.(reference.Canonical)
	if !ok {
		return baseImageArtifact{}, fmt.Errorf("resolved base image %q has no digest", ref)
	}
	d := canonical.Digest()
	if err := d.Validate(); err != nil {
		return baseImageArtifact{}, fmt.Errorf("resolved base image digest: %w", err)
	}
	if d.Algorithm() != digest.SHA256 {
		return baseImageArtifact{}, fmt.Errorf("resolved base image %q must use sha256", ref)
	}
	path := "/" + reference.Path(named)
	if tagged, ok := named.(reference.Tagged); ok {
		path += ":" + tagged.Tag()
	}
	uri := url.URL{Scheme: "oci", Host: strings.ToLower(reference.Domain(named)), Path: path}
	return baseImageArtifact{URI: uri.String(), Digest: map[string]string{d.Algorithm().String(): d.Encoded()}}, nil
}

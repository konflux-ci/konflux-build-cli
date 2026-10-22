package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/spf13/cobra"
)

func TestBaseImageOutputFlags(t *testing.T) {
	cmd := &cobra.Command{}
	config := map[string]common.Parameter{"resolved-base-images-json-output": BuildParamsConfig["resolved-base-images-json-output"]}
	common.RegisterParameters(cmd, config)
	if err := cmd.ParseFlags([]string{"--resolved-base-images-json-output", "images.json"}); err != nil {
		t.Fatal(err)
	}
	var p BuildParams
	if err := common.ParseParameters(cmd, config, &p); err != nil {
		t.Fatal(err)
	}
	if p.BaseImagesJSONOutput != "images.json" {
		t.Fatalf("unexpected parameters: %+v", p)
	}
}

func TestBaseImageOutputs(t *testing.T) {
	d := "sha256:" + strings.Repeat("a", 64)
	for _, count := range []int{0, 1, 6} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			p := baseImageTestPaths(t)
			var images []BaseImage
			var legacy strings.Builder
			for i := range count {
				ref := fmt.Sprintf("registry.example.com/base%d:Release@%s", i, d)
				images = append(images, BaseImage{Ref: ref})
				fmt.Fprintf(&legacy, "%s %s\n", ref, ref)
			}
			if err := (&Build{Params: p}).writeBaseImageOutputs(images); err != nil {
				t.Fatal(err)
			}
			if got := readBaseImageOutput(t, p.ResolvedBaseImagesOutput); got != legacy.String() {
				t.Fatalf("legacy = %q", got)
			}
			var artifacts baseImageArtifacts
			data := readBaseImageOutput(t, p.BaseImagesJSONOutput)
			if err := json.Unmarshal([]byte(data), &artifacts); err != nil {
				t.Fatal(err)
			}
			if len(artifacts.Inputs) != 1 || artifacts.Inputs[0].Name != "base-images" || len(artifacts.Inputs[0].Values) != count {
				t.Fatalf("unexpected artifacts: %s", data)
			}
			if count == 0 && !strings.Contains(data, `"values":[]`) {
				t.Fatalf("empty inputs must be an array: %s", data)
			}
			for i, value := range artifacts.Inputs[0].Values {
				uri, _, _ := strings.Cut(images[i].Ref, "@")
				if value.URI != "oci://"+uri || value.Digest["sha256"] != strings.Repeat("a", 64) || len(value.Digest) != 1 {
					t.Fatalf("unexpected value: %+v", value)
				}
			}
		})
	}
}

func TestBaseImageOutputsResolveOnce(t *testing.T) {
	p := baseImageTestPaths(t)
	calls := 0
	d := "sha256:" + strings.Repeat("b", 64)
	c := &Build{Params: p, CliWrappers: BuildCliWrappers{BuildahCli: &mockBuildahCli{
		ImagesJsonFunc: func(*cliwrappers.BuildahImagesArgs) ([]cliwrappers.BuildahImagesEntry, error) {
			calls++
			return []cliwrappers.BuildahImagesEntry{{Names: []string{"registry.example.com/base:latest"}, Digest: d}}, nil
		},
	}}}
	if err := c.writeBaseImageOutputs([]BaseImage{{Ref: "base:latest"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("resolved %d times, want once", calls)
	}
	if got := readBaseImageOutput(t, p.ResolvedBaseImagesOutput); got != "base:latest registry.example.com/base:latest@"+d+"\n" {
		t.Fatalf("legacy text changed: %q", got)
	}
}

func TestBaseImageOutputValidation(t *testing.T) {
	d := "sha256:" + strings.Repeat("a", 64)
	for _, tt := range []struct {
		name string
		refs []string
		edit func(*BuildParams)
	}{
		{name: "duplicate paths", edit: func(p *BuildParams) { p.BaseImagesJSONOutput = p.ResolvedBaseImagesOutput }},
		{name: "malformed reference", refs: []string{"INVALID/path"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := baseImageTestPaths(t)
			if tt.edit != nil {
				tt.edit(p)
			}
			var images []BaseImage
			for _, name := range tt.refs {
				images = append(images, BaseImage{Ref: "registry.example.com/" + name + "@" + d})
			}
			c := &Build{Params: p}
			if err := c.writeBaseImageOutputs(images); err == nil {
				t.Fatal("expected failure")
			}
			entries, err := os.ReadDir(filepath.Dir(p.ResolvedBaseImagesOutput))
			if err != nil || len(entries) != 0 {
				t.Fatalf("validation wrote outputs: %v (%v)", entries, err)
			}
		})
	}
	for _, ref := range []string{
		"base:latest@" + d,
		"registry.example.com/base",
		"registry.example.com/base@sha256:" + strings.Repeat("A", 64),
		"registry.example.com/base@sha256:abcd",
		"registry.example.com/base@sha512:" + strings.Repeat("a", 128),
	} {
		if _, err := baseImageArtifactFromReference(ref); err == nil {
			t.Errorf("accepted invalid artifact reference %q", ref)
		}
	}
	artifact, err := baseImageArtifactFromReference("REGISTRY.EXAMPLE.COM:5000/base:Release@" + d)
	if err != nil || artifact.URI != "oci://registry.example.com:5000/base:Release" {
		t.Fatalf("authority normalization: %+v (%v)", artifact, err)
	}
}

func TestBaseImageOutputWriteFailure(t *testing.T) {
	p := baseImageTestPaths(t)
	if err := os.Mkdir(p.BaseImagesJSONOutput, 0700); err != nil {
		t.Fatal(err)
	}
	c := &Build{Params: p}
	if err := c.writeBaseImageOutputs([]BaseImage{{Ref: "registry.example.com/base@sha256:" + strings.Repeat("a", 64)}}); err == nil {
		t.Fatal("expected write failure")
	}
}

func TestBaseImageInventoryValidation(t *testing.T) {
	d := "sha256:" + strings.Repeat("a", 64)
	for _, tt := range []struct {
		name    string
		entries []cliwrappers.BuildahImagesEntry
	}{
		{name: "no images"},
		{name: "unnamed image", entries: []cliwrappers.BuildahImagesEntry{{Digest: d}}},
		{name: "ambiguous images", entries: []cliwrappers.BuildahImagesEntry{
			{Names: []string{"registry.example.com/a"}, Digest: d},
			{Names: []string{"registry.example.com/b"}, Digest: d},
		}},
		{name: "invalid digest", entries: []cliwrappers.BuildahImagesEntry{{Names: []string{"registry.example.com/a"}, Digest: "sha256:bad"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &Build{Params: baseImageTestPaths(t), CliWrappers: BuildCliWrappers{BuildahCli: &mockBuildahCli{
				ImagesJsonFunc: func(*cliwrappers.BuildahImagesArgs) ([]cliwrappers.BuildahImagesEntry, error) {
					return tt.entries, nil
				},
			}}}
			if err := c.writeBaseImageOutputs([]BaseImage{{Ref: "base:latest"}}); err == nil {
				t.Fatal("accepted invalid inventory")
			}
		})
	}
	c := &Build{Params: baseImageTestPaths(t), CliWrappers: BuildCliWrappers{BuildahCli: &mockBuildahCli{
		ImagesJsonFunc: func(*cliwrappers.BuildahImagesArgs) ([]cliwrappers.BuildahImagesEntry, error) {
			return []cliwrappers.BuildahImagesEntry{{Names: []string{"other.example.com/alias:latest"}, Digest: d}}, nil
		},
	}}}
	if err := c.writeBaseImageOutputs([]BaseImage{{Ref: "registry.example.com/base:latest"}}); err != nil {
		t.Fatal(err)
	}
	if got := readBaseImageOutput(t, c.Params.ResolvedBaseImagesOutput); got != "registry.example.com/base:latest registry.example.com/base:latest@"+d+"\n" {
		t.Fatalf("qualified input replaced by stored alias: %q", got)
	}
}

func TestBaseImageJSONWithoutSlots(t *testing.T) {
	p := baseImageTestPaths(t)
	var images []BaseImage
	for i := range 6 {
		images = append(images, BaseImage{Ref: fmt.Sprintf("registry.example.com/base%d@sha256:%s", i, strings.Repeat("a", 64))})
	}
	if err := (&Build{Params: p}).writeBaseImageOutputs(images); err != nil {
		t.Fatalf("JSON output must not impose an artifact slot limit: %v", err)
	}
}

func baseImageTestPaths(t *testing.T) *BuildParams {
	t.Helper()
	dir := t.TempDir()
	return &BuildParams{ResolvedBaseImagesOutput: filepath.Join(dir, "legacy"), BaseImagesJSONOutput: filepath.Join(dir, "images.json")}
}

func readBaseImageOutput(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBaseImageOutputsWithBuildArgs(t *testing.T) {
	for _, tt := range []struct {
		name, fileValue, cliValue, want string
	}{
		{"Containerfile default", "", "", "registry.example.com/default:latest"},
		{"CLI override", "", "registry.example.com/cli:latest", "registry.example.com/cli:latest"},
		{"file override", "registry.example.com/file:latest", "", "registry.example.com/file:latest"},
		{"CLI overrides file", "registry.example.com/file:latest", "registry.example.com/cli:latest", "registry.example.com/cli:latest"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			containerfile := filepath.Join(dir, "Containerfile")
			content := "ARG BASE=registry.example.com/default:latest\nFROM ${BASE} AS builder\nFROM builder AS final\n"
			if err := os.WriteFile(containerfile, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			params := &BuildParams{SkipUnusedStages: true}
			if tt.fileValue != "" {
				file := filepath.Join(dir, "build-args")
				if err := os.WriteFile(file, []byte("BASE="+tt.fileValue+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				params.BuildArgsFiles = []string{file}
			}
			if tt.cliValue != "" {
				params.BuildArgs = []string{"BASE=" + tt.cliValue}
			}
			digest := "sha256:" + strings.Repeat("a", 64)
			build := &Build{
				containerfilePath: containerfile,
				Params:            params,
				CliWrappers: BuildCliWrappers{BuildahCli: &mockBuildahCli{
					ImagesJsonFunc: func(args *cliwrappers.BuildahImagesArgs) ([]cliwrappers.BuildahImagesEntry, error) {
						if args.Image != tt.want {
							t.Fatalf("resolving %q, want %q", args.Image, tt.want)
						}
						return []cliwrappers.BuildahImagesEntry{{Names: []string{tt.want}, Digest: digest}}, nil
					},
				}},
			}
			parsed, err := build.parseContainerfile()
			if err != nil {
				t.Fatal(err)
			}
			images, err := build.collectBaseImages(parsed, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(images) != 1 || images[0].Ref != tt.want {
				t.Fatalf("collected %v, want %q", images, tt.want)
			}
			params.BaseImagesJSONOutput = filepath.Join(dir, "artifacts.json")
			if err := build.writeBaseImageOutputs(images); err != nil {
				t.Fatal(err)
			}
			data := []byte(readBaseImageOutput(t, params.BaseImagesJSONOutput))
			var result baseImageArtifacts
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			value := result.Inputs[0].Values[0]
			if value.URI != "oci://"+tt.want || value.Digest["sha256"] != strings.TrimPrefix(digest, "sha256:") {
				t.Fatalf("unexpected artifacts: %s", data)
			}

		})
	}
}

func TestBaseImageOutputsReplacePreviousInputs(t *testing.T) {
	p := baseImageTestPaths(t)
	c := &Build{Params: p}
	ref := "registry.example.com/base@sha256:" + strings.Repeat("a", 64)
	if err := c.writeBaseImageOutputs([]BaseImage{{Ref: ref}, {Ref: ref}}); err != nil {
		t.Fatal(err)
	}
	if err := c.writeBaseImageOutputs(nil); err != nil {
		t.Fatal(err)
	}
	if got := readBaseImageOutput(t, p.BaseImagesJSONOutput); got != `{"inputs":[{"name":"base-images","values":[]}]}` {
		t.Fatalf("stale inputs: %s", got)
	}
}

func TestBaseImageOutputsRejectAliases(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(*BuildParams) error
	}{
		{"symlink", func(p *BuildParams) error {
			return os.Symlink(p.ResolvedBaseImagesOutput, p.BaseImagesJSONOutput)
		}},
		{"dangling symlink", func(p *BuildParams) error {
			return os.Symlink(filepath.Join(filepath.Dir(p.ResolvedBaseImagesOutput), "missing"), p.BaseImagesJSONOutput)
		}},
		{"hard link", func(p *BuildParams) error { return os.Link(p.ResolvedBaseImagesOutput, p.BaseImagesJSONOutput) }},
		{"parent directory alias", func(p *BuildParams) error {
			dir := filepath.Dir(p.ResolvedBaseImagesOutput)
			alias := filepath.Join(dir, "alias")
			if err := os.Symlink(dir, alias); err != nil {
				return err
			}
			p.BaseImagesJSONOutput = filepath.Join(alias, filepath.Base(p.ResolvedBaseImagesOutput))
			return nil
		}},
		{"directory", func(p *BuildParams) error { return os.Mkdir(p.BaseImagesJSONOutput, 0700) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := baseImageTestPaths(t)
			if err := os.WriteFile(p.ResolvedBaseImagesOutput, []byte("previous\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := tt.setup(p); err != nil {
				t.Fatal(err)
			}
			ref := "registry.example.com/base@sha256:" + strings.Repeat("a", 64)
			if err := (&Build{Params: p}).writeBaseImageOutputs([]BaseImage{{Ref: ref}}); err == nil {
				t.Fatal("expected output path rejection")
			}
			if got := readBaseImageOutput(t, p.ResolvedBaseImagesOutput); got != "previous\n" {
				t.Fatalf("validation changed existing output: %q", got)
			}
		})
	}
}

func TestBaseImageOutputsAllowSymlinkedDirectory(t *testing.T) {
	p := baseImageTestPaths(t)
	dir := filepath.Dir(p.ResolvedBaseImagesOutput)
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	p.BaseImagesJSONOutput = filepath.Join(alias, "images.json")
	ref := "registry.example.com/base@sha256:" + strings.Repeat("a", 64)
	if err := (&Build{Params: p}).writeBaseImageOutputs([]BaseImage{{Ref: ref}}); err != nil {
		t.Fatal(err)
	}
	if got := readBaseImageOutput(t, p.ResolvedBaseImagesOutput); got != ref+" "+ref+"\n" {
		t.Fatalf("legacy output = %q", got)
	}
}

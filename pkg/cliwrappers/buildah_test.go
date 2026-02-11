package cliwrappers_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

func setupBuildahCli() (*cliwrappers.BuildahCli, *mockExecutor) {
	executor := &mockExecutor{}
	buildahCli := &cliwrappers.BuildahCli{Executor: executor}
	return buildahCli, executor
}

func ensureRetryerDisabled(t *testing.T) {
	retryerDisabled := cliwrappers.DisableRetryer
	if !retryerDisabled {
		cliwrappers.DisableRetryer = true
		t.Cleanup(func() { cliwrappers.DisableRetryer = false })
	}
}

func TestBuildahCli_Build(t *testing.T) {
	g := NewWithT(t)

	const containerfile = "/path/to/Containerfile"
	const contextDir = "/path/to/context"
	const outputRef = "quay.io/org/image:tag"

	t.Run("should execute buildah correctly", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
		}

		err := buildahCli.Build(buildArgs)

		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(capturedArgs[0]).To(Equal("build"))
		expectArgAndValue(g, capturedArgs, "--file", containerfile)
		expectArgAndValue(g, capturedArgs, "--tag", outputRef)
		g.Expect(capturedArgs[len(capturedArgs)-1]).To(Equal(contextDir))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to execute buildah build")
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
		}

		err := buildahCli.Build(buildArgs)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to execute buildah build"))
	})

	t.Run("should error if args are invalid", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: "",
			ContextDir:    contextDir,
			OutputRef:     outputRef,
		}
		err := buildahCli.Build(buildArgs)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("validating buildah args: containerfile path is empty"))
	})

	t.Run("should turn Secrets into --secret params", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			Secrets: []cliwrappers.BuildahSecret{
				{Src: "/some/file", Id: "mysecret_1"},
				{Src: "/other/file", Id: "mysecret_2"},
			},
		}

		err := buildahCli.Build(buildArgs)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(capturedArgs).To(ContainElement("--secret=src=/some/file,id=mysecret_1"))
		g.Expect(capturedArgs).To(ContainElement("--secret=src=/other/file,id=mysecret_2"))
	})

	t.Run("should turn Volumes into --volume params", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			Volumes: []cliwrappers.BuildahVolume{
				{HostDir: "/host/dir1", ContainerDir: "/container/dir1", Options: ""},
				{HostDir: "/host/dir2", ContainerDir: "/container/dir2", Options: "ro"},
			},
		}

		err := buildahCli.Build(buildArgs)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(capturedArgs).To(ContainElement("--volume=/host/dir1:/container/dir1"))
		g.Expect(capturedArgs).To(ContainElement("--volume=/host/dir2:/container/dir2:ro"))
	})

	t.Run("should turn BuildArgs(File) into --build-arg(-file) params", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			BuildArgs:     []string{"VERSION=1.0.0", "BUILD_DATE=2024-01-01"},
			BuildArgsFile: "/path/to/build-args-file",
		}

		err := buildahCli.Build(buildArgs)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(capturedArgs).To(ContainElement("--build-arg=VERSION=1.0.0"))
		g.Expect(capturedArgs).To(ContainElement("--build-arg=BUILD_DATE=2024-01-01"))
		g.Expect(capturedArgs).To(ContainElement("--build-arg-file=/path/to/build-args-file"))
	})

	t.Run("should turn Envs into --env params", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			Envs:          []string{"FOO=bar", "BAZ=qux"},
		}

		err := buildahCli.Build(buildArgs)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(capturedArgs).To(ContainElement("--env=FOO=bar"))
		g.Expect(capturedArgs).To(ContainElement("--env=BAZ=qux"))
	})

	t.Run("should append extra args before context directory", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		buildArgs := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			ExtraArgs:     []string{"--compat-volumes", "--force-rm"},
		}

		err := buildahCli.Build(buildArgs)

		g.Expect(err).ToNot(HaveOccurred())

		// Verify the command structure
		g.Expect(capturedArgs[0]).To(Equal("build"))
		expectArgAndValue(g, capturedArgs, "--file", containerfile)
		expectArgAndValue(g, capturedArgs, "--tag", outputRef)
		// Extra args should be present
		g.Expect(capturedArgs).To(ContainElement("--compat-volumes"))
		g.Expect(capturedArgs).To(ContainElement("--force-rm"))
		// Context directory should be the last argument
		g.Expect(capturedArgs[len(capturedArgs)-1]).To(Equal(contextDir))
	})
}

func findDigestFile(args []string) string {
	for i, arg := range args {
		if arg == "--digestfile" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestBuildahCli_Push(t *testing.T) {
	g := NewWithT(t)

	const image = "quay.io/org/image:tag"
	const digest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	ensureRetryerDisabled(t)

	mockSuccessfulPush := func(captureArgs *[]string) func(command string, args ...string) (string, string, int, error) {
		return func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			*captureArgs = args

			digestFile := findDigestFile(args)
			g.Expect(digestFile).ToNot(BeEmpty())

			os.WriteFile(digestFile, []byte(digest), 0644)

			return "", "", 0, nil
		}
	}

	t.Run("should push image", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulPush(&capturedArgs)

		pushArgs := &cliwrappers.BuildahPushArgs{
			Image: image,
		}

		returnedDigest, err := buildahCli.Push(pushArgs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs[0]).To(Equal("push"))
		g.Expect(capturedArgs[len(capturedArgs)-1]).To(Equal(image))

		g.Expect(returnedDigest).To(Equal(digest))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to execute buildah push")
		}

		pushArgs := &cliwrappers.BuildahPushArgs{
			Image: image,
		}

		_, err := buildahCli.Push(pushArgs)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to execute buildah push"))
	})

	t.Run("should error if image is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		pushArgs := &cliwrappers.BuildahPushArgs{
			Image: "",
		}
		_, err := buildahCli.Push(pushArgs)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("image arg is empty"))
	})

	t.Run("should clean up digest file after push", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulPush(&capturedArgs)

		pushArgs := &cliwrappers.BuildahPushArgs{
			Image: image,
		}

		_, err := buildahCli.Push(pushArgs)

		g.Expect(err).ToNot(HaveOccurred())

		digestFile := findDigestFile(capturedArgs)
		g.Expect(digestFile).ToNot(BeEmpty())

		// Verify the digest file was cleaned up
		_, statErr := os.Stat(digestFile)
		g.Expect(os.IsNotExist(statErr)).To(BeTrue(), "digest file should be cleaned up")
	})

	t.Run("should handle digest with whitespace", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		digestWithWhitespace := "\n  " + digest + "  \n"
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			digestFile := findDigestFile(args)
			os.WriteFile(digestFile, []byte(digestWithWhitespace), 0644)
			return "", "", 0, nil
		}

		pushArgs := &cliwrappers.BuildahPushArgs{
			Image: image,
		}

		returnedDigest, err := buildahCli.Push(pushArgs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(returnedDigest).To(Equal(digest), "digest should be trimmed")
	})

	t.Run("should include destination when provided", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		const destination = "docker://quay.io/other-org/other-image:tag"
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulPush(&capturedArgs)

		pushArgs := &cliwrappers.BuildahPushArgs{
			Image:       image,
			Destination: destination,
		}

		_, err := buildahCli.Push(pushArgs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs[0]).To(Equal("push"))
		g.Expect(capturedArgs[len(capturedArgs)-2]).To(Equal(image))
		g.Expect(capturedArgs[len(capturedArgs)-1]).To(Equal(destination))
	})
}

func TestBuildahBuildArgs_MakePathsAbsolute(t *testing.T) {
	g := NewWithT(t)

	t.Run("should not modify absolute paths", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "/absolute/path/Containerfile",
			ContextDir:    "/absolute/path/context",
			Secrets: []cliwrappers.BuildahSecret{
				{Src: "/absolute/path/secret", Id: "secret1"},
			},
			Volumes: []cliwrappers.BuildahVolume{
				{HostDir: "/absolute/path/volume", ContainerDir: "/container/dir", Options: ""},
			},
			BuildArgsFile: "/absolute/path/build-args-file",
		}

		err := args.MakePathsAbsolute("/base/dir")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(args.Containerfile).To(Equal("/absolute/path/Containerfile"))
		g.Expect(args.ContextDir).To(Equal("/absolute/path/context"))
		g.Expect(args.Secrets[0].Src).To(Equal("/absolute/path/secret"))
		g.Expect(args.Volumes[0].HostDir).To(Equal("/absolute/path/volume"))
		g.Expect(args.BuildArgsFile).To(Equal("/absolute/path/build-args-file"))
	})

	t.Run("should make relative paths absolute", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "relative/Containerfile",
			ContextDir:    ".",
			Secrets: []cliwrappers.BuildahSecret{
				{Src: "relative/secret", Id: "secret1"},
			},
			Volumes: []cliwrappers.BuildahVolume{
				{HostDir: "relative/volume", ContainerDir: "/container/dir", Options: ""},
			},
			BuildArgsFile: "relative/build-args-file",
		}

		err := args.MakePathsAbsolute("/base/dir")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(args.Containerfile).To(Equal("/base/dir/relative/Containerfile"))
		g.Expect(args.ContextDir).To(Equal("/base/dir"))
		g.Expect(args.Secrets[0].Src).To(Equal("/base/dir/relative/secret"))
		g.Expect(args.Volumes[0].HostDir).To(Equal("/base/dir/relative/volume"))
		g.Expect(args.BuildArgsFile).To(Equal("/base/dir/relative/build-args-file"))
	})

	t.Run("should handle a mix of relative and absolute paths", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "/path/to/Containerfile",
			ContextDir:    "context",
			Secrets: []cliwrappers.BuildahSecret{
				{Src: "secret1/file", Id: "secret1"},
				{Src: "/absolute/secret2/file", Id: "secret2"},
			},
			Volumes: []cliwrappers.BuildahVolume{
				{HostDir: "volume1/dir", ContainerDir: "/container/dir1", Options: ""},
				{HostDir: "/absolute/volume2/dir", ContainerDir: "/container/dir2", Options: "ro"},
			},
		}

		err := args.MakePathsAbsolute("/base/dir")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(args.Containerfile).To(Equal("/path/to/Containerfile"))
		g.Expect(args.ContextDir).To(Equal("/base/dir/context"))
		g.Expect(args.Secrets[0].Src).To(Equal("/base/dir/secret1/file"))
		g.Expect(args.Secrets[1].Src).To(Equal("/absolute/secret2/file"))
		g.Expect(args.Volumes[0].HostDir).To(Equal("/base/dir/volume1/dir"))
		g.Expect(args.Volumes[1].HostDir).To(Equal("/absolute/volume2/dir"))
	})

	t.Run("should use current working directory when baseDir is relative", func(t *testing.T) {
		cwd, err := os.Getwd()
		g.Expect(err).ToNot(HaveOccurred())

		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "Containerfile",
			ContextDir:    "context",
		}

		err = args.MakePathsAbsolute(".")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(args.Containerfile).To(Equal(filepath.Join(cwd, "Containerfile")))
		g.Expect(args.ContextDir).To(Equal(filepath.Join(cwd, "context")))
	})

	t.Run("should not modify empty BuildArgsFile", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "/absolute/path/Containerfile",
			ContextDir:    "/absolute/path/context",
			BuildArgsFile: "",
		}

		err := args.MakePathsAbsolute("/base/dir")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(args.BuildArgsFile).To(Equal(""))
	})
}

func TestBuildahBuildArgs_Validate(t *testing.T) {
	g := NewWithT(t)

	const containerfile = "/path/to/Containerfile"
	const contextDir = "/path/to/context"
	const outputRef = "quay.io/org/image:tag"

	t.Run("should succeed with valid args", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
		}

		err := args.Validate()
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should error if containerfile is empty", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: "",
			ContextDir:    contextDir,
			OutputRef:     outputRef,
		}

		err := args.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("containerfile path is empty"))
	})

	t.Run("should error if context directory is empty", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    "",
			OutputRef:     outputRef,
		}

		err := args.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("context directory is empty"))
	})

	t.Run("should error if output-ref is empty", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     "",
		}

		err := args.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("output-ref is empty"))
	})

	t.Run("should error when volume HostDir contains ':'", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			Volumes:       []cliwrappers.BuildahVolume{{HostDir: "some:dir", ContainerDir: "/foo"}},
		}

		err := args.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("':' in volume mount source path: some:dir"))
	})

	t.Run("should error when volume ContainerDir contains ':'", func(t *testing.T) {
		args := &cliwrappers.BuildahBuildArgs{
			Containerfile: containerfile,
			ContextDir:    contextDir,
			OutputRef:     outputRef,
			Volumes:       []cliwrappers.BuildahVolume{{HostDir: "/foo", ContainerDir: "other:dir"}},
		}

		err := args.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("':' in volume mount target path: other:dir"))
	})
}

func TestBuildahCli_ManifestCreate(t *testing.T) {
	g := NewWithT(t)

	const manifestName = "quay.io/org/myapp:latest"

	t.Run("should create manifest", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		args := &cliwrappers.BuildahManifestCreateArgs{
			ManifestName: manifestName,
		}

		err := buildahCli.ManifestCreate(args)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs).To(Equal([]string{"manifest", "create", manifestName}))
	})

	t.Run("should error if manifest name is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestCreateArgs{
			ManifestName: "",
		}

		err := buildahCli.ManifestCreate(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("manifest name is empty"))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to create manifest")
		}

		args := &cliwrappers.BuildahManifestCreateArgs{
			ManifestName: manifestName,
		}

		err := buildahCli.ManifestCreate(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to create manifest"))
	})
}

func TestBuildahCli_ManifestAdd(t *testing.T) {
	g := NewWithT(t)

	const manifestName = "quay.io/org/myapp:latest"
	const imageRef = "docker://quay.io/org/myapp@sha256:abc123"

	t.Run("should add image to manifest", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		args := &cliwrappers.BuildahManifestAddArgs{
			ManifestName: manifestName,
			ImageRef:     imageRef,
			All:          true,
		}

		err := buildahCli.ManifestAdd(args)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs).To(Equal([]string{"manifest", "add", manifestName, imageRef, "--all"}))
	})

	t.Run("should add image without --all flag", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return "", "", 0, nil
		}

		args := &cliwrappers.BuildahManifestAddArgs{
			ManifestName: manifestName,
			ImageRef:     imageRef,
			All:          false,
		}

		err := buildahCli.ManifestAdd(args)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs).To(Equal([]string{"manifest", "add", manifestName, imageRef}))
	})

	t.Run("should error if manifest name is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestAddArgs{
			ManifestName: "",
			ImageRef:     imageRef,
		}

		err := buildahCli.ManifestAdd(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("manifest name is empty"))
	})

	t.Run("should error if image reference is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestAddArgs{
			ManifestName: manifestName,
			ImageRef:     "",
		}

		err := buildahCli.ManifestAdd(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("image reference is empty"))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to add image")
		}

		args := &cliwrappers.BuildahManifestAddArgs{
			ManifestName: manifestName,
			ImageRef:     imageRef,
			All:          true,
		}

		err := buildahCli.ManifestAdd(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to add image"))
	})
}

func TestBuildahCli_ManifestInspect(t *testing.T) {
	g := NewWithT(t)

	const manifestName = "quay.io/org/myapp:latest"
	const manifestJSON = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json"}`

	t.Run("should inspect manifest and return JSON", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeFunc = func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			capturedArgs = args
			return manifestJSON, "", 0, nil
		}

		args := &cliwrappers.BuildahManifestInspectArgs{
			ManifestName: manifestName,
		}

		result, err := buildahCli.ManifestInspect(args)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs).To(Equal([]string{"manifest", "inspect", manifestName}))
		g.Expect(result).To(Equal(manifestJSON))
	})

	t.Run("should error if manifest name is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestInspectArgs{
			ManifestName: "",
		}

		_, err := buildahCli.ManifestInspect(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("manifest name is empty"))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeFunc = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to inspect manifest")
		}

		args := &cliwrappers.BuildahManifestInspectArgs{
			ManifestName: manifestName,
		}

		_, err := buildahCli.ManifestInspect(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to inspect manifest"))
	})
}

func TestBuildahCli_ManifestPush(t *testing.T) {
	g := NewWithT(t)

	const manifestName = "quay.io/org/myapp:latest"
	const destination = "docker://quay.io/org/myapp:latest"
	const digest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	ensureRetryerDisabled(t)

	mockSuccessfulManifestPush := func(captureArgs *[]string) func(command string, args ...string) (string, string, int, error) {
		return func(command string, args ...string) (string, string, int, error) {
			g.Expect(command).To(Equal("buildah"))
			*captureArgs = args

			digestFile := findDigestFile(args)
			g.Expect(digestFile).ToNot(BeEmpty())

			os.WriteFile(digestFile, []byte(digest), 0644)

			return "", "", 0, nil
		}
	}

	t.Run("should push manifest with default options", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulManifestPush(&capturedArgs)

		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: manifestName,
			Destination:  destination,
			TLSVerify:    true,
		}

		returnedDigest, err := buildahCli.ManifestPush(args)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(capturedArgs[0]).To(Equal("manifest"))
		g.Expect(capturedArgs[1]).To(Equal("push"))
		expectArgAndValue(g, capturedArgs, "--digestfile", findDigestFile(capturedArgs))
		g.Expect(capturedArgs).To(ContainElement("--tls-verify=true"))
		g.Expect(capturedArgs[len(capturedArgs)-2]).To(Equal(manifestName))
		g.Expect(capturedArgs[len(capturedArgs)-1]).To(Equal(destination))
		g.Expect(returnedDigest).To(Equal(digest))
	})

	t.Run("should push manifest with format and retry", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulManifestPush(&capturedArgs)

		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: manifestName,
			Destination:  destination,
			Format:       "oci",
			TLSVerify:    false,
		}

		_, err := buildahCli.ManifestPush(args)

		g.Expect(err).ToNot(HaveOccurred())
		expectArgAndValue(g, capturedArgs, "--format", "oci")
		g.Expect(capturedArgs).To(ContainElement("--tls-verify=false"))
	})

	t.Run("should error if manifest name is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: "",
			Destination:  destination,
		}

		_, err := buildahCli.ManifestPush(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("manifest name is empty"))
	})

	t.Run("should error if destination is empty", func(t *testing.T) {
		buildahCli, _ := setupBuildahCli()
		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: manifestName,
			Destination:  "",
		}

		_, err := buildahCli.ManifestPush(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("destination is empty"))
	})

	t.Run("should error if buildah execution fails", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		executor.executeWithOutput = func(command string, args ...string) (string, string, int, error) {
			return "", "", 1, errors.New("failed to push manifest")
		}

		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: manifestName,
			Destination:  destination,
		}

		_, err := buildahCli.ManifestPush(args)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("failed to push manifest"))
	})

	t.Run("should clean up digest file after push", func(t *testing.T) {
		buildahCli, executor := setupBuildahCli()
		var capturedArgs []string
		executor.executeWithOutput = mockSuccessfulManifestPush(&capturedArgs)

		args := &cliwrappers.BuildahManifestPushArgs{
			ManifestName: manifestName,
			Destination:  destination,
		}

		_, err := buildahCli.ManifestPush(args)

		g.Expect(err).ToNot(HaveOccurred())

		digestFile := findDigestFile(capturedArgs)
		g.Expect(digestFile).ToNot(BeEmpty())

		_, statErr := os.Stat(digestFile)
		g.Expect(os.IsNotExist(statErr)).To(BeTrue(), "digest file should be cleaned up")
	})
}

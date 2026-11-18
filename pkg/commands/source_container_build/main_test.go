package source_container_build

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/testutil"
)

const (
	testBinaryImageURL    = "quay.io/org/app:latest"
	testBinaryImageDigest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testSourceImageDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
)

// setupBuild returns a command whose source directory is a repository with one
// file, and whose registry holds the binary image.
func setupBuild(t *testing.T) (*SourceContainerBuild, *mockSkopeoCli, *mockGobsiCli) {
	t.Helper()

	sourceDir := t.TempDir()
	testutil.WriteFileTree(t, sourceDir, map[string]string{"main.go": "package main\n"})

	gitCli := &mockGitCli{
		RevParseFunc:  func(ref string, short bool, length int) (string, error) { return testCommitSha, nil },
		ConfigGetFunc: func(key string) (string, error) { return "https://github.com/namespace/app.git", nil },
		ShowFunc:      func(format string) (string, error) { return testCommitTime, nil },
		LsFilesFunc:   func(recurseSubmodules bool) ([]string, error) { return []string{"main.go"}, nil },
	}
	skopeoCli := &mockSkopeoCli{
		// A single manifest, which is also an image the registry has.
		InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) { return "{}", nil },
		CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
			if args.DigestFile != "" {
				return os.WriteFile(args.DigestFile, []byte(testSourceImageDigest), 0644)
			}
			return nil
		},
	}
	gobsiCli := &mockGobsiCli{}
	// An SBOM naming no base image, so that a build has nothing to merge in
	// unless the test says otherwise.
	cosignCli := &mockCosignCli{
		DownloadSBOMFunc: func(args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string) (string, error) {
			return `{"bomFormat": "CycloneDX", "formulation": []}`, nil
		},
	}

	scb := &SourceContainerBuild{
		Params: &Params{
			BinaryImageURL:    testBinaryImageURL,
			BinaryImageDigest: testBinaryImageDigest,
			SourceDir:         sourceDir,
			WorkspaceDir:      t.TempDir(),
		},
		GitCli:        gitCli,
		SkopeoCli:     skopeoCli,
		GobsiCli:      gobsiCli,
		CosignCli:     cosignCli,
		ResultsWriter: &mockResultsWriter{},
		AuthFile:      writeAuthFile(t),
	}

	return scb, skopeoCli, gobsiCli
}

// writeAuthFile creates an authentication file holding a credential for the
// registry of the binary image, so that the SBOM download is not left to the
// configuration of whoever runs the tests.
func writeAuthFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"auths": {"quay.io": {"auth": "dGVzdDp0ZXN0"}}}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write the auth file: %s", err)
	}

	return path
}

func Test_build(t *testing.T) {
	g := NewWithT(t)
	wantSourceImage := "quay.io/org/app:sha256-" + testBinaryImageDigest[7:] + ".src"

	t.Run("should build and push the source image", func(t *testing.T) {
		scb, _, gobsiCli := setupBuild(t)
		var gobsiArgs *cliwrappers.GobsiBuildArgs
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			gobsiArgs = args
			return nil
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusSuccess))
		g.Expect(result.ImageURL).To(Equal(wantSourceImage))
		g.Expect(result.ImageDigest).To(Equal(testSourceImageDigest))
		g.Expect(result.DependenciesIncluded).To(BeFalse())
		g.Expect(result.BaseImageSourceIncluded).To(BeFalse())

		// The source archive is the one source every build has.
		g.Expect(gobsiArgs.ExtraSrcDirs).To(HaveLen(1))
		g.Expect(gobsiArgs.ExtraSrcDirs[0]).To(HaveSuffix("source_archive"))
		g.Expect(gobsiArgs.MergeDirs).To(BeEmpty())
		g.Expect(gobsiArgs.SrpmDir).To(BeEmpty())
	})

	t.Run("should mark the source directory safe for its own git commands", func(t *testing.T) {
		scb, _, _ := setupBuild(t)
		gitEnv := map[string]string{}
		scb.GitCli = &mockGitCli{
			SetEnvFunc:    func(key, value string) { gitEnv[key] = value },
			RevParseFunc:  func(ref string, short bool, length int) (string, error) { return testCommitSha, nil },
			ConfigGetFunc: func(key string) (string, error) { return "https://github.com/namespace/app.git", nil },
			ShowFunc:      func(format string) (string, error) { return testCommitTime, nil },
			LsFilesFunc:   func(recurseSubmodules bool) ([]string, error) { return []string{"main.go"}, nil },
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusSuccess))
		g.Expect(gitEnv).To(Equal(map[string]string{
			"GIT_CONFIG_COUNT":   "1",
			"GIT_CONFIG_KEY_0":   "safe.directory",
			"GIT_CONFIG_VALUE_0": scb.Params.SourceDir,
		}))
	})

	t.Run("should drop the build when the binary image is already gone", func(t *testing.T) {
		scb, skopeoCli, gobsiCli := setupBuild(t)
		built := false
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			built = true
			return nil
		}
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
			return "", errors.New("exit status 1: manifest unknown")
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusDrop))
		g.Expect(result.Message).To(ContainSubstring("no longer exists"))
		g.Expect(result.ImageURL).To(BeEmpty())
		// Nothing is worth building once the image it would describe is gone.
		g.Expect(built).To(BeFalse())
	})

	t.Run("should drop the build when the binary image goes away during it", func(t *testing.T) {
		scb, skopeoCli, _ := setupBuild(t)
		// Present when the build starts, gone by the time it would be pushed.
		inspected := 0
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
			inspected++
			if inspected == 1 {
				return "{}", nil
			}
			return "", errors.New("exit status 1: manifest unknown")
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusDrop))
		g.Expect(result.Message).To(ContainSubstring("does not exist anymore"))
		// Nothing was pushed, so there is no source image to report.
		g.Expect(result.ImageURL).To(BeEmpty())
		g.Expect(result.ImageDigest).To(BeEmpty())
	})

	t.Run("should record a failure in the result instead of returning it", func(t *testing.T) {
		scb, _, gobsiCli := setupBuild(t)
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			return errors.New("no SRPMs found")
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusFailure))
		g.Expect(result.Message).To(ContainSubstring("no SRPMs found"))
	})

	t.Run("should merge the sources of the base image", func(t *testing.T) {
		scb, skopeoCli, gobsiCli := setupBuild(t)
		scb.Params.BaseImages = "registry.redhat.io/ubi9/ubi@" + testBaseDigest
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) { return "{}", nil }

		var gobsiArgs *cliwrappers.GobsiBuildArgs
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			gobsiArgs = args
			return nil
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusSuccess))
		g.Expect(result.BaseImageSourceIncluded).To(BeTrue())
		g.Expect(gobsiArgs.MergeDirs).To(HaveLen(1))
		g.Expect(gobsiArgs.MergeDirs[0]).To(HaveSuffix("parent_image_sources"))
	})

	t.Run("should include the prefetched dependencies", func(t *testing.T) {
		scb, _, gobsiCli := setupBuild(t)
		prefetchDir := t.TempDir()
		scb.Params.PrefetchArtifactsDir = prefetchDir
		writeGzip(t, filepath.Join(prefetchDir, "output", "deps", "pip", "requests.tar.gz"))
		writeSrpm(t, filepath.Join(prefetchDir, "output", "foo-1.0.src.rpm"), "foo")

		var gobsiArgs *cliwrappers.GobsiBuildArgs
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			gobsiArgs = args
			return nil
		}

		result := scb.build()

		g.Expect(result.Status).To(Equal(statusSuccess))
		g.Expect(result.DependenciesIncluded).To(BeTrue())
		g.Expect(gobsiArgs.SrpmDir).To(HaveSuffix("rpm_dir"))
		g.Expect(gobsiArgs.ExtraSrcDirs).To(HaveLen(2))
	})

	t.Run("should not hand gobsi an empty SRPM directory", func(t *testing.T) {
		scb, _, gobsiCli := setupBuild(t)
		prefetchDir := t.TempDir()
		scb.Params.PrefetchArtifactsDir = prefetchDir
		writeGzip(t, filepath.Join(prefetchDir, "output", "deps", "pip", "requests.tar.gz"))

		var gobsiArgs *cliwrappers.GobsiBuildArgs
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			gobsiArgs = args
			return nil
		}

		scb.build()

		g.Expect(gobsiArgs.SrpmDir).To(BeEmpty())
	})
}

func Test_Run(t *testing.T) {
	g := NewWithT(t)

	t.Run("should write every result", func(t *testing.T) {
		scb, _, _ := setupBuild(t)
		results := &mockResultsWriter{}
		scb.ResultsWriter = results
		scb.Params.ResultPathBuildResult = "/results/build_result"
		scb.Params.ResultPathSourceImageURL = "/results/url"
		scb.Params.ResultPathSourceImageDigest = "/results/digest"
		scb.Params.ResultPathImageRef = "/results/image_ref"

		err := scb.Run()

		g.Expect(err).ToNot(HaveOccurred())

		wantURL := "quay.io/org/app:sha256-" + testBinaryImageDigest[7:] + ".src"
		g.Expect(results.WrittenResults["/results/url"]).To(Equal(wantURL))
		g.Expect(results.WrittenResults["/results/digest"]).To(Equal(testSourceImageDigest))
		g.Expect(results.WrittenResults["/results/image_ref"]).To(Equal(wantURL + "@" + testSourceImageDigest))

		var result BuildResult
		g.Expect(json.Unmarshal([]byte(results.WrittenResults["/results/build_result"]), &result)).To(Succeed())
		g.Expect(result.Status).To(Equal(statusSuccess))
	})

	t.Run("should report an empty image reference when nothing was pushed", func(t *testing.T) {
		scb, skopeoCli, _ := setupBuild(t)
		results := &mockResultsWriter{}
		scb.ResultsWriter = results
		scb.Params.ResultPathImageRef = "/results/image_ref"
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
			return "", errors.New("exit status 1: manifest unknown")
		}

		err := scb.Run()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(results.WrittenResults["/results/image_ref"]).To(BeEmpty())
	})

	t.Run("should fail the command but still write the result on failure", func(t *testing.T) {
		scb, _, gobsiCli := setupBuild(t)
		results := &mockResultsWriter{}
		scb.ResultsWriter = results
		scb.Params.ResultPathBuildResult = "/results/build_result"
		gobsiCli.BuildFunc = func(args *cliwrappers.GobsiBuildArgs) error {
			return errors.New("no SRPMs found")
		}

		err := scb.Run()

		g.Expect(err).To(MatchError(ContainSubstring("no SRPMs found")))
		g.Expect(results.WrittenResults).To(HaveKey("/results/build_result"))

		var result BuildResult
		g.Expect(json.Unmarshal([]byte(results.WrittenResults["/results/build_result"]), &result)).To(Succeed())
		g.Expect(result.Status).To(Equal(statusFailure))
	})

	t.Run("should reject an invalid binary image digest", func(t *testing.T) {
		scb, _, _ := setupBuild(t)
		scb.Params.BinaryImageDigest = "nonsense"

		err := scb.Run()

		g.Expect(err).To(MatchError(ContainSubstring("digest 'nonsense' is invalid")))
	})

	t.Run("should reject a source directory that is not there", func(t *testing.T) {
		scb, _, _ := setupBuild(t)
		scb.Params.SourceDir = filepath.Join(t.TempDir(), "missing")

		err := scb.Run()

		g.Expect(err).To(MatchError(ContainSubstring("is not accessible")))
	})
}

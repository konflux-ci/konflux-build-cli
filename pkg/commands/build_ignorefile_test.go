package commands

import (
	"os"
	"path/filepath"
	"testing"

	cliwrappers "github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	. "github.com/onsi/gomega"
)

func TestBuild_detectIgnoreFile(t *testing.T) {
	t.Run("should detect .containerignore companion file", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		containerfile := filepath.Join(tmpDir, "Dockerfile")
		ignoreFile := containerfile + ".containerignore"

		err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(ignoreFile, []byte("*.tmp\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		b := &Build{containerfilePath: containerfile}
		b.detectIgnoreFile()

		g.Expect(b.ignoreFilePath).To(Equal(ignoreFile))
	})

	t.Run("should detect .dockerignore companion file", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		containerfile := filepath.Join(tmpDir, "Dockerfile")
		ignoreFile := containerfile + ".dockerignore"

		err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(ignoreFile, []byte("*.tmp\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		b := &Build{containerfilePath: containerfile}
		b.detectIgnoreFile()

		g.Expect(b.ignoreFilePath).To(Equal(ignoreFile))
	})

	t.Run("should prefer .containerignore over .dockerignore", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		containerfile := filepath.Join(tmpDir, "Dockerfile")
		containerIgnore := containerfile + ".containerignore"
		dockerIgnore := containerfile + ".dockerignore"

		err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(containerIgnore, []byte("*.tmp\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(dockerIgnore, []byte("*.log\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		b := &Build{containerfilePath: containerfile}
		b.detectIgnoreFile()

		g.Expect(b.ignoreFilePath).To(Equal(containerIgnore))
	})

	t.Run("should leave ignoreFilePath empty when no companion file exists", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		containerfile := filepath.Join(tmpDir, "Dockerfile")

		err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		b := &Build{containerfilePath: containerfile}
		b.detectIgnoreFile()

		g.Expect(b.ignoreFilePath).To(BeEmpty())
	})

	t.Run("should handle subdirectory Dockerfile path", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		g.Expect(err).ToNot(HaveOccurred())

		containerfile := filepath.Join(subDir, "Containerfile")
		ignoreFile := containerfile + ".dockerignore"

		err = os.WriteFile(containerfile, []byte("FROM scratch\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(ignoreFile, []byte("*.tmp\n"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		b := &Build{containerfilePath: containerfile}
		b.detectIgnoreFile()

		g.Expect(b.ignoreFilePath).To(Equal(ignoreFile))
	})
}

func TestBuild_detectIgnoreFile_fullFlow(t *testing.T) {
	t.Run("should automatically pass detected ignore file to buildah", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		contextDir := filepath.Join(tempDir, "context")
		os.Mkdir(contextDir, 0755)
		os.WriteFile(filepath.Join(contextDir, "Containerfile"), []byte("FROM scratch"), 0644)
		os.WriteFile(filepath.Join(contextDir, "Containerfile.dockerignore"), []byte("*.tmp\n"), 0644)

		mockBuildah := &mockBuildahCli{}
		mockResults := &mockResultsWriter{}

		isBuildCalled := false
		mockBuildah.BuildFunc = func(args *cliwrappers.BuildahBuildArgs) error {
			isBuildCalled = true
			g.Expect(args.IgnoreFile).To(Equal(filepath.Join(contextDir, "Containerfile.dockerignore")))
			return nil
		}

		mockResults.CreateResultJsonFunc = func(result any) (string, error) {
			return "", nil
		}

		c := &Build{
			CliWrappers:   BuildCliWrappers{BuildahCli: mockBuildah},
			ResultsWriter: mockResults,
			Params: &BuildParams{
				OutputRef:      "localhost/test:tag",
				Context:        contextDir,
				Push:           false,
				SkipInjections: true,
				SrcTLSVerify:   true,
				DestTLSVerify:  true,
				SBOMFormat:     "spdx",
			},
		}

		err := c.run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isBuildCalled).To(BeTrue())
	})

	t.Run("should not pass ignore file to buildah when none exists", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		contextDir := filepath.Join(tempDir, "context")
		os.Mkdir(contextDir, 0755)
		os.WriteFile(filepath.Join(contextDir, "Containerfile"), []byte("FROM scratch"), 0644)

		mockBuildah := &mockBuildahCli{}
		mockResults := &mockResultsWriter{}

		isBuildCalled := false
		mockBuildah.BuildFunc = func(args *cliwrappers.BuildahBuildArgs) error {
			isBuildCalled = true
			g.Expect(args.IgnoreFile).To(BeEmpty())
			return nil
		}

		mockResults.CreateResultJsonFunc = func(result any) (string, error) {
			return "", nil
		}

		c := &Build{
			CliWrappers:   BuildCliWrappers{BuildahCli: mockBuildah},
			ResultsWriter: mockResults,
			Params: &BuildParams{
				OutputRef:      "localhost/test:tag",
				Context:        contextDir,
				Push:           false,
				SkipInjections: true,
				SrcTLSVerify:   true,
				DestTLSVerify:  true,
				SBOMFormat:     "spdx",
			},
		}

		err := c.run()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(isBuildCalled).To(BeTrue())
	})
}

package cliwrappers_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

func mockCosignCli() (*cliwrappers.CosignCli, *mockExecutor) {
	executor := &mockExecutor{}
	cosignCli := &cliwrappers.CosignCli{Executor: executor}
	cliwrappers.DisableRetryer = true
	return cosignCli, executor
}

func TestCosignCliSign(t *testing.T) {
	baseArgs := cliwrappers.CosignSignArgs{
		Image:      "quay.io/org/image:latest",
		RekorURL:   "https://rekor.sigstore.dev",
		FulcioURL:  "https://fulcio.sigstore.dev",
		OIDCIssuer: "https://oidc.sigstore.dev",
	}
	expectedCmdArgs := []string{
		"sign",
		baseArgs.Image,
		"--yes",
		"--use-signing-config=false",
		"--rekor-url", baseArgs.RekorURL,
		"--fulcio-url", baseArgs.FulcioURL,
		"--oidc-issuer", baseArgs.OIDCIssuer,
	}

	t.Run("without extra env", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Name).To(Equal("cosign"))
			g.Expect(cmd.Args).To(Equal(expectedCmdArgs))
			g.Expect(cmd.Env).To(Equal(os.Environ()))
			return "", "", 0, nil
		}

		g.Expect(cosignCli.Sign(&baseArgs, "")).To(Succeed())
	})

	t.Run("with Docker registry config", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()
		dockerConfigDir := "/tmp/docker-config"

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Name).To(Equal("cosign"))
			g.Expect(cmd.Args).To(Equal(expectedCmdArgs))
			g.Expect(cmd.Env).To(ContainElement("DOCKER_CONFIG=" + dockerConfigDir))
			return "", "", 0, nil
		}

		g.Expect(cosignCli.Sign(&baseArgs, dockerConfigDir)).To(Succeed())
	})

	t.Run("with OIDC token", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		tokenPath := filepath.Join(t.TempDir(), "token")
		// The trailing newline must not end up in the environment variable.
		g.Expect(os.WriteFile(tokenPath, []byte("abcdef\n"), 0600)).To(Succeed())

		args := baseArgs
		args.OIDCTokenPath = tokenPath

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Name).To(Equal("cosign"))
			g.Expect(cmd.Args).To(Equal(expectedCmdArgs))
			g.Expect(cmd.Env).To(ContainElement("SIGSTORE_ID_TOKEN=abcdef"))
			return "", "", 0, nil
		}

		g.Expect(cosignCli.Sign(&args, "")).To(Succeed())
	})
}

func TestCosignCliInitialize(t *testing.T) {
	baseArgs := cliwrappers.CosignInitializeArgs{
		Root:   "/tmp/cosign-root",
		Mirror: "https://mirror.sigstore.dev",
	}
	expectedCmdArgs := []string{
		"initialize",
		"--root", baseArgs.Root,
		"--mirror", baseArgs.Mirror,
	}

	t.Run("with root and mirror", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Name).To(Equal("cosign"))
			g.Expect(cmd.Args).To(Equal(expectedCmdArgs))
			return "", "", 0, nil
		}

		g.Expect(cosignCli.Initialize(&baseArgs)).To(Succeed())
	})
}

func TestCosignCliDownloadSBOM(t *testing.T) {
	const sbom = `{"bomFormat": "CycloneDX"}`
	const image = "quay.io/org/image@sha256:abc"

	t.Run("should return the SBOM of an image", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Name).To(Equal("cosign"))
			g.Expect(cmd.Args).To(Equal([]string{"download", "sbom", image}))
			// An SBOM is far too large to log.
			g.Expect(cmd.LogOutput).To(BeFalse())
			return sbom, "", 0, nil
		}

		got, err := cosignCli.DownloadSBOM(&cliwrappers.CosignDownloadSBOMArgs{Image: image}, "")

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(Equal(sbom))
	})

	t.Run("should point cosign at the given authentication", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		dockerConfigDir := t.TempDir()
		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			g.Expect(cmd.Env).To(ContainElement("DOCKER_CONFIG=" + dockerConfigDir))
			return sbom, "", 0, nil
		}

		_, err := cosignCli.DownloadSBOM(&cliwrappers.CosignDownloadSBOMArgs{Image: image}, dockerConfigDir)

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should report an image without an SBOM", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, executor := mockCosignCli()

		executor.executeFunc = func(cmd cliwrappers.Cmd) (string, string, int, error) {
			return "", "no sbom attached", 1, errors.New("exit status 1")
		}

		_, err := cosignCli.DownloadSBOM(&cliwrappers.CosignDownloadSBOMArgs{Image: image}, "")

		g.Expect(err).To(MatchError(ContainSubstring("no sbom attached")))
	})

	t.Run("should refuse an empty image", func(t *testing.T) {
		g := NewWithT(t)
		cosignCli, _ := mockCosignCli()

		_, err := cosignCli.DownloadSBOM(&cliwrappers.CosignDownloadSBOMArgs{}, "")

		g.Expect(err).To(MatchError(ContainSubstring("no image")))
	})
}

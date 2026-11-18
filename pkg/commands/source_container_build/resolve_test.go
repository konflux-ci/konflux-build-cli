package source_container_build

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

const (
	testBaseImage  = "registry.redhat.io/ubi9/ubi@sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testBaseDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
)

// imageConfigWithLabels is what skopeo inspect --config reports.
func imageConfigWithLabels(labels string) string {
	return `{"architecture":"amd64","config":{"Labels":{` + labels + `}}}`
}

func Test_generateKonfluxSourceImage(t *testing.T) {
	g := NewWithT(t)

	g.Expect(generateKonfluxSourceImage("quay.io/org/app", "sha256:abc123")).
		To(Equal("quay.io/org/app:sha256-abc123.src"))
}

func Test_registryOf(t *testing.T) {
	testCases := map[string]string{
		"registry.redhat.io/ubi9/ubi:latest": "registry.redhat.io",
		"quay.io:5000/org/app@sha256:abc":    "quay.io:5000",
		"localhost/app:latest":               "localhost",
		"app:latest":                         "app:latest",
	}

	for image, want := range testCases {
		t.Run(image, func(t *testing.T) {
			NewWithT(t).Expect(registryOf(image)).To(Equal(want))
		})
	}
}

func Test_nonEmptyLines(t *testing.T) {
	g := NewWithT(t)

	g.Expect(nonEmptyLines("")).To(BeEmpty())
	g.Expect(nonEmptyLines("  \n\n ")).To(BeEmpty())
	g.Expect(nonEmptyLines("a\n\n  b  \nc\n")).To(Equal([]string{"a", "b", "c"}))
}

func Test_baseImages(t *testing.T) {
	g := NewWithT(t)

	t.Run("should take the base images given directly", func(t *testing.T) {
		scb := &SourceContainerBuild{Params: &Params{
			BaseImages: "from.param/first@sha256:abc\nfrom.param/second@sha256:def",
		}}

		g.Expect(scb.baseImages()).To(Equal([]string{
			"from.param/first@sha256:abc",
			"from.param/second@sha256:def",
		}))
	})

	t.Run("should not read the SBOM when base images were given", func(t *testing.T) {
		downloaded := false
		scb := &SourceContainerBuild{
			Params: &Params{BaseImages: "from.param/image@sha256:abc"},
			CosignCli: &mockCosignCli{
				DownloadSBOMFunc: func(
					args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string,
				) (string, error) {
					downloaded = true
					return "", nil
				},
			},
		}

		g.Expect(scb.baseImages()).To(Equal([]string{"from.param/image@sha256:abc"}))
		g.Expect(downloaded).To(BeFalse())
	})

	t.Run("should report no base images when none are given nor discovered", func(t *testing.T) {
		scb := &SourceContainerBuild{
			Params:    &Params{BinaryImageURL: testBinaryImageURL, BinaryImageDigest: testBinaryImageDigest},
			SkopeoCli: &mockSkopeoCli{},
		}

		g.Expect(scb.baseImages()).To(BeEmpty())
	})
}

func Test_registryHasImage(t *testing.T) {
	g := NewWithT(t)

	t.Run("should report an image the registry has", func(t *testing.T) {
		scb := &SourceContainerBuild{SkopeoCli: &mockSkopeoCli{
			InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				g.Expect(args.Raw).To(BeTrue())
				g.Expect(args.RetryTimes).To(Equal(skopeoRetryTimes))
				return "{}", nil
			},
		}}

		exists, err := scb.registryHasImage("quay.io/org/app:tag")

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(exists).To(BeTrue())
	})

	t.Run("should report an image the registry does not have", func(t *testing.T) {
		// A registry reports absence one way when it knows the repository and
		// another way when it does not.
		for _, message := range []string{
			"exit status 1: reading manifest: manifest unknown",
			"exit status 1: reading manifest sha256:abc in 127.0.0.1:5000/app: " +
				"repository name not known to registry",
		} {
			scb := &SourceContainerBuild{SkopeoCli: &mockSkopeoCli{
				InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
					return "", errors.New(message)
				},
			}}

			exists, err := scb.registryHasImage("quay.io/org/app:tag")

			g.Expect(err).ToNot(HaveOccurred(), message)
			g.Expect(exists).To(BeFalse(), message)
		}
	})

	t.Run("should not mistake a registry problem for a missing image", func(t *testing.T) {
		scb := &SourceContainerBuild{SkopeoCli: &mockSkopeoCli{
			InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				return "", errors.New("exit status 1: unauthorized: access to the requested resource is not authorized")
			},
		}}

		_, err := scb.registryHasImage("quay.io/org/app:tag")

		g.Expect(err).To(MatchError(ContainSubstring("unauthorized")))
	})
}

func Test_resolveSourceImage(t *testing.T) {
	g := NewWithT(t)

	t.Run("should skip a local image", func(t *testing.T) {
		inspected := false
		scb := &SourceContainerBuild{
			Params: &Params{},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				inspected = true
				return "", nil
			}},
		}

		sourceImage, err := scb.resolveSourceImage("localhost/app:latest")

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sourceImage).To(BeEmpty())
		g.Expect(inspected).To(BeFalse())
	})

	t.Run("should resolve an allowlisted registry by version and release", func(t *testing.T) {
		var inspected []string
		scb := &SourceContainerBuild{
			Params: &Params{RegistryAllowlist: []string{"registry.redhat.io"}},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				inspected = append(inspected, args.ImageRef)
				if args.Config {
					return imageConfigWithLabels(`"version":"9.4","release":"1123"`), nil
				}
				return "{}", nil
			}},
		}

		sourceImage, err := scb.resolveSourceImage(testBaseImage)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sourceImage).To(Equal("registry.redhat.io/ubi9/ubi:9.4-1123-source"))
		g.Expect(inspected).To(Equal([]string{
			"registry.redhat.io/ubi9/ubi@" + testBaseDigest,
			"registry.redhat.io/ubi9/ubi:9.4-1123-source",
		}))
	})

	t.Run("should resolve a registry outside the allowlist by digest", func(t *testing.T) {
		var inspected []string
		scb := &SourceContainerBuild{
			Params: &Params{RegistryAllowlist: []string{"registry.redhat.io"}},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				inspected = append(inspected, args.ImageRef)
				return "{}", nil
			}},
		}

		sourceImage, err := scb.resolveSourceImage("quay.io/org/app@" + testBaseDigest)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sourceImage).To(Equal("quay.io/org/app:sha256-" + testBaseDigest[7:] + ".src"))
		// Resolved by name alone, without reading the image config.
		g.Expect(inspected).To(HaveLen(1))
	})

	t.Run("should resolve nothing when the image has no version and release", func(t *testing.T) {
		scb := &SourceContainerBuild{
			Params: &Params{RegistryAllowlist: []string{"registry.redhat.io"}},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				return imageConfigWithLabels(`"name":"ubi9"`), nil
			}},
		}

		sourceImage, err := scb.resolveSourceImage(testBaseImage)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sourceImage).To(BeEmpty())
	})

	t.Run("should resolve nothing when the source image does not exist", func(t *testing.T) {
		scb := &SourceContainerBuild{
			Params: &Params{},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				return "", errors.New("exit status 1: manifest unknown")
			}},
		}

		sourceImage, err := scb.resolveSourceImage("quay.io/org/app@" + testBaseDigest)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sourceImage).To(BeEmpty())
	})

	t.Run("should report an image config that cannot be parsed", func(t *testing.T) {
		scb := &SourceContainerBuild{
			Params: &Params{RegistryAllowlist: []string{"registry.redhat.io"}},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				return "not json", nil
			}},
		}

		_, err := scb.resolveSourceImage(testBaseImage)

		g.Expect(err).To(MatchError(ContainSubstring("failed to parse the config")))
	})

	t.Run("should report a base image without a digest", func(t *testing.T) {
		scb := &SourceContainerBuild{Params: &Params{}, SkopeoCli: &mockSkopeoCli{}}

		_, err := scb.resolveSourceImage("quay.io/org/app:latest")

		g.Expect(err).To(MatchError(ContainSubstring("missing a digest")))
	})
}

func Test_handleBaseImageSources(t *testing.T) {
	g := NewWithT(t)

	t.Run("should do nothing without a base image", func(t *testing.T) {
		scb := &SourceContainerBuild{Params: &Params{}, SkopeoCli: &mockSkopeoCli{}}

		parentSourcesDir, err := scb.handleBaseImageSources(t.TempDir())

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parentSourcesDir).To(BeEmpty())
	})

	t.Run("should handle only the last of several base images", func(t *testing.T) {
		var inspected []string
		scb := &SourceContainerBuild{
			Params: &Params{
				BaseImages: "quay.io/org/builder@" + testBaseDigest + "\nquay.io/org/runtime@" + testBaseDigest,
			},
			SkopeoCli: &mockSkopeoCli{InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
				inspected = append(inspected, args.ImageRef)
				return "", errors.New("exit status 1: manifest unknown")
			}},
		}

		parentSourcesDir, err := scb.handleBaseImageSources(t.TempDir())

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parentSourcesDir).To(BeEmpty())
		g.Expect(inspected).To(HaveLen(1))
		g.Expect(inspected[0]).To(HavePrefix("quay.io/org/runtime:"))
	})

	t.Run("should download the resolved source image", func(t *testing.T) {
		workDir := t.TempDir()
		var copyArgs *cliwrappers.SkopeoCopyArgs
		scb := &SourceContainerBuild{
			Params: &Params{BaseImages: "quay.io/org/app@" + testBaseDigest},
			SkopeoCli: &mockSkopeoCli{
				InspectFunc: func(args *cliwrappers.SkopeoInspectArgs) (string, error) { return "{}", nil },
				CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
					copyArgs = args
					return nil
				},
			},
		}

		parentSourcesDir, err := scb.handleBaseImageSources(workDir)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parentSourcesDir).To(Equal(filepath.Join(workDir, "parent_image_sources")))
		g.Expect(copyArgs.DestinationTransport).To(Equal(cliwrappers.SkopeoTransportOCI))
		g.Expect(copyArgs.RemoveSignatures).To(BeTrue())
	})
}

func Test_downloadParentImageSources(t *testing.T) {
	g := NewWithT(t)
	missingSignature := errors.New("exit status 1: " + cliwrappers.MissingSignatureMessage)

	t.Run("should carry on without the sources of an unsigned image when told to", func(t *testing.T) {
		workDir := t.TempDir()
		scb := &SourceContainerBuild{
			Params:    &Params{IgnoreUnsignedImage: true},
			SkopeoCli: &mockSkopeoCli{CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error { return missingSignature }},
		}

		parentSourcesDir, err := scb.downloadParentImageSources("quay.io/org/app:src", workDir)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parentSourcesDir).To(BeEmpty())
		// A half written layout must not be left for gobsi to merge.
		g.Expect(filepath.Join(workDir, "parent_image_sources")).ToNot(BeAnExistingFile())
	})

	t.Run("should fail on an unsigned image by default", func(t *testing.T) {
		workDir := t.TempDir()
		scb := &SourceContainerBuild{
			Params:    &Params{},
			SkopeoCli: &mockSkopeoCli{CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error { return missingSignature }},
		}

		_, err := scb.downloadParentImageSources("quay.io/org/app:src", workDir)

		g.Expect(err).To(MatchError(ContainSubstring(cliwrappers.MissingSignatureMessage)))
		g.Expect(filepath.Join(workDir, "parent_image_sources")).ToNot(BeAnExistingFile())
	})

	t.Run("should verify the parent source image against the given trust policy", func(t *testing.T) {
		var copyArgs *cliwrappers.SkopeoCopyArgs
		scb := &SourceContainerBuild{
			Params: &Params{
				SignaturePolicy:        "/etc/scb/policy.json",
				SignatureRegistriesDir: "/etc/scb/registries.d",
			},
			SkopeoCli: &mockSkopeoCli{CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
				copyArgs = args
				return nil
			}},
		}

		_, err := scb.downloadParentImageSources("quay.io/org/app:src", t.TempDir())

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(copyArgs.PolicyPath).To(Equal("/etc/scb/policy.json"))
		g.Expect(copyArgs.RegistriesDirPath).To(Equal("/etc/scb/registries.d"))
	})

	t.Run("should report a copy that failed for any other reason", func(t *testing.T) {
		scb := &SourceContainerBuild{
			Params: &Params{IgnoreUnsignedImage: true},
			SkopeoCli: &mockSkopeoCli{CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
				return errors.New("exit status 1: unauthorized")
			}},
		}

		_, err := scb.downloadParentImageSources("quay.io/org/app:src", t.TempDir())

		g.Expect(err).To(MatchError(ContainSubstring("unauthorized")))
	})
}

func Test_pushToRegistry(t *testing.T) {
	g := NewWithT(t)

	t.Run("should push the built layout and report the digest", func(t *testing.T) {
		var copyArgs *cliwrappers.SkopeoCopyArgs
		scb := &SourceContainerBuild{SkopeoCli: &mockSkopeoCli{
			CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
				copyArgs = args
				// skopeo writes the digest of what it pushed.
				return os.WriteFile(args.DigestFile, []byte("sha256:deadbeef\n"), 0644)
			},
		}}

		digest, err := scb.pushToRegistry("/work/source_image_output", "quay.io/org/app:sha256-abc.src")

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(digest).To(Equal("sha256:deadbeef"))
		g.Expect(copyArgs.SourceTransport).To(Equal(cliwrappers.SkopeoTransportOCI))
		g.Expect(copyArgs.SourceImage).To(Equal("/work/source_image_output:" + sourceImageTag))
		g.Expect(copyArgs.DestinationImage).To(Equal("quay.io/org/app:sha256-abc.src"))
	})

	t.Run("should report a failed push", func(t *testing.T) {
		scb := &SourceContainerBuild{SkopeoCli: &mockSkopeoCli{
			CopyFunc: func(args *cliwrappers.SkopeoCopyArgs) error {
				return errors.New("exit status 1: unauthorized")
			},
		}}

		_, err := scb.pushToRegistry("/work/source_image_output", "quay.io/org/app:sha256-abc.src")

		g.Expect(err).To(MatchError(ContainSubstring("unauthorized")))
	})
}

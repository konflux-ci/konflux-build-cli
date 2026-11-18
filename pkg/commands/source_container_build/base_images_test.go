package source_container_build

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

const (
	testSbomBaseRepo = "registry.access.redhat.com/ubi9/ubi-minimal"
	testSbomBasePurl = "pkg:oci/ubi-minimal@" + testBaseDigest +
		"?arch=amd64&repository_url=" + testSbomBaseRepo
)

// cycloneDXSbom is an SBOM in the format syft produces, trimmed to the parts
// the base image is read from.
const cycloneDXSbom = `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "components": [{"name": "some-package", "purl": "pkg:rpm/redhat/bash@5.1"}],
  "formulation": [{
    "components": [
      {
        "name": "not-the-base-image",
        "purl": "pkg:oci/other@sha256:9999?repository_url=registry.io/other",
        "properties": [{"name": "konflux:container:is_builder_image:for_stage", "value": "0"}]
      },
      {
        "name": "ubi-minimal",
        "purl": "` + testSbomBasePurl + `",
        "properties": [{"name": "konflux:container:is_base_image", "value": "true"}]
      }
    ]
  }]
}`

// spdxSbom is the other format Konflux publishes, where the marker is a JSON
// encoded annotation rather than a property.
const spdxSbom = `{
  "spdxVersion": "SPDX-2.3",
  "packages": [
    {
      "name": "some-package",
      "externalRefs": [{"referenceType": "purl", "referenceLocator": "pkg:rpm/redhat/bash@5.1"}]
    },
    {
      "name": "not-the-base-image",
      "annotations": [{"comment": "{\"name\": \"konflux:container:is_builder_image:for_stage\"}"}],
      "externalRefs": [
        {"referenceType": "purl", "referenceLocator": "pkg:oci/other@sha256:9999?repository_url=registry.io/other"}
      ]
    },
    {
      "name": "ubi-minimal",
      "annotations": [
        {"comment": "a plain annotation that is not JSON at all"},
        {"comment": "{\"name\": \"konflux:container:is_base_image\"}"}
      ],
      "externalRefs": [
        {"referenceType": "other", "referenceLocator": "ignore-me"},
        {"referenceType": "purl", "referenceLocator": "` + testSbomBasePurl + `"}
      ]
    }
  ]
}`

func Test_imageFromPurl(t *testing.T) {
	testCases := []struct {
		name    string
		purl    string
		want    string
		wantErr string
	}{
		{
			name: "oci purl with a repository url",
			purl: testSbomBasePurl,
			want: testSbomBaseRepo + "@" + testBaseDigest,
		},
		{
			name: "repository url is preferred over the name in the purl",
			purl: "pkg:oci/ubi@sha256:abcd?repository_url=registry.io/namespace/ubi",
			want: "registry.io/namespace/ubi@sha256:abcd",
		},
		{
			name:    "no repository url",
			purl:    "pkg:oci/ubi@sha256:abcd",
			wantErr: "no repository_url",
		},
		{
			name:    "no digest",
			purl:    "pkg:oci/ubi?repository_url=registry.io/ubi",
			wantErr: "no digest",
		},
		{
			name:    "not a package url",
			purl:    "registry.io/ubi:latest",
			wantErr: "failed to parse the package url",
		},
		{
			name: "empty",
			purl: "",
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := imageFromPurl(tc.purl)

			if tc.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.wantErr)))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tc.want))
		})
	}
}

func Test_baseImageFromSBOM(t *testing.T) {
	wantImage := testSbomBaseRepo + "@" + testBaseDigest

	t.Run("should read the base image out of a CycloneDX SBOM", func(t *testing.T) {
		g := NewWithT(t)

		got, err := baseImageFromSBOM(cycloneDXSbom)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(Equal(wantImage))
	})

	t.Run("should read the base image out of an SPDX SBOM", func(t *testing.T) {
		g := NewWithT(t)

		got, err := baseImageFromSBOM(spdxSbom)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(Equal(wantImage))
	})

	t.Run("should report no base image when nothing is marked", func(t *testing.T) {
		g := NewWithT(t)

		for _, sbom := range []string{
			`{"bomFormat": "CycloneDX", "formulation": []}`,
			`{"bomFormat": "CycloneDX"}`,
			`{"spdxVersion": "SPDX-2.3", "packages": []}`,
			`{"spdxVersion": "SPDX-2.3"}`,
		} {
			got, err := baseImageFromSBOM(sbom)

			g.Expect(err).ToNot(HaveOccurred(), sbom)
			g.Expect(got).To(BeEmpty(), sbom)
		}
	})

	t.Run("should report an SBOM that is not JSON", func(t *testing.T) {
		g := NewWithT(t)

		_, err := baseImageFromSBOM("not an sbom")

		g.Expect(err).To(MatchError(ContainSubstring("failed to parse the SBOM")))
	})
}

func Test_discoverBaseImage(t *testing.T) {
	wantImage := testSbomBaseRepo + "@" + testBaseDigest

	t.Run("should discover the base image from the SBOM", func(t *testing.T) {
		g := NewWithT(t)
		scb, _, _ := setupBuild(t)
		var sbomOf string
		scb.CosignCli = &mockCosignCli{
			DownloadSBOMFunc: func(args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string) (string, error) {
				sbomOf = args.Image
				g.Expect(dockerConfigDir).ToNot(BeEmpty())
				return cycloneDXSbom, nil
			},
		}

		g.Expect(scb.discoverBaseImage()).To(Equal(wantImage))
		g.Expect(sbomOf).To(Equal("quay.io/org/app@" + testBinaryImageDigest))
	})

	t.Run("should read the SBOM of one manifest of an image index", func(t *testing.T) {
		g := NewWithT(t)
		scb, skopeoCli, _ := setupBuild(t)
		childDigest := "sha256:4444444444444444444444444444444444444444444444444444444444444444"
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
			return `{"manifests": [{"digest": "` + childDigest + `"}]}`, nil
		}
		var sbomOf string
		scb.CosignCli = &mockCosignCli{
			DownloadSBOMFunc: func(args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string) (string, error) {
				sbomOf = args.Image
				return cycloneDXSbom, nil
			},
		}

		g.Expect(scb.discoverBaseImage()).To(Equal(wantImage))
		g.Expect(sbomOf).To(Equal("quay.io/org/app@" + childDigest))
	})

	t.Run("should carry on when the SBOM cannot be downloaded", func(t *testing.T) {
		g := NewWithT(t)
		scb, _, _ := setupBuild(t)
		scb.CosignCli = &mockCosignCli{
			DownloadSBOMFunc: func(args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string) (string, error) {
				return "", errors.New("no SBOM attached")
			},
		}

		g.Expect(scb.discoverBaseImage()).To(BeEmpty())
	})

	t.Run("should carry on when the registry has no credential for the image", func(t *testing.T) {
		g := NewWithT(t)
		scb, _, _ := setupBuild(t)
		scb.AuthFile = "/nonexistent/config.json"

		g.Expect(scb.discoverBaseImage()).To(BeEmpty())
	})

	t.Run("should carry on when the manifest cannot be read", func(t *testing.T) {
		g := NewWithT(t)
		scb, skopeoCli, _ := setupBuild(t)
		skopeoCli.InspectFunc = func(args *cliwrappers.SkopeoInspectArgs) (string, error) {
			return "", errors.New("manifest unknown")
		}

		g.Expect(scb.discoverBaseImage()).To(BeEmpty())
	})

	t.Run("should ask for the SBOM more than once before giving up", func(t *testing.T) {
		g := NewWithT(t)
		scb, _, _ := setupBuild(t)
		var attempts int
		scb.CosignCli = &mockCosignCli{
			DownloadSBOMFunc: func(args *cliwrappers.CosignDownloadSBOMArgs, dockerConfigDir string) (string, error) {
				attempts = args.MaxAttempts
				return cycloneDXSbom, nil
			},
		}

		g.Expect(scb.discoverBaseImage()).To(Equal(wantImage))
		g.Expect(attempts).To(Equal(sbomDownloadAttempts))
	})
}

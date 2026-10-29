package config_test

import (
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/config"

	. "github.com/onsi/gomega"
)

func TestNewKeylessSigningConfig(t *testing.T) {
	g := NewWithT(t)

	t.Run("should map raw config fields", func(t *testing.T) {
		cfg := config.NewKeylessSigningConfig(config.KonfluxRawConfig{
			DefaultOIDCIssuer: "https://oidc.example.com",
			RekorInternalURL:  "https://rekor-internal.example.com",
			RekorExternalURL:  "https://rekor-external.example.com",
			FulcioInternalURL: "https://fulcio-internal.example.com",
			FulcioExternalURL: "https://fulcio-external.example.com",
			TufInternalURL:    "https://tuf-internal.example.com",
			TufExternalURL:    "https://tuf-external.example.com",
		})

		g.Expect(cfg.OIDCIssuer).To(Equal("https://oidc.example.com"))
		g.Expect(cfg.RekorInternalURL).To(Equal("https://rekor-internal.example.com"))
		g.Expect(cfg.RekorExternalURL).To(Equal("https://rekor-external.example.com"))
		g.Expect(cfg.FulcioInternalURL).To(Equal("https://fulcio-internal.example.com"))
		g.Expect(cfg.FulcioExternalURL).To(Equal("https://fulcio-external.example.com"))
		g.Expect(cfg.TufInternalURL).To(Equal("https://tuf-internal.example.com"))
		g.Expect(cfg.TufExternalURL).To(Equal("https://tuf-external.example.com"))
	})

	t.Run("should prefer internal URLs", func(t *testing.T) {
		cfg := config.NewKeylessSigningConfig(config.KonfluxRawConfig{
			DefaultOIDCIssuer: "https://oidc.example.com",
			RekorInternalURL:  "https://rekor-internal.example.com",
			RekorExternalURL:  "https://rekor-external.example.com",
			FulcioInternalURL: "https://fulcio-internal.example.com",
			FulcioExternalURL: "https://fulcio-external.example.com",
			TufInternalURL:    "https://tuf-internal.example.com",
			TufExternalURL:    "https://tuf-external.example.com",
		})

		g.Expect(cfg.GetOIDCIssuer()).To(Equal("https://oidc.example.com"))
		g.Expect(cfg.GetRekorURL()).To(Equal("https://rekor-internal.example.com"))
		g.Expect(cfg.GetFulcioURL()).To(Equal("https://fulcio-internal.example.com"))
		g.Expect(cfg.GetTufURL()).To(Equal("https://tuf-internal.example.com"))
		g.Expect(cfg.IsValid()).To(BeTrue())
	})

	t.Run("should fall back to external URLs", func(t *testing.T) {
		cfg := config.NewKeylessSigningConfig(config.KonfluxRawConfig{
			DefaultOIDCIssuer: "https://oidc.example.com",
			RekorExternalURL:  "https://rekor-external.example.com",
			FulcioExternalURL: "https://fulcio-external.example.com",
			TufExternalURL:    "https://tuf-external.example.com",
		})

		g.Expect(cfg.GetRekorURL()).To(Equal("https://rekor-external.example.com"))
		g.Expect(cfg.GetFulcioURL()).To(Equal("https://fulcio-external.example.com"))
		g.Expect(cfg.GetTufURL()).To(Equal("https://tuf-external.example.com"))
		g.Expect(cfg.IsValid()).To(BeTrue())
	})

	t.Run("should be invalid when required URLs are missing", func(t *testing.T) {
		cfg := config.NewKeylessSigningConfig(config.KonfluxRawConfig{
			DefaultOIDCIssuer: "https://oidc.example.com",
			RekorExternalURL:  "https://rekor-external.example.com",
		})

		g.Expect(cfg.IsValid()).To(BeFalse())
	})

	t.Run("should deep copy", func(t *testing.T) {
		cfg := config.NewKeylessSigningConfig(config.KonfluxRawConfig{
			DefaultOIDCIssuer: "https://oidc.example.com",
			RekorExternalURL:  "https://rekor.example.com",
		})
		copy := cfg.DeepCopy()
		copy.OIDCIssuer = "changed"

		g.Expect(cfg.OIDCIssuer).To(Equal("https://oidc.example.com"))
		g.Expect(copy.OIDCIssuer).To(Equal("changed"))
		g.Expect(copy.ToString()).ToNot(BeEmpty())
	})
}

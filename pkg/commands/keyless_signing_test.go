package commands

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/config"

	. "github.com/onsi/gomega"
)

func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	os.Stdout = w
	err = fn()
	if err != nil {
		return "", err
	}

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	os.Stdout = old
	return string(out), nil
}

func TestKeylessSigningResult(t *testing.T) {
	g := NewWithT(t)

	t.Run("omits empty fields when disabled", func(t *testing.T) {
		data, err := json.Marshal(&KeylessSigningResult{Enabled: false})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(MatchJSON(`{"enabled":false}`))
	})

	t.Run("includes configured fields when enabled", func(t *testing.T) {
		data, err := json.Marshal(&KeylessSigningResult{
			Enabled:    true,
			OIDCIssuer: "https://oidc.example.com",
			RekorURL:   "https://rekor.example.com",
			FulcioURL:  "https://fulcio.example.com",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(MatchJSON(`{
			"enabled": true,
			"oidc-issuer": "https://oidc.example.com",
			"rekor-url":   "https://rekor.example.com",
			"fulcio-url":  "https://fulcio.example.com"
		}`))
	})
}

func TestKeylessSigningRun(t *testing.T) {
	g := NewWithT(t)

	const (
		oidcIssuer = "https://oidc.example.com"
		rekorURL   = "https://rekor.example.com"
		fulcioURL  = "https://fulcio.example.com"
		tufURL     = "https://tuf.example.com"
	)

	originalGetKonfluxConfig := config.GetKonfluxConfig
	t.Cleanup(func() { config.GetKonfluxConfig = originalGetKonfluxConfig })

	t.Run("prints disabled when config cannot be read", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return nil, errors.New("oc error")
		}

		initializeCalled := false
		k := &KeylessSigning{
			CosignCli: &mockCosignCli{
				InitializeFunc: func(args *cliwrappers.CosignInitializeArgs) error {
					initializeCalled = true
					return nil
				},
			},
		}

		stdout, err := captureStdout(k.Run)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(initializeCalled).To(BeFalse())
		g.Expect(stdout).To(MatchJSON(`{"enabled":false}`))
	})

	t.Run("prints disabled when keyless signing config is incomplete", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{KeylessSigning: &config.KeylessSigningConfig{TufInternalURL: tufURL}}, nil
		}

		initializeCalled := false
		k := &KeylessSigning{
			CosignCli: &mockCosignCli{
				InitializeFunc: func(args *cliwrappers.CosignInitializeArgs) error {
					initializeCalled = true
					return nil
				},
			},
		}

		stdout, err := captureStdout(k.Run)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(initializeCalled).To(BeFalse())
		g.Expect(stdout).To(MatchJSON(`{"enabled":false}`))
	})

	t.Run("prints enabled result when config is valid", func(t *testing.T) {
		config.GetKonfluxConfig = func() (*config.KonfluxConfig, error) {
			return &config.KonfluxConfig{
				KeylessSigning: &config.KeylessSigningConfig{
					OIDCIssuer:        oidcIssuer,
					RekorInternalURL:  rekorURL,
					FulcioInternalURL: fulcioURL,
					TufInternalURL:    tufURL,
				},
			}, nil
		}

		initializeCalled := false
		k := &KeylessSigning{
			CosignCli: &mockCosignCli{
				InitializeFunc: func(args *cliwrappers.CosignInitializeArgs) error {
					initializeCalled = true
					return nil
				},
			},
		}

		stdout, err := captureStdout(k.Run)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(initializeCalled).To(BeTrue())

		var result KeylessSigningResult
		g.Expect(json.Unmarshal([]byte(stdout), &result)).To(Succeed())
		g.Expect(result.Enabled).To(BeTrue())
		g.Expect(result.OIDCIssuer).To(Equal(oidcIssuer))
		g.Expect(result.RekorURL).To(Equal(rekorURL))
		g.Expect(result.FulcioURL).To(Equal(fulcioURL))
	})
}

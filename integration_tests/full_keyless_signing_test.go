package integration_tests

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
	"github.com/konflux-ci/konflux-build-cli/pkg/commands"
)

const cosignImage = TaskRunnerImageRef

const konfluxConfigPath = "/tmp/konflux-config"
const oidcTokenPath = "/tmp/oidc-token"

func formatKonfluxConfigContent(sigstoreStack *SigstoreStack) string {
	return fmt.Sprintf(`
		[keyless-signing]
		enableKeylessSigning = true
		fulcioInternalUrl = %s
		rekorInternalUrl = %s
		tufInternalUrl = %s
		defaultOIDCIssuer = %s
	`, sigstoreStack.FulcioURL(),
		sigstoreStack.RekorURL(),
		sigstoreStack.TufURL(),
		sigstoreStack.OIDCIssuerURL(),
	)
}

func TestKeylessSigningWorkflow(t *testing.T) {
	SetupGomega(t)
	container := NewBuildCliRunnerContainer(GenerateUniqueTag(t), cosignImage)
	defer container.DeleteIfExists()

	var err error

	// Prepare sigstore stack
	sigstoreStack := NewSigstoreStack()
	err = sigstoreStack.Start()
	Expect(err).ToNot(HaveOccurred())
	defer sigstoreStack.Stop()

	// Prepare image registry
	imageRegistry := SetupImageRegistry(t)
	imageRepoUrl := imageRegistry.GetTestNamespace() + "test-image"
	err = CreateTestImage(TestImageConfig{ImageRef: imageRepoUrl})
	Expect(err).ToNot(HaveOccurred())
	imageDigest, err := PushImage(imageRepoUrl)
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(imageRepoUrl)

	image := fmt.Sprintf("%s@%s", imageRepoUrl, imageDigest)

	// Prepare Konflux config file
	container.AddEnv("PLATFORM_CONFIG_FILE", konfluxConfigPath)
	konfluxConfigContent := formatKonfluxConfigContent(sigstoreStack)

	// Prepare OIDC token
	token, err := sigstoreStack.GetOIDCToken()
	Expect(err).ToNot(HaveOccurred())

	// Start the container with registry integration
	err = container.StartWithRegistryIntegration(imageRegistry)
	Expect(err).ToNot(HaveOccurred())
	err = container.CreateFileInContainer(konfluxConfigPath, konfluxConfigContent)
	Expect(err).ToNot(HaveOccurred())
	err = container.CreateFileInContainer(oidcTokenPath, token)
	Expect(err).ToNot(HaveOccurred())

	// Run the `konflux-build-cli config keyless-signing` command
	stdout, _, err := container.ExecuteBuildCliWithOutput("config", "keyless-signing")
	Expect(err).ToNot(HaveOccurred())

	// Verify the command output
	var output commands.KeylessSigningResult
	Expect(json.Unmarshal([]byte(stdout), &output)).To(Succeed())
	Expect(output.Enabled).To(BeTrue())
	Expect(output.FulcioURL).To(Equal(sigstoreStack.FulcioURL()))
	Expect(output.RekorURL).To(Equal(sigstoreStack.RekorURL()))
	Expect(output.OIDCIssuer).To(Equal(sigstoreStack.OIDCIssuerURL()))

	// Run the `konflux-build-cli image sign` command
	signArgs := []string{
		"image",
		"sign",
		"--image", image,
		"--fulcio-url", output.FulcioURL,
		"--rekor-url", output.RekorURL,
		"--oidc-issuer", output.OIDCIssuer,
		"--oidc-token-path", oidcTokenPath,
	}
	err = container.ExecuteBuildCli(signArgs...)
	Expect(err).ToNot(HaveOccurred())

	verifyArgs := []string{
		"verify",
		"--certificate-identity", SigstoreMockIdentity,
		"--certificate-oidc-issuer", output.OIDCIssuer,
		"--rekor-url", output.RekorURL,
		image,
	}

	err = container.ExecuteCommand("cosign", verifyArgs...)
	Expect(err).ToNot(HaveOccurred())
}

package integration_tests

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
)

// Reimplements the ShellSpec cases from
// https://github.com/konflux-ci/build-definitions/blob/f3b87b703732efa79a19a331d47339ae586d2b40/task/tkn-bundle/0.2/spec/tkn-bundle_spec.sh.

const replacementStepImage = "registry.io/repository/replaced:latest"

// helper struct for testing
type tektonBundlePushTestCase struct {
	context       string
	home          string
	stepsImage    string
	stepNames     string
	includedTasks []string
	excludedTasks []string
	expectedSteps []string
	expectedError string
}

// creates Tekton bundles
func TestTektonBundlePush(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{})
}

// creates Tekton bundles from specific context
func TestTektonBundlePush_SpecificContext(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		context:       "sub",
		includedTasks: []string{"test3"},
		excludedTasks: []string{"test1", "test2"},
	})
}

// creates Tekton bundles when context points to a file
func TestTektonBundlePush_FileContext(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		context:       "test2.yml",
		includedTasks: []string{"test2"},
		excludedTasks: []string{"test1", "test3"},
	})
}

// creates Tekton bundles when context points to a file and a directory
func TestTektonBundlePush_FileAndDirectoryContext(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		context:       "test2.yml,sub",
		includedTasks: []string{"test2", "test3"},
		excludedTasks: []string{"test1"},
	})
}

// creates Tekton bundles when using negation
func TestTektonBundlePush_NegatedContext(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		context:       "!sub",
		includedTasks: []string{"test1", "test2"},
		excludedTasks: []string{"test3"},
	})
}

// allows overriding HOME environment variable
func TestTektonBundlePush_CustomHome(t *testing.T) {
	// Use a writable HOME; registry authentication is injected there by the runner.
	testTektonBundlePush(t, tektonBundlePushTestCase{home: "/tmp/summer-home"})
}

// replaces all step images when STEPS_IMAGE is set without STEPS_IMAGE_STEP_NAMES
func TestTektonBundlePush_ReplaceAllStepImages(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		expectedSteps: []string{
			"build=" + replacementStepImage, "test=" + replacementStepImage,
			"test2-step=" + replacementStepImage, "test3-step=" + replacementStepImage,
		},
	})
}

// replaces only the named step image when STEPS_IMAGE_STEP_NAMES targets one step
func TestTektonBundlePush_ReplaceNamedStepImage(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "build",
		expectedSteps: []string{
			"build=" + replacementStepImage, "test=alpine", "test2-step=ubuntu", "test3-step=ubuntu",
		},
	})
}

// replaces multiple named step images with comma-separated STEPS_IMAGE_STEP_NAMES
func TestTektonBundlePush_ReplaceNamedStepImagesCommaSeparated(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "build,test",
		expectedSteps: []string{
			"build=" + replacementStepImage, "test=" + replacementStepImage,
			"test2-step=ubuntu", "test3-step=ubuntu",
		},
	})
}

// excludes a single step with ! prefix in STEPS_IMAGE_STEP_NAMES
func TestTektonBundlePush_ExcludeSingleStepImage(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "!build",
		expectedSteps: []string{
			"build=ubuntu", "test=" + replacementStepImage,
			"test2-step=" + replacementStepImage, "test3-step=" + replacementStepImage,
		},
	})
}

// excludes multiple steps with ! prefix in STEPS_IMAGE_STEP_NAMES
func TestTektonBundlePush_ExcludeMultipleStepImages(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "!build,!test",
		expectedSteps: []string{
			"build=ubuntu", "test=alpine",
			"test2-step=" + replacementStepImage, "test3-step=" + replacementStepImage,
		},
	})
}

// leaves all images unchanged when STEPS_IMAGE_STEP_NAMES references a non-existent step
func TestTektonBundlePush_NonexistentStepLeavesImagesUnchanged(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "nonexistent",
		expectedSteps: []string{
			"build=ubuntu", "test=alpine", "test2-step=ubuntu", "test3-step=ubuntu",
		},
	})
}

// fails when STEPS_IMAGE_STEP_NAMES mixes include and exclude entries
func TestTektonBundlePush_MixedIncludeAndExcludeFails(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage:    replacementStepImage,
		stepNames:     "build,!test",
		expectedError: "STEPS_IMAGE_STEP_NAMES cannot mix include and exclude (!) entries",
	})
}

// replaces named step images with space-separated STEPS_IMAGE_STEP_NAMES
func TestTektonBundlePush_ReplaceNamedStepImagesSpaceSeparated(t *testing.T) {
	testTektonBundlePush(t, tektonBundlePushTestCase{
		stepsImage: replacementStepImage,
		stepNames:  "build test",
		expectedSteps: []string{
			"build=" + replacementStepImage, "test=" + replacementStepImage,
			"test2-step=ubuntu", "test3-step=ubuntu",
		},
	})
}

func testTektonBundlePush(t *testing.T, tc tektonBundlePushTestCase) {
	t.Helper()
	SetupGomega(t)

	imageRegistry := SetupImageRegistry(t)
	container := NewBuildCliRunnerContainer("tekton-bundle-push", TaskRunnerImageRef)
	if tc.home != "" {
		container.AddEnv("HOME", tc.home)
	}
	t.Cleanup(func() { container.DeleteIfExists() })
	err := container.StartWithRegistryIntegration(imageRegistry)
	Expect(err).ToNot(HaveOccurred())

	const source = "/tmp/source"
	err = container.ExecuteCommand("mkdir", "-p", source+"/sub")
	Expect(err).ToNot(HaveOccurred())

	// Cover both YAML extensions and a task in a subdirectory.
	for _, task := range []struct {
		fixture     string
		destination string
	}{
		{fixture: "test1.yaml", destination: "test1.yaml"},
		{fixture: "test2.yaml", destination: "test2.yml"},
		{fixture: "test3.yaml", destination: "sub/test3.yaml"},
	} {
		fixturePath := filepath.Join(FindRepoRoot(), "integration_tests", "tkn-bundle", task.fixture)
		err = container.CopyFileIntoContainer(fixturePath, source+"/"+task.destination)
		Expect(err).ToNot(HaveOccurred())
	}

	imageRepo := imageRegistry.GetTestNamespace() + "test-tekton-bundle"
	tag := GenerateUniqueTag(t)
	image := imageRepo + ":" + tag
	const digestPath = "/tmp/result-image-digest"
	const urlPath = "/tmp/result-image-url"

	args := []string{
		"tkn-bundle", "push",
		"--image", image,
		"--source", source,
		"--url", "https://example.com",
		"--revision", "main",
		"--result-path-image-digest", digestPath,
		"--result-path-image-url", urlPath,
	}
	if tc.context != "" {
		args = append(args, "--context", tc.context)
	}
	if tc.stepsImage != "" {
		args = append(args, "--steps-image", tc.stepsImage)
	}
	if tc.stepNames != "" {
		args = append(args, "--steps-image-step-names", tc.stepNames)
	}
	stdout, stderr, err := container.ExecuteBuildCliWithOutput(args...)
	output := stdout + "\n" + stderr
	if tc.expectedError != "" {
		Expect(err).To(HaveOccurred())
		Expect(output).To(ContainSubstring(tc.expectedError))
		exists, err := CheckManifestExistence(imageRegistry, imageRepo, tag)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "invalid step selection must fail before pushing a bundle")
		return
	}
	Expect(err).ToNot(HaveOccurred())

	includedTasks := tc.includedTasks
	if includedTasks == nil {
		includedTasks = []string{"test1", "test2", "test3"}
	}
	for _, name := range includedTasks {
		Expect(output).To(ContainSubstring("Added Task: " + name + " to image"))
	}
	for _, name := range tc.excludedTasks {
		Expect(output).ToNot(ContainSubstring("Added Task: " + name + " to image"))
	}
	Expect(output).To(ContainSubstring("Pushed Tekton Bundle to " + imageRepo))

	imageDigest, err := container.GetTaskResultValue(digestPath)
	Expect(err).ToNot(HaveOccurred())
	Expect(imageDigest).To(MatchRegexp(`^sha256:[a-f0-9]{64}$`))
	imageURL, err := container.GetTaskResultValue(urlPath)
	Expect(err).ToNot(HaveOccurred())
	Expect(imageURL).To(Equal(image))

	// Check that the tag was pushed and the reported digest matches its manifest.
	manifest, err := GetImageManifest(imageRegistry, imageRepo, tag)
	Expect(err).ToNot(HaveOccurred())
	Expect(imageDigest).To(Equal(fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))))

	if tc.expectedSteps != nil {
		// Read back the published bundle, so assertions cover the step images
		// consumers receive, including images that should remain unchanged.
		steps, _, err := container.ExecuteCommandWithOutput("tkn", "bundle", "list",
			imageRepo+"@"+imageDigest, "-o=go-template",
			"--template", `{{range .spec.steps}}{{printf "%s=%s\n" .name .image}}{{end}}`)
		Expect(err).ToNot(HaveOccurred())
		Expect(strings.Fields(steps)).To(ConsistOf(tc.expectedSteps))
	}
}

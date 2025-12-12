# Integration tests

## How to run integration tests

Integration tests are located under `integration_tests` directory.

To run specific test from terminal execute:
```bash
go test -run ^TestMyCommand$ ./integration_tests
```
or use your IDE to run or debug one.

To run all integration tests execute:
```bash
go test ./integration_tests
```

## Integration tests settings

There is `integration_tests/framework/settings.go` that holds global integration tests settings.

Available integration tests settings:
 - `Debug`: whether to run the CLI within container in debug mode.
   Useful to troubleshoot single test.
   Note, when debug mode is activated, the CLI won't run until a debugger connects to it (port `2345`).
   Example debug configuration for VSCode:
   ```json
    {
		"name": "Connect into container",
		"type": "go",
		"request": "attach",
		"mode": "remote",
		"port": 2345,
		"host": "127.0.0.1",
		"showLog": true
    }
   ```
 - `LocalRegistry`: whether to use local conteinerazed registry or quay.io.
   See [image registry for integration tests](#image-registry-for-integration-tests) section for more details.

Also, there are the following environment variables:
- `KBC_TEST_CONTAINER_TOOL` defines which container engine to use if both `docker` and `podman` installed.
- `ZOT_REGISTRY_PORT` changes the port Zot registry is run on.
  Note, after changing the port, it's required to edit or regenerate `zot-config.json`.

## Integration test structure for a command

Integration tests for each CLI sub command should have a [common run function](#common-run-function-for-a-command).
It's used to setup and run the test container the same way for all [test cases](#integration-test-cases-for-a-command).
It takes command arguments data and should return the command results, if any.
Actually it represents the CLI sub command run in CI.

### Common run function for a command

Below is a typical example of common run function for a command:
```golang
import (
    ...
	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
)

const MyCommandImage = "quay.io/konflux-ci/task-runner:latest"

type MyCommandParams struct {
	ImageRepoUrl string
	ImageDigest  string
	Tags         []string

    HashResultFilePath string
}

type MyCommandResults struct {
	Hash string
}

func RunApplyTags(myCommandParams *MyCommandParams, ...) (*MyCommandResults, error) {
	var err error

	container := NewBuildCliRunnerContainer("my-command", MyCommandImage)

	// Container settings before start, like adding volumes, environment variables, ports, etc.
    container.AddEnv("MY_ENV_VAR", "value")
    container.AddVolume("/host/path", "/container/path")

    // Run the container
	err = container.Start()
	if err != nil {
		return nil, err
	}
	defer container.Delete()

    // Post start container tuning
	err = container.CopyFileIntoContainer("/host/path", "/container/path")
	if err != nil {
		return nil, err
	}

	// Construct the command arguments
	args := []string{"my-command"}
	args = append(args, "--image-url", myCommandParams.ImageRepoUrl)
	args = append(args, "--digest", myCommandParams.ImageDigest)
	if len(myCommandParams.Tags) > 0 {
		args = append(args, "--tags")
		args = append(args, applyTagsParams.Tags...)
	}

    args = append(args, "--result-hash", myCommandParams.HashResultFilePath)

    // Run the command in the container
	err = container.ExecuteBuildCli(args...)
	if err != nil {
		return nil, err
	}

    // Extract results
    myCommandResults := &MyCommandResults{}

    hashResult, err := container.GetTaskResultValue(myCommandParams.HashResultFilePath)
    if err != nil {
        return nil, err
    }
    myCommandResults.Hash = hashResult

	return myCommandResults, nil
}

```

Another reason to have the common part is to simplify whole pipeline testing.
For more details, see [pipeline integration testing](#integration-test-structure-for-a-pipeline).

### Integration test cases for a command

Test cases should use the common [run function](#common-run-function-for-a-command).

An example of an integration test case:
```golang
import (
    ...
    "testing"
	. "github.com/onsi/gomega"
	. "github.com/konflux-ci/konflux-build-cli/integration_tests/framework"
)

func TestMyCommand(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		fmt.Printf("Test Failure: %s\n", message)
		t.FailNow() // Terminate the test immediately
	})

	err := CompileKonfluxCli()
	Expect(err).ToNot(HaveOccurred(), "failed to compile CLI")

	// Prepare test environment.
    // Includes starting other containers (e.g. registry, git),
    // creation of test files, images, etc.

	// Setup registry
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Create input data
	configFilePath, err := createConfigFile(...)
    Expect(err).ToNot(HaveOccurred())
    defer os.Remove(configFilePath)

    const imageRepoUrl = imageRegistry.GetTestNamespace() + "my-test-image"
	err = CreateTestImage(TestImageConfig{
		ImageRef: imageRepoUrl,
		Files: map[string]string{
			"/etc/config": configFilePath,
		},
	})
	Expect(err).ToNot(HaveOccurred())
	defer DeleteLocalImage(imageRepoUrl)
	imageDigest, err := PushImage(imageRepoUrl)
	Expect(err).ToNot(HaveOccurred())

	// Run the CLI command in containr
	myCommandParams := ApplyTagsParams{
		ImageRepoUrl: imageRepoUrl,
		ImageDigest:  imageDigest,
	}
	results, err := RunMyCommand(myCommandParams)
	Expect(err).ToNot(HaveOccurred())

	// Check the results
    Expect(results.Hash).To(...)
}
```

## Integration test structure for a pipeline

When each command used in the pipeline has own [common run function](#common-run-function-for-a-command),
an integration test for a pipeline will look very similar to an integration test case.
The only difference is that instead of calling one command run function, they will be called sequentially,
sometimes passing one command results as arguments to other command results.
```golang
func TestPipeline(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		fmt.Printf("Test Failure: %s\n", message)
		t.FailNow() // Terminate the test immediately
	})

	err := CompileKonfluxCli()
	Expect(err).ToNot(HaveOccurred(), "failed to compile CLI")

	// Prepare test environment
	// Setup image registry
	imageRegistry := NewImageRegistry()
	err = imageRegistry.Prepare()
	Expect(err).ToNot(HaveOccurred())
	err = imageRegistry.Start()
	Expect(err).ToNot(HaveOccurred())
	defer imageRegistry.Stop()

	// Pipeline tasks

	// Clone
	const repoUrl = "https://github.com/devfile-samples/devfile-sample-go-basic"
	gitCloneParams := &GitCloneParams{
		RepoUrl: repoUrl,
	}

	gitCloneResults, err := RunGitClone(gitCloneParams, volumeDir)

	Expect(err).ToNot(HaveOccurred())
	Expect(gitCloneResults.Url).To(Equal(repoUrl))
	Expect(gitCloneResults.SourceDir).To(Equal("devfile-sample-go-basic"))

	// Build
	imageRepoUrl := imageRegistry.GetTestNamespace() + "build-pipeline-image"
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	tag := "build-goapp-" + timestamp
	additionalTagFromLabel := "tag-from-label"
	imageBuildParams := ImageBuildParams{
		Image:      imageRepoUrl + ":" + tag,
		SourceDir:  gitCloneResults.SourceDir,
		Dockerfile: "docker/Dockerfile",
		labels:     []string{fmt.Sprintf("konflux.additional-tags=%s", additionalTagFromLabel)},

		ImageExpireAfter: "1h",
	}

	imageBuildResults, err := RunImageBuild(imageBuildParams)

	Expect(err).ToNot(HaveOccurred())
	Expect(imageBuildResults.Url).To(HavePrefix(imageRepoUrl + ":" + tag))
	Expect(imageBuildResults.Digest).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))
	builtImageExists, err := imageRegistry.CheckTagExistance(imageBuildResults.Url, tag)
	Expect(err).ToNot(HaveOccurred())
	Expect(builtImageExists).To(BeTrue(), fmt.Sprintf("built image %s:%s does not exist in registry", imageBuildResults.Url, tag))

	// Apply tags
	newTagsFromArg := []string{"test-" + timestamp, "latest"}
	applyTagsParams := ApplyTagsParams{
		ImageRepoUrl: imageRepoUrl,
		ImageDigest:  imageDigest,
		Tags:         newTagsFromArg,
	}

	err = RunApplyTags(applyTagsParams)

	Expect(err).ToNot(HaveOccurred())
	for _, tag := range append(newTagsFromArg, additionalTagFromLabel...) {
		tagExists, err := imageRegistry.CheckTagExistance(imageRepoUrl, tag)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to check for %s tag existance", tag))
		Expect(tagExists).To(BeTrue(), fmt.Sprintf("Expected %s:%s to exist", imageRepoUrl, tag))
	}
}
```

## Image registry for integration tests

It should be possible to run integration tests using any OCI compatible registry.

Currently, the following registries are supported:
- local [Zot](https://zotregistry.dev) registry running in a container
- [quay.io](https://quay.io/)

Whatever registry is used for tests, the actial implementation is incapsulated by `ImageRegistry` interface.
To change the registry, use `LocalRegistry` [test option](#integration-tests-settings).

### Using local Zot registry for integration tests

Using local Zot registry doesn't require any manual configuration.
However, in order to do automatic registry configuration, the following tools have to be available in the system:
- `htpasswd`
- `openssl`

The configuration data is saved under `zotdata` directory within `integration_tests` directory.
Typically, `zotdata` contains:
- `ca.crt` self signed root certifacte that should be added as trusted to other tools using the registry.
- `ca.key` generated private key used to create the root certifacte.
- `server.crt` descendent certificate used in Zot server.
- `server.key` privet key used by Zot server to secure connections.
- `zot-config.json` Zot registry configuration file.
- `htpasswd` auth file for Zot registry user.
- `config.json` docker config json that has credentials to push into Zot registry.
  Use `DOCKER_CONFIG=./integration_tests/zotdata/ docker push/pull localhost:5000/my-image` to access the registry.

### Using quay.io for integration tests

To use `quay.io` as registry for test, provide the following environaments variables:
- `QUAY_NAMESPACE`, for example `username` or `my-org`
- `QUAY_ROBOT_NAME`
- `QUAY_ROBOT_TOKEN`

Also, image repository for tests should be created before the test run and **be public**.
Note, one can start a test which will create the image repository (private by default) and fail (unless run in debug mode),
then the user needs to switch only the visibility in the image repository settings.

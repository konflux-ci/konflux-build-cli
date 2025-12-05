package integration_tests

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	cliWrappers "github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// Edit the following variables according to your test needs.
var Debug = false
var LocalRegistry = true

func NewImageRegistry() ImageRegistry {
	if LocalRegistry {
		return NewZotRegistry()
	}
	return NewQuatRegistry()
}

const (
	KonfluxBuildCli = "konflux-build-cli"
)

var (
	containerTool string
)

func init() {
	// Init logger
	logLevel := "info"
	logLevelEnv := os.Getenv("KBC_LOG_LEVEL")
	if logLevelEnv != "" {
		logLevel = logLevelEnv
	}
	if err := l.InitLogger(logLevel); err != nil {
		fmt.Printf("failed to init logger: %s", err.Error())
		os.Exit(2)
	}

	// Detect container tool to use
	if ct := os.Getenv("KBC_TEST_CONTAINER_TOOL"); ct != "" {
		containerTool = ct
	} else if dockerInstalled, _ := cliWrappers.CheckCliToolAvailable("docker"); dockerInstalled {
		containerTool = "docker"
	} else if podmanInstalled, _ := cliWrappers.CheckCliToolAvailable("podman"); podmanInstalled {
		containerTool = "podman"
	} else {
		l.Logger.Fatal("no container engine found")
	}
}

func FileExists(filepath string) bool {
	stat, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false
	}
	return !stat.IsDir()
}

func EnsureDirectory(dirPath string) {
	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			panic("failed to create directory: " + dirPath)
		}
	}
}

func IsKonfluxCliCompiled() bool {
	return FileExists(path.Join("../", KonfluxBuildCli))
}

func getDlvPath() (string, error) {
	goPath, isSet := os.LookupEnv("GOPATH")
	if !isSet {
		goPath = "~/go"
	}
	dlvPath := path.Join(goPath, "bin", "dlv")
	if !FileExists(dlvPath) {
		return "", fmt.Errorf("dlv is not found")
	}
	return dlvPath, nil
}

// CreateTempDir creates a directory in OS temp dir with given prefix
// and returns full path to the creted directory.
func CreateTempDir(prefix string) (string, error) {
	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	err = os.Chmod(tmpDir, 0777)
	if err != nil {
		return "", err
	}
	return tmpDir, nil
}

func SaveToTempFile(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "tmp-*")
	if err != nil {
		return "", err
	}
	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

func CreateFileWithRandomContent(fileName string, size int64) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = io.CopyN(file, rand.Reader, size); err != nil {
		return err
	}
	return nil
}

type TestImageConfig struct {
	// Image to create and push
	ImageRef string
	// Image to base onto.
	// If empty string, scratch is used.
	BaseImage string
	// Labels to add to the image
	Labels map[string]string
	// Add a ramdom data file of given size.
	// Skip generation if the value is not positive.
	Size int64
}

func CreateTestImage(config TestImageConfig) error {
	const dataFileName = "random-data.bin"

	baseImage := config.BaseImage
	if baseImage == "" {
		baseImage = "scratch"
	}

	dockerfileContent := []string{}
	dockerfileContent = append(dockerfileContent, "FROM "+baseImage)
	for labelName, labelValue := range config.Labels {
		dockerfileContent = append(dockerfileContent, fmt.Sprintf(`LABEL %s="%s"`, labelName, labelValue))
	}

	if config.Size > 0 {
		dockerfileContent = append(dockerfileContent, fmt.Sprintf("COPY %s %s", dataFileName, dataFileName))
	}

	dockerfileContentString := strings.Join(dockerfileContent, "\n")

	testImageDir, err := CreateTempDir("test-image-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(testImageDir)

	if err := os.WriteFile(path.Join(testImageDir, "Dockerfile"), []byte(dockerfileContentString), 0644); err != nil {
		return err
	}

	if config.Size > 0 {
		if err := CreateFileWithRandomContent(path.Join(testImageDir, dataFileName), config.Size); err != nil {
			return err
		}
	}

	executor := cliWrappers.NewCliExecutor()
	stdout, stderr, _, err := executor.ExecuteInDir(testImageDir, containerTool, "build", "--tag", config.ImageRef, ".")
	if err != nil {
		fmt.Printf("failed to build test image: %s\n[stdout]:\n%s\n[stderr]:\n%s\n", err.Error(), stdout, stderr)
		return err
	}

	return nil
}

var digestRegex = regexp.MustCompile(`sha256:[a-f0-9]{64}`)

// PushImage pushes given image into registry and returns its digest.
func PushImage(imageRef string) (string, error) {
	executor := cliWrappers.NewCliExecutor()
	stdout, stderr, _, err := executor.Execute(containerTool, "push", imageRef)
	if err != nil {
		fmt.Printf("failed to push test image: %s\n[stdout]:\n%s\n[stderr]:\n%s\n", err.Error(), stdout, stderr)
		return "", err
	}

	return digestRegex.FindString(stdout + "\n" + stderr), nil
}

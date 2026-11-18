package source_container_build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	"github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var log = logger.Logger.WithField("logger", "SourceContainerBuild")

// Status values of a build result.
const (
	statusSuccess = "success"
	// A build is dropped when the binary image it would describe is gone,
	// which is not a failure and does not fail the task.
	statusDrop    = "drop"
	statusFailure = "failure"
)

type BuildResult struct {
	Status                  string `json:"status"`
	Message                 string `json:"message,omitempty"`
	DependenciesIncluded    bool   `json:"dependencies_included"`
	BaseImageSourceIncluded bool   `json:"base_image_source_included"`
	ImageURL                string `json:"image_url"`
	ImageDigest             string `json:"image_digest"`
}

// imageRef returns the pushed source image as a single reference, or an empty
// string when nothing was pushed.
func (r *BuildResult) imageRef() string {
	if r.ImageURL == "" || r.ImageDigest == "" {
		return ""
	}
	return r.ImageURL + "@" + r.ImageDigest
}

// sourceImageBuildDirs collects the directories handed to gobsi: one directory
// of SRPMs, and any number of directories packed as they are.
type sourceImageBuildDirs struct {
	rpmDir       string
	extraSrcDirs []string
}

type SourceContainerBuild struct {
	Params        *Params
	GitCli        cliwrappers.GitCliInterface
	SkopeoCli     cliwrappers.SkopeoCliInterface
	GobsiCli      cliwrappers.GobsiCliInterface
	CosignCli     cliwrappers.CosignCliInterface
	ResultsWriter common.ResultsWriterInterface

	// AuthFile is the registry authentication the SBOM download borrows a
	// credential from.
	AuthFile string
}

func New(cmd *cobra.Command) (*SourceContainerBuild, error) {
	params := &Params{}
	if err := common.ParseParameters(cmd, ParamsConfig, params); err != nil {
		return nil, err
	}

	executor := cliwrappers.NewCliExecutor()

	gitCli, err := cliwrappers.NewGitCli(executor, params.SourceDir)
	if err != nil {
		return nil, err
	}
	skopeoCli, err := cliwrappers.NewSkopeoCli(executor)
	if err != nil {
		return nil, err
	}

	gobsiCli, err := cliwrappers.NewGobsiCli(executor)
	if err != nil {
		return nil, err
	}

	cosignCli, err := cliwrappers.NewCosignCli(executor)
	if err != nil {
		return nil, err
	}

	return &SourceContainerBuild{
		Params:        params,
		GitCli:        gitCli,
		SkopeoCli:     skopeoCli,
		GobsiCli:      gobsiCli,
		CosignCli:     cosignCli,
		ResultsWriter: common.NewResultsWriter(),
		AuthFile:      common.GetDefaultAuthFile(),
	}, nil
}

func (s *SourceContainerBuild) Run() error {
	common.LogParameters(ParamsConfig, s.Params)

	if err := s.validateParams(); err != nil {
		log.Errorf("error validating parameters: %s", err.Error())
		return err
	}

	result := s.build()

	if err := s.writeResults(&result); err != nil {
		return err
	}

	if result.Status == statusFailure {
		return fmt.Errorf("source container build failed: %s", result.Message)
	}

	return nil
}

func (s *SourceContainerBuild) validateParams() error {
	if !common.IsImageNameValid(common.GetImageName(s.Params.BinaryImageURL)) {
		return fmt.Errorf("binary image url '%s' is invalid", s.Params.BinaryImageURL)
	}

	if !common.IsImageDigestValid(s.Params.BinaryImageDigest) {
		return fmt.Errorf("binary image digest '%s' is invalid", s.Params.BinaryImageDigest)
	}

	if _, err := os.Stat(s.Params.SourceDir); err != nil {
		return fmt.Errorf("source directory '%s' is not accessible: %w", s.Params.SourceDir, err)
	}

	return nil
}

func (s *SourceContainerBuild) build() BuildResult {
	result := BuildResult{Status: statusSuccess}

	// The binary image may be gone before the build even starts, in which case
	// there is nothing to describe and nothing to build.
	binaryImage := common.GetImageName(s.Params.BinaryImageURL) + "@" + s.Params.BinaryImageDigest
	exists, err := s.registryHasImage(binaryImage)
	if err != nil {
		log.Errorf("Failed to look up the binary image: %s", err.Error())
		result.Status = statusFailure
		result.Message = err.Error()
		return result
	}
	if !exists {
		message := fmt.Sprintf("Binary image %s no longer exists in the registry.", binaryImage)
		log.Info(message)
		result.Status = statusDrop
		result.Message = message
		return result
	}

	if err := s.buildSourceImage(&result); err != nil {
		log.Errorf("Failed to build the source image: %s", err.Error())
		result.Status = statusFailure
		result.Message = err.Error()
	}

	return result
}

func (s *SourceContainerBuild) buildSourceImage(result *BuildResult) error {
	// The source directory is checked out by another task, and so is owned by
	// another user than the one running this command. Git refuses to work in a
	// repository owned by somebody else until it is marked safe.
	//
	// The task this replaces ran 'git config --global --add safe.directory',
	// which stays in the configuration of the user and keeps marking that
	// directory safe long after the build. Passing the setting as environment
	// marks it safe for the git commands run from here and nowhere else.
	s.GitCli.SetEnv("GIT_CONFIG_COUNT", "1")
	s.GitCli.SetEnv("GIT_CONFIG_KEY_0", "safe.directory")
	s.GitCli.SetEnv("GIT_CONFIG_VALUE_0", s.Params.SourceDir)

	workDir, err := s.prepareWorkDir()
	if err != nil {
		return err
	}
	log.Debugf("Working directory: %s", workDir)

	sibDirs := &sourceImageBuildDirs{}

	if err := s.makeSourceArchive(workDir, sibDirs); err != nil {
		return err
	}

	parentSourcesDir, err := s.handleBaseImageSources(workDir)
	if err != nil {
		return err
	}

	if s.Params.PrefetchArtifactsDir == "" {
		log.Info("Prefetch artifacts directory is not specified. Skip handling the prefetched sources")
	} else {
		included, err := s.gatherPrefetchedSources(workDir, sibDirs)
		if err != nil {
			return err
		}
		result.DependenciesIncluded = included
	}

	imageOutputDir, err := s.buildWithGobsi(workDir, sibDirs, parentSourcesDir)
	if err != nil {
		return err
	}
	// gobsi merged the sources of the base image into the built image.
	result.BaseImageSourceIncluded = parentSourcesDir != ""

	// The binary image may have been removed while the sources were collected,
	// which leaves nothing for the source image to describe.
	binaryImageRepo := common.GetImageName(s.Params.BinaryImageURL)
	exists, err := s.registryHasImage(binaryImageRepo + "@" + s.Params.BinaryImageDigest)
	if err != nil {
		return err
	}
	if !exists {
		message := fmt.Sprintf("Original image %s@%s does not exist anymore. Stop pushing the built source image.",
			s.Params.BinaryImageURL, s.Params.BinaryImageDigest)
		log.Info(message)
		result.Status = statusDrop
		result.Message = message

		return nil
	}

	result.ImageURL = generateKonfluxSourceImage(binaryImageRepo, s.Params.BinaryImageDigest)

	digest, err := s.pushToRegistry(imageOutputDir, result.ImageURL)
	if err != nil {
		return err
	}
	result.ImageDigest = digest

	return nil
}

// buildWithGobsi builds the source image from the collected sources, merging in
// the sources of the base image, and returns the OCI layout it was built into.
func (s *SourceContainerBuild) buildWithGobsi(
	workDir string, sibDirs *sourceImageBuildDirs, parentSourcesDir string,
) (string, error) {
	imageOutputDir, err := createDir(workDir, "source_image_output")
	if err != nil {
		return "", err
	}

	args := &cliwrappers.GobsiBuildArgs{
		OutputDir:    imageOutputDir,
		ExtraSrcDirs: sibDirs.extraSrcDirs,
	}

	// The directory is always prepared, but only worth handing over once some
	// SRPM has been put in it.
	if sibDirs.rpmDir != "" {
		entries, err := os.ReadDir(sibDirs.rpmDir)
		if err != nil {
			return "", err
		}
		if len(entries) > 0 {
			args.SrpmDir = sibDirs.rpmDir
		}
	}

	if parentSourcesDir != "" {
		// gobsi merges these layers in and drops any source already carried by
		// both images.
		args.MergeDirs = []string{parentSourcesDir}
	}

	if err := s.GobsiCli.Build(args); err != nil {
		return "", err
	}

	return imageOutputDir, nil
}

// prepareWorkDir returns the directory to keep the intermediate build artifacts
// in, creating it when needed.
func (s *SourceContainerBuild) prepareWorkDir() (string, error) {
	workspaceDir := s.Params.WorkspaceDir

	if workspaceDir == "" {
		dir, err := os.MkdirTemp("", "*-source-build-workspace")
		if err != nil {
			return "", fmt.Errorf("failed to create a workspace directory: %w", err)
		}
		workspaceDir = dir
	} else if resolved, err := filepath.EvalSymlinks(workspaceDir); err == nil {
		workspaceDir = resolved
	}
	log.Debugf("Workspace directory: %s", workspaceDir)

	return createDir(workspaceDir, "source-build")
}

func (s *SourceContainerBuild) writeResults(result *BuildResult) error {
	resultJson, err := s.ResultsWriter.CreateResultJson(result)
	if err != nil {
		return fmt.Errorf("failed to serialize the build result: %w", err)
	}

	results := []struct {
		value string
		path  string
	}{
		{resultJson, s.Params.ResultPathBuildResult},
		{result.ImageURL, s.Params.ResultPathSourceImageURL},
		{result.ImageDigest, s.Params.ResultPathSourceImageDigest},
		{result.imageRef(), s.Params.ResultPathImageRef},
	}
	for _, r := range results {
		if err := s.ResultsWriter.WriteResultString(r.value, r.path); err != nil {
			return err
		}
	}

	log.Infof("[result] Build result: %s", resultJson)

	return nil
}

// createDir creates a directory, and the directories leading to it, from the
// given path components.
func createDir(components ...string) (string, error) {
	path := filepath.Join(components...)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return path, nil
}

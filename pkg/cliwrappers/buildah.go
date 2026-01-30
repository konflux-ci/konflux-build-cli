package cliwrappers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var buildahLog = l.Logger.WithField("logger", "BuildahCli")

type BuildahCliInterface interface {
	Build(args *BuildahBuildArgs) error
	Push(args *BuildahPushArgs) (string, error)
}

var _ BuildahCliInterface = &BuildahCli{}

type BuildahCli struct {
	Executor CliExecutorInterface
}

func NewBuildahCli(executor CliExecutorInterface) (*BuildahCli, error) {
	buildahCliAvailable, err := CheckCliToolAvailable("buildah")
	if err != nil {
		return nil, err
	}
	if !buildahCliAvailable {
		return nil, errors.New("buildah CLI is not available")
	}

	return &BuildahCli{
		Executor: executor,
	}, nil
}

type BuildahBuildArgs struct {
	Containerfile string
	ContextDir    string
	OutputRef     string
	Secrets       []BuildahSecret
	ExtraArgs     []string
}

type BuildahSecret struct {
	Src string
	Id  string
}

// Make all paths (containerfile, context dir, secret files, ...) absolute.
func (args *BuildahBuildArgs) MakePathsAbsolute(baseDir string) error {
	ensureAbsolute := func(path *string) error {
		if filepath.IsAbs(*path) {
			return nil
		}
		abspath, err := filepath.Abs(filepath.Join(baseDir, *path))
		if err != nil {
			return fmt.Errorf("finding absolute path of %s in %s: %w", *path, baseDir, err)
		}
		*path = abspath
		return nil
	}

	err := ensureAbsolute(&args.Containerfile)
	if err != nil {
		return err
	}

	err = ensureAbsolute(&args.ContextDir)
	if err != nil {
		return err
	}

	for i := range args.Secrets {
		err = ensureAbsolute(&args.Secrets[i].Src)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *BuildahCli) Build(args *BuildahBuildArgs) error {
	if args.Containerfile == "" {
		return errors.New("containerfile path is empty")
	}
	if args.ContextDir == "" {
		return errors.New("context directory is empty")
	}
	if args.OutputRef == "" {
		return errors.New("output-ref is empty")
	}

	buildahArgs := []string{"build", "--file", args.Containerfile, "--tag", args.OutputRef}

	for _, secret := range args.Secrets {
		secretArg := "src=" + secret.Src + ",id=" + secret.Id
		buildahArgs = append(buildahArgs, "--secret="+secretArg)
	}

	// Append extra arguments before the context directory
	buildahArgs = append(buildahArgs, args.ExtraArgs...)
	// Context directory must be the last argument
	buildahArgs = append(buildahArgs, args.ContextDir)

	buildahLog.Debugf("Running command:\nbuildah %s", strings.Join(buildahArgs, " "))

	_, _, _, err := b.Executor.ExecuteWithOutput("buildah", buildahArgs...)
	if err != nil {
		buildahLog.Errorf("buildah build failed: %s", err.Error())
		return err
	}

	buildahLog.Debug("Build completed successfully")

	return nil
}

type BuildahPushArgs struct {
	Image       string
	Destination string
}

// Push an image from local storage to the registry. Return the digest of the pushed manifest.
func (b *BuildahCli) Push(args *BuildahPushArgs) (string, error) {
	if args.Image == "" {
		return "", errors.New("image arg is empty")
	}

	// Create temp file for digest
	tmpFile, err := os.CreateTemp("", "buildah-digest-")
	if err != nil {
		return "", err
	}
	digestFile := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(digestFile)

	buildahArgs := []string{"push", "--digestfile", digestFile, args.Image}
	if args.Destination != "" {
		buildahArgs = append(buildahArgs, args.Destination)
	}

	buildahLog.Debugf("Running command:\nbuildah %s", strings.Join(buildahArgs, " "))

	_, _, _, err = b.Executor.ExecuteWithOutput("buildah", buildahArgs...)
	if err != nil {
		buildahLog.Errorf("buildah push failed: %s", err.Error())
		return "", err
	}

	buildahLog.Debug("Push completed successfully")

	content, err := os.ReadFile(digestFile)
	if err != nil {
		return "", err
	}

	digest := strings.TrimSpace(string(content))
	return digest, nil
}

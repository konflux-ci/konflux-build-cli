package cliwrappers

import (
	"errors"
	"fmt"
	"strconv"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

const (
	UnsupportedOCIConfigMediaType = "application/vnd.unknown.config.v1+json"
)

// Skopeo image transports. A transport prefixes
// the image name to tell skopeo where the image lives.
const (
	// SkopeoTransportDocker addresses an image in a registry. It is the default
	// for both ends of a copy.
	SkopeoTransportDocker = "docker://"
	// SkopeoTransportOCI addresses an image in a local OCI layout directory,
	// written as "<path>" or "<path>:<reference>".
	SkopeoTransportOCI = "oci:"
)

// MissingSignatureMessage is what skopeo reports when a copy is rejected because
// the source image has no signature, but the policy requires one.
const MissingSignatureMessage = "Source image rejected: A signature was required, but no signature exists"

var skopeoLog = l.Logger.WithField("logger", "ScopeoCli")

type SkopeoCliInterface interface {
	Copy(args *SkopeoCopyArgs) error
	Inspect(args *SkopeoInspectArgs) (string, error)
}

var _ SkopeoCliInterface = &SkopeoCli{}

type SkopeoCli struct {
	Executor CliExecutorInterface
}

func NewSkopeoCli(executor CliExecutorInterface) (*SkopeoCli, error) {
	skopeoCliAvailable, err := CheckCliToolAvailable("skopeo")
	if err != nil {
		return nil, err
	}
	if !skopeoCliAvailable {
		return nil, errors.New("skopeo CLI is not available")
	}

	return &SkopeoCli{
		Executor: executor,
	}, nil
}

type SkopeoCopyArgMultiArch string

const (
	SkopeoCopyArgMultiArchSystem    SkopeoCopyArgMultiArch = "system"
	SkopeoCopyArgMultiArchAll       SkopeoCopyArgMultiArch = "all"
	SkopeoCopyArgMultiArchIndexOnly SkopeoCopyArgMultiArch = "index-only"
)

type SkopeoCopyArgs struct {
	SourceImage      string
	DestinationImage string
	// SourceTransport and DestinationTransport prefix the respective image
	// names. Both default to [SkopeoTransportDocker] when empty.
	SourceTransport      string
	DestinationTransport string
	MultiArch            SkopeoCopyArgMultiArch
	RetryTimes           int
	// PolicyPath is a trust policy for skopeo to verify signatures against,
	// and RegistriesDirPath a directory of per-registry configuration telling
	// it where the signatures of an image are published. Both are global
	// skopeo options, so they precede the subcommand. The configuration of the
	// system is used for whichever is left empty.
	PolicyPath        string
	RegistriesDirPath string
	// DigestFile is a path skopeo writes the digest of the copied image to.
	DigestFile string
	// RemoveSignatures drops signatures from the image while copying. It is
	// required when copying into an OCI layout, which cannot hold signatures.
	RemoveSignatures bool
	ExtraArgs        []string
}

func (s *SkopeoCli) Copy(args *SkopeoCopyArgs) error {
	if args.SourceImage == "" {
		return errors.New("source image is empty, image to copy from must be set")
	}
	if args.DestinationImage == "" {
		return errors.New("destination image is empty, image to copy to must be set")
	}

	scopeoArgs := []string{}

	if args.PolicyPath != "" {
		scopeoArgs = append(scopeoArgs, "--policy", args.PolicyPath)
	}
	if args.RegistriesDirPath != "" {
		scopeoArgs = append(scopeoArgs, "--registries.d", args.RegistriesDirPath)
	}

	scopeoArgs = append(scopeoArgs, "copy")

	if args.MultiArch != "" {
		scopeoArgs = append(scopeoArgs, "--multi-arch", string(args.MultiArch))
	}
	if args.RetryTimes != 0 {
		scopeoArgs = append(scopeoArgs, "--retry-times", strconv.Itoa(args.RetryTimes))
	}
	if args.DigestFile != "" {
		scopeoArgs = append(scopeoArgs, "--digestfile", args.DigestFile)
	}
	if args.RemoveSignatures {
		scopeoArgs = append(scopeoArgs, "--remove-signatures")
	}

	if len(args.ExtraArgs) != 0 {
		scopeoArgs = append(scopeoArgs, args.ExtraArgs...)
	}

	sourceTransport := args.SourceTransport
	if sourceTransport == "" {
		sourceTransport = SkopeoTransportDocker
	}
	destinationTransport := args.DestinationTransport
	if destinationTransport == "" {
		destinationTransport = SkopeoTransportDocker
	}
	scopeoArgs = append(scopeoArgs, sourceTransport+args.SourceImage, destinationTransport+args.DestinationImage)

	skopeoLog.Debugf("Running command:\n%s", shellJoin("skopeo", scopeoArgs...))

	retryer := NewRetryer(func() (string, string, int, error) {
		return s.Executor.Execute(Command("skopeo", scopeoArgs...))
	}).WithImageRegistryPreset().
		StopIfOutputContains("unauthorized").
		// A missing signature is a verdict, not a blip. Retrying cannot change it.
		StopIfOutputContains(MissingSignatureMessage)

	stdout, stderr, _, err := retryer.Run()
	if err != nil {
		skopeoLog.Errorf("skopeo copy failed: %s", err.Error())
		skopeoLog.Infof("[stdout]:\n%s", stdout)
		skopeoLog.Infof("[stderr]:\n%s", stderr)
		// Include stderr so callers can tell apart the reasons skopeo refused.
		return fmt.Errorf("%w: %s", err, stderr)
	}

	skopeoLog.Debug("[stdout]:\n" + stdout)
	skopeoLog.Debug("[stderr]:\n" + stderr)

	return nil
}

type SkopeoInspectArgs struct {
	ImageRef   string
	RetryTimes int
	Raw        bool
	NoTags     bool
	// Config outputs the image configuration (the OCI config blob) rather than
	// skopeo's own summary of the image.
	Config    bool
	Format    string
	ExtraArgs []string
}

func (s *SkopeoCli) Inspect(args *SkopeoInspectArgs) (string, error) {
	if args.ImageRef == "" {
		return "", errors.New("no image to inspect")
	}

	scopeoArgs := []string{"inspect"}

	if args.RetryTimes != 0 {
		scopeoArgs = append(scopeoArgs, "--retry-times", strconv.Itoa(args.RetryTimes))
	}
	if args.Raw {
		scopeoArgs = append(scopeoArgs, "--raw")
	}
	if args.NoTags {
		scopeoArgs = append(scopeoArgs, "--no-tags")
	}
	if args.Config {
		scopeoArgs = append(scopeoArgs, "--config")
	}
	if args.Format != "" {
		scopeoArgs = append(scopeoArgs, "--format", args.Format)
	}

	if len(args.ExtraArgs) != 0 {
		scopeoArgs = append(scopeoArgs, args.ExtraArgs...)
	}

	dockerPrefix := "docker://"
	scopeoArgs = append(scopeoArgs, dockerPrefix+args.ImageRef)

	skopeoLog.Debugf("Running command:\n%s", shellJoin("skopeo", scopeoArgs...))

	retryer := NewRetryer(func() (string, string, int, error) {
		return s.Executor.Execute(Command("skopeo", scopeoArgs...))
	}).WithImageRegistryPreset().
		StopIfOutputContains("unauthorized").
		// Stop on unsupported config media type
		StopIfOutputContains(UnsupportedOCIConfigMediaType)

	stdout, stderr, _, err := retryer.Run()
	if err != nil {
		skopeoLog.Errorf("skopeo inspect failed: %s", err.Error())
		skopeoLog.Infof("[stdout]:\n%s", stdout)
		skopeoLog.Infof("[stderr]:\n%s", stderr)
		return "", fmt.Errorf("%w: %s", err, stderr)
	}

	skopeoLog.Debug("[stdout]:\n" + stdout)
	skopeoLog.Debug("[stderr]:\n" + stderr)

	return stdout, nil
}

package cliwrappers_test

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
)

func TestNewRetryer(t *testing.T) {
	g := NewWithT(t)

	t.Run("should create new Retryer instance", func(t *testing.T) {
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			return "", "", 0, nil
		})

		g.Expect(retryer).ToNot(BeNil())
		g.Expect(retryer.Interval).To(BeNumerically(">", 0))
		g.Expect(retryer.IntervalFactor).To(BeNumerically(">", 0))
		g.Expect(retryer.MaxRetries).To(BeNumerically(">", 0))
	})
}

func TestRetryer_Config(t *testing.T) {
	g := NewWithT(t)

	cliFunc := func() (string, string, int, error) {
		return "", "", 0, nil
	}

	t.Run("should be able to set retry params", func(t *testing.T) {
		const baseInterval = 7 * time.Second
		const intervalFactor float64 = 2.5
		const maxRetries = 8

		retryer := cliwrappers.NewRetryer(cliFunc)

		retryer.
			WithBaseInterval(baseInterval).
			WithIntervalFactor(intervalFactor).
			WithMaxRetries(maxRetries)

		g.Expect(retryer.Interval).To(Equal(baseInterval))
		g.Expect(retryer.IntervalFactor).To(Equal(intervalFactor))
		g.Expect(retryer.MaxRetries).To(Equal(maxRetries))
	})

	t.Run("should be able to set constant interval", func(t *testing.T) {
		retryer := cliwrappers.NewRetryer(cliFunc)

		const constInterval = 15 * time.Second

		retryer.WithConstantInterval(constInterval)

		g.Expect(retryer.Interval).To(Equal(constInterval))
		g.Expect(retryer.IntervalFactor).To(Equal(1.0))
		g.Expect(retryer.MaxRetries).To(BeNumerically(">", 0))
	})

	t.Run("should be able to use configuration presets", func(t *testing.T) {
		configurationPresets := []func(*cliwrappers.Retryer) *cliwrappers.Retryer{
			(*cliwrappers.Retryer).WithImageRegistryPreset,
		}

		for _, preset := range configurationPresets {
			retryer := cliwrappers.NewRetryer(cliFunc)
			oldRetryer := *retryer

			preset(retryer)

			g.Expect(retryer.Interval).To(BeNumerically(">", 0))
			g.Expect(retryer.IntervalFactor).To(BeNumerically(">", 0.0))
			g.Expect(retryer.MaxRetries).To(BeNumerically(">", 0))

			g.Expect(oldRetryer).ToNot(Equal(retryer))
		}
	})
}

func TestRetryer_Run(t *testing.T) {
	g := NewWithT(t)

	t.Run("should be able to retry command execution", func(t *testing.T) {
		const failTimes = 6

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			exitCode := 1
			err := errors.New("command has failed")
			if attempt > failTimes {
				exitCode = 0
				err = nil
			}
			return "stdout", "stderr", exitCode, err
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(failTimes + 4)

		stdout, stderr, exitCode, err := retryer.Run()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(exitCode).To(Equal(0))
		g.Expect(stdout).To(Equal("stdout"))
		g.Expect(stderr).To(Equal("stderr"))
		g.Expect(attempt).To(Equal(failTimes + 1))
	})

	t.Run("should fail command execution if max attempts reached", func(t *testing.T) {
		const maxRetries = 5

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			return "", "", 10, errors.New("command has failed")
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(maxRetries)

		_, _, exitCode, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(exitCode).ToNot(Equal(0))
		g.Expect(attempt).To(Equal(maxRetries))
	})

	t.Run("should increase delay duration on failures", func(t *testing.T) {
		const succeedAtAttempt = 5

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			if attempt == succeedAtAttempt {
				return "", "", 0, nil
			}
			return "", "", 1, errors.New("command has failed")
		}).
			WithMaxRetries(10).
			// According to the retry settings below, the delays should be:
			// 4 + 20 + 100 + 500 = 624 ms
			WithConstantInterval(4 * time.Millisecond).
			WithIntervalFactor(5)

		start := time.Now()
		_, _, _, err := retryer.Run()
		elapsed := time.Since(start)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(attempt).To(Equal(succeedAtAttempt))
		g.Expect(elapsed).To(BeNumerically(">", 500*time.Millisecond))
		g.Expect(elapsed).To(BeNumerically("<", 900*time.Millisecond))

	})

	t.Run("should be able to stop retries on command exit code", func(t *testing.T) {
		const stopExitCode = 123
		const changeExitCodeAtATtempt = 4

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			exitCode := 1
			if attempt == changeExitCodeAtATtempt {
				exitCode = stopExitCode
			}
			return "", "", exitCode, errors.New("command has failed")
		}).WithConstantInterval(1*time.Millisecond).WithMaxRetries(changeExitCodeAtATtempt+5).
			WithStopExitCode(10).WithStopExitCode(stopExitCode).WithStopExitCodes(12, 15)

		_, _, exitCode, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(exitCode).To(Equal(stopExitCode))
		g.Expect(attempt).To(Equal(changeExitCodeAtATtempt))
	})

	t.Run("should be able to stop retries on regex match in stdout", func(t *testing.T) {
		const stopRegexPattern = `Error\s+[45]\d\d`
		const stopStdout = "Starting...\nProcessing...\nError 523 happened\nFailed"
		const returnStopRegexMatchAtAttempt = 3

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			stdout := "Starting...\nProcessing...\nError 345 happened\nFailed"
			if attempt == returnStopRegexMatchAtAttempt {
				stdout = stopStdout
			}
			return stdout, "failure", 1, errors.New("command has failed")
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(returnStopRegexMatchAtAttempt + 2).
			WithStopRegex(stopRegexPattern)

		stdout, _, _, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(stdout).To(Equal(stopStdout))
		g.Expect(attempt).To(Equal(returnStopRegexMatchAtAttempt))
	})

	t.Run("should be able to stop retries on regex match in stderr", func(t *testing.T) {
		const stopRegexPattern = `Error\s+[45]\d\d`
		const stopStderr = "Starting...\nProcessing...\nError 432 happened\nFailed"
		const returnStopRegexMatchAtAttempt = 2

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			stderr := "Starting...\nProcessing...\nError 345 happened\nFailed"
			if attempt == returnStopRegexMatchAtAttempt {
				stderr = stopStderr
			}
			return "working on request", stderr, 1, errors.New("command has failed")
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(returnStopRegexMatchAtAttempt + 3).
			WithStopRegex(stopRegexPattern)

		_, stderr, _, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(stderr).To(Equal(stopStderr))
		g.Expect(attempt).To(Equal(returnStopRegexMatchAtAttempt))
	})

	t.Run("should be able to stop retries on string match in stdout", func(t *testing.T) {
		const stopString = `unauthorized`
		const stopStdout = "Sending request...\n401 Unauthorized\nFailed"
		const returnStopStringAtAttempt = 1

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			stdout := "Sending request...\n503 Service Unavailable\nFailed"
			if attempt == returnStopStringAtAttempt {
				stdout = stopStdout
			}
			return stdout, "failure", 1, errors.New("command has failed")
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(returnStopStringAtAttempt + 5).
			WithStopString(stopString)

		stdout, _, _, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(stdout).To(Equal(stopStdout))
		g.Expect(attempt).To(Equal(returnStopStringAtAttempt))
	})

	t.Run("should be able to stop retries on string match in stderr", func(t *testing.T) {
		const stopString = `forbidden`
		const stopStderr = "Sending request...\n403 Forbidden\nFailed"
		const returnStopStringAtAttempt = 5

		attempt := 0
		retryer := cliwrappers.NewRetryer(func() (string, string, int, error) {
			attempt++
			stderr := "Sending request...\n503 Service Unavailable\nFailed"
			if attempt == returnStopStringAtAttempt {
				stderr = stopStderr
			}
			return "connecting", stderr, 1, errors.New("command has failed")
		}).WithConstantInterval(1 * time.Millisecond).WithMaxRetries(returnStopStringAtAttempt + 2).
			WithStopString(stopString)

		_, stderr, _, err := retryer.Run()

		g.Expect(err).To(HaveOccurred())
		g.Expect(stderr).To(Equal(stopStderr))
		g.Expect(attempt).To(Equal(returnStopStringAtAttempt))
	})
}

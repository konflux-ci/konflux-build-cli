package cliwrappers

import (
	"regexp"
	"slices"
	"time"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var retryerLog = l.Logger.WithField("logger", "Retryer")

// Backdoor for tests
var DisableRetryer bool = false

type Retryer struct {
	Interval       time.Duration
	IntervalFactor float64
	MaxRetries     int

	cliCall func() (stdout string, stderr string, errCode int, err error)

	stopExitCodes          []int
	stopErrorRegexs        []*regexp.Regexp
	stopErrorRegexPatterns []string // needed only for logging
}

func NewRetryer(cliCall func() (stdout string, stderr string, errCode int, err error)) *Retryer {
	return &Retryer{
		Interval:       1 * time.Second,
		IntervalFactor: 2,
		MaxRetries:     3,

		cliCall: cliCall,
	}
}

// Run executes the provided via constructor command with specified retries strategy.
// Returns stdout, stderr, errCode, error of the last run.
func (r *Retryer) Run() (stdout string, stderr string, errCode int, err error) {
	if DisableRetryer {
		return r.cliCall()
	}

	retryerLog.Debugf("Running with max retries %d, %v interval, %.2f interval factor", r.MaxRetries, r.Interval, r.IntervalFactor)

	delay := r.Interval
	for attempt := 1; attempt <= r.MaxRetries; attempt++ {
		stdout, stderr, errCode, err = r.cliCall()
		if err == nil {
			return
		}

		if slices.Contains(r.stopExitCodes, errCode) {
			retryerLog.Debugf("Stopping retries after attempt %d, because cli exited with stop code: %d", attempt, errCode)
			return
		}
		for i, stopRegex := range r.stopErrorRegexs {
			if stopRegex.MatchString(stdout) || stopRegex.MatchString(stderr) {
				retryerLog.Debugf("Stopping retries after attempt %d, because cli output matched stop regex: %s", attempt, r.stopErrorRegexPatterns[i])
				return
			}
		}

		retryerLog.Debugf("Attempt %d failed, output:\n[stdout]:\n%s\n[stderr]:\n%s\nWaiting %v before next retry", attempt, stdout, stderr, delay)
		time.Sleep(delay)
		delay = time.Duration(float64(delay) * r.IntervalFactor)
	}

	retryerLog.Infof("Giving up on command after %d attempts", r.MaxRetries)
	return
}

// WithBaseInterval sets the initial delay after a failure.
// The delay will be increased by IntervalFactor times after each failure.
func (r *Retryer) WithBaseInterval(baseInterval time.Duration) *Retryer {
	r.Interval = baseInterval
	return r
}

// WithIntervalFactor sets the delay increasing factor after a failure.
func (r *Retryer) WithIntervalFactor(intervalFactor float64) *Retryer {
	r.IntervalFactor = intervalFactor
	return r
}

// WithConstantInterval makes all dalays after failures of the same duration.
func (r *Retryer) WithConstantInterval(interval time.Duration) *Retryer {
	r.Interval = interval
	r.IntervalFactor = 1
	return r
}

// WithMaxRetries sets maximum number or attempts before give up and fail.
func (r *Retryer) WithMaxRetries(maxRetries int) *Retryer {
	r.MaxRetries = maxRetries
	return r
}

// WithStopExitCode adds an stop exit code.
// If command exits with such exit code, no more retry attempts performed.
func (r *Retryer) WithStopExitCode(exitCode int) *Retryer {
	r.stopExitCodes = append(r.stopExitCodes, exitCode)
	return r
}

// WithStopExitCodes adds an stop exit codes.
// If command exits with such exit code, no more retry attempts performed.
func (r *Retryer) WithStopExitCodes(exitCodes ...int) *Retryer {
	r.stopExitCodes = append(r.stopExitCodes, exitCodes...)
	return r
}

// WithStopRegex adds a stop regex.
// If command output (stdout or stderr) matches the regex, no more retry attempts performed.
func (r *Retryer) WithStopRegex(regexString string) *Retryer {
	// The given regex is not expected to be user defined.
	// Fail fast if the regex is invalid.
	stopRegex := regexp.MustCompile(regexString)
	r.stopErrorRegexs = append(r.stopErrorRegexs, stopRegex)
	r.stopErrorRegexPatterns = append(r.stopErrorRegexPatterns, regexString)
	return r
}

// WithStopString adds a stop string.
// If command output (stdout or stderr) contains the given string, no more retry attempts performed.
func (r *Retryer) WithStopString(stopString string) *Retryer {
	return r.WithStopRegex("(?i)" + regexp.QuoteMeta(stopString))
}

// WithImageRegistryPreset sets retryer parameters for interacting with an image registry scenario.
func (r *Retryer) WithImageRegistryPreset() *Retryer {
	r.Interval = 2 * time.Second
	r.IntervalFactor = 2
	r.MaxRetries = 5
	return r
}

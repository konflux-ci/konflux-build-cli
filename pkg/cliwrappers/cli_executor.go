package cliwrappers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

var executorLog = l.Logger.WithField("logger", "CliExecutor")

type Cmd struct {
	Name       string   // the name passed to [exec.Command]
	Args       []string // the args passed to [exec.Command]
	Dir        string   // same as [exec.Cmd.Dir]
	LogOutput  bool     // log stdout/stderr lines in real time
	NameInLogs string   // when logging stdout/stderr, prefix lines with this name (defaults to Name)
}

// Command creates a Cmd. Mirrors exec.Command().
func Command(name string, args ...string) Cmd {
	return Cmd{Name: name, Args: args}
}

type CliExecutorInterface interface {
	Execute(cmd Cmd) (stdout, stderr string, exitCode int, err error)
}

var _ CliExecutorInterface = &CliExecutor{}

type CliExecutor struct{}

func NewCliExecutor() *CliExecutor {
	return &CliExecutor{}
}

// Execute runs specified command with given arguments.
// Returns stdout, stderr, exit code, error
func (e *CliExecutor) Execute(c Cmd) (string, string, int, error) {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Dir = c.Dir

	if !c.LogOutput {
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		err := cmd.Run()

		return stdoutBuf.String(), stderrBuf.String(), getExitCodeFromError(err), err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to get stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to get stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", "", -1, fmt.Errorf("failed to start command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	readStream := func(linePrefix string, r io.Reader, buf *bytes.Buffer) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			executorLog.Info(linePrefix + line)
			buf.WriteString(line + "\n")
		}
	}

	nameInLogs := c.NameInLogs
	if nameInLogs == "" {
		nameInLogs = c.Name
	}

	done := make(chan struct{}, 2)
	go func() {
		readStream(nameInLogs+" [stdout] ", stdoutPipe, &stdoutBuf)
		done <- struct{}{}
	}()
	go func() {
		readStream(nameInLogs+" [stderr] ", stderrPipe, &stderrBuf)
		done <- struct{}{}
	}()

	err = cmd.Wait()
	// Wait for both output streams to finish
	<-done
	<-done

	return stdoutBuf.String(), stderrBuf.String(), getExitCodeFromError(err), err
}

func getExitCodeFromError(cmdErr error) int {
	if cmdErr == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(cmdErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return -1
}

func CheckCliToolAvailable(cliTool string) (bool, error) {
	if _, err := exec.LookPath(cliTool); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to determine availability of '%s': %w", cliTool, err)
	}
	return true, nil
}

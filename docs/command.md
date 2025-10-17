# Konflux Build CLI commands architecture

This document describes how commands designed and how to add a new one.

The CLI is built on top of `cobra` go library.
However, it's not nessesary to dig into cobra things much, because Konflux Build CLI has own wrappers over cobra parameters.

All Cobra commands are located in `cmd` package, however,
actual logic is implemented in `pkg/commands` which relies on `pkg/cliwrappers` to execute other CLIs.

## `cmd` package

In the `cmd` package, command headers are located.
The headers do not contain actual command logic and just specify information about commands, their usage, etc.
A typical command header file looks like:
```golang
package cmd

var mycommandCmd = &cobra.Command{
	Use:   "my-command",
	Short: "Short description here",
	Long: `Long, multi line description could be here.`,
	Run: func(cmd *cobra.Command, args []string) {
		myCommand, err := commands.NewMyCommand(cmd)
		if err != nil {
			l.Logger.Fatal(err)
		}
		if err := myCommand.Run(); err != nil {
			l.Logger.Fatal(err)
		}
	},
}

func init() {
	common.RegisterParameters(mycommandCmd, commands.MyCommandParamsConfig)
}
```
The `Run` function is typicall for all commands.

In the `init` function, command parameters should be registered like shown above.

Finally, the command must be added as a subcomand to another command.
It's usually done from the `init` function of the parent command, for example:
```golang
package cmd

var rootCmd = &cobra.Command{
    ...
}

func init() {
    ...
    rootCmd.AddCommand(mycommandCmd)
    ...
}
```
As the result, the command can be run with `cli my-command --args`

### Creating nested commands

It's possible to have nested commands.
For example it's possible to have `cli image build --params`.
It such case, all `image` subcommands should be groupped into own directory.
The `image` comamnd itself will not have `Run`, but just description of the commands group.
The directory structure should be:
```
 |-- cmd
     |-- root.go
     |-- image.go
     |-- image
         |-- build.go
         ...
```
`build` subcommand is added in `init` of `image.go` and `image` is added from `root`.

## `pkg/commands` package

The `commands` package contains actual logic for the commands.
Each command defines parameters and result data in separate structs.

The set of structs with parameters, results, other CLI wrappers and command constructor are typical for all commands.
```golang
package commands

import (
	...
	cliWrappers "github.com/konflux-ci/konflux-build-cli/pkg/cliwrappers"
	"github.com/konflux-ci/konflux-build-cli/pkg/common"
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// ParamsConfig defines parameters for the command.
MyCommandParamsConfig = map[string]common.Parameter{
	"url": { // Name field should be equal to the key
		Name:       "url",
		ShortName:  "u",
		EnvVarName: "URL",
		TypeKind:   reflect.String,
		Usage:      "URL to process",
		Required:   true,
	},
	"count": {
		Name:         "count",
		TypeKind:     reflect.Int,
		DefaultValue: "0",
		Usage:        "Number of ...",
	},
	"array": {
		Name:         "array",
		ShortName:    "a",
		EnvVarName:   "ARRAY",
		TypeKind:     reflect.Array,
		DefaultValue: "item1 item2",
		Usage:        "List of items to process",
	},
	"verbose": {
		Name:         "verbose",
		ShortName:    "v",
		EnvVarName:   "VERBOSE",
		TypeKind:     reflect.Bool,
		Usage:        "Activates verbose mode",
		DefaultValue: "false",
	},
}

// MyCommandParams holds parsed parameter values.
// paramName tag value must equal to the parameter name in ParamsConfig.
type MyCommandParams struct {
	Url       string   `paramName:"url"`
	Counter   string   `paramName:"count"`
	ItemArray []string `paramName:"array"`
	Verbose   bool     `paramName:"verbose"`
}

// MyCommandResultFilesPath holds the path to the file where each result must be written.
// env tag defines environment variable to read result file path from.
// If a result environment variable is not set, it fails with an error.
type MyCommandResultFilesPath struct {
	Location string `env:"RESULT_LOCATION"`
	Hash     string `env:"RESULT_HASH"`
}

type MyCommandCliWrappers struct {
	SomeCli cliWrappers.SomeCliInterface
}

type MyCommand struct {
	Params        *MyCommandParams
	Results       *MyCommandResultFilesPath
	ResultsWriter common.ResultsWriterInterface
	CliWrappers   MyCommandCliWrappers
}

func NewMyCommand(cmd *cobra.Command) (*MyCommand, error) {
	myCommand := &MyCommand{}

	params := &MyCommandParams{}
	if err := common.ParseParameters(cmd, MyCommandParamsConfig, params); err != nil {
		return nil, err
	}
	myCommand.Params = params

	results := &MyCommandResultFilesPath{}
	if err := common.ReadResultFilesPath(results); err != nil {
		return nil, err
	}
	myCommand.Results = results
	myCommand.ResultsWriter = common.NewResultsWriter(myCommand.Params.Verbose)

	if err := myCommand.initCliWrappers(); err != nil {
		return nil, err
	}

	return myCommand, nil
}

func (c *MyCommand) initCliWrappers() error {
	executor := cliWrappers.NewCliExecutor(c.Params.Verbose)

	someCli, err := cliWrappers.NewSomeCli(executor, c.Params.Verbose)
	if err != nil {
		return err
	}
	c.CliWrappers.SomeCli = someCli
	return nil
}

func (c *MyCommand) Run() error {
	if c.Params.Verbose {
		l.Logger.Infof("[param] Resource URL: %s", c.Params.Url)
		l.Logger.Infof("[param] Counter: %s", c.Params.Counter)
		if len(c.Params.ItemArray) > 0 {
			l.Logger.Infof("[param] Items: %s", strings.Join(c.Params.ItemArray, ", "))
		}
	}

	if err := c.validateParams(); err != nil {
		return err
	}

	// Command logic here

	location, err := c.CliWrappers.SomeCli.DoSomething(c.Params.Url)
	if err != nil {
		return fmt.Errorf("failed: %w", err)
	}

	if err := c.ResultsWriter.WriteResultString(location, c.Results.Location); err != nil {
		return err
	}

	if c.Params.Verbose {
		l.Logger.Infof("[result] Location: %s", location)
	}

	return nil
}
```

The parameters struct must be registered in the command header in the `cmd` package.
```golang
package cmd

var mycommandCmd = &cobra.Command{
	...
}

func init() {
	...
	common.RegisterParameters(mycommandCmd, commands.MyCommandParamsConfig)
	...
}
```

## `pkg/cliwrappers` package

The CLI often relies on another CLI tools.
The `cliwrappers` package contains utilities to delegate some work to another tools.
A typical wrapper has the following structure:
```golang
package cliwrappers

import (
	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

type GitCliInterface interface {
	SimpleClone(url, branch string) (string, error)
	Clone(args *GitCloneArgs) (*GitCloneResult, error)
}

var _ GitCliInterface = &GitCli{}

type GitCli struct {
	Executor CliExecutorInterface
	Verbose  bool
}

func NewGitCli(executor CliExecutorInterface, verbose bool) (*GitCli, error) {
	gitCliAvailable, err := CheckCliToolAvailable("git")
	if err != nil {
		return nil, err
	}
	if !gitCliAvailable {
		return nil, errors.New("git CLI is not available")
	}

	return &GitCli{
		Executor: executor,
		Verbose:  verbose,
	}, nil
}

func (g *GitCli) SimpleClone(url, branch string) (string, error) {
	...
}

type GitCloneArgs struct {
	RepoUrl    string
	Branch     string
	Depth      int
	RetryTimes int
	...
	ExtraArgs  []string
}

type GitCloneResult struct {
	Sha string
	...
}

func (g *GitCli) Clone(args *GitCloneArgs) (*GitCloneResult, error) {
	if args.RepoUrl == "" {
		return nil, errors.New("git url must be set to clone")
	}

	gitArgs := []string{"clone", url}

	if args.Branch != "" {
	    gitArgs = append(gitArgs, "--branch", branch)
	}
	if args.Depth != 0 {
		gitArgs = append(gitArgs, "--depth", strconv.Itoa(args.Depth))
	}
	...
	if len(args.ExtraArgs) != 0 {
		gitArgs = append(gitArgs, args.ExtraArgs...)
	}

	if g.Verbose {
		l.Logger.Errorf("[command]:\ngit %s",  strings.Join(gitArgs, " "))
	}

	stdout, stderr, exitCode, err := g.Executor.Execute("git", gitArgs...)
	if err != nil {
		l.Logger.Errorf("[stdout]:\n%s", stdout)
		l.Logger.Errorf("[stderr]:\n%s", stderr)
		return "", fmt.Errorf("git clone failed with exit code %d: %v", exitCode, err)
	}

	if g.Verbose {
		l.Logger.Info("[stdout]:\n" + stdout)
	}

	...
}
```

Note, for long time running commands one might want to use `Executor.ExecuteWithOutput` that prints output in real time.

## Writing unit tests

Unit tests use standard GoLang `testing` mechanism combined with `gomega` for assertions.

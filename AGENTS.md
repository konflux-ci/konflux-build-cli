This file provides guidance on the Konflux Build CLI project.
It's a set of CLI commands from which Konflux Build Pipeline is created.

The commands should be able to run:
 - locally
 - in a local container
 - in a Kubernetus pod or Tekton task

## High-Level Project Structure

- `cmd` Cobra headers for commands, no logic.
- `pkg/commands` business logic of each command.
- `pkg/cliwrappers` wrappers over external CLI tools used.
- `pkg/common` utilities shared between all commands.
- `pkg/config` utility to access global Konflux configuration.
- `docs` documentation.
- `docs/design` design / ADRs.
- `integration_tests` all integration tests.
- `integration_tests/framework` framework to run integration tests.

## Development

After making any code changes, always make sure that:
- unit tests pass
- all linters pass

## Verification

Run these commands before submitting changes:
- `make unit-test` or `go test ./pkg/...` — run all unit tests
- `make lint` — run golangci-lint (installs automatically)
- `make fmt` or `go fmt ./...` — format code

## Testing Conventions

- Use gomega with dot-import: `. "github.com/onsi/gomega"`
- Create gomega instance per test: `g := NewWithT(t)`
- Use `g.Expect(...)` for all assertions with matchers like `BeNil()`,
  `HaveOccurred()`, `Equal()`, `ContainSubstring()`
- Write mock structs by hand implementing the interface
  (no code-generation frameworks)
- Place mocks in dedicated `*_mock_test.go` or `*_mocks_test.go` files
- Include a compile-time interface check:
  `var _ Interface = &mockStruct{}`
- See `pkg/cliwrappers/cli_executor_mock_test.go` and
  `pkg/commands/cli_mocks_test.go` for examples
- Use `testutil.WriteFileTree(t, baseDir, files)` to set up file
  fixtures in tests
- Use `testutil.CaptureLogOutput(fn)` to capture and assert log output

## Platform-Specific Code

Some files use `//go:build linux` and `//go:build !linux` build
constraints. When modifying platform-specific logic, check for and
update both variants:
- `build_linux.go` / `build_other.go`
- `in_user_namespace_linux.go` / `in_user_namespace_other.go`

## References

[Documentation index](docs/index.md) which includes all docs articles.

[Full documentation on a command structure](docs/design/command.md)

[Build and run](docs/build-and-run.md)

[Unit Tests](docs/unit-tests.md)

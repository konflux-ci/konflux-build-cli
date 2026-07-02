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

## Integration Test Conventions

### Checking buildah build output (stderr)

Buildah prints each `RUN` instruction verbatim to stdout as a `STEP` line
before executing it. The CLI re-logs these lines to stderr. If a test uses
`Expect(stderr).To(ContainSubstring("some marker"))` directly, the assertion
will match the echoed instruction text even when the `RUN` command did not
actually produce that output — making the test vacuous.

**Always use `runBuildWithOutput(t, container, buildParams)` instead of
calling `container.ExecuteCommandWithOutput(...)` directly.** This helper
strips buildah `STEP` echo lines from stderr before returning the output.
It also fails the test if no `STEP` lines were found, which detects future
buildah output format changes.

```go
// CORRECT — uses the filtering helper
stderr, err := runBuildWithOutput(t, container, buildParams)
Expect(err).ToNot(HaveOccurred())
Expect(stderr).To(ContainSubstring("expected marker"))

// WRONG — would match the echoed RUN instruction
_, stderr, err := container.ExecuteCommandWithOutput(KonfluxBuildCli, args...)
Expect(stderr).To(ContainSubstring("expected marker"))
```

## References

[Documentation index](docs/index.md) which includes all docs articles.

[Full documentation on a command structure](docs/design/command.md)

[Build and run](docs/build-and-run.md)

[Unit Tests](docs/unit-tests.md)

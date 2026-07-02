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

## Integration Tests

Integration tests live in `integration_tests/`. The primary test file for the
`image build` subcommand is `integration_tests/build_test.go`.

### Buildah output behavior

Buildah echoes each `RUN` instruction verbatim to stdout as it executes. For
example, a Dockerfile line `RUN echo hello` produces a stdout line like:

```
STEP 2/3: RUN echo hello
```

In multi-stage builds, buildah prefixes the step with the stage index:

```
[1/3] STEP 2/5: RUN echo hello
```

`konflux-build-cli` re-logs buildah's stdout to stderr, adding its own
prefixes (logger timestamps, `buildah [stdout]`, etc.). The full line that
appears in stderr looks something like:

```
<logger-prefix> buildah [stdout] [1/3] STEP 2/5: RUN echo hello
```

**Implication for test assertions:** When asserting on stderr content, a
naive `ContainSubstring` check can match the echoed instruction text
rather than actual command output. For example, if a Dockerfile contains
`RUN echo secret-token`, the string `secret-token` appears in stderr
both as the echoed STEP line *and* as the command's real output. This
produces false-positive assertions that pass even when the underlying
behavior is broken.

When writing assertions on stderr, ensure they match the *output* of
commands rather than the echoed instruction line. If the two are
ambiguous, use regex matchers or filter out STEP echo lines.

### Key test helpers

- `runBuild(container, buildParams)` — runs the build and returns only
  the error. Use when you only need to check success/failure.
- `runBuildWithOutput(container, buildParams)` — runs the build and
  returns `(stdout, stderr, error)`. Use when you need to assert on
  build output. Any output filtering should happen at this level or
  above, not inside individual test cases.
- `BuildParams` — struct that configures all build flags. Each field
  maps to a CLI flag (e.g., `Hermetic` → `--hermetic`,
  `ExtraArgs` → arguments after `--`).
- `setupTestContext(t)` — creates a temporary directory for the test
  context and registers cleanup.
- `setupImageRegistry(t)` — starts a local image registry for
  push/pull tests and registers cleanup.

### Test pattern guidance

Test infrastructure changes should establish patterns that are natural
for future test authors to follow. Prefer transparent helpers that
handle buildah-specific concerns (like STEP line filtering) over clever
workarounds in individual test cases. For example, avoid using `printf`
format specifiers or other tricks to make output distinguishable — these
are not patterns anyone would naturally replicate. Instead, solve the
problem at the helper level (e.g., in `runBuildWithOutput` or a shared
filtering function) so all tests benefit automatically.

### File organization

Helpers specific to the `image build` subcommand belong in
`integration_tests/build_test.go` alongside the tests that use them.
The framework directory (`integration_tests/framework/`) is for
utilities shared across all integration test files, not for
build-specific logic.

## References

[Documentation index](docs/index.md) which includes all docs articles.

[Full documentation on a command structure](docs/design/command.md)

[Build and run](docs/build-and-run.md)

[Unit Tests](docs/unit-tests.md)

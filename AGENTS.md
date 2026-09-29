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

## Verification

Run these commands before submitting changes:
- `make unit-test` — run all unit tests
- `make lint` — run golangci-lint (installs automatically)
- `make fmt` — format code

## Code Conventions

### `validateParams()` is pure validation

Every command's `validateParams()` method checks invariants and returns
errors but never mutates `Params` fields or performs side effects.
Parameter derivation belongs in `effective*()` methods or in `run()`
steps after validation.

### `effective*()` pattern for computed values

When a parameter's runtime value differs from the user-supplied value
(e.g., combining source + context into a full path, or resolving a
sentinel to a concrete value), expose it through a trivial
`effective*()` method. See `effectiveContextDir()` and
`effectiveSourceDateEpoch()` in `pkg/commands/build.go`.

### Test placement

End-to-end behavior (parameter combinations that affect the build
outcome) should be tested in `Test_<Command>_Run`. Unit tests for
individual helpers go in dedicated test functions. Use
`Test_<Command>_validateParams` only for pure validation logic.

### Validation ordering

In `validateParams()`, check the most general and important parameters
first (output-ref, context, source) before command-specific or
feature-specific checks.

## Integration Test Conventions

When asserting on build stderr in `image build` integration tests, call
`filterBuildahSteps` first to strip buildah's echoed RUN instructions.

## Renovate / MintMaker Configuration

This repo uses MintMaker for dependency management. MintMaker provides a
platform-level Renovate config that sets `gomod.packageRules`. In Renovate's
config hierarchy, manager-level `packageRules` take precedence over global
`packageRules`. When adding Go-module-specific rules, place them under
`gomod.packageRules` in `renovate.json`, not in the top-level `packageRules`
array.

When modifying `renovate.json`, check the MintMaker config to understand which
manager-level `packageRules` are set upstream.

MintMaker's config:
https://github.com/konflux-ci/mintmaker/blob/main/config/renovate/renovate.json

## References

[Documentation index](docs/index.md) which includes all docs articles.

[Full documentation on a command structure](docs/design/command.md)

[Build and run](docs/build-and-run.md)

[Unit Tests](docs/unit-tests.md)

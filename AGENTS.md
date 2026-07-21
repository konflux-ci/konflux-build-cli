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

## Integration Test Conventions

When asserting on build stderr in `image build` integration tests, call
`filterBuildahSteps` first to strip buildah's echoed RUN instructions.

## Renovate / MintMaker Configuration

This repo uses MintMaker for dependency management. MintMaker provides a
platform-level Renovate config that sets `gomod.packageRules`. During config
merging, Renovate concatenates inherited and local rules and evaluates them
in order. A rule in MintMaker's `gomod.packageRules` cannot be overridden by
a rule in the repo's top-level `packageRules` — the override must also be in
`gomod.packageRules`.

**When to use `gomod.packageRules`:** Place a rule under `gomod.packageRules`
when it needs to override a rule set by MintMaker at the same level. For
example, MintMaker enables Go indirect dependency updates; this repo disables
them, so the disable rule must be in `gomod.packageRules` to take effect.

**When top-level `packageRules` is fine:** Rules that do not conflict with
anything in MintMaker's `gomod.packageRules` work at the top level. For
example, the Go minor/patch/digest grouping rules in this repo's top-level
`packageRules` have no conflicting counterpart upstream.

When modifying `renovate.json`, check the MintMaker config to understand which
manager-level `packageRules` are set upstream.

MintMaker's config:
https://github.com/konflux-ci/mintmaker/blob/main/config/renovate/renovate.json

## References

[Documentation index](docs/index.md) which includes all docs articles.

[Full documentation on a command structure](docs/design/command.md)

[Build and run](docs/build-and-run.md)

[Unit Tests](docs/unit-tests.md)

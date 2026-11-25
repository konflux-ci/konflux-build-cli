# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- image source-container-build: New subcommand that builds the source container
  image accompanying a binary image, replacing the Python `source-build` task.

## 1.0.1 - 2026-10-20

### Changed

- build: Stop searching for the `cachi2.env` file in the prefetch directory.
- build: Temporarily mount hardcoded Cachi2 resources for backwards compatibility.

## 1.0.0 - 2026-10-12

First SemVer release of the existing CLI. Images were already published as
`latest` and git-SHA tags; this version is also tagged `1.0.0`.

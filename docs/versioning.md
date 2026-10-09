# Versioning and releases

`konflux-build-cli` follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The released image `quay.io/konflux-ci/konflux-build-cli` is tagged with the version so
consumers (and Renovate) can tell what a given update contains and which subcommands it
affects.

## Source of truth

- `VERSION` — the current version (e.g. `1.0.0`). Single source of truth for the version.
- `CHANGELOG.md` — human-readable record of what changed in each version.

## How the released image gets its version tag

Nothing in this repo pushes to the released image directly. The flow is:

1. The build task `get-build-params` reads `VERSION` and stamps the image with:
   - annotation `org.opencontainers.image.version` (e.g. `1.0.0`)
   - labels `version` (e.g. `1.0.0`) and `version.major` (e.g. `1`)
2. The build pushes to the build registry (`quay.io/redhat-user-workloads/rhtap-build-tenant/konflux-build-cli`).
3. The Konflux release pipeline promotes the image to `quay.io/konflux-ci/konflux-build-cli`
   and applies the tags defined in the component's `ReleasePlanAdmission` (RPA), which reads
   those image labels and annotations:

```yaml
# ReleasePlanAdmission ... spec.data.mapping.components[].repositories[].tags
tags:
  - latest
  - "{{ oci_version }}"              # e.g. 1.0.0 (from org.opencontainers.image.version)
  - "v{{ labels.version.major }}"   # e.g. v1
  - "{{ git_sha }}"
```

The RPA lives in the `konflux-release-data` repository, not in this repo, at
`config/<cluster>/service/ReleasePlanAdmission/rhtap-build/konflux-build-cli.yaml`.

The released image therefore carries, for example: `1.0.0`, `v1`, `latest`, and the git SHA.
The `vN` tag floats to the latest release of that major version.

## Cutting a release

1. Bump `VERSION` to the new version.
2. Move the `## [Unreleased]` entries in `CHANGELOG.md` under the new version heading.
3. Merge to `main`. The build and release promote the image and tag it `X.Y.Z` and `vX`.

Pick the bump per SemVer, judged from a consumer's point of view:

- **MAJOR** — breaking change to a subcommand's flags, output, or behavior.
- **MINOR** — new subcommand or backward-compatible feature.
- **PATCH** — backward-compatible fix.

## CHANGELOG convention

The changelog follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). So a reader can
see which subcommands an update touches, group each version's entries by subcommand, and keep a
dedicated section for breaking changes:

```markdown
## [1.1.0] - 2026-11-01

### image build
- Added `--foo` flag to control ...

### git-clone
- Fixed handling of ... (backward compatible)

### Breaking
- `image apply-tags`: `--tags` no longer accepts ...
```

Keep an `## [Unreleased]` section at the top for changes landing between releases.

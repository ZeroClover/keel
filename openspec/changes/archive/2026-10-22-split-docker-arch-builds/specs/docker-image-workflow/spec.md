## ADDED Requirements

### Requirement: Native Architecture Docker Builds
The GitHub Actions Docker workflow SHALL build Keel images for `linux/amd64` and `linux/arm64` in separate native GitHub-hosted Linux runner executions. The `linux/amd64` build SHALL run on `ubuntu-24.04`, the `linux/arm64` build SHALL run on `ubuntu-24.04-arm`, and the workflow MUST NOT use QEMU emulation for the ARM64 build.

#### Scenario: Pull request validates both architectures
- **WHEN** a pull request workflow reaches Docker image validation after required test and lint jobs pass
- **THEN** the workflow runs native Docker builds for `linux/amd64` on `ubuntu-24.04` and `linux/arm64` on `ubuntu-24.04-arm`
- **AND** neither build pushes an image to GHCR
- **AND** the workflow does not run `docker/setup-qemu-action`

#### Scenario: Push builds both architectures before publication
- **WHEN** a non-pull-request workflow reaches Docker image publication after required test and lint jobs pass
- **THEN** the workflow runs native Docker builds for `linux/amd64` on `ubuntu-24.04` and `linux/arm64` on `ubuntu-24.04-arm`
- **AND** each architecture is built as a native single-platform image result before final multi-architecture tag publication
- **AND** the workflow does not run `docker/setup-qemu-action`

### Requirement: Architecture-Specific Docker Cache
The GitHub Actions Docker workflow SHALL keep Docker build cache entries architecture-specific when GitHub Actions cache is used for distributed Docker builds.

#### Scenario: Cache scope is separated by platform
- **WHEN** the Docker image workflow enables GitHub Actions cache for distributed `linux/amd64` and `linux/arm64` builds
- **THEN** the cache scope used for the AMD64 build is distinct from the cache scope used for the ARM64 build
- **AND** one architecture's cache export does not overwrite the other architecture's cache export

### Requirement: Multi-Architecture Manifest Publication
The GitHub Actions Docker workflow SHALL publish the final GHCR image tags as a multi-architecture manifest only after both native architecture builds succeed.

#### Scenario: Branch image publication creates a combined manifest
- **WHEN** a branch push workflow publishes Docker images
- **THEN** the final GHCR tag set is created from both the `linux/amd64` and `linux/arm64` image digests
- **AND** each published tag preserves the existing metadata-derived branch tag behavior

#### Scenario: Tag image publication preserves release tags
- **WHEN** a tag push workflow publishes Docker images
- **THEN** the final GHCR tag set preserves the existing metadata-derived tag behavior, including semver tags and `latest` for version tags
- **AND** each published tag is created from both the `linux/amd64` and `linux/arm64` image digests

### Requirement: Published Manifest Verification
The GitHub Actions Docker workflow SHALL verify every published GHCR tag before downstream release jobs run.

#### Scenario: Published tags contain both target platforms
- **WHEN** a non-pull-request workflow finishes publishing Docker image tags
- **THEN** the workflow runs `docker buildx imagetools inspect` or an equivalent Buildx inspection for each published tag
- **AND** the verification fails unless the inspected image index or manifest list contains `platform.os == linux` with `platform.architecture == amd64`
- **AND** the verification fails unless the inspected image index or manifest list contains `platform.os == linux` with `platform.architecture == arm64`

### Requirement: Chart Release Waits For Verified Image Manifest
The GitHub Actions Helm chart release workflow SHALL wait for completed Docker image manifest verification before releasing charts for version tags.

#### Scenario: Version tag release ordering
- **WHEN** a workflow runs for a `refs/tags/v*` ref
- **THEN** the Helm chart release job starts only after the Docker image manifest verification job succeeds

## Why

Keel no longer needs CGO for its container build, but the GitHub Actions Docker job still follows the old single-runner multi-platform shape that depends on QEMU for ARM64. Splitting AMD64 and ARM64 image builds onto native runners removes obsolete emulation overhead and should make multi-architecture image publication faster while keeping pull request validation on both target platforms.

## What Changes

- Update `.github/workflows/ci.yml` so Docker image builds are distributed onto native AMD64 and ARM64 runners instead of one emulated multi-platform job.
- Use Docker's maintained GitHub Builder reusable workflow to handle native runner distribution, digest publication, cache wiring, and final manifest assembly.
- Remove the Docker workflow's QEMU setup and rely on the Dockerfile's existing CGO-disabled build path.
- Verify published image tags with `docker buildx imagetools inspect` after manifest publication.
- Keep pull request Docker validation non-publishing while still validating both target architectures.
- Keep chart release ordering dependent on completed Docker image publication and verification for tagged releases.

## Capabilities

### New Capabilities
- `docker-image-workflow`: GitHub Actions builds and publishes Keel Docker images for AMD64 and ARM64 using native runners and a combined multi-architecture manifest.

### Modified Capabilities

## Impact

- Affected workflow: `.github/workflows/ci.yml`.
- Affected systems: GitHub Actions CI, GitHub Container Registry publishing, Docker image pull behavior for `linux/amd64` and `linux/arm64`, Docker-related status checks, and tag-triggered Helm chart release ordering.
- No application API, runtime configuration, database schema, or container image name changes are intended.

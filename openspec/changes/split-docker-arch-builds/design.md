## Context

The current `.github/workflows/ci.yml` has one `docker` job that runs after unit tests and UI linting on `ubuntu-latest`. That job installs QEMU, sets up Buildx, and asks `docker/build-push-action` to build `linux/amd64,linux/arm64` together. The Dockerfile already builds Keel with `CGO_ENABLED=0`, so the workflow no longer needs an emulated ARM64 build path to support CGO-backed SQLite.

GitHub-hosted Linux runners now expose explicit native labels for both target architectures: `ubuntu-24.04` for x64 and `ubuntu-24.04-arm` for ARM64. Docker's `docker/github-builder` reusable workflow already supports distributed one-platform-per-runner builds, GHCR authentication, metadata tags, cache wiring, digest-based image publication, and final manifest assembly.

## Goals / Non-Goals

**Goals:**
- Remove QEMU from the Docker image workflow.
- Build `linux/amd64` on `ubuntu-24.04` and `linux/arm64` on `ubuntu-24.04-arm`.
- Preserve existing GHCR image names and metadata-derived tags for branch, tag, semver, and latest publication.
- Keep pull request Docker validation non-publishing while validating both target architectures.
- Verify published multi-architecture tags before downstream release jobs run.
- Keep tag-triggered Helm chart release dependent on completed Docker image publication and verification.

**Non-Goals:**
- Do not change the Dockerfile's runtime image, entrypoint, exposed port, or build stages.
- Do not change application build flags beyond relying on the existing CGO-disabled Dockerfile build.
- Do not introduce image signing, SBOM publication, provenance policy changes, or Docker Build Cloud.
- Do not split the Dockerfile UI build into a separate artifact pipeline in this change.
- Do not change unit test or UI lint jobs.

## Decisions

1. Use Docker GitHub Builder instead of hand-written matrix and manifest glue.

   Replace the current Docker job with a job that calls `docker/github-builder/.github/workflows/build.yml@v1`. The reusable workflow owns Buildx setup, per-platform runner distribution, GHCR login, metadata handling, cache setup, digest publication, and manifest creation. This is a better fit than adding local upload/download-artifact and `imagetools create` scripting because the requested behavior matches the maintained Docker workflow's core path.

   The workflow should still verify the `v1` tag, current stable release, and input contract before implementation. The local workflow should keep Keel-specific policy explicit through the reusable workflow inputs rather than relying on implicit defaults where the behavior matters.

2. Pin the native runner mapping.

   Configure the reusable workflow with:

   ```yaml
   runner: |
     default=ubuntu-24.04
     linux/arm64=ubuntu-24.04-arm
   platforms: linux/amd64,linux/arm64
   distribute: true
   setup-qemu: false
   ```

   `ubuntu-latest-arm` is not a GitHub-hosted runner label, and leaving the ARM64 label abstract would push an implementation decision into the apply phase. Pinning `ubuntu-24.04` on the AMD64 side avoids asymmetry with an explicit ARM64 image label.

3. Keep Docker publication semantics while disabling out-of-scope features.

   The reusable workflow should keep the current GHCR image name and metadata tags:

   ```yaml
   meta-images: ghcr.io/${{ github.repository }}
   meta-tags: |
     type=ref,event=branch
     type=ref,event=tag
     type=semver,pattern={{version}}
     type=raw,value=latest,enable=${{ startsWith(github.ref, 'refs/tags/') }}
   ```

   It should use `output: image`, `push: ${{ github.event_name != 'pull_request' }}`, and GHCR credentials from `${{ github.actor }}` / `${{ secrets.GITHUB_TOKEN }}`. Set `sign: false` and `sbom: false` so this workflow change does not introduce signing, attestation, or SBOM behavior outside the user's requested scope.

4. Keep cache entries architecture-specific.

   If GitHub Actions cache is enabled for Docker builds, cache scope must be platform-specific. Docker GitHub Builder appends the platform suffix to its cache scope when distributed builds are enabled; the workflow should either rely on that behavior intentionally or set a base `cache-scope` that still remains platform-separated by the reusable workflow. Do not copy the old unscoped `type=gha` cache configuration into separate native builds.

5. Add an explicit manifest verification job.

   After non-pull-request image publication, add a lightweight verification job that inspects each published metadata-derived tag with `docker buildx imagetools inspect`. The job must fail unless the inspected image index or manifest list contains both `linux/amd64` and `linux/arm64`. This makes manifest completeness a release gate instead of a manual post-run check.

6. Move downstream release dependencies to the verification job.

   The chart release job currently depends on the single `docker` job. After the split, it should depend on manifest verification so tag releases do not publish Helm chart metadata before the multi-architecture image tags are both published and verified.

## Risks / Trade-offs

- GitHub-hosted ARM64 capacity can still queue during service incidents or regional pressure -> pin the documented `ubuntu-24.04-arm` label and treat CI queuing as an operational risk, not an implementation unknown.
- Pull request workflows will schedule distributed Docker build work for both platforms -> this increases job count and runner-minute consumption, but it removes ARM64 emulation and keeps both target images validated before merge.
- The Dockerfile's `yarn-build` stage produces platform-independent UI assets, but distributed per-platform image builds will execute that stage once per target platform -> accept the short-term duplication in this workflow change; a separate UI artifact pipeline would be a broader Dockerfile/workflow redesign.
- Docker-related status check names may change when replacing the current `docker` job with a reusable workflow call and verification job -> current `master` branch protection is not enabled, but required status checks should be reviewed before merging if repository settings change.
- `docker/github-builder@v1` centralizes implementation details outside this repository -> verify the tag and input contract before applying the change, and keep all Keel-specific publication policy visible in `.github/workflows/ci.yml`.

## Migration Plan

1. Replace the single Docker job with a `docker/github-builder/.github/workflows/build.yml@v1` workflow-call job that depends on `test` and `lint-ui`.
2. Configure explicit runner mapping, platforms, GHCR auth, metadata tags, cache behavior, and out-of-scope feature toggles.
3. Add a non-pull-request manifest verification job that inspects all published tags for both target platforms.
4. Update `chart-release.needs` to depend on manifest verification for tag releases.
5. Check repository required status checks before merging; no current `master` branch protection was found, but this must be rechecked if repository settings change.
6. Validate workflow syntax and run the pull request path to confirm both architecture builds execute without registry pushes.
7. Validate a non-pull-request or tag workflow run to confirm both architecture builds publish and the final GHCR tags inspect as `linux/amd64` plus `linux/arm64`.

Rollback is to restore the current single Docker job with QEMU and `platforms: linux/amd64,linux/arm64`.

## Open Questions

None.

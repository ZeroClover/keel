## 1. Workflow Restructure

- [x] 1.1 Replace the single `.github/workflows/ci.yml` Docker job with a `docker/github-builder/.github/workflows/build.yml@v1` workflow-call job that preserves the existing dependency on `test` and `lint-ui`.
- [x] 1.2 Configure the Docker build workflow with `platforms: linux/amd64,linux/arm64`, `distribute: true`, `setup-qemu: false`, and runner mapping `default=ubuntu-24.04` plus `linux/arm64=ubuntu-24.04-arm`.
- [x] 1.3 Configure the reusable workflow with `output: image`, `push: ${{ github.event_name != 'pull_request' }}`, the existing GHCR image name, and the existing metadata tag rules for branch, tag, semver, and `latest`.
- [x] 1.4 Configure GHCR authentication through the reusable workflow's `registry-auths` secret using `${{ github.actor }}` and `${{ secrets.GITHUB_TOKEN }}`.
- [x] 1.5 Keep out-of-scope reusable workflow features disabled or unchanged for this change, including `sign: false` and `sbom: false`.
- [x] 1.6 Enable or preserve GitHub Actions Docker build cache only with platform-separated cache scope; do not copy the old unscoped `type=gha` cache configuration into separate native builds.

## 2. Image Publication and Release Ordering

- [x] 2.1 Add a non-pull-request Docker manifest verification job that waits for the reusable Docker build job and inspects every published metadata-derived tag with `docker buildx imagetools inspect` or equivalent Buildx output.
- [x] 2.2 Make the verification job fail unless each inspected published tag contains both `linux/amd64` and `linux/arm64`.
- [x] 2.3 Keep pull request Docker builds validating both architectures with registry pushes disabled and without running publish-only manifest verification.
- [x] 2.4 Update the Helm chart release job so version-tag chart releases depend on completed Docker manifest verification.
- [x] 2.5 Check repository required status checks before merging; if Docker-related checks are required, update the repository setting or add an explicit aggregate check before removing the old `Build Docker Image` check name.

## 3. Verification

- [x] 3.1 Verify `docker/github-builder/.github/workflows/build.yml@v1` exists, points at the current stable v1 release, and supports the configured `with:` inputs and `registry-auths` secret.
- [x] 3.2 Run `actionlint .github/workflows/ci.yml`, parse `.github/workflows/ci.yml` as YAML, and run `git diff --check -- .github/workflows/ci.yml`.
- [x] 3.3 Inspect the workflow diff to confirm `docker/setup-qemu-action` is absent, the runner mapping uses `ubuntu-24.04` and `ubuntu-24.04-arm`, PR builds do not push images, and no workflow-level CGO re-enablement was introduced.
- [x] 3.4 Run `openspec validate split-docker-arch-builds --strict`.
- [x] 3.5 After CI runs, confirm the pull request path builds both architectures without pushing images and a non-pull-request publication path creates GHCR tags whose inspected manifest contains both `linux/amd64` and `linux/arm64`.

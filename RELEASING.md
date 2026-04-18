# Releasing

kleym uses workflow-dispatch publishing. The release workflow is triggered manually from the GitHub Actions UI, and it creates the tag, container image, and GitHub Release in a single atomic run.

## Procedure

1. Merge work to `main` via PR.
2. Wait for CI to pass on `main`.
3. Decide the next version:

   ```bash
   make release-plan
   ```

   This shows all commits since the last tag and suggests a version bump based on Conventional Commit prefixes (`feat` → minor, `fix`/maintenance → patch, breaking → major).

4. Go to **Actions → Release → Run workflow**.
   - Branch: `main`
   - Version: the `vX.Y.Z` value from step 3.
5. The workflow:
   - Validates the version format and that the tag does not already exist.
   - Runs tests.
   - Builds `dist/install.yaml` and `dist/kleym-crds.yaml`.
   - Builds and pushes a multi-arch container image to GHCR (`ghcr.io/sonda-red/kleym:vX.Y.Z` and `latest`).
   - Creates an annotated git tag and pushes it.
   - Creates a GitHub Release with auto-generated release notes from merged PR titles.

6. Consume `install.yaml` or the GHCR image from the release.

## Why workflow_dispatch

- **Immutable releases**: the repo has immutable releases enabled. If a tag is created through the GitHub UI (which auto-creates a release), the workflow cannot modify that release. `workflow_dispatch` makes the workflow the single creator of both the tag and release.
- **Reliable triggering**: `on: push: tags:` does not always fire for tags created via the GitHub UI or API. `workflow_dispatch` is explicitly invoked and always runs.
- **No race conditions**: tag creation, artifact build, and release creation happen in sequence within a single workflow run.

## Version Scheme

Versions follow [Semantic Versioning](https://semver.org/):

- **Major** (`vX.0.0`): breaking API or behavioral changes.
- **Minor** (`v0.X.0`): new features, backward compatible.
- **Patch** (`v0.0.X`): bug fixes, performance improvements, refactors.

## Release Notes

GitHub auto-generates release notes from merged PR titles and labels. Conventional Commit PR titles (enforced by the [PR title workflow](.github/workflows/pr-title.yml)) keep these notes readable.

Dependency-only updates use `chore(deps)` and do not trigger a release unless you explicitly tag.

## Release Artifacts

Each release includes:

| Artifact | Description |
|----------|-------------|
| `install.yaml` | Full operator deployment manifest (CRDs + controller + RBAC) |
| `kleym-crds.yaml` | CRD-only bundle for standalone CRD installation |
| GHCR image | `ghcr.io/sonda-red/kleym:vX.Y.Z` and `latest` |

## What Does Not Trigger a Release

- Merging to `main` — releases are only created by the workflow_dispatch trigger.
- Pushing a tag manually — the workflow does not listen for tag pushes.
- Dependency-only PRs (`chore(deps)`) — these are included in the next release only when you choose to run the workflow.

Do not create tags locally or via the GitHub UI. The release workflow is the sole creator of tags.

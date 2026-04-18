# Releasing

kleym uses tag-based publishing. Version authority is an annotated git tag in the form `vX.Y.Z`. CI never creates versions, tags, or release commits.

## Procedure

1. Merge work to `main` via PR.
2. Wait for CI to pass on `main`.
3. Create an annotated tag on the target commit:

   ```bash
   git tag -a v0.1.0 -m "v0.1.0"
   ```

4. Push the tag:

   ```bash
   git push origin v0.1.0
   ```

5. The [release workflow](.github/workflows/release.yml) runs automatically. It:
   - Verifies the tagged commit is reachable from `origin/main`.
   - Runs tests.
   - Builds `dist/install.yaml` and `dist/kleym-crds.yaml`.
   - Builds and pushes a multi-arch container image to GHCR (`ghcr.io/sonda-red/kleym:vX.Y.Z` and `latest`).
   - Creates a GitHub Release with auto-generated release notes from merged PR titles.

6. Consume `install.yaml` or the GHCR image from the release.

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

- Merging to `main` without a tag push.
- Dependency-only PRs (`chore(deps)`).
- Documentation, style, test, build, or CI changes.

No workflow pushes commits or tags back to `main`.

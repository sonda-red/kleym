# Semantic Versioning

This project uses automated semantic versioning with [semantic-release](https://semantic-release.gitbook.io/).

## How It Works

When code is merged to the `main` branch, a GitHub Action automatically:
1. Analyzes commit messages since the last release
2. Determines the next version number based on [Conventional Commits](https://www.conventionalcommits.org/)
3. Generates a changelog
4. Creates a git tag
5. Publishes a GitHub release

## Commit Message Format

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types and Version Bumps

- **Major version bump** (e.g., 1.0.0 → 2.0.0):
  - Commits with `BREAKING CHANGE:` in the footer
  - Example: 
    ```
    feat: new API endpoint
    
    BREAKING CHANGE: removed deprecated /v1/users endpoint
    ```

- **Minor version bump** (e.g., 1.0.0 → 1.1.0):
  - `feat:` - New features
  - Example: `feat(api): add InferenceTrustProfile validation`

- **Patch version bump** (e.g., 1.0.0 → 1.0.1):
  - `fix:` - Bug fixes
  - `perf:` - Performance improvements
  - `refactor:` - Code refactoring
  - `revert:` - Reverting previous changes
  - Examples:
    - `fix(controller): handle nil pointer in reconcile loop`
    - `perf(cache): optimize memory usage`

- **No version bump**:
  - `docs:` - Documentation changes
  - `style:` - Code style changes (formatting, etc.)
  - `test:` - Test additions or updates
  - `build:` - Build system changes
  - `ci:` - CI configuration changes
  - `chore:` - Other changes that don't modify src or test files

### Scopes (optional)

Scopes provide additional context:
- `api` - API changes
- `controller` - Controller changes
- `config` - Configuration changes
- `deps` - Dependency updates

### Examples

```bash
# Minor version bump
git commit -m "feat(controller): add retry logic for failed reconciliations"

# Patch version bump
git commit -m "fix(api): correct validation for InferenceTrustProfile status"

# Major version bump
git commit -m "feat(api): redesign CRD structure

BREAKING CHANGE: InferenceTrustProfile.spec.provider field renamed to InferenceTrustProfile.spec.providerConfig"

# No version bump
git commit -m "docs: update README with installation instructions"
git commit -m "test: add unit tests for controller reconciliation"
git commit -m "ci: update GitHub Actions workflow"
```

## Initial Release

If there are no previous tags, semantic-release will create version `1.0.0` on the first run.

## Skipping CI

The release commit itself includes `[skip ci]` to prevent triggering another workflow run.

## Configuration

The semantic versioning configuration is in [.releaserc.json](.releaserc.json).

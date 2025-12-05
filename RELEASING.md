# Release Process

This document describes the release process for Program Director, following [Semantic Versioning](https://semver.org/) and [Keep a Changelog](https://keepachangelog.com/) best practices.

## Semantic Versioning

Program Director follows semantic versioning (semver): `MAJOR.MINOR.PATCH`

- **MAJOR** version: Incompatible API changes or breaking changes
- **MINOR** version: New functionality added in a backward-compatible manner
- **PATCH** version: Backward-compatible bug fixes

### Examples

- `v1.0.0` → `v1.0.1`: Bug fix (PATCH)
- `v1.0.1` → `v1.1.0`: New feature (MINOR)
- `v1.1.0` → `v2.0.0`: Breaking change (MAJOR)

## Release Checklist

### 1. Update CHANGELOG.md

Move changes from `[Unreleased]` to a new version section:

```markdown
## [Unreleased]

### Added

### Changed

### Fixed

### Security

## [1.1.0] - 2025-12-15

### Added
- New feature X
- New feature Y

### Changed
- Improved Z

### Fixed
- Fixed bug A
```

Update the comparison links at the bottom:

```markdown
[Unreleased]: https://github.com/geekxflood/program-director/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/geekxflood/program-director/releases/tag/v1.1.0
[1.0.0]: https://github.com/geekxflood/program-director/releases/tag/v1.0.0
```

### 2. Update Chart.yaml (if Helm chart changed)

Update the Helm chart version in `charts/program-director/Chart.yaml`:

```yaml
version: 1.1.0  # Chart version
appVersion: "1.1.0"  # Application version
```

**Note**: Chart version and app version can differ:
- Chart version: Incremented when chart templates/configuration change
- App version: Must match the application release version

### 3. Commit Changes

```bash
git add CHANGELOG.md charts/program-director/Chart.yaml
git commit -m "chore: Prepare release v1.1.0"
git push
```

### 4. Create and Push Tag

```bash
# Create annotated tag
git tag -a v1.1.0 -m "Release v1.1.0

Brief description of the release.

See CHANGELOG.md for complete release notes.

🤖 Generated with Claude Code
"

# Push tag to trigger release workflow
git push origin v1.1.0
```

### 5. Verify Release Workflow

The GitHub Actions workflow `.github/workflows/release.yml` will automatically:

1. Extract version from the tag
2. Generate release notes from CHANGELOG.md
3. Create GitHub Release
4. Build and push multi-architecture Docker images
5. Generate and upload SBOM

Monitor the workflow at: https://github.com/geekxflood/program-director/actions

### 6. Post-Release

After release is published:

1. Verify Docker images are available:
   ```bash
   docker pull ghcr.io/geekxflood/program-director:1.1.0
   docker pull ghcr.io/geekxflood/program-director:latest
   ```

2. Test the release in a staging environment:
   ```bash
   helm upgrade program-director ./charts/program-director \
     --values charts/program-director/values-staging.yaml
   ```

3. Update production when ready:
   ```bash
   helm upgrade program-director ./charts/program-director \
     --values charts/program-director/values-production.yaml
   ```

## Version Increment Guidelines

### PATCH (1.0.0 → 1.0.1)

Use for:
- Bug fixes
- Security patches
- Documentation updates
- Performance improvements (no API changes)
- Internal refactoring

Examples:
- Fix crash in sync command
- Fix incorrect cooldown calculation
- Update README
- Optimize database queries

### MINOR (1.0.0 → 1.1.0)

Use for:
- New features (backward-compatible)
- New CLI commands or flags
- New API endpoints
- New configuration options
- Deprecations (with backward compatibility)

Examples:
- Add new `export` command
- Add `--format json` flag to existing command
- Add new HTTP endpoint `/api/v1/stats`
- Add Plex integration alongside existing Tunarr

### MAJOR (1.0.0 → 2.0.0)

Use for:
- Breaking API changes
- Removed features or commands
- Changed configuration format
- Database schema changes requiring migration
- Changed CLI command structure
- Changed HTTP API response format

Examples:
- Remove deprecated `--old-flag`
- Change config file format from YAML to TOML
- Rename `/api/v1/media` to `/api/v2/media` with different response
- Change database from SQLite to PostgreSQL only

## Hotfix Process

For urgent fixes to production:

1. Create a branch from the release tag:
   ```bash
   git checkout -b hotfix/1.0.1 v1.0.0
   ```

2. Make the fix and test thoroughly

3. Update CHANGELOG.md:
   ```markdown
   ## [1.0.1] - 2025-12-10

   ### Fixed
   - Critical bug in playlist generation causing crashes
   ```

4. Commit and tag:
   ```bash
   git commit -m "fix: Critical bug in playlist generation"
   git tag -a v1.0.1 -m "Hotfix v1.0.1"
   git push origin hotfix/1.0.1
   git push origin v1.0.1
   ```

5. Merge back to main:
   ```bash
   git checkout main
   git merge hotfix/1.0.1
   git push origin main
   ```

## Pre-release Versions

For testing before official release:

```bash
git tag -a v1.1.0-rc.1 -m "Release Candidate 1 for v1.1.0"
git push origin v1.1.0-rc.1
```

The release workflow will mark these as "pre-release" automatically (any version with `-` is considered pre-release per semver).

## Release Cadence

- **MAJOR**: As needed (rare, planned breaking changes)
- **MINOR**: Every 2-4 weeks (feature releases)
- **PATCH**: As needed (bug fixes, can be released anytime)

## Troubleshooting

### Tag Already Exists

If you need to recreate a tag:

```bash
# Delete local tag
git tag -d v1.1.0

# Delete remote tag
git push origin :refs/tags/v1.1.0

# Recreate and push
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
```

**Warning**: Only do this before the release is publicly announced!

### Release Workflow Failed

Check the GitHub Actions logs and retry:

1. Fix the issue (usually in workflow file)
2. Delete the tag and release
3. Recreate the tag to trigger workflow again

### Wrong Version Released

If wrong version was released:

1. Mark the GitHub release as "Draft" or delete it
2. Delete the Docker image tags if needed
3. Follow the hotfix process to release the correct version

## Best Practices

1. **Always update CHANGELOG.md** before creating a tag
2. **Test thoroughly** before tagging
3. **Use annotated tags** (not lightweight tags)
4. **Never rewrite released versions** - release a new version instead
5. **Keep release notes concise** but comprehensive
6. **Follow semver strictly** for predictability
7. **Coordinate MAJOR releases** with users (announce ahead)
8. **Automate what you can** (already done with GitHub Actions)

## References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [GitHub Releases Documentation](https://docs.github.com/en/repositories/releasing-projects-on-github)

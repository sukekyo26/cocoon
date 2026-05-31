<!--
Release PR: develop → main. Authored by the version-bump / pr-create skills, not by hand.
Title MUST be `feat: release vX.Y.Z` (Conventional Commits; the release commit on develop uses the same).
On merge, the VERSION change triggers `.github/workflows/release.yml`:
tag vX.Y.Z → cross-compile → SHA256SUMS → gh release.

Normal feature / fix PRs use the default `.github/pull_request_template.md` instead.
-->

## Release

`vA.B.C` → `vX.Y.Z`
<!-- semver bump: major / minor / patch — and why (Added → minor; Fixed/Changed/Removed only → patch). -->

## Released changes
<!-- Mirror, verbatim, the `## [X.Y.Z]` section just added to CHANGELOG.md / docs/CHANGELOG.ja.md.
     Keep only the categories that actually have entries; delete the empty ones.
     Categories are Added / Changed / Fixed / Removed only — no Security heading
     (security fixes live under Fixed with a `**Security**:` prefix). -->

### Added
-

### Changed
-

### Fixed
-

### Removed
-

## Breaking changes
<!-- pre-1.0 minor bumps may still break. List each BREAKING entry and its migration path
     (config key / flag / API surface + the rewrite users follow), or write "None". -->
None

## Release checklist
- [ ] `VERSION` and `internal/version/version.go` bumped to `X.Y.Z` in lockstep
- [ ] `## [X.Y.Z] - YYYY-MM-DD` added to **both** `CHANGELOG.md` and `docs/CHANGELOG.ja.md`; `[Unreleased]` left in place but empty; compare links updated in both
- [ ] `[Unreleased]` reviewed for the net diff — counterbalanced / out-of-scope entries removed (changelog skill)
- [ ] `just ci` green locally
- [ ] On merge, `release.yml` tags `vX.Y.Z` and publishes the GitHub release

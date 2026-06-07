<!--
PR title must follow Conventional Commits — examples:
  feat(cli): add `gen workspace` subcommand
  fix(plugin): reject CRLF install scripts
  feat(config)!: rename [container].os → image (also tick 💥 below)
  refactor(generate): extract shared yaml helpers
  docs: clarify plugin authoring contract
  feat: release v0.7.0           ← develop → main release PR — use .github/PULL_REQUEST_TEMPLATE/release.md

Scope = package or subsystem (cli / gen / plugin / dockerfile / config / ...).
-->

## Summary
<!-- Why this change: the problem it solves, what triggered it, and the intended outcome.
     Aim for clarity over length — a short paragraph is fine. -->


## Type of change
<!-- Tick every applicable box. KEEP EVERY LINE, even unticked ones — reviewers use the
     unchecked list to see what categories were considered. At least one box must be ticked. -->
- [ ] ✨ New feature (feat) — user-facing additions
- [ ] 🐛 Bug fix (fix) — user-visible bug repair
- [ ] 💥 Breaking change (BREAKING) — config / flag / API surface incompatible with the previous release; ALSO fill in "Breaking change details" below
- [ ] 🔒 Security fix (security) — vulnerability or hardening
- [ ] ⚡ Performance improvement (perf) — measurable end-user gain
- [ ] ♻️ Refactor (refactor) — internal cleanup with no observable behaviour change
- [ ] 📝 Docs (docs) — README / docs/*.md / inline godoc only
- [ ] ✅ Tests (test) — adding or improving tests
- [ ] 🔧 Build / CI / chore (chore) — tooling, lint config, workflow, dependency bump

## Changes
<!-- Bullet list of externally-visible diff (config / flag / output / behaviour).
     Skip implementation detail — that's `git log`'s job. -->
-

## Related issues
<!-- One entry per line. Use `Closes #N` for issues this PR resolves, `Refs #N` for related
     context. Write "None" if there isn't one. -->
None

## Breaking change details
<!-- Required when 💥 is ticked above; otherwise write "None".
     For breaking changes, describe:
       - which config key / flag / API surface changed,
       - the migration path users follow (sample cocoon.toml diff or shell snippet),
       - the matching CHANGELOG `### BREAKING` entry. -->
None

## Documentation
<!-- cocoon ships paired English / Japanese docs and CHANGELOG. Tick the boxes that apply. -->
- [ ] User-facing `docs/*.md` and `docs/*.ja.md` updated and kept in sync
- [ ] `README.md` and `docs/README.ja.md` updated when the install / entry surface changed
- [ ] No doc change needed (internal-only refactor / test / CI)

## Test plan
<!-- Tick what you actually ran. Add a short note after the line for non-default cases. -->
- [ ] `just ci` green locally (mandatory for any code change)
- [ ] `just regen-snapshots` re-run and resulting `testdata/` committed (when generators, plugin mutators, `cocoon init` output, or help golden changed)
- [ ] End-to-end round-trip verified: `cocoon init && cocoon gen && docker compose -f .devcontainer/docker-compose.yml up -d` (when generator output or plugin install scripts changed)
- [ ] Affected subcommands / plugins exercised manually — list them:
  -

## CHANGELOG
<!-- Decision rule: does this change something an end user or plugin author observes
     (configuration key, CLI flag, output file, runtime behaviour, security posture)?
       Yes → update both `CHANGELOG.md` and `docs/CHANGELOG.ja.md` `[Unreleased]` synchronously
             (Added / Fixed / Changed / Deprecated / Removed / Security / BREAKING).
       No  → tick "out of scope". Out-of-scope examples: test additions, CI workflow tweaks,
             internal refactor with no behaviour change, lint / formatter config, golden-file
             refresh, doc typo fixes that do not alter content.
     Tick exactly one box. -->
- [ ] Updated the `[Unreleased]` section of both `CHANGELOG.md` and `docs/CHANGELOG.ja.md`
- [ ] Out of CHANGELOG scope (tests / CI / refactor / lint config / etc.)

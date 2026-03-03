# Versioning and Commit Policy

## Semantic Versioning

Project versions follow `MAJOR.MINOR.PATCH`.

- MAJOR: incompatible protocol/API changes.
- MINOR: backward-compatible feature additions.
- PATCH: backward-compatible fixes and maintenance changes.

Tag format:

- `v<MAJOR>.<MINOR>.<PATCH>`
- Example: `v0.1.0`

## Conventional Commits

All commits should use conventional commit prefixes:

- `feat:` new features
- `fix:` bug fixes
- `chore:` maintenance/tooling
- `docs:` documentation changes
- `refactor:` code restructuring without behavior change
- `test:` test changes
- `ci:` CI workflow changes

Examples:

- `feat(collector): add stream dedupe index`
- `fix(agent): handle rotated file identity`
- `docs(protocol): clarify optional field behavior`

## Release Artifact Baseline

At minimum, each release must produce:

- source archive: `diag-gateway-<version>.tar.gz`
- checksum file: `diag-gateway-<version>.sha256`

Automation:

- `.github/workflows/release-artifacts.yml`
- `scripts/release/create_release_artifacts.sh`

## Compatibility Rule

Any change affecting protocol required fields or compatibility guarantees must:

1. be logged in `project_management/Decision_Matrix.md`
2. include an ADR update
3. bump version according to semantic impact

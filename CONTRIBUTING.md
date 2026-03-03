# Contributing

## Ground Rules

- Keep protocol idempotent and backward compatible.
- Enforce TLS-only communication.
- Preserve append-only log reconstruction.
- Maintain disk-backed durability and bounded memory behavior.
- Feature-flag all experimental capabilities.
- Update docs and project management artifacts in every sprint.

## Workflow

1. Align work to `project_management/Backlog.md`.
2. Implement only stories in `project_management/Current_Sprint.md`.
3. Log architectural-impact decisions in `project_management/Decision_Matrix.md`.
4. Close sprint with test summary, security summary, and architecture summary.

## Standard Commands

- `make bootstrap` to verify required repository structure.
- `make check` to run lint and tests when toolchains are present.
- `make build` to execute build pipelines when modules exist.
- `make release-artifacts VERSION=x.y.z` to create release archives and checksum manifests.

## Architecture Decisions

- Create ADRs from `docs/architecture/ADR_TEMPLATE.md`.
- Add each new ADR to `docs/architecture/ADR_INDEX.md`.

## Versioning and Commits

- Follow conventional commits and semantic versioning rules in `docs/planning/VERSIONING_AND_COMMITS.md`.

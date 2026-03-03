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

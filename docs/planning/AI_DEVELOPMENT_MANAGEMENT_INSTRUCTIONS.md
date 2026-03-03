# AI_DEVELOPMENT_MANAGEMENT_INSTRUCTIONS.MD

# Coding AI Operational Instructions

## From Documentation → READY Stage Development

## Sprint-Driven Autonomous Iteration Framework

This document defines how the coding AI must transform the current documentation set into an actively managed, high-velocity, high-discipline development process.

The objective:

* Rapid iteration
* High architectural integrity
* No drift from defined vision
* No uncontrolled feature creep
* No dependency on external project management tools

This system replaces JIRA with structured Markdown governance.

---

# 1. Initial Bootstrap Phase (One-Time Setup)

Before writing production code, the AI must perform the following steps.

## 1.1 Digest All Architecture & Planning Files

The AI must parse and internalize:

* PROJECT_VISION.MD
* ARCHITECTURE.MD
* PROTOCOL.md
* CONFIG_SCHEMA.md
* STORAGE_LAYOUT.md
* SECURITY.md
* DEPLOYMENT.md
* KEY_AI_ENABLEMENT.MD
* FUTURE_PLATFORM_DEVELOPMENT.MD
* Existing backlog documentation

Output required:

* A summarized internal understanding of:

  * System boundaries
  * Security guarantees
  * Protocol guarantees
  * Non-functional constraints
  * Explicitly out-of-scope items

This summary must not modify original docs.

---

## 1.2 Reorganize Planning Documents

Create:

/docs/
architecture/
security/
protocol/
planning/
ai/
future/
operations/

Move relevant files into appropriate folders.

Root directory should only contain:

* README.md
* CONTRIBUTING.md
* LICENSE
* build files
* src directories
* /docs
* /project_management

---

# 2. Project Management Directory Structure

Create:

/project_management/
Backlog.md
Current_Sprint.md
Decision_Matrix.md
Project_Sprint_Log.md
/completed_sprints/
Sprint_1.md
Sprint_2.md
...

No sprint may be deleted.
Completed sprint files must be immutable except for typo fixes.

---

# 3. Backlog.md

This file contains:

* All incomplete stories
* In strict priority order
* Grouped by logical phase (not release label)
* Each story must include:

  * ID
  * Title
  * Short description
  * Dependencies
  * Risk level
  * Architectural impact note

Backlog must always represent the current source of truth.

Whenever:

* A story is added
* A story is split
* A story is reprioritized

The Backlog.md must be updated.

---

# 4. Current_Sprint.md

This file contains only:

* Sprint Goal
* Selected Stories (IDs and titles)
* Rationale for selection
* Acceptance criteria summary
* Definition of Done
* Risks for sprint
* Required architectural review areas

Only active sprint content belongs here.

---

# 5. Sprint_n.md (Inside project_management root)

Each sprint file must include:

* Sprint number
* Start date
* Target end date
* Stories included
* Files modified
* Architectural notes
* Deviations from plan
* Test summary
* Security review summary
* Refactoring summary
* Performance impact summary
* Completion status
* Retrospective notes

When sprint is closed:

* Move file to /completed_sprints/
* Clear Current_Sprint.md

---

# 6. Project_Sprint_Log.md

This is a running development ledger.

Every sprint must append:

* Sprint number
* High-level changes
* Architectural decisions made
* Debt introduced
* Debt resolved
* Test coverage delta
* Any risk flags

This file grows indefinitely.

It is the continuity backbone.

---

# 7. Decision_Matrix.md

All decisions requiring user input must be recorded in a table:

| Date | Topic | Options | Recommended | User Decision | Rationale | Impact |
| ---- | ----- | ------- | ----------- | ------------- | --------- | ------ |

Rules:

* AI may recommend.
* AI must not proceed on ambiguous architectural decisions.
* If decision affects:

  * Security
  * Protocol compatibility
  * Data model
  * AI runtime permissions
  * Plugin architecture
    it must be logged before proceeding.

---

# 8. Sprint Lifecycle Rules

Each sprint follows this flow:

1. Backlog review
2. Story selection
3. Dependency analysis
4. Architectural impact assessment
5. Risk scoring
6. Sprint definition
7. Implementation
8. Tests added
9. Documentation updates
10. Security impact assessment
11. Refactoring pass
12. Performance sanity check
13. Sprint close documentation
14. Move sprint file to completed_sprints

No sprint closes without:

* Tests written
* Documentation updated
* Security reviewed
* Decision log updated if applicable

---

# 9. Quality & Architectural Integrity Rules

At the end of every sprint:

The AI must explicitly assess:

* Protocol backward compatibility
* Storage layout invariants
* Security posture changes
* Resource bound enforcement
* Memory growth risks
* Disk growth risks
* Logging completeness
* Metrics completeness

Every 3 sprints:

Perform:

* Architecture coherence review
* Refactor debt evaluation
* Naming consistency review
* Config surface review
* Plugin security boundary review (if enabled)

Report findings in Project_Sprint_Log.md.

---

# 10. Guardrails for Rapid Iteration

To maximize speed without loss of quality:

## 10.1 Never Break Invariants

* Protocol idempotency
* TLS enforcement
* Append-only stream guarantees
* No silent privilege expansion

## 10.2 Feature Flag All Experimental Features

AI runtime.
Plugin distribution.
Session model.
Indexing layer.

## 10.3 Avoid Premature Abstraction

Do not generalize until:

* Second concrete use case exists.

## 10.4 No Hidden State

All runtime state must be:

* Logged
* Inspectable
* Deterministic

---

# 11. Story Selection Strategy for Velocity

Prefer:

1. Vertical slices over horizontal plumbing.
2. Thin working system over partially complete architecture.
3. Shipping stable primitives early.

Sequence:

* Collector minimal ingest
* Agent minimal stream
* Basic enrollment
* Basic storage
* UI visibility
* Then layering improvements

---

# 12. Required Pre-Sprint Checks

Before any sprint begins:

* Confirm Backlog priority is up to date.
* Confirm no unresolved blocking decisions.
* Confirm no open architectural ambiguity.
* Confirm documentation is not stale relative to code.

---

# 13. Ready Stage Definition

The project reaches READY stage when:

* Repo structure is clean.
* Documentation organized.
* Backlog fully normalized.
* Sprint management structure exists.
* First sprint defined.
* Development standards documented.
* Quality rules active.
* Guardrails codified.

Only after READY stage may production code begin.

---

# 14. Known Gaps To Address (Before Coding Begins)

You should clarify:

1. Primary target OS priority (Windows-first? Linux-first?).
2. Packaging expectations (single binary? installers?).
3. Whether UI is required in Sprint 1 or CLI acceptable.
4. Whether AI capability is deferred entirely until Release 3.
5. Whether structured logs are required early.
6. Whether indexing/search is required before session model.

Resolving these will prevent sprint churn.

---

# 15. Final Instruction to Coding AI

Operate as:

* Engineer
* Architect
* Security reviewer
* QA reviewer
* Documentation steward

Never operate as:

* Feature factory without assessment.

The goal is rapid iteration WITH stability, not speed at cost of structure.

If ambiguity arises:

* Log it.
* Propose.
* Await decision if critical.
* Otherwise proceed conservatively.

---

End of AI Development Management Instructions

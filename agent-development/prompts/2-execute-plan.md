# Prompt: Execute an Approved Plan

> **Usage:** Copy this prompt into a new agent conversation. Replace `<PLAN_FILE>` with a reference to the approved plan file sitting in `queued/` (e.g., `@0-initialization-plan.md`).

---

Familiarize yourself with the project context by reading all documents in [@agent-specs](agent-specs):

- `agent-specs/agent-instructions.md`
- `agent-specs/application-overview.md`
- `agent-specs/architecture-breakdown.md`

Then, read the following approved implementation plan:

**→ `<PLAN_FILE>`**

Execute every step in the plan. Follow it precisely — the plan has already been reviewed and approved by the project owner.

## Rules

1. **Read all `agent-specs/` documents first** — they contain coding standards, design patterns, and constraints you must follow.
2. **Read the plan in full before writing any code** — understand the complete scope before starting.
3. **Execute steps in order** — the plan's steps are sequenced intentionally. Do not skip ahead or reorder.
4. **Follow the plan exactly** — do not add features, refactor existing code, or make architectural decisions that aren't in the plan. If something seems wrong or missing, flag it but still follow the plan as written.
5. **Run all verification checks** — the plan includes verification commands and checklists. Run every single one and confirm they pass before finishing.
6. **If a verification fails**, fix the issue and re-run the check. Make at most 2 attempts per check; if it still fails, report the failure clearly.

## Post-Execution Housekeeping

After all steps are complete and all verifications pass, perform these file moves:

1. **Move the plan to done:** Move the executed plan file from `agent-development/queued/` to `agent-development/done/plans/`.
2. **Move the request to done:** Find the corresponding request file in `agent-development/pending/` (the plan references it — match by task number) and move it to `agent-development/done/requests/`.

For example, if you executed `agent-development/queued/0-initialization-plan.md`, then:
- Move `agent-development/queued/0-initialization-plan.md` → `agent-development/done/plans/0-initialization-plan.md`
- Move `agent-development/pending/0-initialization.md` → `agent-development/done/requests/0-initialization.md`

## Final Report

Once everything is done, provide a short summary:

- ✅ List of steps completed
- ⚠️ Any warnings or issues encountered (even if resolved)
- 📁 Files created or modified
- 🔀 Files moved (plan and request)
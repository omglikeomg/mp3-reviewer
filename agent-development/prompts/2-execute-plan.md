# Prompt: Execute an Approved Plan

> **Usage:** Copy this prompt into a new agent conversation. Replace `<PLAN_FILE>` with a reference to the approved plan file sitting in `queued/` (e.g., `@0-initialization-plan.md`).

---

Familiarize yourself with the project context by reading all documents in [@agent-specs](agent-specs):

- `agent-specs/agent-instructions.md`
- `agent-specs/application-overview.md`
- `agent-specs/architecture-breakdown.md`

Also read the development workflow guide:

- `agent-development/DEVELOPMENT-GUIDE.md`

Then, read the following approved implementation plan:

**→ `<PLAN_FILE>`**

Execute every step in the plan. Follow it precisely — the plan has already been reviewed and approved by the project owner.

## Pre-Execution: Verify Open Questions Are Resolved

Before writing any code, locate the **"Open Questions & Decisions"** section in the plan.

1. **If the section says "None"** — proceed to execution.
2. **If there are questions listed**, check that **every** question has its `Human decision:` field filled in (i.e., it no longer says `PENDING`). The human answers these during the approval process before moving the plan to `queued/`.
3. **If any question still says `PENDING`** — **STOP. Do NOT execute the plan.** Report which questions are unresolved and ask the user to answer them first. A plan with unresolved questions should not be in `queued/`.

When questions are resolved, treat the human's decisions as **binding requirements** — they override the agent's original recommendation. Incorporate those decisions into your implementation wherever relevant.

## Rules

1. **Read all `agent-specs/` documents first** — they contain coding standards, design patterns, and constraints you must follow.
2. **Read the plan in full before writing any code** — understand the complete scope before starting.
3. **Execute steps in order** — the plan's steps are sequenced intentionally. Do not skip ahead or reorder.
4. **Follow the plan exactly** — do not add features, refactor existing code, or make architectural decisions that aren't in the plan. If something seems wrong or missing, flag it but still follow the plan as written.
5. **Respect human decisions** — if the "Open Questions & Decisions" section contains resolved questions, follow the human's chosen option exactly, even if the agent's original recommendation differed.
6. **Run all verification checks** — the plan includes verification commands and checklists. Run every single one and confirm they pass before finishing.
7. **If a verification fails**, fix the issue and re-run the check. Make at most 2 attempts per check; if it still fails, report the failure clearly.

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
- 🔑 Open Questions — list each resolved question and the human's decision that was applied
- ⚠️ Any warnings or issues encountered (even if resolved)
- 📁 Files created or modified
- 🔀 Files moved (plan and request)
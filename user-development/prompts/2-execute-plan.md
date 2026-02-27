# Prompt: Execute an Approved Plan

> **Usage:** Copy this prompt into a new agent conversation. Replace `<PLAN_FILE>` with a reference to the approved plan file sitting in `queued/` (e.g., `@0-initialization-plan.md`).

---

Read and execute the approved plan at **→ `<PLAN_FILE>`**

## Before Starting

- Check the **"Open Questions & Decisions"** section in the plan. If any question still says `PENDING`, **STOP** and report which questions are unresolved. Do not execute a plan with pending decisions.
- Treat resolved human decisions as **binding requirements** — they override the agent's original recommendation.

## While Executing

- Follow the plan exactly. Do not add features, refactor code, or make architectural decisions that aren't in the plan.
- Execute steps in order. Run all verification checks listed in the plan.
- If a verification fails, fix the issue and retry (max 2 attempts per check). If it still fails, report the failure clearly and continue with the remaining steps.

## After All Steps Pass

Update any diagrams in `diagrams/` that were affected by the changes made during execution. These are documentation for human readers — add new types, methods, state transitions, packages, or architectural components to the relevant `.mmd` files so the visual docs stay current. The source code is the source of truth; diagrams are supplementary.

Then perform these file moves:

1. **Move the plan:** `agent-development/queued/<plan-file>` → `agent-development/done/plans/<plan-file>`
2. **Move the matching request:** `agent-development/pending/<N>-<name>.md` → `agent-development/done/requests/<N>-<name>.md` (match by task number referenced in the plan)

## Final Report

Provide a short summary:

- ✅ Steps completed
- 🔑 Open questions — list each resolved question and the human decision that was applied
- ⚠️ Warnings or issues encountered (even if resolved)
- 📁 Files created or modified
- 📊 Diagrams updated (documentation for human readers)
- 🔀 Files moved (plan and request)
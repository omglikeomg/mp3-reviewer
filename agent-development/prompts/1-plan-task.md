# Prompt: Plan a Task

> **Usage:** Copy this prompt into a new agent conversation. Replace `<TASK_FILE>` with a reference to the pending request file (e.g., `@0-initialization.md`).

---

Familiarize yourself with the project context by reading all documents in [@agent-specs](agent-specs):

- `agent-specs/agent-instructions.md`
- `agent-specs/application-overview.md`
- `agent-specs/architecture-breakdown.md`

Then, read the following task request:

**→ `<TASK_FILE>`**

Using the request and the context you gathered, create a **detailed implementation plan** following the template at `@_TEMPLATE-plan.md` in the `agent-development/plans/` directory.

## Rules

1. **Check the current project state** — look at the existing directory structure, existing source files, `go.mod`, and any previously completed plans in `agent-development/done/plans/` to understand what has already been built. Your plan must build on top of the current state, not conflict with it.
2. **Follow the plan template exactly** — use `agent-development/plans/_TEMPLATE-plan.md` as your structural guide.
3. **Be exhaustive** — another AI agent will read this plan and implement it. It will have no context beyond the plan itself and the `agent-specs/` documents. Every file to create/modify, every function signature, every shell command must be spelled out.
4. **Name the plan file** using the pattern `N-short-name-plan.md` where `N` matches the task number from the request filename (e.g., task `0-initialization.md` → plan `0-initialization-plan.md`).
5. **Save the plan** in `agent-development/plans/`.
6. **Do NOT implement any code.** This prompt is only for planning.

## Open Questions & Decisions (IMPORTANT)

The plan template includes an **"Open Questions & Decisions"** section. You **must** populate this section thoughtfully:

- **Surface any ambiguity** — if the task request is vague about a design choice, API contract, naming convention, data format, or trade-off, write it up as a question.
- **Present options** — for each question, list the realistic options with pros/cons.
- **Give a recommendation** — state which option you'd choose and why, but mark the human decision as `PENDING`.
- **If there are genuinely no open questions**, write "None — this plan is fully self-contained." and briefly explain why.

The human will review this section during the **approval process**. They will answer each question, and only then move the plan from `plans/` → `queued/` for execution. Do NOT assume answers to open questions — leave them for the human.
# Prompt: Plan a Task

> **Usage:** Copy this prompt into a new agent conversation. Replace `<TASK_FILE>` with a reference to the pending request file (e.g., `@5-api-integration-and-enrichment.md`).

---

Familiarize yourself with the project context by reading all documents in `agent-development/agent-specs/`:

- `agent-development/agent-specs/agent-instructions.md`
- `agent-development/agent-specs/application-overview.md`
- `agent-development/agent-specs/architecture-breakdown.md`

Then, read the following task request:

**→ `<TASK_FILE>`**

Using the request and the context you gathered, create a **detailed implementation plan** following the template at `@_TEMPLATE-plan.md` in the `agent-development/plans/` directory.

## Rules

1. **Read all diagrams first** — before inspecting any source code, read all diagram files in `diagrams/` (`data-structures.mmd`, `software-architecture.mmd`, `ui-state-machine.mmd`, `task-lifecycle.mmd`, `component-data-flow.mmd`) and `diagrams/FOLDER-STRUCTURE.md` for project orientation. Self-assess your confidence in understanding the relevant areas on a scale of 1–10. Only proceed to inspecting source files if your confidence is **below 9/10**. Focus source-code reading on the specific areas where your understanding is lacking.
2. **Check the current project state** — look at the existing directory structure, existing source files, `go.mod`, and any previously completed plans in `agent-development/done/plans/` to understand what has already been built. Your plan must build on top of the current state, not conflict with it.
3. **Follow the plan template exactly** — use `agent-development/plans/_TEMPLATE-plan.md` as your structural guide.
4. **Be exhaustive** — another AI agent will read this plan and implement it. It will have no context beyond the plan itself and the `agent-development/agent-specs/` documents. Every file to create/modify, every function signature, every shell command must be spelled out.
5. **Name the plan file** using the pattern `N-short-name-plan.md` where `N` matches the task number from the request filename (e.g., task `5-api-integration-and-enrichment.md` → plan `5-api-integration-and-enrichment-plan.md`).
6. **Save the plan** in `agent-development/plans/`.
7. **Include a diagram update section** — the plan must include a section or checklist item specifying which diagrams in `diagrams/` need to be created or updated as part of this task, and what changes are expected (new types, new states, new dependencies, etc.).
8. **Do NOT implement any code.** This prompt is only for planning.

## Open Questions & Decisions (IMPORTANT)

The plan template includes an **"Open Questions & Decisions"** section. You **must** populate this section thoughtfully:

- **Surface any ambiguity** — if the task request is vague about a design choice, API contract, naming convention, data format, or trade-off, write it up as a question.
- **Present options** — for each question, list the realistic options with pros/cons.
- **Give a recommendation** — state which option you'd choose and why, but mark the human decision as `PENDING`.
- **If there are genuinely no open questions**, write "None — this plan is fully self-contained." and briefly explain why.

The human will review this section during the **approval process**. They will answer each question, and only then move the plan from `plans/` → `queued/` for execution. Do NOT assume answers to open questions — leave them for the human.
# Prompt: Request a New Feature

> **Usage:** Copy this prompt into a new agent conversation. Replace `<FEATURE_DESCRIPTION>` with a plain-language description of the feature you want added to the project.

---

Familiarize yourself with the project context by reading all documents in [@agent-specs](agent-specs):

- `agent-specs/agent-instructions.md`
- `agent-specs/application-overview.md`
- `agent-specs/architecture-breakdown.md`

Then, review the existing task requests to understand the current scope and numbering:

- List all files in `agent-development/pending/`
- List all files in `agent-development/done/requests/`

Using this context, create a **new task request** for the following feature:

**→ `<FEATURE_DESCRIPTION>`**

## Rules

1. **Read all `agent-specs/` documents first** — the new request must be consistent with the existing architecture, design patterns, and coding standards.
2. **Check existing requests** — look at both `agent-development/pending/` and `agent-development/done/requests/` to understand what tasks already exist. The new request must not duplicate existing work.
3. **Determine the correct task number** — find the highest-numbered task file across both `pending/` and `done/requests/`, and use the next number in sequence. For example, if `8-reliability.md` is the highest, the new task should be numbered `9`.
4. **Follow the request template exactly** — use `agent-development/pending/_TEMPLATE-request.md` as your structural guide.
5. **Name the file** using the pattern `N-short-descriptive-name.md` where `N` is the task number determined above. Use lowercase kebab-case for the name portion.
6. **Save the file** in `agent-development/pending/`.
7. **Scope it correctly** — each request should represent a single coherent unit of work. If the feature is too large, split it into multiple sequential requests and explain the dependency chain in each one's Context section.
8. **Do NOT create a plan or write any code.** This prompt is only for writing the request document.

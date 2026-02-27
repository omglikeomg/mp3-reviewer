# Development Guide: Spec-Driven Agent Workflow

This document describes the development workflow used to build the **Song Reviewer CLI**. The project is developed incrementally through a structured pipeline where AI agents plan and implement tasks under human supervision. Every meaningful decision goes through a human approval gate.

---

## Table of Contents

- [Philosophy](#philosophy)
- [Directory Layout](#directory-layout)
- [The Pipeline](#the-pipeline)
  - [Stage 1: Request](#stage-1-request)
  - [Stage 2: Plan](#stage-2-plan)
  - [Stage 3: Approve](#stage-3-approve)
  - [Stage 4: Execute](#stage-4-execute)
  - [Stage 5: Done](#stage-5-done)
- [File Lifecycle](#file-lifecycle)
- [The Open Questions Mechanism](#the-open-questions-mechanism)
- [Prompt Templates](#prompt-templates)
- [Document Templates](#document-templates)
- [Project Specs](#project-specs)
- [Conventions](#conventions)
- [Quick Reference: Common Actions](#quick-reference-common-actions)

---

## Philosophy

This project follows a **spec-driven development** model:

1. **Humans define *what* to build** — through task requests and by resolving open questions.
2. **Agents figure out *how* to build it** — through detailed implementation plans.
3. **Humans approve before anything is built** — every plan goes through a review gate.
4. **Agents execute approved plans** — mechanically following the steps that were already vetted.

The key principle is that **no code is written without an approved plan**, and **no plan is approved without human review**. This creates a clear audit trail and prevents agents from making unsupervised architectural decisions.

---

## Directory Layout

```
user-development/                   ← Human-facing development assets
├── DEVELOPMENT-GUIDE.md            ← You are here
└── prompts/                        ← Reusable prompt templates for humans
    ├── 1-plan-task.md
    ├── 2-execute-plan.md
    └── 3-request-feature.md

agent-development/                  ← Agent-only pipeline (requests, plans, execution)
├── agent-specs/                    ← Project-level specifications (read-only context)
│   ├── agent-instructions.md       ← Coding standards and dos/don'ts
│   ├── application-overview.md     ← What the app does
│   ├── architecture-breakdown.md   ← Folder structure, design patterns, tech stack
│   └── FOLDER-STRUCTURE.md         ← Quick-reference project directory tree & package deps
├── pending/                        ← Task requests waiting to be planned
│   ├── _TEMPLATE-request.md        ← Template for new requests
│   ├── 6-code-audit.md
│   ├── 7-application-assembly-and-lifecycle.md
│   ├── 8-reliability.md
│   └── 9-tui-interface-and-tests.md
├── plans/                          ← Implementation plans waiting for approval
│   └── _TEMPLATE-plan.md           ← Template for new plans
├── queued/                         ← Approved plans ready for execution
└── done/                           ← Completed work
    ├── plans/                      ← Executed plans (archive)
    └── requests/                   ← Fulfilled requests (archive)
```

---

## The Pipeline

Every piece of work flows through five stages. Agents read source code directly as the source of truth.

```
 ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
 │ REQUEST  │────▶│   PLAN   │────▶│ APPROVE  │────▶│ EXECUTE  │────▶│   DONE   │
 │          │     │          │     │          │     │          │     │          │
 │ (human + │     │ (agent)  │     │ (human)  │     │ (agent)  │     │ (auto)   │
 │  agent)  │     │          │     │          │     │          │     │          │
 └──────────┘     └──────────┘     └──────────┘     └──────────┘     └──────────┘
   pending/         plans/        plans/ → queued/     queued/       done/plans/
                                                                    done/requests/
```

### Stage 1: Request

**Who:** Human (optionally assisted by an agent using prompt `3-request-feature.md`).

**What happens:**
- A task request file is created in `agent-development/pending/` following the `_TEMPLATE-request.md` structure.
- The file is numbered sequentially (e.g., `9-new-feature.md` if `8-reliability.md` is the highest existing number).
- The request defines *what* needs to be done — goal, context, requirements, and a checklist — but NOT *how*.

**Output:** A new file in `agent-development/pending/`.

### Stage 2: Plan

**Who:** An AI agent, guided by prompt `user-development/prompts/1-plan-task.md`.

**What happens:**
- The agent reads all `agent-development/agent-specs/` documents for context.
- The agent reads the specific task request from `agent-development/pending/`.
- The agent reads the relevant source code to understand the current state of the project (the code is the source of truth). `agent-development/agent-specs/FOLDER-STRUCTURE.md` can be used for quick orientation.
- The agent produces a detailed, step-by-step implementation plan following `_TEMPLATE-plan.md`.
- The agent surfaces any ambiguities or decisions it cannot make on its own in the **"Open Questions & Decisions"** section.

**Output:** A new plan file in `agent-development/plans/` (e.g., `0-initialization-plan.md`).

### Stage 3: Approve

**Who:** Human (you).

**What happens:**
- You read the plan carefully.
- You review the **"Open Questions & Decisions"** section:
  - For each question marked `PENDING`, you write your decision inline.
  - You can also modify any part of the plan if you disagree with the agent's approach.
- Once satisfied, you **move the plan** from `plans/` to `queued/`.

**The move is the approval signal.** A plan in `queued/` means "this has been reviewed and is ready to execute." A plan still in `plans/` means "this is a draft — do not implement."

**Output:** The plan file moves from `agent-development/plans/` → `agent-development/queued/`, with all open questions resolved.

### Stage 4: Execute

**Who:** An AI agent, guided by prompt `user-development/prompts/2-execute-plan.md`.

**What happens:**
- The agent reads the approved plan from `agent-development/queued/`.
- The agent verifies that all open questions have been resolved (no `PENDING` markers remain).
- The agent executes every step in order, following the plan precisely.
- The agent runs all verification checks and confirms they pass.
- After successful execution, the agent performs housekeeping moves:
  - Plan: `agent-development/queued/` → `agent-development/done/plans/`
  - Request: `agent-development/pending/` → `agent-development/done/requests/`

**Output:** Code is written/modified. Plan and request are archived in `agent-development/done/`.

### Stage 5: Done

**Who:** Automatic (performed by the executing agent).

**What happens:**
- The plan and its originating request are both moved to `agent-development/done/` subdirectories.
- They serve as a historical record of what was built and why.

---

## File Lifecycle

A task request file moves through the system like this:

```
agent-development/pending/3-tui-foundations.md          ← Created (Stage 1)
  └─ stays here while plan is being written and reviewed
  └─ moved to agent-development/done/requests/ after execution (Stage 4)
```

A plan file moves through the system like this:

```
agent-development/plans/3-tui-foundations-plan.md       ← Created by planning agent (Stage 2)
  └─ human reviews and resolves open questions (Stage 3)
agent-development/queued/3-tui-foundations-plan.md      ← Moved here when approved (Stage 3)
  └─ executing agent reads and implements (Stage 4)
agent-development/done/plans/3-tui-foundations-plan.md  ← Moved here after execution (Stage 4)
```

---

## The Open Questions Mechanism

This is the most important human-in-the-loop mechanism in the workflow.

### Why It Exists

AI agents are good at following specifications but bad at making subjective decisions. When a planning agent encounters something ambiguous — a naming choice, a trade-off between simplicity and flexibility, a missing requirement — it should **not guess**. Instead, it writes up the question in the plan's **"Open Questions & Decisions"** section.

### How It Works

1. **Planning agent** writes each question with:
   - A short title
   - Context explaining why the question matters
   - Concrete options (A, B, C...) with trade-offs
   - The agent's recommendation (or "no recommendation")
   - `Human decision: PENDING`

2. **Human reviewer** (during approval) replaces `PENDING` with their chosen option and any notes.

3. **Executing agent** (during execution) checks that no `PENDING` markers remain. If any do, it **stops and refuses to execute**. Resolved decisions are treated as binding requirements.

### Example

Before approval:
```
### Q1: Should the config file use TOML or JSON?

**Context:** TOML is more human-readable for config; JSON is already used elsewhere.

**Options:**
- **A)** TOML — better readability, but adds a dependency.
- **B)** JSON — consistent with existing files, no new deps.

**Agent's recommendation:** B (JSON) — consistency wins for a small config file.

**Human decision:** `PENDING`
```

After approval:
```
**Human decision:** B — agreed, let's keep JSON. Also make sure we pretty-print with 2-space indent.
```

---

## Prompt Templates

Located in `user-development/prompts/`. These are copy-paste prompts the human uses to start agent conversations.

| Prompt | File | Purpose |
|---|---|---|
| Plan a Task | `user-development/prompts/1-plan-task.md` | Give to an agent to produce a plan from a `pending/` request |
| Execute a Plan | `user-development/prompts/2-execute-plan.md` | Give to an agent to implement an approved plan from `queued/` |
| Request a Feature | `user-development/prompts/3-request-feature.md` | Give to an agent to write a new task request in `pending/` |

Each prompt has a `<PLACEHOLDER>` that you replace with a file reference before using it.

---

## Document Templates

| Template | Location | Used For |
|---|---|---|
| Request template | `agent-development/pending/_TEMPLATE-request.md` | Creating new task requests |
| Plan template | `agent-development/plans/_TEMPLATE-plan.md` | Creating new implementation plans |

Agents are instructed to follow these templates exactly when creating new documents.

---

## Project Specs

The `agent-development/agent-specs/` directory contains documents that provide global context to every agent conversation:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | What the Song Reviewer CLI does, its core workflows, and UX goals |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Folder structure, design patterns (MVU, Adapter), technology stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, metadata guidelines |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Complete project directory tree and package dependency graph |

These files are the **source of truth** for the project. If a plan or request contradicts them, the specs take precedence (or the specs should be updated first).

---

## Conventions

### File Naming

- **Requests:** `N-short-kebab-name.md` (e.g., `3-tui-foundations.md`)
- **Plans:** `N-short-kebab-name-plan.md` (e.g., `3-tui-foundations-plan.md`)
- **Templates:** `_TEMPLATE-*.md` (underscore prefix keeps them sorted first)
- **Task numbers** are sequential across `agent-development/pending/` and `agent-development/done/requests/` combined.

### Configuration Files

This project uses a **template-and-copy** convention for configuration:

| Git-tracked template (committed) | Runtime copy (gitignored) |
|---|---|
| `settings.example.json` | `config/settings.json` |
| `manual_review.example.json` | `data/manual_review.json` |

After cloning, users must copy the `.example.json` files to their runtime locations. The `.gitignore` is already configured to ignore the runtime copies. Never commit personal paths or API keys.

### Spec Updates

If a task introduces new packages, interfaces, or changes the architecture, the executing agent must update `agent-development/agent-specs/architecture-breakdown.md`, `agent-development/agent-specs/FOLDER-STRUCTURE.md`, and/or `agent-development/agent-specs/agent-instructions.md` as part of the task. These updates ensure future agents have accurate context.

---

## Quick Reference: Common Actions

### "I want to add a new feature"

1. Open a new agent conversation.
2. Paste the contents of `user-development/prompts/3-request-feature.md`.
3. Replace `<FEATURE_DESCRIPTION>` with what you want.
4. The agent creates a numbered file in `agent-development/pending/`.

### "I want to plan the next task"

1. Open a new agent conversation.
2. Paste the contents of `user-development/prompts/1-plan-task.md`.
3. Replace `<TASK_FILE>` with a reference to the `agent-development/pending/` file (e.g., `@5-api-integration-and-enrichment.md`).
4. The agent reads specs and source code, then creates a plan in `agent-development/plans/`.
5. **Review the plan and resolve all open questions.**
6. Move the plan from `agent-development/plans/` to `agent-development/queued/`.

### "I want to execute an approved plan"

1. Open a new agent conversation.
2. Paste the contents of `user-development/prompts/2-execute-plan.md`.
3. Replace `<PLAN_FILE>` with a reference to the `agent-development/queued/` file.
4. The agent reads the source code, implements the plan, then archives both the plan and request to `agent-development/done/`.

### "I want to see what's in progress"

- **What needs planning?** → Check `agent-development/pending/` (minus `_TEMPLATE-request.md`)
- **What's been planned but not approved?** → Check `agent-development/plans/` (minus `_TEMPLATE-plan.md`)
- **What's approved and ready to build?** → Check `agent-development/queued/`
- **What's done?** → Check `agent-development/done/plans/` and `agent-development/done/requests/`

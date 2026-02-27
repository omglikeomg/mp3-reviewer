# Diagrams — Visual Documentation

## Purpose

Diagrams are **visual documentation for humans**. They provide a high-level overview of the system's architecture, data model, state machine, and data flow in a format that is easier to scan than source code. They are maintained alongside the code so that project contributors (human and AI) can quickly orient themselves.

**The source code is the source of truth.** When a diagram and the code disagree, the code wins. Agents should read source code directly to understand the system — diagrams are a supplementary reference, not a prerequisite.

## Role of Diagrams for Agents

- **Do NOT treat diagrams as a source of truth.** They may lag behind the code.
- **Do NOT read diagrams before source code.** Read the code first; consult diagrams only if you want a bird's-eye view or need to understand a cross-cutting concern visually.
- **Do update diagrams after making code changes.** Diagram updates are a documentation deliverable — part of keeping the project well-documented for human readers.

## Files

### Mermaid Diagrams

| File | Diagram Type | What It Covers |
|---|---|---|
| `data-structures.mmd` | Mermaid `classDiagram` | All domain types, config types, and provider types — their fields, methods, and relationships (composition, dependency, interface satisfaction). |
| `software-architecture.mmd` | Mermaid `flowchart` or `classDiagram` | Packages as groups/subgraphs, key structs and interfaces with their public methods/functions, inter-package dependencies, and call relationships. |
| `ui-state-machine.mmd` | Mermaid `stateDiagram-v2` | All `AppState` values the TUI can be in, which screens/views correspond to each state, and the transitions between them triggered by user actions or system events. |
| `task-lifecycle.mmd` | Mermaid `stateDiagram-v2` | How a Task (song) moves through the review pipeline: loading from JSON → queued for review → playing/reviewing → genre selection → tag writing → state persistence → advancing to the next song or undoing. Covers the data-flow perspective across provider, TUI, metadata, and audio packages. |
| `component-data-flow.mmd` | Mermaid `flowchart` | How data flows through the Bubble Tea MVU pipeline: `tea.Cmd` dispatch, `tea.Msg` return paths, Model/Update/View interactions, external component invocations (audio, metadata, provider, API) via Cmds, ticker/spinner refresh cycles, and the genre-selection two-step data flow. |

### Reference Documents

| File | Format | What It Covers |
|---|---|---|
| `FOLDER-STRUCTURE.md` | Markdown | Complete project directory tree, package dependency graph, and key conventions. A quick-orientation reference for navigating the codebase. |

## Maintenance Rule

**Every task that changes the code should update affected diagrams as part of its documentation deliverables.** Diagrams are living documents that evolve with the code. Specifically:

- If you **add a new struct, interface, or type** → update `data-structures.mmd`.
- If you **add a new package, public function, or change inter-package dependencies** → update `software-architecture.mmd`.
- If you **add or change TUI states or the screens/views associated with them** → update `ui-state-machine.mmd`.
- If you **change how a Task moves through the review pipeline** (loading, reviewing, tagging, persisting, skipping, undoing) → update `task-lifecycle.mmd`.
- If you **change how data flows through the MVU pipeline** (new Cmds, new Msgs, new component interactions, new refresh cycles) → update `component-data-flow.mmd`.
- If you **add or remove top-level directories, packages, or significant files** → update `FOLDER-STRUCTURE.md`.

Diagram updates are part of documentation — not blockers for code changes. If a diagram is wrong or stale, fix it when you notice it, or as part of the current task.

## Format

All diagrams use [Mermaid](https://mermaid.js.org/) syntax in `.mmd` files. Mermaid is a text-based diagramming language that renders in GitHub, VS Code (with extensions), and many other tools.

Conventions for `.mmd` files in this project:

- Start each file with a `%% <Title>` comment explaining the diagram's purpose.
- Use clear, descriptive labels for all nodes and edges.
- Group related items using Mermaid's `namespace`, `subgraph`, or class grouping features where appropriate.
- Keep diagrams accurate — an outdated diagram is worse than no diagram.
# Diagrams — Agent Knowledge Base

## Purpose

Diagrams are the **primary knowledge base for agents**. Before reading source code, agents must consult the diagrams in this directory to build understanding of the system. These Mermaid diagrams capture the canonical structure, relationships, and behavior of the Song Reviewer CLI at a level of detail sufficient for most planning and implementation tasks.

## Diagram-First Rule

When beginning any task (planning or execution), agents must:

1. **Read all relevant diagrams** in this directory.
2. **Self-assess understanding** of the relevant area on a scale of **1–10**.
3. **If confidence is 9 or above** — proceed with the task using diagram knowledge alone.
4. **If confidence is below 9** — go to the source code for the specific areas of uncertainty, then re-assess.

The goal is to minimize unnecessary source-code reading. Diagrams should provide enough context for most tasks. If they don't, that's a signal the diagrams need to be improved as part of the current task.

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
| `FOLDER-STRUCTURE.md` | Markdown | Complete project directory tree, package dependency graph, and key conventions. A quick-orientation reference so agents can navigate the codebase without scanning the filesystem. |

## Maintenance Rule

**Every task (plan or execution) must create or update diagrams as appropriate.** Diagrams are living documents that evolve with the code. Specifically:

- If you **add a new struct, interface, or type** → update `data-structures.mmd`.
- If you **add a new package, public function, or change inter-package dependencies** → update `software-architecture.mmd`.
- If you **add or change TUI states or the screens/views associated with them** → update `ui-state-machine.mmd`.
- If you **change how a Task moves through the review pipeline** (loading, reviewing, tagging, persisting, skipping, undoing) → update `task-lifecycle.mmd`.
- If you **change how data flows through the MVU pipeline** (new Cmds, new Msgs, new component interactions, new refresh cycles) → update `component-data-flow.mmd`.
- If you **add or remove top-level directories, packages, or significant files** → update `FOLDER-STRUCTURE.md`.

Diagram updates are **mandatory deliverables** — they are as important as code changes.

## Format

All diagrams use [Mermaid](https://mermaid.js.org/) syntax in `.mmd` files. Mermaid is a text-based diagramming language that renders in GitHub, VS Code (with extensions), and many other tools.

Conventions for `.mmd` files in this project:

- Start each file with a `%% <Title>` comment explaining the diagram's purpose.
- Use clear, descriptive labels for all nodes and edges.
- Group related items using Mermaid's `namespace`, `subgraph`, or class grouping features where appropriate.
- Keep diagrams accurate — an outdated diagram is worse than no diagram.
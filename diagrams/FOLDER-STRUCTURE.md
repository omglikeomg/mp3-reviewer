# Folder Structure — Quick Reference

This file provides agents with an at-a-glance view of the entire project layout so they can orient themselves without scanning the filesystem.

> **Last updated:** Task 5 — API integration and enrichment features implemented.
> **Maintenance rule:** If a task adds or removes top-level directories, packages, or significant files, update this document as part of the task deliverables.

---

## Project Root

```
mp3-reviewer/
├── cmd/
│   └── reviewer/
│       └── main.go                 ← Entry point: loads config, builds queue, starts Bubble Tea
│
├── internal/
│   ├── domain/
│   │   └── models.go               ← Pure data structures (Task, AppConfig, ReviewQueue)
│   ├── provider/
│   │   ├── json_provider.go        ← TaskProvider interface + ManualReviewProvider + SaveState
│   │   └── json_provider_test.go
│   ├── audio/
│   │   ├── engine.go               ← Audio Engine: Play, Seek, TogglePause, GetState, Close
│   │   └── engine_test.go
│   ├── tui/
│   │   ├── model.go                ← Model struct, AppState enum, tea.Msg types, tea.Cmd factories
│   │   ├── update.go               ← Init, Update, key handling, message dispatch
│   │   └── view.go                 ← View rendering, lipgloss styles
│   ├── metadata/
│   │   └── writer.go               ← WriteTags, WriteBPM, WriteYear (ID3v2 tag writing)
│   └── api/
│       ├── musicbrainz.go          ← FetchYear, FetchBPM (MusicBrainz JSON API client)
│       └── musicbrainz_test.go     ← Unit tests with httptest mock server
│
├── config/
│   └── settings.json               ← Runtime config (gitignored — copy from settings.example.json)
│
├── data/
│   └── manual_review.json          ← Review queue data (gitignored — copy from manual_review.example.json)
│
├── diagrams/                        ← Mermaid diagrams — primary agent knowledge base
│   ├── README.md                    ← Diagram conventions, diagram-first rule, maintenance rule
│   ├── FOLDER-STRUCTURE.md          ← You are here
│   ├── data-structures.mmd          ← classDiagram: all domain/config/provider/audio/TUI types
│   ├── software-architecture.mmd    ← flowchart: packages, dependencies, call flows
│   ├── ui-state-machine.mmd         ← stateDiagram-v2: all AppState values and transitions
│   ├── task-lifecycle.mmd           ← stateDiagram-v2: review-queue task lifecycle and data flow
│   └── component-data-flow.mmd      ← flowchart: MVU component lifecycle, message/command data flow
│
├── user-development/                ← Human-facing development assets
│   ├── DEVELOPMENT-GUIDE.md         ← Spec-driven workflow documentation
│   └── prompts/                     ← Reusable prompt templates for humans
│       ├── 0-create-initial-diagrams.md
│       ├── 1-plan-task.md
│       ├── 2-execute-plan.md
│       └── 3-request-feature.md
│
├── agent-development/               ← Agent-only pipeline (specs, requests, plans)
│   ├── agent-specs/                 ← Project-level specifications (read-only context)
│   │   ├── agent-instructions.md    ← Coding standards, dos/don'ts, workflow, diagram-first rules
│   │   ├── application-overview.md  ← What the app does, core workflows, UX goals
│   │   └── architecture-breakdown.md ← Folder structure, design patterns, tech stack
│   ├── pending/                     ← Task requests waiting to be planned
│   │   ├── _TEMPLATE-request.md
│   │   ├── 5-api-integration-and-enrichment.md
│   │   ├── 6-code-audit.md
│   │   ├── 7-application-assembly-and-lifecycle.md
│   │   ├── 8-reliability.md
│   │   └── 9-tui-interface-and-tests.md
│   ├── plans/                       ← Implementation plans waiting for approval
│   │   └── _TEMPLATE-plan.md
│   ├── queued/                      ← Approved plans ready for execution
│   │   └── 5-api-integration-and-enrichment-plan.md
│   └── done/                        ← Completed work
│       ├── plans/                   ← Executed plans (archive)
│       └── requests/                ← Fulfilled requests (archive)
│
├── .gitignore
├── go.mod
├── go.sum
├── README.md
├── settings.example.json            ← Git-tracked config template
└── manual_review.example.json       ← Git-tracked review queue template
```

---

## Package Dependency Graph (text summary)

```
cmd/reviewer
  └── imports: internal/domain, internal/provider, internal/audio, internal/tui

internal/tui
  └── imports: internal/domain, internal/audio, internal/metadata, internal/provider

internal/provider
  └── imports: internal/domain

internal/audio
  └── imports: (external only: faiface/beep)

internal/metadata
  └── imports: (external only: bogem/id3v2)

internal/api
  └── imports: (standard library only: encoding/json, fmt, net/http, net/url, sort, strings, time)

internal/domain
  └── imports: (none — pure data)
```

---

## Key Conventions

| Convention | Detail |
|---|---|
| **Config files** | `.example.json` files are git-tracked templates; runtime copies in `config/` and `data/` are gitignored. |
| **TUI file split** | `model.go` (structs + Cmd factories), `update.go` (message dispatch), `view.go` (rendering + styles). Never merge. |
| **Task numbering** | Sequential across `agent-development/pending/` and `agent-development/done/requests/` combined. |
| **Diagram format** | All `.mmd` files use [Mermaid](https://mermaid.js.org/) syntax. |
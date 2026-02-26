# Task 0: Project Bootstrapping & Workspace Setup

## Goal
Initialize the Go module, create the directory hierarchy, and generate the initial configuration and README.

## Context
Before we write any logic, we need a consistent environment. We will use a standard Go project layout to separate the entry point from the internal library logic.

## Requirements
Initialize Go Module: Run go mod init song-reviewer.

### Create Directory Structure:

`cmd/reviewer/`: For main.go.
`internal/domain/`: For core data models.
`internal/audio/`: For the Beep wrapper.
`internal/provider/`: For JSON loading/saving.
`internal/tui/`: For Bubble Tea components.
`internal/metadata/`: For ID3 tag manipulation.
`internal/api/`: For external service clients.
`data/`: To hold the manual_review.json.
`config/`: To hold settings.json.

- Touch Essential Files: * Create empty .go files in each directory to satisfy the package structure.
- Create a settings.json with the structure defined in the project overview.
- Documentation: Create a README.md that provides a high-level summary of how to build and run the tool.

### Implementation Script (Reference for Agent)
The agent should execute (or simulate) the following:

```bash
mkdir -p cmd/reviewer internal/{domain,audio,provider,tui,metadata,api} data config
touch cmd/reviewer/main.go
touch internal/domain/models.go
touch internal/audio/engine.go
touch internal/provider/json_provider.go
touch internal/tui/model.go internal/tui/view.go internal/tui/update.go
touch internal/metadata/writer.go
touch internal/api/musicbrainz.go
touch config/settings.json
touch README.md
```

## Deliverables
- A fully initialized go.mod file.
- The complete folder skeleton.
- A `README.md` containing:
  - Project Title.
  - Installation instructions (go build).
  - A "Usage" section explaining the keybindings.
  - A `config/settings.json` containing placeholder paths for `music_folder` and `review_json_path`.

## Agent Checklist
- [ ] Is the go.mod initialized correctly?
- [ ] Do all packages have the correct package header in their respective .go files?
- [ ] Does the README.md reflect the `architecture-breakdown.md`?

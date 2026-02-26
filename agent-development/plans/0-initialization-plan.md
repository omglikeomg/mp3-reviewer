# Implementation Plan: Task 0 — Project Bootstrapping & Workspace Setup

## Overview

This plan details every step an implementing agent must follow to bootstrap the `song-reviewer` Go project. The goal is to produce a fully initialized Go module with the correct directory hierarchy, empty but valid Go source files, a `config/settings.json` with placeholder values, and a comprehensive `README.md`. No application logic is written in this task — only scaffolding.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/0-initialization.md` | The task being implemented |
| Existing `settings.json` | `settings.json` (project root) | Reference for config structure |
| Existing `manual_review.json` | `manual_review.json` (project root) | Reference for review data structure |

---

## Pre-Conditions

- Go 1.21+ must be installed and available on `$PATH`.
- The working directory for all commands is the project root: `/Users/dmolinari/vimwiki/mp3-reviewer`.
- No `go.mod` file exists yet. No `cmd/`, `internal/`, `data/`, or `config/` directories exist yet.

---

## Step-by-Step Implementation

### Step 1: Initialize the Go Module

**Action:** Run `go mod init song-reviewer` from the project root.

**Expected outcome:** A `go.mod` file is created at the project root with the following content (Go version may vary):

```
module song-reviewer

go 1.21
```

**Verification:** Confirm `go.mod` exists and contains `module song-reviewer`.

---

### Step 2: Create the Directory Structure

**Action:** Create the following directories. Use `mkdir -p` or the equivalent tool to create parent directories as needed.

```
cmd/reviewer/
internal/domain/
internal/audio/
internal/provider/
internal/tui/
internal/metadata/
internal/api/
data/
config/
```

**Full list of directories to create (9 total):**

1. `cmd/reviewer`
2. `internal/domain`
3. `internal/audio`
4. `internal/provider`
5. `internal/tui`
6. `internal/metadata`
7. `internal/api`
8. `data`
9. `config`

**Verification:** Run `find . -type d` and confirm all 9 directories exist (plus `internal/` and `cmd/` as implicit parents).

---

### Step 3: Create Go Source Files with Correct Package Headers

Each `.go` file must contain **only** the `package` declaration line. No imports, no functions, no comments beyond a brief file-level comment. The package name must match the directory name (except `cmd/reviewer/main.go` which uses `package main`).

#### 3.1 — `cmd/reviewer/main.go`

```go
package main

func main() {
	// TODO: Initialize Bubble Tea program.
}
```

**Why `func main()` is included:** Without a `main` function, the `main` package would not compile. Even though it's a stub, the function signature is required for `go build` to succeed on this package.

#### 3.2 — `internal/domain/models.go`

```go
package domain
```

This file will later hold the `Task`, `AppConfig`, and `ReviewQueue` structs.

#### 3.3 — `internal/audio/engine.go`

```go
package audio
```

This file will later hold the Beep library wrapper for playback, seeking, and device management.

#### 3.4 — `internal/provider/json_provider.go`

```go
package provider
```

This file will later hold the `TaskProvider` interface and `ManualReviewProvider` implementation.

#### 3.5 — `internal/tui/model.go`

```go
package tui
```

This file will later hold the Bubble Tea `Model` struct.

#### 3.6 — `internal/tui/view.go`

```go
package tui
```

This file will later hold the `View()` rendering function.

#### 3.7 — `internal/tui/update.go`

```go
package tui
```

This file will later hold the `Update()` message-handling function.

#### 3.8 — `internal/metadata/writer.go`

```go
package metadata
```

This file will later hold ID3 tag read/write logic.

#### 3.9 — `internal/api/musicbrainz.go`

```go
package api
```

This file will later hold the MusicBrainz and BPM API HTTP clients.

**Total Go files: 9**

**Verification:** Run `go build ./...` from the project root. It must succeed with zero errors. The only "buildable" target is `cmd/reviewer`, and since `main()` is a no-op stub, it should compile cleanly.

---

### Step 4: Create `config/settings.json`

**Action:** Create the file `config/settings.json` with the following content:

```json
{
  "music_folder": "/Users/username/Music",
  "review_json_path": "./data/manual_review.json",
  "genres": [
    "Rock",
    "Jazz",
    "Blues",
    "Electronic",
    "Hip-Hop",
    "Classical",
    "Folk",
    "Psych-Rock",
    "Techno",
    "House"
  ],
  "api_keys": {
    "musicbrainz_user_agent": "MySongReviewer/1.0.0 ( contact@example.com )"
  }
}
```

**Notes:**
- The `music_folder` value is a **placeholder**. The user will replace it with their actual music directory path.
- The `review_json_path` points to `./data/manual_review.json`, which is relative to the project root.
- The `genres` array matches the existing `settings.json` in the project root. This is intentional — we are reproducing it inside the `config/` directory as the canonical config location for the Go application.
- The `api_keys` object contains a placeholder MusicBrainz user agent string that the user must customize.

**Verification:** Confirm the file is valid JSON by parsing it (e.g., `python3 -c "import json; json.load(open('config/settings.json'))"`).

---

### Step 5: Create `data/manual_review.json`

**Action:** Create a sample/placeholder `data/manual_review.json` so that the `review_json_path` in `settings.json` points to a valid file. Use the same structure as the existing `manual_review.json` in the project root:

```json
{
  "manual_review": [
    {
      "filepath": "Cream/Cream - Strange Brew.mp3",
      "reason": "Genre not in taxonomy",
      "confidence": 4
    }
  ]
}
```

**Rationale:** The `settings.json` references `./data/manual_review.json`. Having this file present ensures consistency and lets future tasks (especially Task 1, which builds the JSON adapter) have a concrete file to test against.

---

### Step 6: Create `README.md`

**Action:** Create `README.md` at the project root with the following content. The README must reflect the architecture described in `architecture-breakdown.md` and the application purpose from `application-overview.md`.

---

**Full README.md content:**

````markdown
# Song Reviewer CLI

A high-performance Go-based CLI tool for music enthusiasts to manually categorize song genres and enrich metadata (BPM, Release Year) through an immersive terminal interface.

## Purpose

Song Reviewer bridges the gap between automated genre-classification scripts (which may produce uncertain results) and manual ID3 tagging. It reads a JSON queue of songs flagged for manual review, plays them back in the terminal, and lets you assign genres and metadata with zero mouse interaction.

## Features

- **Review Queue** — Reads a JSON file of songs marked for `manual_review` and presents them one at a time.
- **Immersive Playback** — Songs auto-play on selection. Seek ±30s to find the defining section of the track.
- **Dual-Tier Genre Tagging** — Assign a Primary Genre (e.g., "Rock") and an optional Secondary Genre (e.g., "Psych-Rock").
- **Data Enrichment** — Fetch original release year from MusicBrainz and BPM from external APIs.
- **Persistence** — Writes changes directly to MP3/FLAC ID3 tags and updates the source JSON to reflect "Applied" status.
- **Undo Support** — Mis-categorized a song? Press `Ctrl+U` to undo and go back.

## Architecture

```
cmd/reviewer/       — Entry point. Initializes the Bubble Tea program.
internal/domain/    — Pure data structures (Task, Config). No dependencies.
internal/provider/  — JSON parser adapters (TaskProvider interface).
internal/audio/     — Wrapper for the beep library (playback, seeking, device).
internal/tui/       — Bubble Tea components (Model, Update, View).
internal/metadata/  — ID3 tag read/write logic (pure Go).
internal/api/       — External HTTP clients (MusicBrainz, BPM APIs).
data/               — Holds the manual_review.json queue file.
config/             — Holds settings.json with app configuration.
```

### Design Patterns

- **Model-View-Update (MVU):** Strict Bubble Tea pattern — Model holds state, Update handles messages, View renders strings.
- **Adapter Pattern:** The `provider` package uses a `TaskProvider` interface so different JSON schemas can be supported without changing TUI code.
- **Concurrency:** Audio playback and API fetching run in background goroutines via `tea.Cmd` to keep the UI responsive.

### Technology Stack

| Component | Library |
|---|---|
| Language | Go 1.21+ |
| TUI Framework | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` |
| Audio Playback | `faiface/beep` |
| Metadata (ID3) | `dhowden/tag` |

## Prerequisites

- [Go 1.21+](https://go.dev/dl/) installed and on your `$PATH`.

## Installation

```bash
# Clone the repository
git clone <repo-url>
cd song-reviewer

# Build the binary
go build -o song-reviewer ./cmd/reviewer

# Or install it into your $GOPATH/bin
go install ./cmd/reviewer
```

## Configuration

Edit `config/settings.json` before running:

```json
{
  "music_folder": "/path/to/your/music/library",
  "review_json_path": "./data/manual_review.json",
  "genres": ["Rock", "Jazz", "Blues", "Electronic", "Hip-Hop", "Classical", "Folk", "Psych-Rock", "Techno", "House"],
  "api_keys": {
    "musicbrainz_user_agent": "YourAppName/1.0.0 ( your@email.com )"
  }
}
```

| Field | Description |
|---|---|
| `music_folder` | Absolute path to your music library root. Song file paths in the review JSON are resolved relative to this. |
| `review_json_path` | Path to the JSON file containing songs flagged for manual review. |
| `genres` | List of genre labels available for tagging. Customize to match your taxonomy. |
| `api_keys.musicbrainz_user_agent` | Required by MusicBrainz API. Must include your app name and contact email. |

## Usage

Launch the reviewer:

```bash
./song-reviewer
```

### Keybindings

| Key | Action |
|---|---|
| `←` / `→` | Seek backward / forward 30 seconds |
| `Enter` / `Space` | Open genre selection menu |
| `Esc` | Skip current song and move to next |
| `Ctrl+U` | Undo last genre assignment |
| `Ctrl+C` | Quit (cleanly shuts down audio device) |

## Review JSON Format

The `manual_review.json` file should follow this structure:

```json
{
  "manual_review": [
    {
      "filepath": "Artist/Artist - Song Title.mp3",
      "reason": "Genre not in taxonomy",
      "confidence": 4
    }
  ]
}
```

| Field | Description |
|---|---|
| `filepath` | Path to the audio file, relative to `music_folder`. |
| `reason` | Why this song was flagged for manual review. |
| `confidence` | Numeric confidence score from the automated classifier (informational). |

## License

TBD
````

---

### Step 7: Final Verification

The implementing agent must run the following checks and confirm all pass:

#### 7.1 — `go.mod` is correct

Run: `cat go.mod`

Expected: The file contains `module song-reviewer` and a valid Go version directive (e.g., `go 1.21`).

#### 7.2 — All packages compile

Run: `go build ./...`

Expected: Zero errors, zero output. This confirms every `.go` file has the correct `package` declaration and there are no syntax issues.

#### 7.3 — Directory structure is complete

Run: `find cmd internal data config -type f | sort`

Expected output (exact):

```
cmd/reviewer/main.go
config/settings.json
data/manual_review.json
internal/api/musicbrainz.go
internal/audio/engine.go
internal/domain/models.go
internal/metadata/writer.go
internal/provider/json_provider.go
internal/tui/model.go
internal/tui/update.go
internal/tui/view.go
```

#### 7.4 — `config/settings.json` is valid JSON

Run: `go run -mod=mod encoding/json < config/settings.json` or use any JSON validator.

#### 7.5 — `README.md` exists and is non-empty

Run: `test -s README.md && echo "OK"`

Expected: `OK`

---

## File Manifest

Summary of every file created in this task:

| # | File Path | Type | Content Summary |
|---|---|---|---|
| 1 | `go.mod` | Generated | Go module definition (`song-reviewer`) |
| 2 | `cmd/reviewer/main.go` | Go source | `package main` with empty `func main()` |
| 3 | `internal/domain/models.go` | Go source | `package domain` (empty) |
| 4 | `internal/audio/engine.go` | Go source | `package audio` (empty) |
| 5 | `internal/provider/json_provider.go` | Go source | `package provider` (empty) |
| 6 | `internal/tui/model.go` | Go source | `package tui` (empty) |
| 7 | `internal/tui/view.go` | Go source | `package tui` (empty) |
| 8 | `internal/tui/update.go` | Go source | `package tui` (empty) |
| 9 | `internal/metadata/writer.go` | Go source | `package metadata` (empty) |
| 10 | `internal/api/musicbrainz.go` | Go source | `package api` (empty) |
| 11 | `config/settings.json` | JSON | App configuration with placeholder values |
| 12 | `data/manual_review.json` | JSON | Sample review queue (matches existing root file) |
| 13 | `README.md` | Markdown | Project documentation |

**Total files: 13**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go.mod` exists at the project root with `module song-reviewer`
- [ ] All 9 directories under `cmd/`, `internal/`, `data/`, and `config/` exist
- [ ] All 9 `.go` files have the correct `package` declaration matching their directory name
- [ ] `cmd/reviewer/main.go` has a `func main()` stub (even if empty) so the main package compiles
- [ ] `go build ./...` succeeds with zero errors
- [ ] `config/settings.json` is valid JSON with `music_folder`, `review_json_path`, `genres`, and `api_keys` fields
- [ ] `data/manual_review.json` exists and matches the expected schema
- [ ] `README.md` exists and contains: Project Title, Architecture overview, Installation instructions (`go build`), Usage section with keybindings table, Configuration section explaining `settings.json`
- [ ] The `README.md` architecture section is consistent with `agent-specs/architecture-breakdown.md`
- [ ] No extraneous files were created outside of the manifest above

---

## Notes for the Implementing Agent

1. **Do NOT install any dependencies in this task.** There are no `import` statements yet. Dependencies like `bubbletea`, `beep`, and `tag` will be added in later tasks (Tasks 1–5) when the code that uses them is written. Running `go mod tidy` should be a no-op.

2. **Do NOT add build tags or `//go:generate` directives.** This is pure scaffolding.

3. **The existing `settings.json` and `manual_review.json` at the project root should NOT be deleted or modified.** They are reference files. The new copies live under `config/` and `data/` respectively, which is where the application will look for them.

4. **All file paths in this plan are relative to the project root** (`/Users/dmolinari/vimwiki/mp3-reviewer`). The implementing agent must ensure all tool calls and shell commands operate from this root.
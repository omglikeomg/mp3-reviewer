# Implementation Plan: Task 0 — Project Bootstrapping & Workspace Setup

## Overview

This plan details every step an implementing agent must follow to bootstrap the `song-reviewer` Go project. The goal is to produce a fully initialized Go module with the correct directory hierarchy, empty but valid Go source files, git-tracked `.example.json` configuration templates, and a comprehensive `README.md`. No application logic is written in this task — only scaffolding.

**Important convention:** Configuration files that contain user-specific data (local paths, API keys) are **gitignored**. Instead, `.example.json` template files are committed to the repository. Users (and the implementing agent during setup) copy these templates to their runtime names to create their local configuration.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/0-initialization.md` | The task being implemented |
| `.gitignore` | `.gitignore` (project root) | Understand which files are tracked vs. ignored |
| Existing `settings.example.json` | `settings.example.json` (project root) | Git-tracked config template |
| Existing `manual_review.example.json` | `manual_review.example.json` (project root) | Git-tracked review data template |

---

## Pre-Conditions

- Go 1.21+ must be installed and available on `$PATH`.
- The working directory for all commands is the project root: `/Users/dmolinari/vimwiki/mp3-reviewer`.
- No `go.mod` file exists yet. No `cmd/`, `internal/`, `data/`, or `config/` directories exist yet.
- The following files already exist at the project root and **must NOT be deleted or modified**:
  - `settings.example.json` — git-tracked configuration template
  - `manual_review.example.json` — git-tracked review data template
  - `.gitignore` — already configured to ignore runtime config files

---

## The Example File Convention

This project uses a **template-and-copy** pattern for configuration:

| Git-tracked template (committed) | Runtime copy (gitignored) | Purpose |
|---|---|---|
| `settings.example.json` | `config/settings.json` | App configuration (music folder path, genres, API keys) |
| `manual_review.example.json` | `data/manual_review.json` | Review queue data |

The `.gitignore` already contains rules to ignore the runtime copies:
- `config/settings.json`
- `data/manual_review.json`
- `settings.json`
- `manual_review.json`

The `.example.json` files are **not** gitignored and serve as the canonical reference for new users or fresh clones.

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

**Verification:** Run `find . -type d -not -path './.git*' -not -path './agent-*' | sort` and confirm all 9 directories exist (plus `internal/` and `cmd/` as implicit parents).

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

### Step 4: Copy Example Templates to Runtime Locations

This step creates the local (gitignored) runtime configuration files by copying the git-tracked `.example.json` templates into their expected locations.

#### 4.1 — Copy `settings.example.json` → `config/settings.json`

**Action:** Copy the file `settings.example.json` (project root) to `config/settings.json`.

**Command:**
```bash
cp settings.example.json config/settings.json
```

**Expected outcome:** `config/settings.json` exists and is identical to `settings.example.json`. It contains:

```json
{
  "music_folder": "/Users/username/Music",
  "review_json_path": "./data/manual_review.json",
  "genres": [
    "Rock", "Jazz", "Blues", "Electronic", "Hip-Hop", 
    "Classical", "Folk", "Psych-Rock", "Techno", "House"
  ],
  "api_keys": {
    "musicbrainz_user_agent": "MySongReviewer/1.0.0 ( contact@example.com )"
  }
}
```

**Verification:**
1. `config/settings.json` exists.
2. `diff settings.example.json config/settings.json` produces no output (files are identical).
3. The file is valid JSON: `python3 -c "import json; json.load(open('config/settings.json'))"` exits with code 0.

**Git status note:** `config/settings.json` is listed in `.gitignore` and will NOT be tracked by git. This is intentional — users customize this file with their local paths and API keys.

#### 4.2 — Copy `manual_review.example.json` → `data/manual_review.json`

**Action:** Copy the file `manual_review.example.json` (project root) to `data/manual_review.json`.

**Command:**
```bash
cp manual_review.example.json data/manual_review.json
```

**Expected outcome:** `data/manual_review.json` exists and is identical to `manual_review.example.json`. It contains:

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

**Verification:**
1. `data/manual_review.json` exists.
2. `diff manual_review.example.json data/manual_review.json` produces no output (files are identical).
3. The file is valid JSON: `python3 -c "import json; json.load(open('data/manual_review.json'))"` exits with code 0.

**Git status note:** `data/manual_review.json` is listed in `.gitignore` and will NOT be tracked by git. This is intentional — this file contains user-specific review data that changes at runtime.

---

### Step 5: Create `README.md`

**Action:** Create `README.md` at the project root with the following content. The README must reflect the architecture described in `architecture-breakdown.md` and the application purpose from `application-overview.md`. It must also document the example-file convention so users know how to set up their local configuration.

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

This project uses **example files** as git-tracked templates. The actual runtime configuration files are gitignored so your local paths and API keys stay out of version control.

### Initial Setup

After cloning, copy the example templates to create your local configuration:

```bash
# Create your local settings (customize after copying)
cp settings.example.json config/settings.json

# Create your local review queue
cp manual_review.example.json data/manual_review.json
```

### Settings Reference

Edit `config/settings.json` with your local values:

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

### File Convention

| Git-tracked template | Runtime copy (gitignored) | Purpose |
|---|---|---|
| `settings.example.json` | `config/settings.json` | App configuration |
| `manual_review.example.json` | `data/manual_review.json` | Review queue data |

> **Note:** Never edit the `.example.json` files with your personal data. They are shared templates. Edit only the copies in `config/` and `data/`.

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

### Step 6: Final Verification

The implementing agent must run the following checks and confirm all pass:

#### 6.1 — `go.mod` is correct

Run: `cat go.mod`

Expected: The file contains `module song-reviewer` and a valid Go version directive (e.g., `go 1.21`).

#### 6.2 — All packages compile

Run: `go build ./...`

Expected: Zero errors, zero output. This confirms every `.go` file has the correct `package` declaration and there are no syntax issues.

#### 6.3 — Directory structure is complete

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

#### 6.4 — Runtime config files match their templates

Run:
```bash
diff settings.example.json config/settings.json
diff manual_review.example.json data/manual_review.json
```

Expected: Both commands produce no output (files are identical).

#### 6.5 — Runtime config files are gitignored

Run:
```bash
git check-ignore config/settings.json data/manual_review.json
```

Expected output:
```
config/settings.json
data/manual_review.json
```

If git is not yet initialized, verify by inspecting `.gitignore` and confirming it contains rules for `config/settings.json` and `data/manual_review.json`.

#### 6.6 — Example template files are NOT gitignored

Run:
```bash
git check-ignore settings.example.json manual_review.example.json
```

Expected: No output (the command exits with code 1, meaning these files are NOT ignored — they are tracked).

#### 6.7 — `config/settings.json` is valid JSON

Run: `python3 -c "import json; json.load(open('config/settings.json'))"`

Expected: Exits with code 0, no output.

#### 6.8 — `data/manual_review.json` is valid JSON

Run: `python3 -c "import json; json.load(open('data/manual_review.json'))"`

Expected: Exits with code 0, no output.

#### 6.9 — `README.md` exists and is non-empty

Run: `test -s README.md && echo "OK"`

Expected: `OK`

---

## Open Questions & Decisions

None — this plan is fully self-contained. Task 0 is pure scaffolding with no ambiguous design decisions. The directory structure, file names, and configuration schema are all prescribed by the architecture spec and the existing `.example.json` templates.

---

## File Manifest

Summary of every file created in this task:

| # | File Path | Action | Git-tracked? | Content Summary |
|---|---|---|---|---|
| 1 | `go.mod` | Created | ✅ Yes | Go module definition (`song-reviewer`) |
| 2 | `cmd/reviewer/main.go` | Created | ✅ Yes | `package main` with empty `func main()` |
| 3 | `internal/domain/models.go` | Created | ✅ Yes | `package domain` (empty) |
| 4 | `internal/audio/engine.go` | Created | ✅ Yes | `package audio` (empty) |
| 5 | `internal/provider/json_provider.go` | Created | ✅ Yes | `package provider` (empty) |
| 6 | `internal/tui/model.go` | Created | ✅ Yes | `package tui` (empty) |
| 7 | `internal/tui/view.go` | Created | ✅ Yes | `package tui` (empty) |
| 8 | `internal/tui/update.go` | Created | ✅ Yes | `package tui` (empty) |
| 9 | `internal/metadata/writer.go` | Created | ✅ Yes | `package metadata` (empty) |
| 10 | `internal/api/musicbrainz.go` | Created | ✅ Yes | `package api` (empty) |
| 11 | `config/settings.json` | Copied from `settings.example.json` | ❌ Gitignored | Local app configuration (user-specific) |
| 12 | `data/manual_review.json` | Copied from `manual_review.example.json` | ❌ Gitignored | Local review queue (user-specific) |
| 13 | `README.md` | Created | ✅ Yes | Project documentation |

**Files already existing (not created, not modified):**

| File Path | Git-tracked? | Role |
|---|---|---|
| `settings.example.json` | ✅ Yes | Template for `config/settings.json` |
| `manual_review.example.json` | ✅ Yes | Template for `data/manual_review.json` |
| `.gitignore` | ✅ Yes | Ignores runtime config and OS/IDE junk |

**Total files created: 13 (11 tracked, 2 gitignored copies)**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go.mod` exists at the project root with `module song-reviewer`
- [ ] All 9 directories under `cmd/`, `internal/`, `data/`, and `config/` exist
- [ ] All 9 `.go` files have the correct `package` declaration matching their directory name
- [ ] `cmd/reviewer/main.go` has a `func main()` stub (even if empty) so the main package compiles
- [ ] `go build ./...` succeeds with zero errors
- [ ] `config/settings.json` exists and is identical to `settings.example.json`
- [ ] `data/manual_review.json` exists and is identical to `manual_review.example.json`
- [ ] `config/settings.json` and `data/manual_review.json` are gitignored (confirmed via `.gitignore` rules or `git check-ignore`)
- [ ] `settings.example.json` and `manual_review.example.json` are NOT gitignored (they are tracked templates)
- [ ] `README.md` exists and contains: Project Title, Architecture overview, Installation instructions (`go build`), Configuration section with the example-file copy instructions, Usage section with keybindings table
- [ ] The `README.md` architecture section is consistent with `agent-specs/architecture-breakdown.md`
- [ ] No extraneous files were created outside of the manifest above
- [ ] The `.example.json` files at the project root were NOT modified or deleted

---

## Notes for the Implementing Agent

1. **Do NOT install any dependencies in this task.** There are no `import` statements yet. Dependencies like `bubbletea`, `beep`, and `tag` will be added in later tasks (Tasks 1–5) when the code that uses them is written. Running `go mod tidy` should be a no-op.

2. **Do NOT add build tags or `//go:generate` directives.** This is pure scaffolding.

3. **Do NOT modify the `.example.json` files.** They are git-tracked templates that already exist. Your job is to *copy* them into the `config/` and `data/` directories — never edit, rename, or delete the originals.

4. **Do NOT modify `.gitignore`.** It already contains the correct rules to ignore `config/settings.json`, `data/manual_review.json`, `settings.json`, and `manual_review.json`.

5. **The copy step (Step 4) is essential.** The application will look for `config/settings.json` and `data/manual_review.json` at runtime. Without these copies, the app would fail to start once logic is implemented in later tasks. The `.example.json` files are only templates — they are not read by the application.

6. **All file paths in this plan are relative to the project root** (`/Users/dmolinari/vimwiki/mp3-reviewer`). The implementing agent must ensure all tool calls and shell commands operate from this root.
# Implementation Plan: Task 11 — CLI Flags, README Cleanup & Makefile

## Overview

This plan covers three small, independent operational improvements to the `song-reviewer` project:

1. **CLI flags** — Add `--help` / `-h` and `--version` / `-v` flags to `cmd/reviewer/main.go`. The version string is injected at build time via `-ldflags`; unrecognised flags exit with code 1. The standard library only (`os`, `fmt`) — no `flag` package.
2. **README correction** — Remove all inaccurate FLAC references. The project supports MP3 only; `bogem/id3v2` is ID3-specific and no FLAC code path has ever been written. Additionally, add a "Development" section near Installation documenting the `make` commands.
3. **Makefile** — Add a `Makefile` at the project root with `build`, `test`, `install`, and `lint` targets and a `VERSION ?= dev` variable.

None of the three changes touches domain logic, the TUI, audio, or any internal package. The only Go file modified is `cmd/reviewer/main.go`. All other changes are purely text (Markdown, Makefile, doc files).

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree |
| Task Definition | `agent-development/pending/11-cli-flags-readme-cleanup-and-makefile.md` | The task being implemented |
| Entry Point | `cmd/reviewer/main.go` | The only Go file being modified |
| README | `README.md` | The documentation file being corrected and extended |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | The doc file being corrected for FLAC |
| App Overview | `agent-development/agent-specs/application-overview.md` | Also contains a FLAC reference that must be corrected |

---

## Pre-Conditions

- Tasks 0–10 are complete. The project builds and all tests pass (`go build ./...` and `go test ./...` succeed on the current codebase).
- `cmd/reviewer/main.go` exists and contains the `main()` function that currently calls `tui.New()` unconditionally.
- `README.md` exists at the project root.
- `agent-development/agent-specs/architecture-breakdown.md` exists.
- `agent-development/agent-specs/application-overview.md` exists.
- `agent-development/agent-specs/FOLDER-STRUCTURE.md` exists.
- No `Makefile` currently exists at the project root.

---

## Step-by-Step Implementation

### Step 1: Add `var version` and `const helpText` to `cmd/reviewer/main.go`

**Action:**

Open `cmd/reviewer/main.go`. Make two additions at the **package level**, between the `import` block and `func main()`:

1. Add a package-level `var` for the version string (a `var`, not a `const`, because `-ldflags` injection requires an addressable variable):

```mp3-reviewer/cmd/reviewer/main.go#L1-3
var version = "dev"
```

2. Add a package-level `const` for the help text using a raw string literal (backtick-delimited). The constant must be named `helpText` and placed in the same file. Write it as follows:

```mp3-reviewer/cmd/reviewer/main.go#L1-30
const helpText = `song-reviewer — interactively review and tag MP3 files from a manual-review queue.

Usage: song-reviewer [--help] [--version]

Configuration:
  Copy settings.example.json to config/settings.json and set your paths before first run.
  Key fields:
    music_folder          Absolute path to your music library root.
    review_json_path      Path to the JSON file of songs to review.
    genres                List of genre labels available for tagging.
    seek_delta_seconds    Seek step in seconds (default: 30).
    skip_applied          Omit already-tagged songs from the queue (default: false).
    api_keys.musicbrainz_user_agent
                          Required MusicBrainz User-Agent string.

  Example: config/settings.json  (template: settings.example.json)

Keybindings:
  ← / →         Seek backward / forward (seek_delta_seconds)
  p             Pause / Resume playback
  Enter / Space Open genre selection menu
  t             Tap to the beat — calculates BPM (8 taps required)
  Ctrl+1        Commit BPM to TBPM tag
  Ctrl+2        Commit Year to year tag
  Esc           Skip current song
  Ctrl+U        Undo last genre assignment
  Ctrl+O        Open Settings overlay
  Ctrl+C        Quit

For full documentation see README.md.
`
```

**Expected outcome:** `cmd/reviewer/main.go` now has a package-level `var version = "dev"` and a package-level `const helpText = \`...\`` before `func main()`.

**Verification:** `go build ./cmd/reviewer` compiles without errors.

---

### Step 2: Add flag-handling logic at the top of `main()`

**Action:**

In `cmd/reviewer/main.go`, inside `func main()`, add the following block as the **very first thing** in the function body — before the config load, before any other logic:

```mp3-reviewer/cmd/reviewer/main.go#L1-12
if len(os.Args) > 1 {
    switch os.Args[1] {
    case "--help", "-h":
        fmt.Print(helpText)
        os.Exit(0)
    case "--version", "-v":
        fmt.Println("song-reviewer", version)
        os.Exit(0)
    default:
        fmt.Fprintf(os.Stderr, "song-reviewer: unknown flag: %s\nRun 'song-reviewer --help' for usage.\n", os.Args[1])
        os.Exit(1)
    }
}
```

This block must come before the `// ── Load configuration ──` comment and all existing code in `main()`.

No new imports are needed — `os` and `fmt` are already imported in the file.

**Expected outcome:** The function `main()` now exits early for `--help`, `-h`, `--version`, `-v`, and unknown flags before any TUI or audio code runs. The existing startup path (no arguments) is completely unchanged.

**Verification:** `go build ./cmd/reviewer` succeeds. Running `./song-reviewer --help` prints help text and exits 0. Running `./song-reviewer --version` prints `song-reviewer dev` and exits 0. Running `./song-reviewer --unknown` prints an error to stderr and exits 1. (These can be verified by inspection after build in Step 6.)

---

### Step 3: Create `Makefile` at the project root

**Action:**

Create a new file at `mp3-reviewer/Makefile` with the following exact content. Use **tabs** for recipe indentation (not spaces — `make` requires real tab characters):

```mp3-reviewer/Makefile#L1-28
# Makefile — song-reviewer
#
# Available targets:
#   build    Compile the binary to ./song-reviewer
#   test     Run the full test suite
#   install  Install the binary to $GOPATH/bin
#   lint     Run go vet (and staticcheck if available)
#
# VERSION defaults to "dev"; override with: make build VERSION=1.2.3

VERSION ?= dev

.PHONY: build test install lint

build:
	go build -ldflags "-X main.version=$(VERSION)" -o song-reviewer ./cmd/reviewer

test:
	go test ./...

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/reviewer

lint:
	go vet ./...
	@if command -v staticcheck > /dev/null 2>&1; then staticcheck ./...; fi
```

**Expected outcome:** `mp3-reviewer/Makefile` exists at the project root with the four targets and the `VERSION ?= dev` variable.

**Verification:** Running `make build` from the project root produces a `song-reviewer` binary. Running `make test` runs the test suite. Running `make lint` runs `go vet ./...` without error.

---

### Step 4: Remove FLAC references from `README.md`

**Action:**

Open `README.md`. There is exactly **one** FLAC occurrence to fix in this file:

- **Line 15** — The Features bullet reads:
  > `**Persistence** — Writes changes directly to MP3/FLAC ID3 tags and updates...`

  Change `MP3/FLAC ID3 tags` to `MP3 ID3 tags`.

The Architecture table row for `internal/metadata/` (line 27) already reads "ID3 tag read/write logic (pure Go, `bogem/id3v2`)" and does not mention FLAC — no change needed there.

After making the FLAC fix, also add a **"Development" section** to the README, positioned between `## Installation` and `## Configuration`. The new section must document the four `make` targets as the canonical build/test/install/lint commands:

```mp3-reviewer/README.md#L1-25
## Development

With the `Makefile` at the project root you can use the following commands instead of raw `go` commands:

```bash
# Build the binary (output: ./song-reviewer)
make build

# Run the full test suite
make test

# Install into $GOPATH/bin
make install

# Run linter (go vet; also staticcheck if installed)
make lint
```

To embed a version string in the binary at build time:

```bash
make build VERSION=1.2.3
./song-reviewer --version   # prints: song-reviewer 1.2.3
```
```

Place this "Development" section as a new `##`-level heading immediately after the closing ` ``` ` fence of the `## Installation` code block and before the `## Configuration` heading.

**Expected outcome:** `README.md` contains zero occurrences of "FLAC" (case-insensitive). It also contains a new "## Development" section between Installation and Configuration.

**Verification:** Run `grep -i flac README.md` — must return no output. Visually confirm the "## Development" section is present with the four `make` commands.

---

### Step 5: Correct FLAC references in agent spec documents

**Action:**

Two agent spec files contain inaccurate FLAC references. Fix both:

**File 1: `agent-development/agent-specs/application-overview.md`**

- Locate the Persistence bullet (section "Core Workflows", item 5):
  > `Changes are written directly to the MP3/FLAC ID3 tags and the source JSON...`
- Change `MP3/FLAC ID3 tags` to `MP3 ID3 tags`.

**File 2: `agent-development/agent-specs/architecture-breakdown.md`**

- Search for any occurrence of `MP3/FLAC` in the document.
- At the time of writing this plan, a grep of the file returned **no matches** for "FLAC" — the architecture breakdown already uses MP3-only language. However, the implementing agent must **re-verify** by running `grep -i flac agent-development/agent-specs/architecture-breakdown.md` before and after the step. If a match is found, correct it to MP3-only. If no match is found, no change is needed for that file.

**Expected outcome:**
- `grep -i flac agent-development/agent-specs/application-overview.md` returns no output.
- `grep -i flac agent-development/agent-specs/architecture-breakdown.md` returns no output.

**Verification:** Run the two `grep` commands above and confirm zero output.

---

### Step 6: Update `agent-development/agent-specs/FOLDER-STRUCTURE.md`

**Action:**

Open `agent-development/agent-specs/FOLDER-STRUCTURE.md`.

1. **Update the "Last updated" line** at the top from:
   > `Task 10 — SkipApplied config flag; StateQueueComplete TUI state; metadata integration tests and testdata fixture added.`
   to:
   > `Task 11 — CLI flags (--help/--version); Makefile added; FLAC references corrected to MP3-only.`

2. **Add `Makefile` to the Project Root tree.** In the directory tree under `## Project Root`, find the section listing root-level files (near the bottom, where `.gitignore`, `go.mod`, `go.sum`, `README.md`, `settings.example.json`, `manual_review.example.json` are listed). Add a new line for the `Makefile`:

```mp3-reviewer/agent-development/agent-specs/FOLDER-STRUCTURE.md#L1-5
├── Makefile                         ← Build targets: build, test, install, lint
```

Place it before `.gitignore` (alphabetically it would come before `README.md` but after lowercase entries; for clarity, place it after `go.sum` and before `README.md` to keep the `go.*` files together and the `Makefile` adjacent to the build tooling context).

**Expected outcome:** `FOLDER-STRUCTURE.md` lists `Makefile` in the project root tree and the "Last updated" line reflects Task 11.

**Verification:** Visually confirm the `Makefile` entry is present in the directory tree and the last-updated note is correct.

---

### Step 7: Build and verify

**Action:**

Run the following commands from the project root and confirm each succeeds:

```mp3-reviewer/Makefile#L1-5
go build ./...
go vet ./...
go test ./...
```

Then run each CLI flag scenario:

```mp3-reviewer/Makefile#L1-8
# Build with default version
make build
./song-reviewer --help
./song-reviewer -h
./song-reviewer --version
./song-reviewer -v

# Build with injected version
go build -ldflags "-X main.version=1.2.3" -o song-reviewer ./cmd/reviewer
./song-reviewer --version   # expected: song-reviewer 1.2.3

# Unknown flag (should exit 1 and print to stderr)
./song-reviewer --unknown; echo "exit code: $?"
```

Then verify FLAC is gone:

```mp3-reviewer/Makefile#L1-3
grep -i flac README.md
grep -i flac agent-development/agent-specs/application-overview.md
grep -i flac agent-development/agent-specs/architecture-breakdown.md
```

All three `grep` commands must produce **no output**.

**Expected outcome:** All builds succeed, tests pass, flag outputs match expectations, and no FLAC references remain.

**Verification:** All commands above exit 0 (except the `--unknown` case which exits 1 as expected).

---

## Open Questions & Decisions

### Q1: Scope of FLAC cleanup — `.gitignore` and archived plans

**Context:** The `grep -rn -i "flac"` scan of the full project found FLAC mentions in several places beyond the README and agent specs:
- `STATE-OF-DEVELOPMENT.md` — references FLAC as a known gap (this is a factual statement about a gap, not a false feature claim).
- `.gitignore` — has a `*.flac` entry (this is a harmless practical entry, not a capability claim).
- `agent-development/done/plans/0-initialization-plan.md` and `agent-development/done/plans/6-code-audit-plan.md` — archived plans that historically mentioned FLAC.

The task request says to remove FLAC references from `README.md` and the architecture doc. It does not explicitly address `STATE-OF-DEVELOPMENT.md`, `.gitignore`, or the archived done plans.

**Options:**
- **A)** Fix only the files explicitly named in the task (`README.md`, `architecture-breakdown.md`) plus the closely related `application-overview.md` which also contains an inaccurate claim. Leave `STATE-OF-DEVELOPMENT.md`, `.gitignore`, and archived plans untouched.
- **B)** Fix every single FLAC occurrence across the entire repository, including `.gitignore`, `STATE-OF-DEVELOPMENT.md`, and the archived done plans.

**Agent's recommendation:** Option **A**. The `STATE-OF-DEVELOPMENT.md` FLAC mentions are factually correct observations about a gap — they don't mislead users and actively serve as audit trail. The `.gitignore` entry is harmless practical hygiene. The archived plans are historical records and should not be retroactively edited. Correcting only the user-facing and spec documents is the minimally invasive fix that satisfies the task's stated goal.

**Human decision:** Fix only the files explicitly named in the task. Option A.

---

### Q2: Placement of the "## Development" section in README

**Context:** The task request says to add a "Development" or "Build" section near the Installation section showing the four `make` commands. The README currently has this structure:
```
## Features
## Architecture
## Prerequisites
## Installation
## Configuration
## Usage
...
```
"Near Installation" could mean immediately after `## Installation` (before Configuration) or as a sub-section under Installation.

**Options:**
- **A)** Insert `## Development` as a new top-level `##` section immediately after `## Installation` and before `## Configuration`. This keeps it close to installation content and makes it easy to find.
- **B)** Rename the existing `## Installation` section to `## Installation & Development` and add the `make` commands as a subsection (`### Build Commands`) inside it.
- **C)** Add a `## Development` section at the very end of the README (before License), treating it as contributor-facing content.

**Agent's recommendation:** Option **A**. It is the most natural placement for a user scanning top to bottom: after they know how to install, the next thing they want to know is how to build from source. It keeps the `## Installation` section clean and doesn't require renaming it.

**Human decision:** Let's go with option A, looks more natural.

---

### Q3: `helpText` coverage of keybindings

**Context:** The task requires the `helpText` constant to list "the primary keys (seek, pause, tag, BPM tap, commit BPM/Year, skip, undo, settings, quit) — these can be condensed from the README table." The full keybindings table in the README has 10 entries. The task says "condensed," which means some paraphrasing is acceptable, but it is ambiguous whether the full list should be reproduced verbatim.

**Options:**
- **A)** Include all 10 keybindings from the README table, preserving each key and description exactly as they appear in the README (most informative for a `--help` output).
- **B)** Include all 10 keybindings but write condensed one-line descriptions to keep the `--help` output compact (e.g., `Enter/Space  Open genre selection` instead of the full sentence).
- **C)** Include only the 9 named in the task requirements (seek, pause, tag, BPM tap, commit BPM/Year, skip, undo, settings, quit) which maps to 10 key rows anyway since BPM and Year are separate commits.

**Agent's recommendation:** Option **B**. A `--help` screen should be scannable in a terminal. Condensed one-line descriptions give users everything they need without the `--help` output scrolling off screen. The README remains the place for full descriptions.

**Human decision:** Let's go with option B. Condensed one-line descriptions will keep the `--help` output compact.

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `cmd/reviewer/main.go` | Modified | Add `var version = "dev"`, `const helpText`, and flag-dispatch block at top of `main()` |
| 2 | `Makefile` | Created | `build`, `test`, `install`, `lint` targets with `VERSION ?= dev` |
| 3 | `README.md` | Modified | Replace `MP3/FLAC` with `MP3` on the Persistence bullet; add `## Development` section with `make` commands |
| 4 | `agent-development/agent-specs/application-overview.md` | Modified | Replace `MP3/FLAC ID3 tags` with `MP3 ID3 tags` in the Persistence bullet |
| 5 | `agent-development/agent-specs/architecture-breakdown.md` | Modified (conditional) | Correct any `MP3/FLAC` occurrences to `MP3` — verify by grep first |
| 6 | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Modified | Add `Makefile` to root tree; update "Last updated" line |

**Total files created: 1 | Total files modified: 5**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go vet ./...` exits 0 with no output.
- [ ] `go test ./...` passes with zero failures (no existing tests broken).
- [ ] `make build` produces a `song-reviewer` binary in the project root.
- [ ] `make test` runs the full test suite without failures.
- [ ] `make lint` runs `go vet ./...` without error.
- [ ] `./song-reviewer --help` prints usage text and exits 0.
- [ ] `./song-reviewer -h` prints usage text and exits 0.
- [ ] `./song-reviewer --version` prints `song-reviewer dev` and exits 0.
- [ ] `./song-reviewer -v` prints `song-reviewer dev` and exits 0.
- [ ] `./song-reviewer --unknown` prints an error to stderr and exits with code 1.
- [ ] `go build -ldflags "-X main.version=1.2.3" -o song-reviewer ./cmd/reviewer && ./song-reviewer --version` prints `song-reviewer 1.2.3`.
- [ ] `grep -i flac README.md` returns no matches.
- [ ] `grep -i flac agent-development/agent-specs/application-overview.md` returns no matches.
- [ ] `grep -i flac agent-development/agent-specs/architecture-breakdown.md` returns no matches.
- [ ] `README.md` contains a "## Development" section with `make build`, `make test`, `make install`, and `make lint` commands.
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` lists `Makefile` in the project root tree.
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` "Last updated" line references Task 11.
- [ ] No unrelated files were modified or deleted.

---

## Notes for the Implementing Agent

1. **The source code is the source of truth** — read `cmd/reviewer/main.go` directly before making changes. Do not rely on summaries.
2. **`var version`, not `const version`** — the `-ldflags "-X main.version=..."` mechanism requires an addressable (non-constant) package-level variable. Using `const` will cause the linker injection to silently fail.
3. **`helpText` must be a `const`** — unlike `version`, `helpText` never needs to be overridden at link time. Declare it `const` as specified by the task.
4. **Tab characters in Makefile** — if you write the Makefile content programmatically, ensure recipe lines start with a real ASCII tab (`\t`), not spaces. `make` will produce a cryptic `missing separator` error if spaces are used.
5. **Flag dispatch is the very first thing in `main()`** — place the `if len(os.Args) > 1 { switch ... }` block before the config load. This ensures that `--help` and `--version` work even if `config/settings.json` is missing.
6. **Do not use the `flag` package** — the task explicitly requires a plain `switch os.Args[1]` approach using only `os` and `fmt`.
7. **Only one FLAC occurrence in README.md** — the grep result confirmed a single FLAC mention on line 15 of the Persistence bullet. The Architecture table's `internal/metadata/` row does not mention FLAC. Change only what needs changing.
8. **`application-overview.md` also needs a FLAC fix** — the task request only explicitly names `README.md` and `architecture-breakdown.md`, but the grep scan confirmed `application-overview.md` also contains `MP3/FLAC`. Fix it as part of the same cleanup pass since it is an agent-facing spec document and the same principle applies.
9. **Do not modify `STATE-OF-DEVELOPMENT.md`, `.gitignore`, or any file in `agent-development/done/`** — these are out of scope per the recommendation in Q1 (pending human confirmation).
10. **No new Go packages or interfaces are introduced** — this task does not change the package dependency graph. The only Go change is two package-level declarations and a flag-dispatch block in `cmd/reviewer/main.go`.

# Song Reviewer CLI

A high-performance Go-based CLI tool for music enthusiasts to manually categorize song genres and enrich metadata (BPM, Release Year) through an immersive terminal interface.

## Purpose

Song Reviewer bridges the gap between automated genre-classification scripts (which may produce uncertain results) and manual ID3 tagging. It reads a JSON queue of songs flagged for manual review, plays them back in the terminal, and lets you assign genres and metadata with zero mouse interaction.

## Features

- **Review Queue** — Reads a JSON file of songs marked for `manual_review` and presents them one at a time.
- **Immersive Playback** — Songs auto-play on selection. Seek ±30s to find the defining section of the track.
- **Dual-Tier Genre Tagging** — Assign a Primary Genre (e.g., "Rock") and an optional Secondary Genre (e.g., "Psych-Rock").
- **Data Enrichment** — Fetches the original release year and BPM automatically from MusicBrainz when a song loads. If no BPM is found, it can be calculated via **Tap Tempo** (press `t` to the beat; 8 taps required). Both values can be committed to the ID3 tags with `Ctrl+1` (BPM) and `Ctrl+2` (Year).
- **Persistence** — Writes changes directly to MP3/FLAC ID3 tags and updates the source JSON to reflect "Applied" status.
- **Undo Support** — Mis-categorized a song? Press `Ctrl+U` to undo and go back.

## Architecture

```
cmd/reviewer/               — Entry point. Loads config, builds queue, starts Bubble Tea program.
internal/domain/            — Pure data structures (Task, Config). No dependencies.
internal/provider/          — JSON parser adapters (TaskProvider interface + SaveState persistence).
internal/audio/             — Audio engine (Engine struct). Handles device init, MP3 decoding, play, seek ±N seconds, pause/resume, and clean shutdown.
internal/tui/               — Bubble Tea TUI: header, progress bar, status bar, genre selection modal, keybinding dispatch.
internal/metadata/          — ID3 tag write logic (pure Go, bogem/id3v2).
internal/api/               — External HTTP clients (MusicBrainz, BPM APIs).
data/                       — Holds the manual_review.json queue file.
config/                     — Holds settings.json with app configuration.

diagrams/                   — Mermaid diagrams & references: visual documentation for humans.
├── README.md               — Diagram conventions and maintenance rule.
├── FOLDER-STRUCTURE.md     — Complete project directory tree and package dependency graph.
├── data-structures.mmd     — Class diagram of all domain types, fields, and relationships.
├── software-architecture.mmd — Packages, structs, interfaces, public methods, and call relationships.
├── ui-state-machine.mmd    — All AppState values, screens/views, and state transitions.
├── task-lifecycle.mmd      — Review-queue task lifecycle: load → review → tag → persist → advance.
└── component-data-flow.mmd — MVU pipeline data flow: Cmds, Msgs, component interactions.

user-development/           — Human-facing development assets (prompts, guides).
├── DEVELOPMENT-GUIDE.md    — Spec-driven workflow documentation.
└── prompts/                — Reusable prompt templates for humans to start agent conversations.

agent-development/          — Agent-facing pipeline (specs, requests, plans).
├── agent-specs/            — Project-level specifications (read-only context for agents).
│   ├── agent-instructions.md
│   ├── application-overview.md
│   └── architecture-breakdown.md
├── pending/                — Task requests waiting to be planned.
├── plans/                  — Implementation plans waiting for approval.
├── queued/                 — Approved plans ready for execution.
└── done/                   — Completed work (archived plans and requests).
```

### Diagrams

The `diagrams/` directory contains Mermaid (`.mmd`) diagrams and reference documents that provide visual documentation of the system's data structures, software architecture, UI state machine, task lifecycle, and component data flow. These are maintained for human readers who prefer a visual overview. The source code is the source of truth — agents read code directly and update diagrams as documentation deliverables. See `diagrams/README.md` for conventions and `diagrams/FOLDER-STRUCTURE.md` for a quick-orientation project tree.

### Design Patterns

- **Model-View-Update (MVU):** Strict Bubble Tea pattern — Model holds state, Update handles messages, View renders strings.
- **Adapter Pattern:** The `provider` package uses a `TaskProvider` interface so different JSON schemas can be supported without changing TUI code.
- **Concurrency:** Audio playback, ID3 tag writing, and JSON persistence run in background goroutines via `tea.Cmd` to keep the UI responsive.

### Technology Stack

| Component | Library |
|---|---|
| Language | Go 1.21+ |
| TUI Framework | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` |
| Audio Playback | `faiface/beep` |
| Metadata (ID3) | `bogem/id3v2` |

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
  "seek_delta_seconds": 30,
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
| `seek_delta_seconds` | Seek step in seconds for the `←` / `→` keys. Defaults to `30` if omitted. |
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

The first song in the review queue plays automatically on launch. Use the keybindings below to seek, tag, skip, or undo.

### Keybindings

| Key | Action |
|---|---|
| `←` / `→` | Seek backward / forward 30 seconds |
| `p` | Pause / Resume playback |
| `Enter` / `Space` | Open genre selection menu (two-step: Primary then Secondary) |
| `t` | Tap to the beat — calculates BPM (8 taps required; resets on irregular tapping) |
| `Ctrl+1` | Commit suggested BPM to the MP3's TBPM tag (only active when BPM is ready) |
| `Ctrl+2` | Commit suggested Year to the MP3's year tag (only active when year is found) |
| `Esc` | Skip current song and move to next |
| `Ctrl+U` | Undo last genre assignment |
| `Ctrl+C` | Quit (cleanly shuts down audio device) |

## Genre Tagging

Pressing `Enter` or `Space` opens a two-step genre selection modal:

1. **Step 1 — Primary Genre:** Scroll or type to filter the list. Press `Enter` to confirm.
2. **Step 2 — Secondary Genre:** A `[NONE]` option is available at the top of the list. Press `Enter` to confirm.

After both steps, the app:
- Writes the genre to the MP3's ID3v2 tags: a `TCON` (Content Type) frame for the primary genre and, if a secondary genre was selected, a second `TCON` frame plus a custom `TXXX` frame with description `TGENRE2` containing the secondary value.
- Updates the source JSON (`data/manual_review.json`) to set `status: "applied"` and records both genre fields.
- Automatically advances to the next song in the queue.

Press `Esc` at any point during genre selection to cancel and return to the review screen without making any changes.

## Metadata Enrichment

When a song loads, the app immediately fetches metadata in the background:

### Release Year (MusicBrainz)

The app queries the [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API) to find the **original** release year for the current song (not a remaster date). It searches by Artist and Title, finds the earliest release group's `first-release-date`, and displays the 4-digit year.

- **Yellow** — year found, not yet committed. Press `Ctrl+2` to write it to the ID3 tag.
- **Green ✓** — year committed to the file's year tag.
- **Grey italic** — loading or not found.

> **Note:** MusicBrainz requires a descriptive `User-Agent` header. Set `api_keys.musicbrainz_user_agent` in `config/settings.json` to a string like `"MySongReviewer/1.0 ( your@email.com )"`.

### BPM (MusicBrainz + Tap Tempo)

The app first attempts to fetch BPM from MusicBrainz user-contributed tags. Since MusicBrainz rarely has BPM data, the app falls back to **Tap Tempo**: press `t` repeatedly to the beat of the song. After 8 taps, the app calculates the average BPM from the inter-tap intervals.

- Tapping pauses > 3 seconds apart reset the sequence automatically.
- If tapping is too irregular (any interval deviates > 40% from the average), the sequence resets.
- The UI shows how many taps remain before a BPM value is calculated.
- **Yellow** — BPM calculated, not yet committed. Press `Ctrl+1` to write it to the ID3 `TBPM` tag.
- **Green ✓** — BPM committed to the file.

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
| `status` | Written back by the app after a successful tag write. Value will be `"applied"`. Omit or leave blank for pending songs. |
| `primary_genre` | Primary genre written by the app after tagging. |
| `secondary_genre` | Secondary genre written by the app. Empty string (field omitted) if `[NONE]` was selected. |
| `bpm` | BPM value written by the app after committing with `Ctrl+1`. Omit for songs without BPM. |
| `year` | Release year written by the app after committing with `Ctrl+2`. Omit for songs without year data. |

After tagging a song the entry will look like:

```json
{
  "filepath": "Artist/Artist - Song Title.mp3",
  "reason": "Genre not in taxonomy",
  "confidence": 4,
  "status": "applied",
  "primary_genre": "Rock",
  "secondary_genre": "Psych-Rock",
  "bpm": "120",
  "year": "1971"
}
```

## License

TBD
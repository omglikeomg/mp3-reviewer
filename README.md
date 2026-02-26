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
internal/audio/     — Audio engine (Engine struct). Handles device init, MP3 decoding, play, seek ±N seconds, pause/resume, and clean shutdown.
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
| `p` | Pause / Resume playback |
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
| `status` | Written back by the app after a successful tag write. Value will be `"Applied"`. Omit or leave blank for pending songs. |

## License

TBD
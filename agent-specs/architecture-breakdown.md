# Architecture Breakdown & Design Patterns

## Folder Structure
- `/cmd/reviewer`: Entry point. Initializes the Bubble Tea program.
- `/internal/domain`: Pure data structures (Task, Config). No dependencies.
- `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`, which parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`.
- `/internal/audio`: Wrapper for the `beep` library. Exposes `Engine` — a concurrency-safe struct that manages device initialization, MP3 decoding, playback, seeking, and pause/resume. Public surface: `NewEngine()`, `Play(path)`, `Seek(delta)`, `TogglePause()`, `GetState()`, `Close()`.
- `/internal/tui`: Bubble Tea components (Model, Update, View).
- `/internal/metadata`: ID3 tag read/write logic using pure Go libraries.
- `/internal/api`: External HTTP clients for MusicBrainz/BPM APIs.

## Design Patterns
1. **Model-View-Update (MVU):** We strictly follow the Bubble Tea pattern.
   - **Model:** Holds the current song index, audio position, and menu state.
   - **Update:** Handles messages (keypresses, clock ticks, API responses).
   - **View:** Pure string rendering. No logic allowed here.
2. **Adapter Pattern:** The `provider` package exposes a `TaskProvider` interface (`GetTasks() ([]domain.Task, error)`). The concrete implementation `ManualReviewProvider` parses the `manual_review` JSON schema. New schemas (e.g., a flat file list) can be added by implementing the same interface without touching any TUI code.
3. **Concurrency:** Audio playback and API fetching must run in background goroutines to prevent UI freezing. Use `tea.Cmd` to communicate results back to the TUI.
4. **Single Speaker Device:** `speaker.Init` is called exactly once for the process lifetime (lazy, on first `Play` call). All MP3 streams are resampled to a fixed 44100 Hz sample rate via `beep.Resample` so that files with non-standard sample rates play at the correct pitch and speed. The speaker lock (`speaker.Lock` / `speaker.Unlock`) is used exclusively for position reads and seeks — never held during file I/O.

## Technology Stack
- **Language:** Go 1.21+
- **TUI:** `charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **Audio:** `faiface/beep`
- **Metadata:** `dhowden/tag` (Pure Go)

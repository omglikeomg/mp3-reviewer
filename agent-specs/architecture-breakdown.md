# Architecture Breakdown & Design Patterns

## Folder Structure
- `/cmd/reviewer`: Entry point. Initializes the Bubble Tea program.
- `/internal/domain`: Pure data structures (Task, Config). No dependencies.
- `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`, which parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`.
- `/internal/audio`: Wrapper for the `beep` library. Handles the audio device state.
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

## Technology Stack
- **Language:** Go 1.21+
- **TUI:** `charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **Audio:** `faiface/beep`
- **Metadata:** `dhowden/tag` (Pure Go)

# Architecture Breakdown & Design Patterns

## Folder Structure
- `/cmd/reviewer`: Entry point. Loads `config/settings.json`, constructs `ReviewQueue` via `ManualReviewProvider`, creates `audio.Engine`, calls `tui.New()`, and runs the Bubble Tea program with alt-screen enabled.
- `/internal/domain`: Pure data structures (Task, Config). No dependencies.
- `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`. `GetTasks()` parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`, also populating `Genre1`/`Genre2` from any previously-saved `primary_genre`/`secondary_genre` fields. `SaveState(tasks []domain.Task) error` writes genre assignments back into the source JSON atomically (temp file + rename), setting `status="applied"`, `primary_genre`, and `secondary_genre` on matched entries. `SaveState` is a concrete method on `ManualReviewProvider` only — it is not part of the `TaskProvider` interface.
- `/internal/audio`: Wrapper for the `beep` library. Exposes `Engine` — a concurrency-safe struct that manages device initialization, MP3 decoding, playback, seeking, and pause/resume. Public surface: `NewEngine()`, `Play(path)`, `Seek(delta)`, `TogglePause()`, `GetState()`, `Close()`.
- `/internal/tui`: Bubble Tea components split across three files. `model.go` defines `Model`, `AppState` enum (`StateReviewing`, `StateGenreSelection`), all message types (`TickMsg`, `PlayErrMsg`, `TagWrittenMsg`, `SaveStateErrMsg`), the `genreItem` list adapter, and `New(queue, engine, cfg, provider)`. It also contains all `tea.Cmd` factories (`tickCmd`, `playCmd`, `writeTagsCmd`, `saveStateCmd`) and the `makeGenreList`/`openGenreModal` helpers. `update.go` implements `Init` and `Update` (key handling, tick refresh, play-error handling, genre selection dispatch via `bubbles/list` delegation, tag-write and save-state message handling). `view.go` renders two screens: the main three-row layout (`viewReviewing` — header, progress bar, status bar) and the genre selection overlay (`viewGenreModal` — two-step `bubbles/list` modal with step heading and keybind footer). Genre selection is a two-step flow: Step 1 = Primary Genre (no `[NONE]` option), Step 2 = Secondary Genre (`[NONE]` prepended). Confirmation fires `writeTagsCmd` then `saveStateCmd`; both run as non-blocking `tea.Cmd`s. `Esc` in the modal dismisses it without skipping the song.
- `/internal/metadata`: ID3 tag write logic. Exposes `WriteTags(path, primary, secondary string) error`, which opens the MP3 file and writes genre data using ID3v2 frames. Tagging strategy (Q2 decision C+D): all existing `TCON` (Content Type / Genre) frames are replaced with two new ones — the first contains the primary genre, the second the secondary genre (if non-empty). Additionally, when a secondary genre is provided, a custom `TXXX` frame with description `"TGENRE2"` is written to preserve the secondary genre for tools that read user-defined text frames. If secondary is empty (i.e. `[NONE]` was selected), only a single `TCON` frame is written and no `TXXX` frame is created. Uses `github.com/bogem/id3v2/v2` (pure Go, no CGO).
- `/internal/api`: External HTTP clients for MusicBrainz/BPM APIs.

## Design Patterns
1. **Model-View-Update (MVU):** We strictly follow the Bubble Tea pattern.
   - **Model:** Holds the current song index, audio position, and menu state.
   - **Update:** Handles messages (keypresses, clock ticks, API responses, tag-write results, save-state results).
   - **View:** Pure string rendering. No logic allowed here.
2. **Adapter Pattern:** The `provider` package exposes a `TaskProvider` interface (`GetTasks() ([]domain.Task, error)`). The concrete implementation `ManualReviewProvider` parses the `manual_review` JSON schema. New schemas (e.g., a flat file list) can be added by implementing the same interface without touching any TUI code.
3. **Concurrency:** Audio playback, ID3 tag writing, and JSON persistence must run in background goroutines to prevent UI freezing. Use `tea.Cmd` to communicate results back to the TUI.
4. **Single Speaker Device:** `speaker.Init` is called exactly once for the process lifetime (lazy, on first `Play` call). All MP3 streams are resampled to a fixed 44100 Hz sample rate via `beep.Resample` so that files with non-standard sample rates play at the correct pitch and speed. The speaker lock (`speaker.Lock` / `speaker.Unlock`) is used exclusively for position reads and seeks — never held during file I/O.

## Technology Stack
- **Language:** Go 1.21+
- **TUI:** `charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **Audio:** `faiface/beep`
- **Metadata:** `bogem/id3v2` (pure Go, read + write support)
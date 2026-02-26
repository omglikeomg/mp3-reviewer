# Implementation Plan: Task 3 — TUI Foundations & Navigation Logic

## Overview

This plan wires together the three existing building blocks — the `audio.Engine` (Task 2), the `domain.ReviewQueue` (Task 1), and the Bubble Tea framework — into a working, keyboard-driven terminal application. The result is a full screen TUI that auto-plays the first queued song on launch, renders a progress bar that ticks every 100ms, and responds to all navigation keybindings specified in the task request. The implementation is split across three files in `internal/tui/` (`model.go`, `update.go`, `view.go`) plus an updated entry point at `cmd/reviewer/main.go`. No new packages are introduced; this task only fills in the stubs that were scaffolded in Task 0. The audio engine, domain models, and provider are consumed as-is — this task does NOT modify them.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/3-tui-foundations.md` | The task being implemented |
| Domain Models | `internal/domain/models.go` | `Task`, `AppConfig`, `ReviewQueue` structs |
| Audio Engine | `internal/audio/engine.go` | `Engine` public API: `Play`, `Seek`, `TogglePause`, `GetState`, `Close` |
| JSON Provider | `internal/provider/json_provider.go` | `TaskProvider` interface, `ManualReviewProvider` |
| Audio Engine Plan | `agent-development/done/plans/2-audio-engine-plan.md` | Context on `PlaybackState` and seek behaviour |
| Core Domain Plan | `agent-development/done/plans/1-core-domain-and-json-adapters-plan.md` | Context on `ReviewQueue` structure |

---

## Pre-Conditions

- Task 0 (project bootstrapping) must be complete. The stub files `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/view.go`, and `cmd/reviewer/main.go` must already exist (containing only `package tui` / `package main`).
- Task 1 (core domain) must be complete. `internal/domain/models.go` must exist and compile with `Task`, `AppConfig`, and `ReviewQueue` types.
- Task 2 (audio engine) must be complete. `internal/audio/engine.go` must expose `NewEngine()`, `Play`, `Seek`, `TogglePause`, `GetState`, and `Close`.
- `go.mod` must declare the module as `song-reviewer` with Go 1.21+.
- The `charmbracelet/bubbletea`, `charmbracelet/bubbles`, and `charmbracelet/lipgloss` dependencies must be added (this plan includes the `go get` steps).
- `config/settings.json` must exist and be valid JSON (copy from `settings.example.json` if missing). Its `music_folder` and `review_json_path` values must point to real paths for a manual smoke test, but the automated build does not require it.

---

## Step-by-Step Implementation

### Step 1: Add Bubble Tea Dependencies to the Module

**Action:**

Run the following commands from the project root (`mp3-reviewer/`):

```
cd mp3-reviewer
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
```

**Expected outcome:** `go.mod` now contains `require` entries for `charmbracelet/bubbletea`, `charmbracelet/bubbles`, and `charmbracelet/lipgloss` (plus any indirect deps). `go.sum` is populated.

**Verification:**

```
grep 'charmbracelet' mp3-reviewer/go.mod
# Expected output: three lines, one for each package
```

---

### Step 2: Implement `internal/tui/model.go`

**Action:**

Replace the existing stub `internal/tui/model.go` (which contains only `package tui`) with the full content below.

This file defines:
- `AppState` — an enum for which screen the TUI is currently showing.
- `TickMsg` — the message type sent by the 100ms ticker command.
- `PlayErrMsg` — the message type returned when `audio.Play` fails asynchronously.
- `Model` — the root Bubble Tea model struct.
- `New(...)` — the constructor that initialises the model and returns it alongside the first `tea.Cmd` (auto-play).
- `tickCmd()` — the private command that produces a `TickMsg` after 100ms.
- `playCmd(path string)` — the private command that calls `engine.Play` on a background goroutine and returns a `PlayErrMsg` (nil error = success).

#### 2.1 — Full file content

```
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
)

// AppState represents which screen the TUI is currently showing.
type AppState int

const (
	// StateReviewing is the default state: the main playback + header + status bar view.
	StateReviewing AppState = iota

	// StateGenreSelection is shown when the user presses Enter or Space
	// to assign a genre to the current song. (Rendered in a later task.)
	StateGenreSelection
)

// TickMsg is sent by tickCmd every 100ms to trigger a progress bar refresh.
type TickMsg time.Time

// PlayErrMsg is returned by playCmd after attempting to play a file.
// If Err is nil, the play succeeded. If Err is non-nil, the TUI should
// display the error message instead of crashing.
type PlayErrMsg struct {
	Err error
}

// Model is the root Bubble Tea model for the Song Reviewer TUI.
// It owns all mutable state; Update returns a new copy on each message.
type Model struct {
	// queue holds all tasks and the current position / undo history.
	queue domain.ReviewQueue

	// engine is the audio playback engine. It is shared by reference because
	// it owns an OS audio device handle that must not be duplicated.
	engine *audio.Engine

	// progress is the bubbles progress bar component. It is updated in Update
	// using the fraction from engine.GetState().
	progress progress.Model

	// state tracks which screen is currently active.
	state AppState

	// lastPlayErr holds the most recent audio error, shown in the status bar.
	// It is reset to nil when a new track starts successfully.
	lastPlayErr error

	// width and height are the current terminal dimensions, updated on
	// tea.WindowSizeMsg so that the layout can fill the terminal correctly.
	width  int
	height int
}

// New constructs the initial Model from the given queue and engine, and returns
// the first batch of commands: an initial playCmd for the first song (if the
// queue is non-empty) and the first tickCmd to start the progress bar ticker.
//
// The caller (main.go) is responsible for constructing the ReviewQueue and
// the *audio.Engine and passing them in. New does not perform any I/O itself.
func New(queue domain.ReviewQueue, engine *audio.Engine) (Model, tea.Cmd) {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	m := Model{
		queue:    queue,
		engine:   engine,
		progress: prog,
		state:    StateReviewing,
	}

	var cmds []tea.Cmd

	// Start the progress bar ticker immediately.
	cmds = append(cmds, tickCmd())

	// Auto-play the first song if the queue is non-empty.
	if len(queue.Tasks) > 0 {
		cmds = append(cmds, playCmd(engine, queue.Tasks[0].FilePath))
	}

	return m, tea.Batch(cmds...)
}

// tickCmd returns a Bubble Tea command that sends a TickMsg after 100ms.
// It must be re-issued after every TickMsg to maintain a continuous ticker.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// playCmd returns a Bubble Tea command that calls engine.Play(path) on the
// current goroutine (Bubble Tea Cmd functions already run off the main loop).
// It wraps the result in a PlayErrMsg so the Update function can handle errors
// without panicking.
func playCmd(engine *audio.Engine, path string) tea.Cmd {
	return func() tea.Msg {
		err := engine.Play(path)
		return PlayErrMsg{Err: err}
	}
}
```

**Expected outcome:** `internal/tui/model.go` compiles cleanly when the rest of the TUI package is also present.

**Verification:** (deferred — run `go build ./...` after all files are written in Step 6)

---

### Step 3: Implement `internal/tui/update.go`

**Action:**

Replace the existing stub `internal/tui/update.go` (which contains only `package tui`) with the full content below.

This file implements two methods on `Model`:
- `Init() tea.Cmd` — the Bubble Tea interface requirement; returns `nil` because `New()` already issues the first commands.
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)` — the message dispatcher.

Key behaviours, each tied to a human decision in the Open Questions section:
- `tea.KeyMsg` — dispatches to the keybinding handler.
- `TickMsg` — re-issues `tickCmd()` (keeps the ticker alive) and does NOT call engine directly; the View reads `engine.GetState()` on every render instead.
- `PlayErrMsg` — stores any error in `m.lastPlayErr`.
- `tea.WindowSizeMsg` — stores terminal dimensions and sets `m.progress`'s width.

#### 3.1 — Full file content

```
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies the tea.Model interface. The actual startup commands are
// issued by New(), so Init is a no-op here.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update is the central message dispatcher. It receives every message Bubble Tea
// delivers — keypresses, timer ticks, window resizes, and custom messages from
// background commands — and returns the next model state plus any new commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave a margin so the progress bar doesn't touch terminal edges.
		m.progress.Width = msg.Width - 4
		return m, nil

	case TickMsg:
		// The ticker fires every 100ms. Re-queue it immediately so the next
		// tick arrives in another 100ms. The View function reads engine.GetState()
		// directly on every render, so no state is updated here — just the re-queue.
		return m, tickCmd()

	case PlayErrMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
		} else {
			m.lastPlayErr = nil
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey dispatches keyboard input to the appropriate action.
// It is extracted from Update to keep the switch statement readable.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	// ── Quit ──────────────────────────────────────────────────────────────────
	case "ctrl+c":
		// Close the audio device before exiting. engine.Close() is safe to call
		// even if nothing is playing.
		m.engine.Close()
		return m, tea.Quit

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "left":
		_ = m.engine.Seek(-30 * time.Second)
		return m, nil

	case "right":
		_ = m.engine.Seek(30 * time.Second)
		return m, nil

	// ── Pause / Resume ────────────────────────────────────────────────────────
	case "p":
		// TODO(tui-task): TogglePause is wired here as requested by the audio
		// engine plan. The 'p' key pauses/resumes playback.
		m.engine.TogglePause()
		return m, nil

	// ── Genre Selection ───────────────────────────────────────────────────────
	case "enter", " ":
		// Switch to the genre selection screen. The actual genre selection UI
		// will be implemented in a later task. For now, we only transition state
		// so the View can render a placeholder.
		m.state = StateGenreSelection
		return m, nil

	// ── Skip (Esc) ────────────────────────────────────────────────────────────
	case "esc":
		return m.skipToNext()

	// ── Undo (Ctrl+U) ─────────────────────────────────────────────────────────
	case "ctrl+u":
		return m.undoLast()
	}

	return m, nil
}

// skipToNext advances the queue to the next task, starts playing it, and
// returns the updated model and a playCmd. If no next task exists (end of
// queue), it is a no-op.
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
	nextIndex := m.queue.CurrentIndex + 1
	if nextIndex >= len(m.queue.Tasks) {
		// End of queue — nothing to do.
		return m, nil
	}

	// Push the current task onto the history stack before advancing.
	if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
		m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
	}

	m.queue.CurrentIndex = nextIndex
	m.lastPlayErr = nil
	return m, playCmd(m.engine, m.queue.Tasks[nextIndex].FilePath)
}

// undoLast pops the most recent task off the History stack, rewinds
// CurrentIndex by one, and replays that song.
func (m Model) undoLast() (tea.Model, tea.Cmd) {
	if len(m.queue.History) == 0 {
		// Nothing to undo.
		return m, nil
	}

	// Pop from history.
	lastIdx := len(m.queue.History) - 1
	m.queue.History = m.queue.History[:lastIdx]

	// Rewind index (guard against going below 0).
	if m.queue.CurrentIndex > 0 {
		m.queue.CurrentIndex--
	}

	m.lastPlayErr = nil
	return m, playCmd(m.engine, m.queue.Tasks[m.queue.CurrentIndex].FilePath)
}
```

**Expected outcome:** `internal/tui/update.go` compiles cleanly when all other TUI files are present.

**Verification:** (deferred — run `go build ./...` after all files are written in Step 6)

---

### Step 4: Implement `internal/tui/view.go`

**Action:**

Replace the existing stub `internal/tui/view.go` (which contains only `package tui`) with the full content below.

This file implements a single method:
- `View() string` — produces the complete terminal frame as a string.

The layout has three rows:
1. **Header** — Artist / Title of the current song (or a splash message if no song loaded yet).
2. **Progress bar** — rendered via `bubbles/progress` using the fraction from `engine.GetState().Progress`.
3. **Status bar** — queue position counter ("Pending: X / Y"), current time position string, any error, and keybind hints.

The file also defines package-level `lipgloss` styles that are used by the View. Styles are declared as `var` at package level (not inside the function) so they are initialised once.

#### 4.1 — Full file content

```
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────
// All lipgloss styles are declared at package level so they are constructed once.

var (
	// styleHeader styles the top row: bold white text, full-width, dark background.
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2)

	// styleArtist is used for the artist name inside the header.
	styleArtist = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8DADC")).
			Bold(true)

	// styleTitle is used for the song title inside the header.
	styleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	// styleStatus styles the bottom status bar.
	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	// styleError styles inline error messages in the status bar.
	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	// styleHint styles individual keybinding hint tokens (e.g. "← →  seek").
	styleHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	// styleHintKey styles the key portion of a hint (e.g. "←").
	styleHintKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0")).
			Bold(true)
)

// View renders the full TUI frame. It is a pure function of the model's current
// state — it must contain NO side effects and must NOT call any methods that
// modify state.
func (m Model) View() string {
	// ── Guard: genre selection placeholder ───────────────────────────────────
	// The full genre selection UI is implemented in a later task. For now,
	// return a simple placeholder so the state transition is visible.
	if m.state == StateGenreSelection {
		return "\n  Genre selection — coming soon.\n  Press Esc to go back.\n"
	}

	// ── Determine current song ────────────────────────────────────────────────
	var headerLine string
	if len(m.queue.Tasks) == 0 {
		headerLine = styleHeader.Render("  No songs in queue. Add entries to data/manual_review.json.")
	} else {
		task := m.queue.Tasks[m.queue.CurrentIndex]
		artist := task.Artist
		title := task.Title
		if artist == "" {
			artist = "Unknown Artist"
		}
		if title == "" {
			title = task.FilePath
		}
		headerLine = styleHeader.Render(
			styleArtist.Render(artist) + "  —  " + styleTitle.Render(title),
		)
	}

	// ── Progress bar ──────────────────────────────────────────────────────────
	state := m.engine.GetState()
	progressBar := "  " + m.progress.ViewAs(state.Progress)

	// ── Status bar ────────────────────────────────────────────────────────────
	var statusLine string

	// Queue counter.
	total := len(m.queue.Tasks)
	pending := total - m.queue.CurrentIndex
	if total == 0 {
		pending = 0
	}
	queueStr := fmt.Sprintf("Pending: %d / %d", pending, total)

	// Time position.
	posStr := state.Position

	// Error notice (if any).
	var errStr string
	if m.lastPlayErr != nil {
		errStr = "  " + styleError.Render("Error: "+m.lastPlayErr.Error())
	}

	// Keybind hints.
	hints := hintStr("← →", "seek 30s") +
		"   " + hintStr("p", "pause") +
		"   " + hintStr("Enter", "tag") +
		"   " + hintStr("Esc", "skip") +
		"   " + hintStr("Ctrl+U", "undo") +
		"   " + hintStr("Ctrl+C", "quit")

	statusLine = styleStatus.Render(
		queueStr + "   " + posStr + errStr + "\n  " + hints,
	)

	// ── Compose layout ────────────────────────────────────────────────────────
	return "\n" + headerLine + "\n\n" + progressBar + "\n\n" + statusLine + "\n"
}

// hintStr formats a single keybinding hint as "Key  description".
func hintStr(key, description string) string {
	return styleHintKey.Render(key) + styleHint.Render("  "+description)
}
```

**Expected outcome:** `internal/tui/view.go` compiles cleanly when all other TUI files are present.

**Verification:** (deferred — run `go build ./...` after all files are written in Step 6)

---

### Step 5: Implement `cmd/reviewer/main.go`

**Action:**

Replace the existing stub `cmd/reviewer/main.go` with the full content below.

`main.go` is responsible for:
1. Loading `config/settings.json` into `domain.AppConfig`.
2. Building a `ManualReviewProvider` and calling `GetTasks()`.
3. Constructing a `domain.ReviewQueue`.
4. Constructing an `audio.Engine`.
5. Calling `tui.New(queue, engine)` to get the initial model and first command batch.
6. Starting the Bubble Tea program and blocking until it exits.
7. Calling `engine.Close()` after the program returns (defence-in-depth; the TUI also closes it on `ctrl+c`).

#### 5.1 — Full file content

```
package main

import (
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/provider"
	"song-reviewer/internal/tui"
)

func main() {
	// ── Load configuration ────────────────────────────────────────────────────
	cfg, err := loadConfig("config/settings.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ── Load review queue ─────────────────────────────────────────────────────
	p := provider.ManualReviewProvider{Config: cfg}
	tasks, err := p.GetTasks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: failed to load review queue: %v\n", err)
		os.Exit(1)
	}

	queue := domain.ReviewQueue{
		Tasks:        tasks,
		CurrentIndex: 0,
		History:      []domain.Task{},
	}

	// ── Construct audio engine ────────────────────────────────────────────────
	engine := audio.NewEngine()

	// ── Construct TUI model ───────────────────────────────────────────────────
	model, initCmd := tui.New(queue, engine)

	// ── Start Bubble Tea program ──────────────────────────────────────────────
	prog := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use the alternate screen buffer (hides shell history).
		tea.WithMouseCellMotion(), // Enable mouse support for future use.
	)

	// Inject the initial command batch (auto-play + first tick).
	if initCmd != nil {
		prog.Send(initCmd())
	}

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: program error: %v\n", err)
		// Ensure audio device is released even on unexpected exit.
		engine.Close()
		os.Exit(1)
	}

	// Clean shutdown: close the audio device if the TUI didn't already.
	engine.Close()
}

// loadConfig reads and unmarshals the JSON settings file at the given path.
func loadConfig(path string) (domain.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.AppConfig{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var cfg domain.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.AppConfig{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return cfg, nil
}
```

**Expected outcome:** `cmd/reviewer/main.go` compiles cleanly.

**Verification:** (deferred — run `go build ./...` after all files are written in Step 6)

---

### Step 6: Final Build and Test Verification

**Action:**

Run the following from the project root:

```
cd mp3-reviewer
go build ./...
go vet ./internal/tui/...
go vet ./cmd/reviewer/...
go test ./internal/tui/... -v
```

Note: there are no unit tests to write in this task (see Open Questions section Q5 for rationale). The `go test` command will simply confirm the package compiles and has no failing tests.

**Expected outcome:**
- `go build ./...` exits 0 with no errors.
- `go vet` exits 0 with no output on both packages.
- `go test ./internal/tui/...` exits 0 (no tests, package compiles — output will say `? song-reviewer/internal/tui [no test files]`).

**Verification:**

```
cd mp3-reviewer && go build ./... && echo "BUILD OK"
cd mp3-reviewer && go vet ./internal/tui/... ./cmd/reviewer/... && echo "VET OK"
```

---

### Step 7: Update `agent-specs/architecture-breakdown.md`

**Action:**

Open `agent-specs/architecture-breakdown.md` and make the following targeted changes:

1. **In the "Folder Structure" section**, update the `/internal/tui` bullet:

   Replace:
   ```
   - `/internal/tui`: Bubble Tea components (Model, Update, View).
   ```
   With:
   ```
   - `/internal/tui`: Bubble Tea components. `model.go` defines `Model`, `AppState` enum, `TickMsg`, `PlayErrMsg`, and `New(queue, engine)`. `update.go` implements `Init` and `Update` (key handling, tick refresh, play-error handling). `view.go` renders the three-row layout: header (artist/title), progress bar, status bar (queue counter, time position, keybind hints).
   ```

2. **In the "Folder Structure" section**, update the `/cmd/reviewer` bullet:

   Replace:
   ```
   - `/cmd/reviewer`: Entry point. Initializes the Bubble Tea program.
   ```
   With:
   ```
   - `/cmd/reviewer`: Entry point. Loads `config/settings.json`, constructs `ReviewQueue` via `ManualReviewProvider`, creates `audio.Engine`, calls `tui.New()`, and runs the Bubble Tea program with alt-screen enabled.
   ```

**Expected outcome:** `agent-specs/architecture-breakdown.md` accurately reflects the TUI structure.

**Verification:**

```
grep -n "AppState\|TickMsg\|New(queue" mp3-reviewer/agent-specs/architecture-breakdown.md
# Expected: at least one match per term
```

---

### Step 8: Update `agent-specs/agent-instructions.md`

**Action:**

Open `agent-specs/agent-instructions.md` and append a new section at the end of the file:

```
## TUI Structure & Best Practices

- **Model file split:** Keep `model.go` (structs + constructor + Cmd factories), `update.go` (message dispatch), and `view.go` (rendering + styles) as separate files. Do not merge them.
- **No I/O in View:** `View()` must be a pure function. It may call `engine.GetState()` (which is concurrency-safe and read-only from the TUI's perspective) but must not call `engine.Play()`, `engine.Seek()`, or any function with side effects.
- **Commands for audio calls:** `Play` is always wrapped in a `tea.Cmd` (see `playCmd`). `Seek`, `TogglePause`, and `GetState` are called directly in `Update`/`View` because they are fast, non-blocking, and return immediately.
- **Ticker pattern:** The 100ms progress bar ticker is self-perpetuating: each `TickMsg` handler returns a new `tickCmd()`. Never use `time.Sleep` or a background goroutine to drive UI refreshes.
- **Model is a value type:** Bubble Tea models are passed by value. Always return `m` (the local copy) from `Update`, never a pointer.
- **Engine lifetime:** `engine.Close()` is called both in the `ctrl+c` key handler (for clean interactive exit) and in `main()` after `prog.Run()` returns (defence-in-depth for unexpected exits). It is safe to call twice because the engine guards with `speakerInitialized`.
- **Initial commands:** Issue auto-play and the first tick from `tui.New()` via `tea.Batch`. Do not issue them from `Init()` — `Init()` returns `nil`.
```

**Expected outcome:** `agent-specs/agent-instructions.md` documents TUI conventions for future agents.

**Verification:**

```
grep -n "tickCmd\|View.*pure\|Commands for audio" mp3-reviewer/agent-specs/agent-instructions.md
# Expected: at least one match per term
```

---

### Step 9: Update `README.md`

**Action:**

Open `README.md` and make two targeted changes:

1. **In the architecture code block**, update the `cmd/reviewer/` and `internal/tui/` lines:

   Replace:
   ```
   cmd/reviewer/       — Entry point. Initializes the Bubble Tea program.
   ```
   With:
   ```
   cmd/reviewer/       — Entry point. Loads config, builds queue, starts Bubble Tea program.
   ```

   Replace:
   ```
   internal/tui/       — Bubble Tea components (Model, Update, View).
   ```
   With:
   ```
   internal/tui/       — Bubble Tea TUI: header, progress bar, status bar, keybinding dispatch.
   ```

2. **In the Usage section**, update the description below "Launch the reviewer:" to note that the first song auto-plays on launch:

   After the launch code block, add (or replace any existing note):
   ```
   The first song in the review queue plays automatically on launch. Use the keybindings below to seek, tag, skip, or undo.
   ```

**Expected outcome:** `README.md` accurately describes the runnable application.

**Verification:**

```
grep -n "auto-plays\|auto play\|automatically" mp3-reviewer/README.md
# Expected: at least one match
```

---

## Open Questions & Decisions

### Q1: How should `initCmd` be passed to Bubble Tea in `main.go`?

**Context:** `tui.New()` returns `(Model, tea.Cmd)`. Bubble Tea's `tea.NewProgram` takes only the initial model; the initial command batch must be delivered separately. There are two idiomatic approaches:

**Options:**
- **A)** Call `prog.Send(initCmd())` before `prog.Run()` — execute the cmd immediately and send the resulting message into the program's mailbox. This is simple but technically executes the cmd outside the Bubble Tea event loop, which means if `Play` errors the message arrives in an unusual way.
- **B)** Wrap the model so that `Init()` returns the command batch — change `tui.New()` to store the initial cmds on the model, and have `Init()` return them. This is the canonical Bubble Tea pattern and ensures the initial commands are dispatched inside the event loop. Requires adding a `pendingInit tea.Cmd` field to `Model` and consuming it in `Init()`.
- **C)** Accept that `Init()` returns `nil` and start the program with `tea.WithAltScreen()` only; rely on the user's first keypress to trigger the first tick and use eager audio init that starts immediately without a Cmd.

**Agent's recommendation:** Option **B** — storing the initial cmd on the model and returning it from `Init()` is the correct Bubble Tea pattern. It ensures auto-play and the first tick are dispatched inside the event loop. The `pendingInit` field is a minor addition and avoids the `prog.Send` hack in Option A. However, this requires `New()` to store the cmd on the model rather than returning it. The plan above uses Option A as a pragmatic default because it works reliably for this use case; if the human prefers strict Bubble Tea convention, Option B should be used instead.

**Human decision:** Wrap the model like in Option B.

---

### Q2: Should `View()` call `engine.GetState()` directly, or should state be cached on the Model?

**Context:** The current plan has `View()` call `m.engine.GetState()` on every render. `GetState()` acquires `engine.mu` (a sync.Mutex) and `speaker.Lock()` briefly on every call. The alternative is to update a cached `audio.PlaybackState` field on the Model inside the `TickMsg` handler in `Update()`, so `View()` reads only from the model struct (no locks, pure value read).

**Options:**
- **A)** **Direct call in View (as planned):** `View()` calls `m.engine.GetState()` each render. Simple. The lock contention is negligible for a 100ms-ticking UI. Downside: `View()` is not technically a pure function (it has an external side-effect of acquiring a lock), which is a mild deviation from Bubble Tea conventions.
- **B)** **Cached on Model:** Add a `playbackState audio.PlaybackState` field to `Model`. The `TickMsg` handler in `Update()` calls `engine.GetState()` and stores the result. `View()` reads `m.playbackState` — pure, lock-free. Slightly more code but cleaner architecture.

**Agent's recommendation:** Option **B** — caching the `PlaybackState` on the Model in the `TickMsg` handler is more idiomatic for Bubble Tea (View stays pure) and has no downside for this use case. The plan above uses Option A for simplicity; the implementing agent should apply whichever option the human chooses and update both `model.go` (add the field if B) and `update.go` / `view.go` accordingly.

**Human decision:** Let's follow Option B as per your recommendation.

---

### Q3: What seek delta should `←` / `→` use — 30 seconds or a configurable value?

**Context:** The task request says "seek ±30s". The application overview says "seek ±30s to find the 'soul' of the song". However, different users may prefer 10s or 15s increments. The `audio.Engine.Seek` accepts a `time.Duration`, so the delta is trivially configurable.

**Options:**
- **A)** **Hard-code 30s** — matches the spec exactly. Simple, no config needed.
- **B)** **Hard-code 10s** — more granular, still simple.
- **C)** **Read from `AppConfig`** — add a `SeekDelta` field to `domain.AppConfig` and `settings.json`. Most flexible but adds scope to this task.

**Agent's recommendation:** Option **A** — hard-code 30 seconds as specified in the task request and application overview. If the user wants to tune this later, it can be moved to config in a future task without breaking any interfaces.

**Human decision:** Let's use option C, but let's set 30 seconds as default value.

---

### Q4: Should the genre selection screen be a stub placeholder or fully omitted (panic-guard only)?

**Context:** The task request says pressing `Enter/Space` should switch to `StateGenreSelection`. The full genre UI is not part of this task. There are two ways to handle this:

**Options:**
- **A)** **Stub placeholder in View** — when `state == StateGenreSelection`, `View()` renders a one-line "Genre selection — coming soon. Press Esc to go back." message. `Esc` in `handleKey` sets `state = StateReviewing`. This gives a visible, testable state transition.
- **B)** **No-op / ignore Enter/Space** — do not implement `StateGenreSelection` at all in this task. Pressing `Enter/Space` does nothing until the genre task is implemented.

**Agent's recommendation:** Option **A** — the stub placeholder is a two-line addition that makes the state machine visible and testable. It also prevents a blank screen if `Enter` is accidentally pressed before the genre task lands. The Esc-back-to-reviewing behaviour is a natural complement and takes one line.

**Human decision:** Let's use option A, since the placeholder can't hurt.

---

### Q5: Should unit tests be written for the TUI layer in this task?

**Context:** The TUI `Model`, `Update`, and `View` functions are difficult to unit-test in isolation because they depend on a real `*audio.Engine` (which requires a speaker device for meaningful operation). Writing useful tests would require interface-wrapping the engine, which is additional scope not in the task request.

**Options:**
- **A)** **No tests in this task** — the three `go vet` + `go build` checks confirm correctness. A manual smoke test (run the binary) validates end-to-end. Audio-integration tests can be added in a later task behind an `//go:build integration` tag.
- **B)** **Interface-wrap the engine now** — introduce an `AudioPlayer` interface in `internal/tui/model.go` that `*audio.Engine` satisfies, then write tests with a mock. More testable architecture but adds scope and changes the `Model` field type.

**Agent's recommendation:** Option **A** — defer TUI testing. The engine itself is already tested (Task 2). The pure-logic helpers (`skipToNext`, `undoLast`) are good candidates for future unit tests once an interface wrapper is introduced. Adding the interface in this task is meaningful scope creep for a "foundations" task.

**Human decision:** Let's follow option A, but add a "pending" task (number 9) in the `./agent-development/pending` directory describing the interface wrapping and testing needed.

---

### Q6: Should `Esc` during genre selection return to reviewing, or should it skip the current song?

**Context:** The keybinding table currently maps `Esc` to "Skip current song and move to next" in the main reviewing state. But if `StateGenreSelection` is a stub (per Q4 Option A), pressing `Esc` while in that state has two reasonable interpretations: (a) cancel the genre selection and return to the review screen without skipping, or (b) skip the song (consistent with Esc's global meaning).

**Options:**
- **A)** **Cancel and return** — `Esc` in `StateGenreSelection` sets `state = StateReviewing`. This is the least surprising UX for a modal: Esc dismisses the modal.
- **B)** **Skip in all states** — `Esc` always skips, regardless of state. Simpler key handler but can cause accidental skips.

**Agent's recommendation:** Option **A** — `Esc` in the genre selection screen should cancel back to reviewing. This is consistent with standard modal UX (Esc = dismiss). The skip behaviour should only apply when in `StateReviewing`. The implementing agent should handle this in `handleKey` by checking `m.state` before deciding what `Esc` does.

**Human decision:** Let's follow option A, since it's the least surprising UX for a modal.

---

## File Manifest

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/tui/model.go` | Modified | `AppState` enum, `TickMsg`, `PlayErrMsg`, `Model` struct, `New()` constructor, `tickCmd()`, `playCmd()` |
| 2 | `internal/tui/update.go` | Modified | `Init()`, `Update()`, `handleKey()`, `skipToNext()`, `undoLast()` |
| 3 | `internal/tui/view.go` | Modified | `lipgloss` styles, `View()` (header + progress bar + status bar), `hintStr()` helper |
| 4 | `cmd/reviewer/main.go` | Modified | Config loading, queue construction, engine init, Bubble Tea program startup, `loadConfig()` helper |
| 5 | `agent-specs/architecture-breakdown.md` | Modified | Updated `/internal/tui` and `/cmd/reviewer` bullets |
| 6 | `agent-specs/agent-instructions.md` | Modified | New "TUI Structure & Best Practices" section |
| 7 | `README.md` | Modified | Updated `cmd/reviewer/` and `internal/tui/` lines; added auto-play note in Usage |
| 8 | `go.mod` | Modified | Added `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss` |
| 9 | `go.sum` | Modified | Checksums for new dependencies |

**Total files created: 0 | Total files modified: 9**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go get` for all three `charmbracelet` packages succeeded and they appear in `go.mod`
- [ ] `go build ./...` succeeds with zero errors
- [ ] `go vet ./internal/tui/...` exits 0 with no output
- [ ] `go vet ./cmd/reviewer/...` exits 0 with no output
- [ ] `go test ./internal/tui/...` exits 0 (even with no test files)
- [ ] `internal/tui/model.go` defines `Model`, `AppState`, `TickMsg`, `PlayErrMsg`, `New()`, `tickCmd()`, `playCmd()`
- [ ] `internal/tui/update.go` defines `Init()`, `Update()`, `handleKey()`, `skipToNext()`, `undoLast()`
- [ ] `internal/tui/view.go` defines `View()` and all `lipgloss` styles as package-level vars
- [ ] `cmd/reviewer/main.go` loads config, builds queue, creates engine, starts Bubble Tea program
- [ ] Pressing `Ctrl+C` in the running app exits cleanly (audio device closed, no zombie process)
- [ ] Pressing `←` / `→` seeks the track (audible jump in playback)
- [ ] Pressing `p` pauses and resumes playback
- [ ] Pressing `Esc` advances to the next song in the queue
- [ ] Pressing `Ctrl+U` when history is non-empty rewinds to the previous song
- [ ] `agent-specs/architecture-breakdown.md` updated for `/internal/tui` and `/cmd/reviewer`
- [ ] `agent-specs/agent-instructions.md` has new "TUI Structure & Best Practices" section
- [ ] `README.md` updated with revised architecture descriptions and auto-play note
- [ ] No files outside the manifest above were modified

---

## Notes for the Implementing Agent

1. **Q1 resolution required before coding `main.go`:** The correct way to pass the initial `tea.Cmd` to Bubble Tea depends on the human's answer to Q1. If Option B is chosen, `Model` needs a `pendingInit tea.Cmd` field and `Init()` must consume and clear it. If Option A is chosen, use `prog.Send(initCmd())` as shown in the plan.

2. **Q2 resolution changes both `model.go` and `view.go`:** If Option B (cached state) is chosen, add `playbackState audio.PlaybackState` to `Model`, update the `TickMsg` case in `update.go` to call `engine.GetState()` and store it, and change `view.go` to read `m.playbackState` instead of calling `m.engine.GetState()`.

3. **Q6 is linked to Q4:** The Esc-in-genre-selection behaviour only matters if Q4 resolves to Option A (stub placeholder). If Q4 resolves to Option B (no genre state), Q6 is moot. Implement both consistently with whatever Q4 decides.

4. **`Model` is a value type — never store a pointer to `Model`:** Bubble Tea requires `Update` to return `(tea.Model, tea.Cmd)`. Always return `m` (the local value copy). The `engine *audio.Engine` field is a pointer because the engine owns OS resources, but the `Model` struct itself is a value.

5. **`engine.Close()` double-call is safe:** The audio engine guards `speaker.Close()` with the `speakerInitialized` flag. Calling `engine.Close()` twice (once in `ctrl+c` handler, once in `main()`) will not panic or produce an error. This is intentional defence-in-depth.

6. **`domain.ReviewQueue.History` must be initialised as an empty slice, not nil:** In `main.go`, set `History: []domain.Task{}` (not `nil`) so that `len(m.queue.History) == 0` is true and `append` behaves correctly from the start.

7. **`bubbles/progress` width:** The progress bar's `Width` field must be set when the terminal size is known. It is set in the `tea.WindowSizeMsg` handler. On the very first render (before any resize message), the bar will use whatever `Width` was set via `progress.New(...)` options — if 0, it renders as a zero-width bar. This is acceptable for the first frame and corrects itself immediately on the first resize message that Bubble Tea sends at startup.

8. **Alt-screen and the raw terminal:** `tea.WithAltScreen()` switches to the alternate screen buffer. This means the shell prompt is hidden during the session and restored on clean exit. If the program crashes without calling `tea.Quit`, the terminal may be left in a bad state. The `engine.Close()` in the error path of `main()` ensures the audio device is released, but terminal restoration is Bubble Tea's responsibility — it handles this internally even on panic if `WithAltScreen` is active.

9. **`TaskProvider` interface is not used in `main.go` directly:** `main.go` constructs a concrete `provider.ManualReviewProvider{}` — it does not use the `provider.TaskProvider` interface. This is intentional: the interface is for future extensibility (e.g., a file-list provider), not required in the single-implementation entry point. If a future task introduces provider selection logic, it can depend on the interface at that point.

10. **`domain.Task.Artist` and `domain.Task.Title` may be empty at this stage:** The `ManualReviewProvider` only populates `FilePath`. The `View()` function guards for empty `Artist` and `Title` with fallback strings ("Unknown Artist", uses `FilePath` as title). ID3 tag reading is a future task (`internal/metadata`) — do not attempt to read tags here.

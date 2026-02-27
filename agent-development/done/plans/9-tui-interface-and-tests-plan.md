# Implementation Plan: Task 9 — TUI AudioPlayer Interface & Unit Tests

## Overview

This plan introduces a thin `AudioPlayer` interface inside `internal/tui/model.go` so that
the TUI's `engine` field can be satisfied by a mock in tests rather than requiring a real OS
audio device. Once the interface is in place, two new test files are added to the `tui`
package: a `mockPlayer` implementation and a table of 10 unit tests that cover the most
important queue-manipulation and key-handling paths in `update.go`.

The change to production code is minimal and non-breaking: `*audio.Engine` already implements
every method in the interface by name and signature, so `cmd/reviewer/main.go` continues to
compile unchanged — Go's structural typing means no explicit cast or adapter is needed. The
only source files that change in production code are `internal/tui/model.go` (interface
declaration + two type changes) and the documentation files.

**Scope:** Two type changes + one new interface declaration in `model.go`; two new test-only
files; one documentation update. No new packages, no `go get`, no schema changes.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree and package dependency graph |
| Task Definition | `agent-development/pending/9-tui-interface-and-tests.md` | The task being implemented |
| TUI Model | `internal/tui/model.go` | Full Model struct, field types, New() signature, playCmd, all tea.Cmd factories |
| TUI Update | `internal/tui/update.go` | handleKey, skipToNext, undoLast, TickMsg handler — the functions being tested |
| Audio Engine | `internal/audio/engine.go` | Concrete method signatures that the interface must match exactly |
| Domain Models | `internal/domain/models.go` | Task, ReviewQueue, AppConfig structs used to build test fixtures |
| Entry Point | `cmd/reviewer/main.go` | Confirm tui.New() call site — must compile unchanged after the interface swap |

---

## Pre-Conditions

- Tasks 0 through 8 are complete and `go build ./...` and `go test ./...` both pass cleanly.
- `internal/tui/model.go` exists and defines `Model` with `engine *audio.Engine`.
- `internal/audio/engine.go` exists and defines `*Engine` with methods `Play`, `Seek`,
  `TogglePause`, `GetState`, and `Close` matching the signatures listed in Step 2.
- `cmd/reviewer/main.go` calls `tui.New(queue, engine, cfg, p)` where `engine` is
  `*audio.Engine` returned by `audio.NewEngine()`.
- No existing test files exist in `internal/tui/` (confirmed by project inspection).
- No `go get` or module changes are required — all needed packages are already in `go.mod`.

---

## Step-by-Step Implementation

### Step 1: Confirm the Baseline

**Action:**
```
cd mp3-reviewer && go build ./... && go test ./...
```

**Expected outcome:**
All packages compile and all existing tests pass. Zero failures. This confirms the starting
state before any changes are made.

**Verification:**
Terminal output shows no `FAIL` lines and no build errors.

---

### Step 2: Add `AudioPlayer` Interface and Update `Model` + `New()` in `model.go`

**Action:**
Open `internal/tui/model.go`. Make three targeted changes:

**2.1 — Remove the `internal/audio` import from `model.go`.**

Currently `model.go` imports:
```
"song-reviewer/internal/audio"
```
This import is used in three places:
- The `engine *audio.Engine` field type on `Model`.
- `New()` parameter `engine *audio.Engine`.
- `playCmd(engine *audio.Engine, path string)` function signature.
- `audio.PlaybackState` type used as `playbackState` field type and in `GetState()` return.

After the change, `audio` is still needed for `audio.PlaybackState` (the return type of
`GetState()` in the interface, and the type of `Model.playbackState`). Therefore **keep**
the `"song-reviewer/internal/audio"` import — it is still required.

**2.2 — Declare the `AudioPlayer` interface.**

Add the following block **immediately before** the `// Model is the root...` comment
(i.e., just before the `Model` struct declaration). The interface must appear in `model.go`
so it is visible to both `update.go` (which calls engine methods) and the test files.

```go
// AudioPlayer is the subset of *audio.Engine methods used by the TUI.
// Declaring this interface in the tui package (rather than audio) allows the TUI
// to be tested with a mock without importing a real audio device.
// *audio.Engine satisfies this interface automatically via Go structural typing —
// no changes are needed in internal/audio.
type AudioPlayer interface {
	Play(path string) error
	Seek(delta time.Duration) error
	TogglePause()
	GetState() audio.PlaybackState
	Close()
}
```

**2.3 — Change the `engine` field type on `Model`.**

Find the field:
```go
// engine is the audio playback engine. It is shared by reference because
// it owns an OS audio device handle that must not be duplicated.
engine *audio.Engine
```

Change the type to `AudioPlayer`:
```go
// engine is the audio playback engine. It is shared by reference because
// it owns an OS audio device handle that must not be duplicated.
// The AudioPlayer interface allows tests to substitute a mock without a
// real OS audio device.
engine AudioPlayer
```

**2.4 — Change the `New()` parameter type.**

Find the function signature:
```go
func New(queue domain.ReviewQueue, engine *audio.Engine, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
```

Change `engine *audio.Engine` to `engine AudioPlayer`:
```go
func New(queue domain.ReviewQueue, engine AudioPlayer, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
```

**2.5 — Change `playCmd`'s engine parameter type.**

Find the `playCmd` factory function:
```go
func playCmd(engine *audio.Engine, path string) tea.Cmd {
```

Change it to:
```go
func playCmd(engine AudioPlayer, path string) tea.Cmd {
```

No other changes to the body of `playCmd` are needed.

**Expected outcome:**
`internal/tui/model.go` compiles. The `AudioPlayer` interface is declared. `Model.engine`
is type `AudioPlayer`. `New()` accepts `AudioPlayer`. `playCmd` accepts `AudioPlayer`.
`cmd/reviewer/main.go` still compiles without any changes because `*audio.Engine` satisfies
`AudioPlayer` structurally.

**Verification:**
```
cd mp3-reviewer && go build ./...
```
Zero errors. In particular `cmd/reviewer` must compile — it passes `*audio.Engine` to
`tui.New()` and that assignment must succeed via implicit interface satisfaction.

---

### Step 3: Create `internal/tui/mock_player_test.go`

**Action:**
Create a new file `internal/tui/mock_player_test.go` with the content below.
This file is in package `tui` (not `tui_test`) so it has access to unexported fields
and types, which the tests need (e.g. `enrichStatus`, `StateReviewing`, etc.).

```go
package tui

import (
	"time"

	"song-reviewer/internal/audio"
)

// mockPlayer is a test-only implementation of AudioPlayer.
// All methods record their calls so tests can assert on them.
// Zero value is safe to use; all error fields default to nil (no errors).
type mockPlayer struct {
	// Configuration fields — set these before calling methods.
	playErr  error              // Error to return from Play. nil = success.
	seekErr  error              // Error to return from Seek. nil = success.
	state    audio.PlaybackState // Value to return from GetState.

	// Observation fields — inspect these after calling methods.
	playCalled  []string        // Paths passed to Play, in order.
	seekDeltas  []time.Duration // Deltas passed to Seek, in order.
	toggleCount int             // Number of times TogglePause was called.
	closed      bool            // True if Close was called at least once.
}

func (m *mockPlayer) Play(path string) error {
	m.playCalled = append(m.playCalled, path)
	return m.playErr
}

func (m *mockPlayer) Seek(delta time.Duration) error {
	m.seekDeltas = append(m.seekDeltas, delta)
	return m.seekErr
}

func (m *mockPlayer) TogglePause() {
	m.toggleCount++
}

func (m *mockPlayer) GetState() audio.PlaybackState {
	return m.state
}

func (m *mockPlayer) Close() {
	m.closed = true
}
```

**Expected outcome:**
File compiles as part of `package tui`. `mockPlayer` satisfies `AudioPlayer` — the compiler
will verify this implicitly when tests pass a `*mockPlayer` wherever `AudioPlayer` is
expected. No test functions are defined in this file; it is a shared helper for
`update_test.go`.

**Verification:**
```
cd mp3-reviewer && go build ./internal/tui/...
```
Zero errors. (Test files are compiled by `go test`, not `go build`, so this just checks
there are no syntax errors visible to the compiler at this stage. The full compile-and-run
check happens in Step 5.)

---

### Step 4: Create `internal/tui/update_test.go`

**Action:**
Create a new file `internal/tui/update_test.go` in `package tui` with the ten test
functions described below.

**4.1 — Test helper: `newTestModel`**

Define a private helper that constructs a minimal `Model` suitable for tests. It uses
`*mockPlayer` as the engine, a pre-populated queue with two songs, a default config, and
a no-op `provider.ManualReviewProvider`.

**4.2 — The ten test functions**

Below is the complete file content to create:

```go
package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/provider"
)

// newTestModel builds a minimal Model for use in unit tests.
// It uses a *mockPlayer so no OS audio device is required.
// The queue is pre-populated with two fake tasks so that skipToNext and
// undoLast have something to work with.
func newTestModel(mp *mockPlayer) Model {
	tasks := []domain.Task{
		{FilePath: "/music/song1.mp3", Title: "Song One", Artist: "Artist A"},
		{FilePath: "/music/song2.mp3", Title: "Song Two", Artist: "Artist B"},
	}
	queue := domain.ReviewQueue{
		Tasks:        tasks,
		CurrentIndex: 0,
		History:      []domain.Task{},
	}
	cfg := domain.AppConfig{
		SeekDeltaSeconds: 10,
		GenreList:        []string{"Rock", "Jazz", "Electronic"},
	}
	p := provider.ManualReviewProvider{}

	return New(queue, mp, cfg, p)
}

// cmdType returns the reflect type name of the underlying function in a tea.Cmd.
// We cannot inspect tea.Cmd internals directly, so we check for non-nil as a
// proxy for "a command was issued". Use this to assert that a command was or
// was not returned.
func isNilCmd(cmd tea.Cmd) bool {
	return cmd == nil
}

// ── Seek tests ────────────────────────────────────────────────────────────────

// TestHandleKey_SeekForward verifies that pressing the right-arrow key calls
// Seek with a positive delta equal to the configured seekDelta.
func TestHandleKey_SeekForward(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRight})

	if len(mp.seekDeltas) != 1 {
		t.Fatalf("expected 1 Seek call, got %d", len(mp.seekDeltas))
	}
	want := 10 * time.Second
	if mp.seekDeltas[0] != want {
		t.Errorf("Seek delta = %v, want %v", mp.seekDeltas[0], want)
	}
}

// TestHandleKey_SeekBackward verifies that pressing the left-arrow key calls
// Seek with a negative delta equal to -seekDelta.
func TestHandleKey_SeekBackward(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})

	if len(mp.seekDeltas) != 1 {
		t.Fatalf("expected 1 Seek call, got %d", len(mp.seekDeltas))
	}
	want := -10 * time.Second
	if mp.seekDeltas[0] != want {
		t.Errorf("Seek delta = %v, want %v", mp.seekDeltas[0], want)
	}
}

// ── TogglePause test ──────────────────────────────────────────────────────────

// TestHandleKey_TogglePause verifies that pressing 'p' calls TogglePause once.
func TestHandleKey_TogglePause(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})

	if mp.toggleCount != 1 {
		t.Errorf("TogglePause call count = %d, want 1", mp.toggleCount)
	}
}

// ── Genre modal open test ─────────────────────────────────────────────────────

// TestHandleKey_EnterOpensGenreSelection verifies that pressing Enter while in
// StateReviewing transitions the model to StateGenreSelection.
func TestHandleKey_EnterOpensGenreSelection(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	if m.state != StateReviewing {
		t.Fatalf("precondition: expected StateReviewing, got %v", m.state)
	}

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	resultModel := result.(Model)

	if resultModel.state != StateGenreSelection {
		t.Errorf("after Enter: state = %v, want StateGenreSelection", resultModel.state)
	}
}

// ── Esc behaviour tests ───────────────────────────────────────────────────────

// TestHandleKey_EscInReviewingSkips verifies that pressing Esc while in
// StateReviewing advances CurrentIndex by 1 and issues a non-nil command (play).
func TestHandleKey_EscInReviewingSkips(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	if m.queue.CurrentIndex != 0 {
		t.Fatalf("precondition: CurrentIndex = %d, want 0", m.queue.CurrentIndex)
	}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1", resultModel.queue.CurrentIndex)
	}
	if isNilCmd(cmd) {
		t.Error("expected a non-nil command (playCmd) after skip, got nil")
	}
}

// TestHandleKey_EscInGenreSelectionCancels verifies that pressing Esc while in
// StateGenreSelection returns the model to StateReviewing without advancing the queue.
func TestHandleKey_EscInGenreSelectionCancels(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)
	m.state = StateGenreSelection // Force into genre selection state.

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	resultModel := result.(Model)

	if resultModel.state != StateReviewing {
		t.Errorf("after Esc in genre selection: state = %v, want StateReviewing", resultModel.state)
	}
	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 (no skip should have occurred)", resultModel.queue.CurrentIndex)
	}
	if !isNilCmd(cmd) {
		t.Error("expected nil command after Esc in genre modal (no side effects), got non-nil")
	}
}

// ── Undo tests ────────────────────────────────────────────────────────────────

// TestHandleKey_CtrlU_NoHistory verifies that pressing Ctrl+U with an empty
// History slice is a no-op: CurrentIndex does not change and no command is issued.
func TestHandleKey_CtrlU_NoHistory(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Confirm empty history.
	if len(m.queue.History) != 0 {
		t.Fatalf("precondition: expected empty history, got %d items", len(m.queue.History))
	}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 after no-op undo", resultModel.queue.CurrentIndex)
	}
	if !isNilCmd(cmd) {
		t.Error("expected nil command for no-op undo, got non-nil")
	}
}

// TestHandleKey_CtrlU_WithHistory verifies that pressing Ctrl+U with a non-empty
// History rewinds CurrentIndex by 1, pops the last history entry, and issues a
// non-nil command (playCmd for the previous song).
func TestHandleKey_CtrlU_WithHistory(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Simulate having moved to song 2: CurrentIndex = 1, History = [song1].
	m.queue.CurrentIndex = 1
	m.queue.History = []domain.Task{m.queue.Tasks[0]}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 after undo", resultModel.queue.CurrentIndex)
	}
	if len(resultModel.queue.History) != 0 {
		t.Errorf("History length = %d, want 0 after undo popped the last entry", len(resultModel.queue.History))
	}
	if isNilCmd(cmd) {
		t.Error("expected a non-nil command (playCmd) after undo, got nil")
	}
}

// ── skipToNext end-of-queue test ──────────────────────────────────────────────

// TestSkipToNext_EndOfQueue verifies that skipToNext is a no-op when
// CurrentIndex is already at the last task.
func TestSkipToNext_EndOfQueue(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Position at the last task (index 1, length 2).
	m.queue.CurrentIndex = 1

	resultModel, cmd := m.skipToNext()

	if resultModel.queue.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1 (no advance past end)", resultModel.queue.CurrentIndex)
	}
	if !isNilCmd(cmd) {
		t.Error("expected nil command when at end of queue, got non-nil")
	}
}

// ── TickMsg playback state caching test ──────────────────────────────────────

// TestTickMsg_CachesPlaybackState verifies that a TickMsg causes the model to
// call engine.GetState() and cache the result in m.playbackState.
func TestTickMsg_CachesPlaybackState(t *testing.T) {
	wantState := audio.PlaybackState{
		Progress: 0.42,
		Elapsed:  "01:00",
		Total:    "02:22",
		Position: "01:00 / 02:22",
	}
	mp := &mockPlayer{state: wantState}
	m := newTestModel(mp)

	// Ensure the initial cached state is the zero value (or whatever New() set).
	// Then fire a TickMsg and verify the cache was updated.
	result, _ := m.Update(TickMsg(time.Now()))
	resultModel := result.(Model)

	got := resultModel.playbackState
	if got.Progress != wantState.Progress {
		t.Errorf("playbackState.Progress = %v, want %v", got.Progress, wantState.Progress)
	}
	if got.Position != wantState.Position {
		t.Errorf("playbackState.Position = %q, want %q", got.Position, wantState.Position)
	}
}
```

**Expected outcome:**
File compiles as part of `package tui`. All 10 test functions are present. The file
references only types and functions already in scope within `package tui` plus standard
library and the project's own packages.

**Verification:**
```
cd mp3-reviewer && go vet ./internal/tui/...
```
Zero diagnostics.

---

### Step 5: Run the Full Test Suite

**Action:**
```
cd mp3-reviewer && go build ./... && go test ./internal/tui/... -v -count=1
```

**Expected outcome:**
All 10 test functions in `update_test.go` pass. Sample expected output:
```
=== RUN   TestHandleKey_SeekForward
--- PASS: TestHandleKey_SeekForward (0.00s)
=== RUN   TestHandleKey_SeekBackward
--- PASS: TestHandleKey_SeekBackward (0.00s)
=== RUN   TestHandleKey_TogglePause
--- PASS: TestHandleKey_TogglePause (0.00s)
=== RUN   TestHandleKey_EnterOpensGenreSelection
--- PASS: TestHandleKey_EnterOpensGenreSelection (0.00s)
=== RUN   TestHandleKey_EscInReviewingSkips
--- PASS: TestHandleKey_EscInReviewingSkips (0.00s)
=== RUN   TestHandleKey_EscInGenreSelectionCancels
--- PASS: TestHandleKey_EscInGenreSelectionCancels (0.00s)
=== RUN   TestHandleKey_CtrlU_NoHistory
--- PASS: TestHandleKey_CtrlU_NoHistory (0.00s)
=== RUN   TestHandleKey_CtrlU_WithHistory
--- PASS: TestHandleKey_CtrlU_WithHistory (0.00s)
=== RUN   TestSkipToNext_EndOfQueue
--- PASS: TestSkipToNext_EndOfQueue (0.00s)
=== RUN   TestTickMsg_CachesPlaybackState
--- PASS: TestTickMsg_CachesPlaybackState (0.00s)
PASS
ok  	song-reviewer/internal/tui	0.XXXs
```

**Verification:**
Terminal output shows no `FAIL` lines. Run the full suite to confirm no regressions:
```
cd mp3-reviewer && go test ./...
```

---

### Step 6: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**
Open `agent-development/agent-specs/architecture-breakdown.md`.

Find the `/internal/tui` bullet. It currently opens with:
> `/internal/tui`: Bubble Tea components split across three files. `model.go` defines `Model`, `AppState` enum ...

**6.1 — Update the model.go description within the `/internal/tui` bullet.**

Find this phrase inside the `/internal/tui` bullet:
> `model.go` defines `Model`, `AppState` enum (`StateReviewing`, `StateGenreSelection`, `StateSettings`), all message types

Change the opening of that phrase to:
> `model.go` defines the `AudioPlayer` interface (the subset of `*audio.Engine` methods used by the TUI, enabling mock substitution in tests), `Model`, `AppState` enum (`StateReviewing`, `StateGenreSelection`, `StateSettings`), all message types

**6.2 — Update the package dependency comment for `/internal/tui`.**

Find the `Adapter Pattern` item in the Design Patterns section or any sentence describing
`/internal/tui` as depending on `*audio.Engine`. Add a sentence at the end of the
`/internal/tui` bullet (just before the period that ends the sentence about `Ctrl-C` saving
state) clarifying the interface:

> `Model.engine` is typed as the `AudioPlayer` interface (declared in `model.go`) rather than `*audio.Engine` directly, which decouples the TUI from the concrete audio implementation and allows unit tests to run without an OS audio device.

**Expected outcome:**
`architecture-breakdown.md` accurately describes the `AudioPlayer` interface and explains
its purpose for testability.

**Verification:**
Open the file and confirm both updated passages are present and accurate.

---

### Step 7: Update `agent-development/agent-specs/FOLDER-STRUCTURE.md`

**Action:**
Open `agent-development/agent-specs/FOLDER-STRUCTURE.md`. Find the "Last updated" line:

> **Last updated:** Task 8 — ID3 tag read on queue load; filename fallback improved.

Change it to:

> **Last updated:** Task 9 — AudioPlayer interface introduced in tui; TUI unit tests added.

Also update the package dependency graph comment for `internal/tui`. Find:
```
internal/tui
  └── imports: internal/domain, internal/audio, internal/metadata, internal/provider
```
No change is needed to the import list (tui still imports `internal/audio` for the
`audio.PlaybackState` type). The graph is already correct.

Also update the `writer.go` entry under `internal/metadata/` to note the new test files:
```
├── tui/
│   ├── model.go                ← Model struct, AppState enum, tea.Msg types, tea.Cmd factories
│   ├── update.go               ← Init, Update, key handling, message dispatch
│   └── view.go                 ← View rendering, lipgloss styles
```
Change it to:
```
├── tui/
│   ├── model.go                ← Model struct, AudioPlayer interface, AppState enum, tea.Msg types, tea.Cmd factories
│   ├── update.go               ← Init, Update, key handling, message dispatch
│   ├── view.go                 ← View rendering, lipgloss styles
│   ├── mock_player_test.go     ← Test-only mockPlayer implementing AudioPlayer
│   └── update_test.go          ← Unit tests for handleKey, skipToNext, undoLast, TickMsg
```

**Expected outcome:**
Both the "Last updated" line and the tui directory listing are updated.

**Verification:**
Open the file and confirm the updated lines are present.

---

## Open Questions & Decisions

### Q1: Should the test file use `package tui` (white-box) or `package tui_test` (black-box)?

**Context:**
Go supports two test file conventions:
- `package tui` (same package, "white-box") — can access unexported identifiers:
  `StateReviewing`, `StateGenreSelection`, `enrichStatus`, `handleKey()`, `skipToNext()`,
  `mockPlayer`, enrichment constants, etc. Required for the tests as specified in the
  task request, since `handleKey`, `skipToNext`, and `undoLast` are unexported methods.
- `package tui_test` (external package, "black-box") — can only access exported
  identifiers. This would require exporting `handleKey`, `skipToNext`, and `undoLast`, or
  testing only through the public `Update()` interface.

The task request asks to test `handleKey`, `skipToNext`, and `undoLast` by name. These are
unexported methods. Only `package tui` grants access to them without making API changes.
Additionally, `StateReviewing`, `StateGenreSelection`, and the enrichment constants are
also unexported.

**Options:**
- **A)** `package tui` — white-box tests. Direct access to all unexported methods and
  constants. No changes to exported API. This is the approach described in the plan above.
  - Pro: No API surface changes. Tests exactly the internal methods the task requests.
  - Con: Tests are more coupled to internals; renaming a method requires updating tests.
- **B)** `package tui_test` — black-box tests. Export `HandleKey`, `SkipToNext`,
  `UndoLast`, `StateReviewing`, `StateGenreSelection`, plus all enrichment constants.
  - Pro: Tests against the public API; more resilient to internal refactors.
  - Con: Forces unnecessary API surface exports. Significantly more invasive: 6+ exports
    needed just to enable the tests. Contradicts the Go idiom for internal logic testing.

**Agent's recommendation:** **Option A** (`package tui`). The methods being tested are
intentionally unexported internal helpers. Exporting them solely for testing would pollute
the public API and contradict the task's stated goal of testing "pure-logic helpers that
are currently untested." White-box testing is the standard Go pattern for this use case.

**Human decision:** Let's do white-box testing and follow Option A.

---

### Q2: Should `newTestModel` call the real `New()` constructor, or build a `Model{}` struct literal directly?

**Context:**
`newTestModel` in the test helper must produce a valid `Model`. There are two approaches:

- **A)** Call the real `New()` constructor. This exercises the constructor code path,
  initialises all `bubbles` sub-components (`progress.Model`, `spinner.Model`,
  `textinput.Model`, `list.Model`) to valid states, and stores the `pendingInit` batch.
  The downside is that `New()` calls `playCmd(engine, task.FilePath)` which creates a
  `tea.Cmd` closure capturing the mock's `Play` method — that closure is never executed
  in unit tests (no event loop), so `mp.playCalled` will be empty at the start of each
  test. Tests that check `Play` calls must trigger them via `handleKey`/`skipToNext`.
- **B)** Build a `Model{}` struct literal by hand. More surgical control over initial
  state; avoids sub-component initialisation. The downside is that partially-initialised
  `bubbles` components (e.g. a zero `progress.Model` or zero `list.Model`) may panic or
  behave unexpectedly when `Update()` passes messages through them. Maintaining this
  struct literal is also brittle — any new field added to `Model` is silently ignored
  (zero-valued) in tests.

**Agent's recommendation:** **Option A** (call the real `New()` constructor). It produces
a fully-valid model and exercises the initialisation path. The only subtlety is that
`New()` will store a `playCmd` closure in `pendingInit`; since `Init()` is never called in
tests, the play command is never dispatched and `mp.playCalled` starts empty — which is the
correct pre-condition for tests that assert on seek, pause, skip, and undo.

**Human decision:** Let's use Option A as per your recommendation.

---

### Q3: How should the `Ctrl+U` key be represented in the `tea.KeyMsg` in tests?

**Context:**
Bubble Tea's `tea.KeyMsg` can represent Ctrl+key combinations in two ways:
- `tea.KeyMsg{Type: tea.KeyCtrlU}` — uses the `Type` field with a named constant.
- `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{21}}` — uses raw rune value (ASCII ETB for
  Ctrl+U = 21). This is how the terminal would deliver it in some configurations.

In `update.go`, the key is matched as `msg.String() == "ctrl+u"`. The `String()` method on
`tea.KeyMsg` returns `"ctrl+u"` when `Type == tea.KeyCtrlU`. The rune form may or may not
produce the same string depending on the bubbletea version.

Looking at the existing code in `update.go`, all special keys (`ctrl+c`, `ctrl+o`,
`ctrl+1`, `ctrl+2`, `ctrl+u`) are matched via `msg.String()`. The `tea.KeyCtrlU` constant
maps directly to `String()` returning `"ctrl+u"`.

**Options:**
- **A)** `tea.KeyMsg{Type: tea.KeyCtrlU}` — use the named constant. Clean and readable.
  Matches the internal `msg.String()` dispatch path in `handleKey`. This is what the plan
  above uses.
- **B)** `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+u")}` — pass a literal
  string as runes. This does **not** work: `"ctrl+u"` as runes would be matched as the
  literal characters c, t, r, l, etc., not as the Ctrl+U key combination.

**Agent's recommendation:** **Option A** (`tea.KeyMsg{Type: tea.KeyCtrlU}`). There is no
real ambiguity here; the named constant is the correct way to represent this key in tests.
Option B is incorrect. This question is included for completeness so the implementing agent
is not confused.

**Human decision:** As you say: there's no real ambiguity here. The named constant is correct. Let's do Option A.

---

### Q4: Should `TestHandleKey_EscInGenreSelectionCancels` assert that the returned `cmd` is nil, or only assert on model state?

**Context:**
When `Esc` is pressed in `StateGenreSelection`, `handleKey` in `update.go` does:
```go
case "esc":
    m.state = StateReviewing
    return m, nil
```
It explicitly returns `nil` for the command. The test plan above asserts both the state
transition AND that `cmd == nil`. However, asserting on `cmd == nil` is a white-box
assertion about the implementation: if a future refactor adds a side-effect command to
the cancel path (e.g. to re-focus something), this assertion would fail even if the
user-visible behaviour is correct.

**Options:**
- **A)** Assert both state transition (`StateReviewing`) and that `cmd == nil`. Tighter
  coverage; enforces the "cancel produces no side effects" contract explicitly.
  - Pro: Catches accidental addition of side effects on cancel.
  - Con: More fragile to intentional future changes.
- **B)** Assert only the state transition (`StateReviewing`) and `CurrentIndex == 0`.
  Do not assert on `cmd`.
  - Pro: More resilient to benign future changes.
  - Con: Less precise coverage of the cancel-is-a-no-op contract.

**Agent's recommendation:** **Option A** (assert both). The "Esc in genre selection is a
pure no-op with no side-effect commands" is a meaningful behavioural contract worth
asserting on. If a future change intentionally adds a command, the test failure is a useful
signal to review whether the no-side-effects guarantee still holds.

**Human decision:** Let's use Option A as per your recommendation.

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/tui/model.go` | Modified | Add `AudioPlayer` interface declaration; change `engine` field from `*audio.Engine` to `AudioPlayer`; change `New()` parameter from `*audio.Engine` to `AudioPlayer`; change `playCmd` parameter from `*audio.Engine` to `AudioPlayer` |
| 2 | `internal/tui/mock_player_test.go` | Created | Test-only `mockPlayer` struct implementing `AudioPlayer`; records all calls for assertion |
| 3 | `internal/tui/update_test.go` | Created | 10 unit test functions covering seek, pause, genre modal, Esc behaviour, undo, end-of-queue, and TickMsg state caching; `newTestModel` helper |
| 4 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Add `AudioPlayer` interface mention to `/internal/tui` bullet; add decoupling note |
| 5 | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Modified | Update "Last updated" line; update tui directory listing to include new test files and updated `model.go` annotation |

**Total files created: 2 | Total files modified: 3**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes with zero failures (all pre-existing tests still pass)
- [ ] `go test ./internal/tui/... -v -count=1` shows all 10 TUI tests passing with `--- PASS`
- [ ] `go vet ./internal/tui/...` exits 0 with no output
- [ ] `internal/tui/model.go`: `AudioPlayer` interface is declared with exactly 5 methods (`Play`, `Seek`, `TogglePause`, `GetState`, `Close`) and correct signatures
- [ ] `internal/tui/model.go`: `Model.engine` field type is `AudioPlayer` (not `*audio.Engine`)
- [ ] `internal/tui/model.go`: `New()` accepts `AudioPlayer` as second parameter
- [ ] `internal/tui/model.go`: `playCmd` accepts `AudioPlayer` as first parameter
- [ ] `cmd/reviewer/main.go` was **not modified** and compiles (proves implicit interface satisfaction)
- [ ] `internal/audio/engine.go` was **not modified**
- [ ] `internal/tui/mock_player_test.go` exists in `package tui` with `mockPlayer` struct and all 5 interface methods
- [ ] `internal/tui/update_test.go` exists in `package tui` with all 10 named test functions and `newTestModel` helper
- [ ] No unrelated files were modified or deleted
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated to mention `AudioPlayer` interface in the `/internal/tui` entry
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` "Last updated" line updated to Task 9 and tui directory listing updated

---

## Notes for the Implementing Agent

1. **Read all source files before making any edit.** Specifically read `internal/tui/model.go`
   in full (all 511 lines) to understand the exact current state of the `engine` field, the
   `New()` signature, and the `playCmd` function before touching them.

2. **The `audio` import stays in `model.go`.** After changing `engine` from `*audio.Engine`
   to `AudioPlayer`, the import `"song-reviewer/internal/audio"` is still needed because:
   - `AudioPlayer.GetState()` returns `audio.PlaybackState`.
   - `Model.playbackState` is typed `audio.PlaybackState`.
   - `New()` calls `engine.GetState()` to initialise `m.playbackState`.
   Do NOT remove the audio import.

3. **Implicit interface satisfaction check.** After Step 2, run `go build ./...` immediately.
   If `cmd/reviewer` fails to compile with a message like "cannot use engine (type
   *audio.Engine) as type tui.AudioPlayer", it means one of the method signatures in the
   interface does not exactly match the concrete method on `*Engine`. Double-check the
   signatures against `internal/audio/engine.go`:
   - `Play(path string) error`
   - `Seek(delta time.Duration) error`
   - `TogglePause()` — no parameters, no return value
   - `GetState() audio.PlaybackState`
   - `Close()` — no parameters, no return value

4. **`newTestModel` calls `New()`, which stores a `playCmd` in `pendingInit`.** This means
   `*mockPlayer.Play` will have been called 0 times at the start of each test — the
   `playCmd` closure is created but never executed because there is no Bubble Tea event
   loop in tests. Tests that want to verify a Play call must trigger one by calling
   `handleKey` or `skipToNext` and then check `mp.playCalled`.

5. **`tea.KeyMsg` field names.** The `tea.KeyMsg` struct has these relevant fields:
   - `Type tea.KeyType` — use named constants like `tea.KeyRight`, `tea.KeyLeft`,
     `tea.KeyEnter`, `tea.KeyEscape`, `tea.KeyCtrlU`, `tea.KeyCtrlC`.
   - `Runes []rune` — for printable key presses like `"p"`.
   For rune-based keys (e.g. `"p"`), construct: `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}`.
   For special keys, use only the `Type` field: `tea.KeyMsg{Type: tea.KeyEnter}`.

6. **`Update()` returns `(tea.Model, tea.Cmd)`.** Tests must type-assert the returned
   `tea.Model` back to `Model` before inspecting fields:
   ```go
   result, cmd := m.Update(someMsg)
   resultModel := result.(Model)
   ```
   If the type assertion panics, it means `Update` returned something other than `Model` —
   which should not happen with the current code, but is worth knowing for debugging.

7. **Do NOT export any currently unexported identifiers** (`handleKey`, `skipToNext`,
   `undoLast`, `StateReviewing`, etc.) just to make tests compile. Use `package tui` (same
   package) for the test files, which grants access to all package-level identifiers.

8. **Do NOT modify `internal/audio/engine.go`** for any reason. The interface is defined in
   the `tui` package, not in `audio`, so `audio` has no knowledge of it and needs no
   changes.

9. **After completing all steps**, move `agent-development/pending/9-tui-interface-and-tests.md`
   to `agent-development/done/requests/9-tui-interface-and-tests.md` and move this plan
   from `agent-development/plans/9-tui-interface-and-tests-plan.md` to
   `agent-development/done/plans/9-tui-interface-and-tests-plan.md`.

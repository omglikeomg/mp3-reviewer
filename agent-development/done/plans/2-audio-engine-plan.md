# Implementation Plan: Task 2 — Audio Engine Implementation

## Overview

This plan implements the audio engine for the Song Reviewer CLI. The engine lives in `internal/audio/engine.go` and wraps the `faiface/beep` and `faiface/beep/mp3` libraries behind a clean, concurrency-safe `Engine` struct. It exposes three core operations — `Play`, `Seek`, and `GetState` — plus a `Close` method for teardown. A single `sync.Mutex` guards all streamer access so the TUI goroutine and the speaker's background goroutine cannot race during playback transitions. The speaker device is initialized once at startup (via `InitSpeaker`) at a fixed sample rate; all MP3 streams are resampled to match. This plan has no dependency on the TUI or metadata packages — it is a standalone, self-contained audio layer that later tasks will call into.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/2-audio-engine.md` | The task being implemented |
| Domain Models | `internal/domain/models.go` | The `Task` struct (for context on how audio integrates) |
| Initialization Plan | `agent-development/done/plans/0-initialization-plan.md` | What was set up in Task 0 |
| Core Domain Plan | `agent-development/done/plans/1-core-domain-and-json-adapters-plan.md` | What was set up in Task 1 |

---

## Pre-Conditions

- Task 0 (project bootstrapping) must be complete. The `internal/audio/engine.go` stub file (containing only `package audio`) must already exist at that path.
- Task 1 (core domain models) must be complete. `internal/domain/models.go` must exist and compile.
- `go.mod` must declare the module as `song-reviewer` with Go 1.21+.
- The `faiface/beep` dependency must be added to `go.mod` / `go.sum` (this plan includes the `go get` step).
- The host machine must have a working audio output device. The `faiface/beep` library uses `hajimehoshi/oto` under the hood; on macOS no extra system packages are needed. On Linux, `libasound2-dev` may be required but that is outside the scope of this plan.

---

## Step-by-Step Implementation

### Step 1: Add `faiface/beep` to the Module

**Action:**

Run the following commands from the project root (`mp3-reviewer/`):

```/dev/null/shell.sh
cd mp3-reviewer
go get github.com/faiface/beep@v1.1.0
```

This will:
1. Download `github.com/faiface/beep v1.1.0` and its transitive dependencies.
2. Update `go.mod` to add the `require` directive.
3. Populate `go.sum` with all checksums.

**Expected outcome:** `go.mod` now contains a `require` block that includes `github.com/faiface/beep v1.1.0` (plus any indirect deps such as `hajimehoshi/oto`). `go.sum` is populated.

**Verification:**

```/dev/null/shell.sh
grep 'faiface/beep' mp3-reviewer/go.mod
# Expected output: github.com/faiface/beep v1.1.0
```

---

### Step 2: Implement `internal/audio/engine.go`

**Action:**

Replace the existing stub file `internal/audio/engine.go` (which contains only `package audio`) with the full implementation described below. Every field, constant, type, and function is specified exactly — the agent must not deviate from these signatures unless resolving a genuine compile error.

#### 2.1 — Package declaration and imports

```mp3-reviewer/internal/audio/engine.go#L1-20
package audio

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)
```

#### 2.2 — Package-level constants

```mp3-reviewer/internal/audio/engine.go#L14-20
const (
	// sampleRate is the fixed sample rate used to initialize the speaker device.
	// All MP3 streams are resampled to this rate via a beep.Resampler.
	sampleRate = beep.SampleRate(44100)

	// resampleQuality is passed to beep.Resample. A value of 3 gives good quality
	// with low CPU cost — appropriate for real-time on-the-fly resampling.
	resampleQuality = 3
)
```

#### 2.3 — The `PlaybackState` struct (return value for `GetState`)

```mp3-reviewer/internal/audio/engine.go#L22-32
// PlaybackState is a snapshot of the current audio playback position.
// It is safe to read from any goroutine after GetState returns.
type PlaybackState struct {
	// Progress is the playback position expressed as a fraction in [0.0, 1.0].
	// 0.0 means the beginning; 1.0 means the end. Returns 0 when nothing is playing.
	Progress float64

	// Elapsed is a human-readable string of the current position, e.g. "01:30".
	Elapsed string

	// Total is a human-readable string of the total duration, e.g. "04:00".
	Total string

	// Position is the elapsed/total string combined, e.g. "01:30 / 04:00".
	Position string
}
```

#### 2.4 — The `Engine` struct

```mp3-reviewer/internal/audio/engine.go#L35-60
// Engine manages audio playback for a single file at a time.
// All public methods are safe to call from multiple goroutines.
type Engine struct {
	mu sync.Mutex

	// streamer is the raw MP3 decoder for the currently loaded file.
	// It implements beep.StreamSeekCloser, which gives us Seek, Len, Position, and Close.
	// It is nil when no file is loaded.
	streamer beep.StreamSeekCloser

	// format holds the beep.Format (SampleRate, NumChannels, Precision) of the
	// currently loaded stream. Required to calculate seek positions.
	format beep.Format

	// ctrl wraps the resampled streamer and allows pause/resume without
	// stopping the speaker. ctrl.Paused = true streams silence instead of audio.
	ctrl *beep.Ctrl

	// fileHandle is the *os.File underlying the current stream.
	// We keep a reference so we can close it explicitly when switching tracks.
	fileHandle *os.File

	// speakerInitialized tracks whether speaker.Init has already been called.
	// The speaker must be initialized exactly once for the lifetime of the process.
	speakerInitialized bool
}
```

#### 2.5 — `NewEngine` constructor

```mp3-reviewer/internal/audio/engine.go#L63-70
// NewEngine creates a new Engine. The underlying speaker device is NOT initialized
// here; it is initialized lazily on the first call to Play. This avoids opening
// the audio device when the binary is run with --help or in headless test environments.
func NewEngine() *Engine {
	return &Engine{}
}
```

#### 2.6 — `initSpeaker` (private, called under the lock)

```mp3-reviewer/internal/audio/engine.go#L73-88
// initSpeaker initializes the speaker device if it has not been initialized yet.
// This must be called while the Engine's mu lock is held.
// The buffer is set to 100ms worth of samples, which provides a good balance
// between UI responsiveness and playback reliability.
func (e *Engine) initSpeaker() error {
	if e.speakerInitialized {
		return nil
	}
	bufferSize := sampleRate.N(time.Second / 10)
	if err := speaker.Init(sampleRate, bufferSize); err != nil {
		return fmt.Errorf("audio: initializing speaker: %w", err)
	}
	e.speakerInitialized = true
	return nil
}
```

#### 2.7 — `stopCurrent` (private, called under the lock)

```mp3-reviewer/internal/audio/engine.go#L91-115
// stopCurrent stops whatever is currently playing and releases all associated
// resources. It is safe to call when nothing is playing (all fields nil/zero).
// This must be called while the Engine's mu lock is held.
func (e *Engine) stopCurrent() {
	// Clear the speaker's queue so it stops pulling from ctrl immediately.
	// speaker.Clear() is safe to call even if nothing is playing.
	speaker.Clear()

	// Nil out ctrl so the speaker (if still running briefly) streams silence.
	if e.ctrl != nil {
		e.ctrl.Streamer = nil
		e.ctrl = nil
	}

	// Close the beep streamer (releases its internal buffers).
	if e.streamer != nil {
		_ = e.streamer.Close()
		e.streamer = nil
	}

	// Close the underlying file handle.
	if e.fileHandle != nil {
		_ = e.fileHandle.Close()
		e.fileHandle = nil
	}

	// Reset format to zero value.
	e.format = beep.Format{}
}
```

#### 2.8 — `Play(path string) error` (public)

```mp3-reviewer/internal/audio/engine.go#L118-168
// Play stops any currently playing audio, opens the MP3 file at path, decodes
// it, and starts streaming it through the speaker. The file remains open for the
// duration of playback; it is closed when Play is called again, Seek causes an
// error, or Close is called.
//
// Returns an error if the file cannot be opened or decoded.
func (e *Engine) Play(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize the speaker device on first use.
	if err := e.initSpeaker(); err != nil {
		return err
	}

	// Stop and clean up whatever was playing before.
	e.stopCurrent()

	// Open the file.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("audio: opening file %q: %w", path, err)
	}

	// Decode the MP3 stream.
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("audio: decoding MP3 %q: %w", path, err)
	}

	// Store state.
	e.fileHandle = f
	e.streamer = streamer
	e.format = format

	// Wrap the streamer in a Ctrl so we can pause/resume later.
	// Resample to the fixed speaker sample rate to handle MP3s with non-standard rates.
	resampled := beep.Resample(resampleQuality, format.SampleRate, sampleRate, streamer)
	e.ctrl = &beep.Ctrl{Streamer: resampled, Paused: false}

	// Hand off to the speaker. speaker.Play is non-blocking; it starts a background
	// goroutine that pulls samples from ctrl.
	speaker.Play(e.ctrl)

	return nil
}
```

#### 2.9 — `Seek(delta time.Duration) error` (public)

This function calculates the new absolute sample position. The seek-wrapping rules are:
- If `newPos > totalSamples`: seek to position 0 (start of track).
- If `newPos < 0`: seek to position `totalSamples - 1` (end of track, effectively paused at last sample).

```mp3-reviewer/internal/audio/engine.go#L171-225
// Seek moves the playback position by delta (which may be negative for rewind).
// If the resulting position would exceed the total duration, playback loops back
// to the beginning. If the resulting position would go before the start, playback
// jumps to the end of the track.
//
// Returns an error if no track is loaded or if the underlying seek fails.
func (e *Engine) Seek(delta time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamer == nil {
		return fmt.Errorf("audio: seek called with no track loaded")
	}

	totalSamples := e.streamer.Len()
	currentPos := e.streamer.Position()

	// Convert the time delta into a sample offset using the stream's native sample rate.
	deltaInSamples := e.format.SampleRate.N(delta)
	newPos := currentPos + deltaInSamples

	// Apply wrap-around rules.
	if newPos >= totalSamples {
		// Overshot the end — loop to the very beginning.
		newPos = 0
	} else if newPos < 0 {
		// Undershot the start — jump to the end of the track.
		// We use totalSamples-1 to land on the last valid sample position.
		newPos = totalSamples - 1
		if newPos < 0 {
			newPos = 0 // guard for degenerate zero-length stream
		}
	}

	// Lock the speaker before seeking to prevent the speaker goroutine from
	// reading the streamer's position mid-seek.
	speaker.Lock()
	err := e.streamer.Seek(newPos)
	speaker.Unlock()

	if err != nil {
		return fmt.Errorf("audio: seeking to sample %d: %w", newPos, err)
	}

	return nil
}
```

#### 2.10 — `TogglePause()` (public)

```mp3-reviewer/internal/audio/engine.go#L228-246
// TogglePause pauses playback if currently playing, or resumes if currently paused.
// It is a no-op if no track is loaded.
func (e *Engine) TogglePause() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctrl == nil {
		return
	}

	speaker.Lock()
	e.ctrl.Paused = !e.ctrl.Paused
	speaker.Unlock()
}
```

#### 2.11 — `GetState() PlaybackState` (public)

```mp3-reviewer/internal/audio/engine.go#L249-297
// GetState returns a snapshot of the current playback position.
// If no track is loaded, it returns a zero PlaybackState (Progress = 0, empty strings).
// This method is safe to call from any goroutine, including the TUI render loop.
func (e *Engine) GetState() PlaybackState {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamer == nil {
		return PlaybackState{
			Progress: 0,
			Elapsed:  "00:00",
			Total:    "00:00",
			Position: "00:00 / 00:00",
		}
	}

	// Lock the speaker while reading position to avoid a race with the
	// background streaming goroutine advancing e.streamer.Position().
	speaker.Lock()
	currentPos := e.streamer.Position()
	speaker.Unlock()

	totalSamples := e.streamer.Len()

	// Calculate progress fraction.
	var progress float64
	if totalSamples > 0 {
		progress = float64(currentPos) / float64(totalSamples)
		if progress > 1.0 {
			progress = 1.0
		}
	}

	// Convert sample counts to durations using the stream's native sample rate.
	elapsed := e.format.SampleRate.D(currentPos)
	total := e.format.SampleRate.D(totalSamples)

	elapsedStr := formatDuration(elapsed)
	totalStr := formatDuration(total)

	return PlaybackState{
		Progress: progress,
		Elapsed:  elapsedStr,
		Total:    totalStr,
		Position: elapsedStr + " / " + totalStr,
	}
}
```

#### 2.12 — `Close()` (public)

```mp3-reviewer/internal/audio/engine.go#L300-315
// Close stops all playback, releases the audio file handle, and closes the
// speaker device. It should be called exactly once when the application exits
// (e.g., in the Bubble Tea shutdown hook).
//
// After Close returns, the Engine must not be used again.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopCurrent()

	if e.speakerInitialized {
		speaker.Close()
		e.speakerInitialized = false
	}
}
```

#### 2.13 — `formatDuration` (private helper)

```mp3-reviewer/internal/audio/engine.go#L318-330
// formatDuration converts a time.Duration into a "MM:SS" string.
// Hours are not displayed; durations >= 60 minutes will show as e.g. "75:30".
// This keeps the display compact for typical song lengths.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
```

**Complete assembled file structure summary:**

The file `internal/audio/engine.go` must contain, in order:
1. `package audio` + imports block
2. Package-level constants (`sampleRate`, `resampleQuality`)
3. `PlaybackState` struct
4. `Engine` struct
5. `NewEngine() *Engine`
6. `(e *Engine) initSpeaker() error` (private)
7. `(e *Engine) stopCurrent()` (private)
8. `(e *Engine) Play(path string) error`
9. `(e *Engine) Seek(delta time.Duration) error`
10. `(e *Engine) TogglePause()`
11. `(e *Engine) GetState() PlaybackState`
12. `(e *Engine) Close()`
13. `formatDuration(d time.Duration) string` (private)

**Expected outcome:** `internal/audio/engine.go` compiles cleanly and exposes all public methods described above.

**Verification:**

```/dev/null/shell.sh
cd mp3-reviewer
go build ./internal/audio/...
# Expected: exits with code 0, no output
```

---

### Step 3: Write `internal/audio/engine_test.go`

**Action:**

Create a new file `internal/audio/engine_test.go` with unit tests for the pure logic paths. Note that tests requiring a real speaker device are skipped in CI by checking for the `DISPLAY`/audio device environment; however, for this project we test the logic that does NOT require an initialized speaker by testing `formatDuration` and the seek position math in isolation, plus a smoke test for `NewEngine()`.

The full test file content:

```mp3-reviewer/internal/audio/engine_test.go#L1-120
package audio

import (
	"testing"
	"time"
)

// TestFormatDuration verifies the MM:SS formatting helper for typical song lengths.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "00:00",
		},
		{
			name:     "30 seconds",
			input:    30 * time.Second,
			expected: "00:30",
		},
		{
			name:     "1 minute 30 seconds",
			input:    90 * time.Second,
			expected: "01:30",
		},
		{
			name:     "4 minutes exactly",
			input:    4 * time.Minute,
			expected: "04:00",
		},
		{
			name:     "75 minutes 30 seconds (no hour rollover)",
			input:    75*time.Minute + 30*time.Second,
			expected: "75:30",
		},
		{
			name:     "negative duration is clamped to zero",
			input:    -5 * time.Second,
			expected: "00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestNewEngine verifies the constructor returns a non-nil Engine with clean state.
func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine() returned nil")
	}
	if e.streamer != nil {
		t.Error("expected streamer to be nil on a new Engine")
	}
	if e.ctrl != nil {
		t.Error("expected ctrl to be nil on a new Engine")
	}
	if e.fileHandle != nil {
		t.Error("expected fileHandle to be nil on a new Engine")
	}
	if e.speakerInitialized {
		t.Error("expected speakerInitialized to be false on a new Engine")
	}
}

// TestGetState_NoTrack verifies GetState returns a sane zero-value state when
// no track is loaded (no speaker device needed).
func TestGetState_NoTrack(t *testing.T) {
	e := NewEngine()
	state := e.GetState()

	if state.Progress != 0 {
		t.Errorf("expected Progress=0, got %f", state.Progress)
	}
	if state.Elapsed != "00:00" {
		t.Errorf("expected Elapsed=%q, got %q", "00:00", state.Elapsed)
	}
	if state.Total != "00:00" {
		t.Errorf("expected Total=%q, got %q", "00:00", state.Total)
	}
	if state.Position != "00:00 / 00:00" {
		t.Errorf("expected Position=%q, got %q", "00:00 / 00:00", state.Position)
	}
}

// TestSeek_NoTrack verifies Seek returns an error gracefully when no track is loaded.
func TestSeek_NoTrack(t *testing.T) {
	e := NewEngine()
	err := e.Seek(30 * time.Second)
	if err == nil {
		t.Fatal("expected an error when seeking with no track loaded, got nil")
	}
}

// TestClose_Idempotent verifies that calling Close on a never-used Engine does not panic.
func TestClose_Idempotent(t *testing.T) {
	e := NewEngine()
	// Must not panic.
	e.Close()
	// Second close must also not panic.
	e.Close()
}
```

**Expected outcome:** All five test functions compile and pass without requiring an audio device.

**Verification:**

```/dev/null/shell.sh
cd mp3-reviewer
go test ./internal/audio/... -v -run "TestFormatDuration|TestNewEngine|TestGetState_NoTrack|TestSeek_NoTrack|TestClose_Idempotent"
# Expected: PASS for all five tests
```

---

### Step 4: Update `agent-specs/architecture-breakdown.md`

**Action:**

Open `agent-specs/architecture-breakdown.md` and make the following targeted modifications:

1. **In the "Folder Structure" section**, expand the `/internal/audio` bullet to describe what `engine.go` now provides:

   Replace:
   ```/dev/null/old.md#L1-2
   - `/internal/audio`: Wrapper for the `beep` library. Handles the audio device state.
   ```
   With:
   ```/dev/null/new.md#L1-3
   - `/internal/audio`: Wrapper for the `beep` library. Exposes `Engine` — a concurrency-safe struct that manages device initialization, MP3 decoding, playback, seeking, and pause/resume. Public surface: `NewEngine()`, `Play(path)`, `Seek(delta)`, `TogglePause()`, `GetState()`, `Close()`.
   ```

2. **In the "Technology Stack" section**, verify that `faiface/beep` is already listed (it is in the existing doc). No change needed there.

3. **Add a new "Audio Engine" subsection** under the "Design Patterns" section (after the Concurrency bullet):

   Append the following:
   ```/dev/null/addition.md#L1-7
   4. **Single Speaker Device:** `speaker.Init` is called exactly once for the process lifetime (lazy, on first `Play` call). All MP3 streams are resampled to a fixed 44100 Hz sample rate via `beep.Resample` so that files with non-standard sample rates play at the correct pitch and speed. The speaker lock (`speaker.Lock` / `speaker.Unlock`) is used exclusively for position reads and seeks — never held during file I/O.
   ```

**Expected outcome:** `agent-specs/architecture-breakdown.md` accurately reflects the audio engine design.

**Verification:**

```/dev/null/shell.sh
grep -n "NewEngine\|speaker.Init\|44100" mp3-reviewer/agent-specs/architecture-breakdown.md
# Expected: at least one matching line for each term
```

---

### Step 5: Update `README.md`

**Action:**

Open `README.md` and make the following targeted modifications:

1. **In the "Technology Stack" table**, confirm `faiface/beep` is listed (it already is from Task 0). No change needed there.

2. **In the "Architecture" section**, update the `internal/audio/` line to describe the completed engine:

   Replace:
   ```/dev/null/old.md#L1-2
   internal/audio/     — Wrapper for the beep library (playback, seeking, device).
   ```
   With:
   ```/dev/null/new.md#L1-2
   internal/audio/     — Audio engine (Engine struct). Handles device init, MP3 decoding, play, seek ±N seconds, pause/resume, and clean shutdown.
   ```

3. **In the "Keybindings" table**, add a row for pause/resume if not already present. The current table has seek (←/→) but no pause. Add:

   After the `←` / `→` row, insert:
   ```/dev/null/keybinding.md#L1-2
   | `p` | Pause / Resume playback |
   ```

   > **Note:** The keybinding key itself (`p`) is a recommendation from the audio layer's perspective. If the TUI task (Task 3) chooses a different key, this row should be updated then. For now, document the intended behavior.

**Expected outcome:** `README.md` accurately describes the audio engine's capabilities.

**Verification:**

```/dev/null/shell.sh
grep -n "Audio engine\|Pause\|engine.go" mp3-reviewer/README.md
# Expected: at least one line for each term
```

---

### Step 6: Final Build and Test Verification

**Action:**

Run the full project build and the audio package tests from the project root:

```/dev/null/shell.sh
cd mp3-reviewer
go build ./...
go test ./internal/audio/... -v
go vet ./internal/audio/...
```

**Expected outcome:**
- `go build ./...` exits 0 with no errors.
- `go test ./internal/audio/... -v` shows all tests passing (PASS).
- `go vet ./internal/audio/...` exits 0 with no output.

**Verification:**

All three commands exit with code 0.

---

## Open Questions & Decisions

### Q1: Seek wrap-around behavior — end-of-track direction

**Context:** The task spec says: *"If `newPos > duration`, loop to the start. If `newPos < 0`, loop to the end."* The word "loop" is used for both directions. For forward overshoot, jumping to position 0 (start) is unambiguous. For backward undershoot (seeking before position 0), the spec says loop to the "end" — but it is not clear whether "end" means the very last sample (effectively a paused-at-end state) or whether playback should continue from that end position backward (which isn't meaningful). This plan interprets "loop to end" as: set position to `totalSamples - 1` (last sample), so the track is at its final frame and will drain naturally within one buffer cycle.

**Options:**
- **A)** Jump to `totalSamples - 1` (last sample). Track immediately drains — effectively stops. Clean but might surprise users who expect the track to keep playing from the end.
- **B)** Jump to position 0 (start) for both overshot AND undershot. Symmetric "always restart" behavior. Simpler to reason about; may be more UX-friendly if the user accidentally hits rewind at the beginning.
- **C)** For backward undershoot, jump to position 0 (start) rather than the end, consistent with how most audio players handle the edge case.

**Agent's recommendation:** Option **C** — jumping to position 0 for *both* overshoot and undershoot is the most intuitive UX behavior (analogous to hitting seek-back on a media player when already near the start). It avoids the confusing "stuck at the end" state of Option A, and it is symmetric. However, the spec explicitly says *"loop to end"* for undershot, so this requires human confirmation.

**Human decision:** Let's do option C and not loop backwards. Let's simply stay at position 0.

---

### Q2: Speaker initialization — eager vs. lazy

**Context:** This plan initializes the speaker device lazily on the first `Play` call. The alternative is to initialize it eagerly in `NewEngine()`. Lazy init is cleaner for tests (no device needed to construct an `Engine`) and prevents opening the audio device if the user never plays anything. Eager init catches device errors at startup rather than mid-playback.

**Options:**
- **A)** **Lazy** (as planned): `speaker.Init` called inside `Play`, guarded by `speakerInitialized` flag.
- **B)** **Eager**: Add `InitSpeaker() error` as a separate public method called at app startup (e.g., from `cmd/reviewer/main.go`), keeping `NewEngine()` pure.
- **C)** **Eager via constructor**: `NewEngine()` returns `(*Engine, error)` and immediately calls `speaker.Init`.

**Agent's recommendation:** Option **A** (lazy, as implemented in this plan). It keeps `NewEngine()` infallible, simplifies test setup, and is consistent with the *agent instructions* guidance to avoid blocking the main loop unnecessarily. The downside (audio device errors surface mid-playback rather than at startup) is acceptable because the `Play` method already returns an error which the TUI can display.

**Human decision:** Option A is the way to go here. We don't need redundant checks at startup.

---

### Q3: Fixed sample rate vs. per-file sample rate for speaker initialization

**Context:** The `beep` speaker must be initialized at a single fixed sample rate for the lifetime of the process. MP3 files can have varying sample rates (44100 Hz is common, but 48000 Hz and 22050 Hz also exist). This plan initializes the speaker at 44100 Hz and resamples every stream to match. An alternative is to re-initialize the speaker for each new file at that file's native sample rate, which avoids resampling overhead but requires `speaker.Close()` + `speaker.Init()` on every track change.

**Options:**
- **A)** **Fixed 44100 Hz** (as planned): One `speaker.Init` per process. All streams resampled. Small CPU overhead per sample but seamless track transitions.
- **B)** **Per-file sample rate**: Reinitialize speaker on each `Play` call with the new file's `format.SampleRate`. No resampling overhead, but introduces a brief audio gap/click on every track switch due to device reset.
- **C)** **First-file sample rate**: Initialize the speaker using the sample rate of the *first* file played, and resample all subsequent files to match. No device reset after first play, but first-play sample rate is non-deterministic.

**Agent's recommendation:** Option **A** (fixed 44100 Hz). It is the approach demonstrated in the official `beep` documentation and examples, avoids audible glitches during track transitions, and 44100 Hz is the de-facto standard for MP3s. The resampling quality setting of 3 is negligible overhead for real-time playback.

**Human decision:** Let's do option A as per your recommendation here. We can use a standard and the fixed bitrate has enough quality for the app's purpose.

---

### Q4: Should `TogglePause` be part of this task or deferred to the TUI task?

**Context:** The task spec (Task 2) does not explicitly list `TogglePause` as a required function — it lists `Play`, `Seek`, and `GetState`. However, `beep.Ctrl` (which the task spec explicitly says to store) exists specifically to support pausing. Implementing `TogglePause` now is a two-line function and rounds out the engine API, preventing a code churn when the TUI needs it. However, it could be argued that implementing unspecified functions is scope creep.

**Options:**
- **A)** **Include `TogglePause`** (as planned): Completes the natural API surface of `beep.Ctrl`. The TUI task will need it and having it here avoids reopening this file.
- **B)** **Defer to TUI task**: Implement only what the spec lists. Cleaner scope separation. Risk: the implementing agent for the TUI task must modify `engine.go`.

**Agent's recommendation:** Option **A** — include `TogglePause`. It directly leverages the `ctrl` field that the spec requires, has zero risk of breakage, and prevents unnecessary future churn. The function is already specified in this plan with a complete implementation.

**Human decision:** Let's implement TogglePause like in option A, let's leave a TODO: comment in code to remind the agent of the future to implement TogglePause as part of the TUI and that it should have a shortcut (maybe 'p').

---

## File Manifest

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/audio/engine.go` | Modified | Full audio engine: `Engine` struct, `PlaybackState` struct, `NewEngine`, `Play`, `Seek`, `TogglePause`, `GetState`, `Close`, private helpers `initSpeaker`, `stopCurrent`, `formatDuration` |
| 2 | `internal/audio/engine_test.go` | Created | Unit tests for `formatDuration`, `NewEngine`, `GetState` (no-track), `Seek` (no-track error), and `Close` (idempotent) |
| 3 | `agent-specs/architecture-breakdown.md` | Modified | Expanded `/internal/audio` bullet; added "Single Speaker Device" design pattern note |
| 4 | `README.md` | Modified | Updated `internal/audio/` description in architecture section; added pause/resume keybinding row |
| 5 | `go.mod` | Modified | Added `require github.com/faiface/beep v1.1.0` (and indirect deps) |
| 6 | `go.sum` | Modified | Populated with checksums for `faiface/beep` and transitive dependencies |

**Total files created: 1 | Total files modified: 5**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go get github.com/faiface/beep@v1.1.0` succeeded and `go.mod` contains the dependency
- [ ] `go build ./...` succeeds with zero errors
- [ ] `go vet ./internal/audio/...` exits 0 with no output
- [ ] `go test ./internal/audio/... -v` passes all five tests without requiring an audio device
- [ ] `internal/audio/engine.go` contains all 13 components listed in Step 2's summary
- [ ] `internal/audio/engine_test.go` exists and contains all five test functions
- [ ] `Engine.Play` calls `stopCurrent()` before opening a new file (no resource leak on track switch)
- [ ] `Engine.Close` calls `stopCurrent()` and then `speaker.Close()` (full teardown)
- [ ] `Engine.Seek` uses `speaker.Lock()`/`speaker.Unlock()` around the actual `streamer.Seek()` call
- [ ] `Engine.GetState` uses `speaker.Lock()`/`speaker.Unlock()` around `streamer.Position()` read
- [ ] `formatDuration` correctly handles negative input (clamps to 0, not panic)
- [ ] `agent-specs/architecture-breakdown.md` updated to describe the engine's public API
- [ ] `README.md` updated with revised audio description and pause keybinding
- [ ] No files outside `internal/audio/`, `agent-specs/architecture-breakdown.md`, `README.md`, `go.mod`, `go.sum` were modified

---

## Notes for the Implementing Agent

1. **`speaker.Lock` vs `Engine.mu`**: These are two distinct locks with different purposes. `Engine.mu` prevents concurrent access to the `Engine`'s own fields from multiple goroutines (e.g., TUI calling `Seek` while a timer goroutine calls `GetState`). `speaker.Lock()` prevents the speaker's internal goroutine from pulling new samples while we mutate the streamer's position. Always acquire `Engine.mu` first (via method entry), then `speaker.Lock()` second — never the reverse order, to avoid deadlock.

2. **Do NOT call `speaker.Init` more than once**: The `speakerInitialized` flag guards this. Calling `speaker.Init` a second time while audio is playing causes the oto library to panic or produce garbled output on some platforms.

3. **The `beep.Resampler` is NOT stored on the struct**: The task spec mentions storing a `resampler` field, but `beep.Ctrl` wraps the resampled streamer — the resampler itself does not need to be accessed directly after construction. Storing it would add complexity without benefit. If a future task requires dynamic speed control, the resampler can be surfaced then.

4. **`mp3.Decode` returns a `beep.StreamSeekCloser`**: This interface combines `Streamer`, `Len()`, `Position()`, `Seek(int) error`, and `Close() error`. The `Engine.streamer` field is typed as `beep.StreamSeekCloser` to make all these methods available without type assertions.

5. **File handle lifetime**: The `*os.File` must stay open for the entire duration of playback because `mp3.Decode` streams lazily from the file — it does not buffer the entire MP3 in memory. Closing the file while the speaker is streaming from it will cause a `read: file already closed` error. `stopCurrent()` closes the file only after clearing the speaker and nilifying the streamer.

6. **`go test` without a speaker**: Tests must not call `Play` because that calls `speaker.Init` which requires a real audio device (and will fail in CI). The five tests in this plan are carefully scoped to avoid any `speaker.Init` call. If future tests need to exercise `Play`, use build tags (`//go:build integration`) to gate them.

7. **Error messages**: All errors must be wrapped with `fmt.Errorf("audio: <context>: %w", err)` so the TUI can display `"audio: ..."` prefixed messages consistently. Do not use bare `errors.New`.

8. **Seeking at the end of a track**: When a track finishes naturally (the speaker drains the streamer), `e.streamer` is NOT automatically nilled. `GetState` will report `Progress = 1.0` and `Elapsed == Total`. The next `Play` or `Seek` call will behave correctly because `stopCurrent()` / the position guards handle this state.

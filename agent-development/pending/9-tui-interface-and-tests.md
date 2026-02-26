# Task 9: TUI Interface Wrapping & Unit Tests

## Context

This task was deferred from Task 3 (TUI Foundations). The TUI `Model` currently holds a concrete `*audio.Engine` pointer, which makes unit testing impossible without a real audio device. This task introduces a thin `AudioPlayer` interface in the `internal/tui` package so the TUI logic can be tested with a mock, and adds unit tests for the pure-logic helpers that are currently untested.

## Background

In Task 3, the decision was made (Q5, Option A) to skip TUI unit tests to avoid scope creep. The following note was left in the code:

- `skipToNext()` and `undoLast()` in `update.go` contain meaningful queue-manipulation logic that should be covered by tests.
- `handleKey()` state transitions (StateReviewing → StateGenreSelection → StateReviewing) should be verified.
- The `TickMsg` handler's `playbackState` caching should be verified.

All of these require the `engine` field to be mockable.

## Implementation Details

1. **Define `AudioPlayer` interface (`internal/tui/model.go`):**
   - Extract the subset of `*audio.Engine` methods used by the TUI into an interface:
     ```
     type AudioPlayer interface {
         Play(path string) error
         Seek(delta time.Duration) error
         TogglePause()
         GetState() audio.PlaybackState
         Close()
     }
     ```
   - Change the `engine` field on `Model` from `*audio.Engine` to `AudioPlayer`.
   - Update `New(queue domain.ReviewQueue, engine AudioPlayer, cfg domain.AppConfig) Model` signature accordingly.
   - `*audio.Engine` already satisfies this interface — no changes needed in `internal/audio/`.

2. **Update `cmd/reviewer/main.go`:**
   - `audio.NewEngine()` returns `*audio.Engine`, which satisfies `AudioPlayer`. The call site in `main.go` passes it to `tui.New()` unchanged — no explicit cast needed.

3. **Create `internal/tui/mock_player_test.go`:**
   - Define a `mockPlayer` struct (unexported, test-only) that implements `AudioPlayer`.
   - Fields: `playErr error`, `seekErr error`, `state audio.PlaybackState`, `playCalled []string`, `seekDelta []time.Duration`, `toggleCount int`, `closed bool`.
   - All methods record their calls and return the configured error/state values.

4. **Create `internal/tui/update_test.go`:**
   - Write the following test functions:
     - `TestHandleKey_SeekForward` — verifies right-arrow calls `Seek(+seekDelta)`.
     - `TestHandleKey_SeekBackward` — verifies left-arrow calls `Seek(-seekDelta)`.
     - `TestHandleKey_TogglePause` — verifies `p` calls `TogglePause()`.
     - `TestHandleKey_EnterOpensGenreSelection` — verifies Enter/Space transitions to `StateGenreSelection`.
     - `TestHandleKey_EscInReviewingSkips` — verifies Esc in `StateReviewing` advances `CurrentIndex` and issues a `playCmd`.
     - `TestHandleKey_EscInGenreSelectionCancels` — verifies Esc in `StateGenreSelection` returns to `StateReviewing` without skipping.
     - `TestHandleKey_CtrlU_NoHistory` — verifies `Ctrl+U` with empty history is a no-op.
     - `TestHandleKey_CtrlU_WithHistory` — verifies `Ctrl+U` with non-empty history rewinds `CurrentIndex` and issues a `playCmd`.
     - `TestSkipToNext_EndOfQueue` — verifies `skipToNext()` is a no-op at the last song.
     - `TestTickMsg_CachesPlaybackState` — verifies that a `TickMsg` updates `m.playbackState` from the mock engine.

5. **Update `agent-specs/architecture-breakdown.md`:**
   - Add note that `internal/tui` depends on the `AudioPlayer` interface (not the concrete type) for testability.

6. **Update `README.md`:**
   - No user-facing changes required.

## Agent Checklist

- [ ] Add `AudioPlayer` interface to `internal/tui/model.go`.
- [ ] Change `Model.engine` field type from `*audio.Engine` to `AudioPlayer`.
- [ ] Update `New()` signature to accept `AudioPlayer` instead of `*audio.Engine`.
- [ ] Confirm `cmd/reviewer/main.go` compiles without changes (implicit interface satisfaction).
- [ ] Create `internal/tui/mock_player_test.go` with `mockPlayer`.
- [ ] Create `internal/tui/update_test.go` with all 10 test functions listed above.
- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go test ./internal/tui/... -v` shows all 10 tests passing.
- [ ] `go vet ./internal/tui/...` exits 0 with no output.
- [ ] Update `agent-specs/architecture-breakdown.md` to mention the `AudioPlayer` interface.
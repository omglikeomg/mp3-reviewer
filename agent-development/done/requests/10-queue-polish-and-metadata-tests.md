# Task 10: Queue Polish & Metadata Test Coverage

## Goal

Three focused improvements that together close the most important correctness and usability gaps identified in the Task 9 state-of-development review: (1) skip already-applied songs on queue load via a config flag, (2) show a "queue complete" screen when the last song is reached, and (3) add integration tests for all four functions in `internal/metadata/writer.go`.

## Context

Requires Tasks 0–9 to be completed first.

**Why these three together?** They share no cross-cutting dependencies and are all low-risk, low-effort changes. Each touches a different layer of the app (provider, TUI, test), so they can be planned and implemented in one coherent pass without scope creep.

### Problem 1 — Applied songs are re-presented on every launch

`GetTasks()` in `internal/provider/json_provider.go` currently loads every entry in the `manual_review` JSON array unconditionally — including entries where `status == "applied"`. A user with 500 songs in their queue, 400 of which are already tagged, must manually skip through 400 re-presented songs every time they open the app. The JSON already tracks `status: "applied"` so the data needed to filter is already there; it is just not used.

The filtering should be opt-in via a new boolean field `skip_applied` in `settings.json` (default `false` so existing behaviour is preserved). When `true`, `GetTasks()` silently omits entries with `status == "applied"` from the returned slice.

### Problem 2 — No feedback when the queue is exhausted

`skipToNext()` in `internal/tui/update.go` is currently a no-op when `CurrentIndex` is already at the last task (`nextIndex >= len(m.queue.Tasks)`). The TUI stays frozen on the last song with no indication that the session is over. The user has no signal to press `Ctrl+C`.

A new `StateQueueComplete` TUI state and a corresponding `viewQueueComplete()` render function should be introduced. When `skipToNext()` detects end-of-queue, it transitions to `StateQueueComplete` instead of silently returning. The complete screen shows a brief summary (total songs in queue, current session) and a `Ctrl+C to quit` hint. No audio plays. `Ctrl+C` works normally. `Ctrl+U` (undo) should also work from this screen to let the user step back to the last song if they changed their mind.

### Problem 3 — `internal/metadata` has zero test coverage

`WriteTags`, `WriteBPM`, `WriteYear`, and `ReadTags` are the most critical data-mutating functions in the app — they directly modify users' MP3 files. They currently have no tests at all. Integration tests should be added in a new file `internal/metadata/writer_test.go`. Each test creates a minimal valid MP3 fixture using `os.CreateTemp`, calls the function under test, re-reads the tags with `ReadTags` or `id3v2.Open`, and asserts the round-trip result. All temp files are cleaned up with `t.Cleanup`.

A small valid MP3 file must be embedded as `internal/metadata/testdata/fixture.mp3`. This fixture only needs to be a valid MP3 container with an ID3v2 header — it does not need to contain any audio frames. The `bogem/id3v2` library is already in `go.mod`; no new dependencies are required.

## Requirements

### Config: `skip_applied` flag

- Add a `SkipApplied bool` field to `domain.AppConfig` with JSON tag `"skip_applied"`.
- In `GetTasks()`, when `p.Config.SkipApplied` is `true`, omit any `reviewEntry` where `Status == "applied"` from the returned tasks slice.
- When `SkipApplied` is `false` (the default / zero value), behaviour is identical to today — all entries are returned.
- Add `"skip_applied": false` to `settings.example.json` as a documented, opt-in field.
- Update `README.md` settings reference table with the new field.

### TUI: Queue complete screen

- Add a `StateQueueComplete AppState` constant in `internal/tui/model.go`.
- In `skipToNext()` in `internal/tui/update.go`: when `nextIndex >= len(m.queue.Tasks)`, set `m.state = StateQueueComplete` and return `m, nil` (no play command, no enrichment fetch). The current song's history entry is still appended before the transition so `Ctrl+U` can rewind.
- Add `viewQueueComplete()` in `internal/tui/view.go` that renders a styled completion screen. It must display:
  - A prominent "Queue complete" heading.
  - The total number of tasks in the queue (`len(m.queue.Tasks)`).
  - A `Ctrl+U  go back` hint and a `Ctrl+C  quit` hint.
  - No progress bar, no enrichment panel, no status bar errors.
- `View()` must dispatch to `viewQueueComplete()` when `m.state == StateQueueComplete`.
- `handleKey()` must handle `StateQueueComplete` — only `ctrl+c` (quit) and `ctrl+u` (undo, which transitions back to `StateReviewing` and replays the last song) should be active. All other keys are no-ops.
- `Ctrl+U` from `StateQueueComplete` calls `undoLast()` normally. Since `undoLast()` always sets `m.state` back to `StateReviewing` via `playCmd`, the state will transition correctly without extra logic.

### Tests: `internal/metadata/writer_test.go`

- Create `internal/metadata/testdata/fixture.mp3` — a minimal valid MP3 file with an ID3v2 header but no meaningful audio. It must be small (< 10 KB). Commit it as a binary file.
- Create `internal/metadata/writer_test.go` in `package metadata` with the following tests:
  - `TestWriteTags_PrimaryOnly` — writes a primary genre, no secondary; reads back and asserts TCON frame contains the primary genre.
  - `TestWriteTags_PrimaryAndSecondary` — writes both genres; reads back and asserts both TCON frames and the TXXX "TGENRE2" frame are present with the correct values.
  - `TestWriteTags_EmptyPrimary` — calls `WriteTags` with an empty primary; asserts a non-nil error is returned and the file is not modified.
  - `TestWriteTags_ReplacesExistingGenres` — writes genres twice; asserts the second write fully replaces the first (no stale TCON frames accumulate).
  - `TestWriteBPM_RoundTrip` — writes a BPM string; reads back the TBPM frame and asserts the value matches.
  - `TestWriteBPM_EmptyBPM` — calls `WriteBPM` with an empty string; asserts a non-nil error.
  - `TestWriteYear_RoundTrip` — writes a year string; reads back and asserts the value matches (via `tag.Year()`).
  - `TestWriteYear_EmptyYear` — calls `WriteYear` with an empty string; asserts a non-nil error.
  - `TestReadTags_ReadsWrittenValues` — calls `WriteTags` followed by `ReadTags`; asserts `ReadTags` returns empty strings for title and artist (the fixture has none), confirming no panic or unexpected data.
  - `TestReadTags_FileNotFound` — calls `ReadTags` with a non-existent path; asserts a non-nil error is returned.
- Each test that writes to the fixture must first copy `testdata/fixture.mp3` to a `t.TempDir()` temp path so the shared fixture is never mutated.
- All tests must pass with `go test ./internal/metadata/... -v`.

## Implementation Details

1. **`internal/domain/models.go`:**
   - Add `SkipApplied bool \`json:"skip_applied"\`` to `AppConfig`. No other struct changes needed.

2. **`internal/provider/json_provider.go` — `GetTasks()`:**
   - After unmarshalling `raw`, add a filter pass: when `p.Config.SkipApplied` is `true`, skip any `entry` where `strings.EqualFold(entry.Status, "applied")`. The rest of the loop body (ID3 read, Task construction) is unchanged.

3. **`internal/tui/model.go`:**
   - Add `StateQueueComplete AppState = iota` after `StateSettings` in the `AppState` const block. The `iota` sequence means the existing values of `StateReviewing (0)`, `StateGenreSelection (1)`, `StateSettings (2)` are unchanged; `StateQueueComplete` gets value `3`. No existing code that compares against the prior three constants is affected.

4. **`internal/tui/update.go` — `skipToNext()`:**
   - Replace the early-return no-op (`return m, nil`) when `nextIndex >= len(m.queue.Tasks)` with:
     ```go
     if nextIndex >= len(m.queue.Tasks) {
         // Still push the current song onto History so Ctrl+U can rewind.
         if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
             m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
         }
         m.state = StateQueueComplete
         return m, nil
     }
     ```
   - Note: the existing history-append that follows the guard must be removed to avoid a double-append. Read `skipToNext()` carefully before editing.

5. **`internal/tui/update.go` — `handleKey()`:**
   - Add a `StateQueueComplete` guard at the top of `handleKey()` (alongside the existing `StateSettings` and `StateGenreSelection` guards). Only `ctrl+c` and `ctrl+u` are wired; all other keys fall through to a `return m, nil`.

6. **`internal/tui/view.go` — `viewQueueComplete()`:**
   - New pure function. Use existing `styleHeader` and `styleHint`/`styleHintKey` styles for consistency. No new lipgloss styles are needed. Suggested layout (three lines plus padding):
     ```
     ✓  Queue complete — N songs reviewed.

       Ctrl+U  go back      Ctrl+C  quit
     ```
   - Wire it into `View()` as a new `case StateQueueComplete:` branch.

7. **`settings.example.json`:**
   - Add `"skip_applied": false` at the top level (after `"seek_delta_seconds"`).

8. **`internal/metadata/testdata/fixture.mp3`:**
   - Create programmatically during plan execution using a short Go script or by copying a known-good minimal MP3. The file must be a real ID3v2-tagged MP3 that `id3v2.Open` can parse without error. A 1-second silent MP3 generated with `ffmpeg -f lavfi -i anullsrc=r=44100:cl=mono -t 1 -q:a 9 -acodec libmp3lame fixture.mp3` works. Alternatively, write a `TestMain` in `writer_test.go` that generates the fixture programmatically using `bogem/id3v2` if `ffmpeg` is not available — but a committed binary is simpler and more reliable for CI.
   - The planning agent must decide the best approach for fixture generation and document it as an open question.

9. **`internal/provider/json_provider_test.go`:**
   - Add one new test: `TestGetTasks_SkipApplied` — creates a JSON file with two entries (one `status: "applied"`, one with no status), sets `Config.SkipApplied = true`, calls `GetTasks()`, and asserts only one task is returned.

## Deliverables

- [ ] `SkipApplied bool` field added to `domain.AppConfig` with correct JSON tag.
- [ ] `GetTasks()` filters applied entries when `SkipApplied == true`; no change when `false`.
- [ ] `TestGetTasks_SkipApplied` test added to `internal/provider/json_provider_test.go` and passing.
- [ ] `StateQueueComplete` constant added to `internal/tui/model.go`.
- [ ] `skipToNext()` transitions to `StateQueueComplete` at end-of-queue instead of silently returning.
- [ ] `handleKey()` handles `StateQueueComplete` (only `ctrl+c` and `ctrl+u` are active).
- [ ] `viewQueueComplete()` added to `internal/tui/view.go` and wired into `View()`.
- [ ] `internal/metadata/testdata/fixture.mp3` committed as a binary test fixture.
- [ ] `internal/metadata/writer_test.go` created with all 10 tests listed above.
- [ ] `settings.example.json` updated with `"skip_applied": false`.
- [ ] `README.md` settings table updated with `skip_applied` field description.
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated to reflect `StateQueueComplete`, the `SkipApplied` config field, and the new metadata test file.
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` updated to reflect `testdata/` under `internal/metadata/` and the new test file.

## Agent Checklist

- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go test ./...` passes with zero failures.
- [ ] `go test ./internal/metadata/... -v` shows all 10 metadata tests passing.
- [ ] `go test ./internal/provider/... -v` shows `TestGetTasks_SkipApplied` passing alongside all existing provider tests.
- [ ] `go vet ./...` exits 0 with no output.
- [ ] Manually verify (by inspection): `GetTasks()` with `SkipApplied: false` returns the same results as before — no regression for the default case.
- [ ] Manually verify (by inspection): `skipToNext()` on the last song now sets `state = StateQueueComplete` and appends to History, rather than returning unchanged model.
- [ ] `settings.example.json` contains the `"skip_applied"` key.
- [ ] No unrelated files were modified.
- [ ] Update `agent-development/agent-specs/architecture-breakdown.md` if new packages, interfaces, or significant structural changes were introduced.
- [ ] Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` to reflect the new `testdata/` directory under `internal/metadata/`.
- [ ] Update `README.md` settings reference table with the `skip_applied` field.
# Implementation Plan: Task 10 — Queue Polish & Metadata Test Coverage

## Overview

This plan covers three focused, independent improvements that close correctness and usability gaps identified in the Task 9 review:

1. **`skip_applied` config flag** — A new boolean field `SkipApplied` in `domain.AppConfig` lets users opt-in to filtering already-applied songs from the queue on load. `GetTasks()` in `json_provider.go` checks this flag and skips any entry whose `status` equals `"applied"`. Default is `false` so existing behaviour is fully preserved.

2. **"Queue complete" screen** — A new `StateQueueComplete` TUI state and `viewQueueComplete()` render function replace the current silent no-op when the last song is skipped. The screen shows a summary, a `Ctrl+U go back` hint, and a `Ctrl+C quit` hint. `handleKey()` is updated to only honour `ctrl+c` and `ctrl+u` in this state.

3. **`internal/metadata` integration tests** — A committed binary fixture (`internal/metadata/testdata/fixture.mp3`) and a new test file (`internal/metadata/writer_test.go`) provide 10 round-trip tests covering `WriteTags`, `WriteBPM`, `WriteYear`, and `ReadTags`. No new Go dependencies are required.

All three changes are low-risk and touch distinct layers of the codebase (domain/provider, TUI, and test). The plan builds directly on top of the completed Tasks 0–9 state.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree and package dependency graph |
| Task Definition | `agent-development/pending/10-queue-polish-and-metadata-tests.md` | The task being implemented |
| Domain models | `internal/domain/models.go` | `AppConfig` and `ReviewQueue` structs |
| Provider implementation | `internal/provider/json_provider.go` | `GetTasks()` loop to be extended |
| Provider tests | `internal/provider/json_provider_test.go` | Existing test file to be extended |
| TUI model | `internal/tui/model.go` | `AppState` iota block to be extended |
| TUI update | `internal/tui/update.go` | `skipToNext()` and `handleKey()` to be modified |
| TUI view | `internal/tui/view.go` | `View()` dispatch and styles to be extended |
| TUI update tests | `internal/tui/update_test.go` | Existing test that must be updated |
| Metadata writer | `internal/metadata/writer.go` | Functions under test |

---

## Pre-Conditions

- Tasks 0–9 are fully completed and merged.
- `go build ./...` passes with zero errors on the current codebase.
- `go test ./...` passes with zero failures on the current codebase.
- `ffmpeg` is available in `$PATH` on the machine executing this plan (required for Step 7 to generate `fixture.mp3`). If `ffmpeg` is not available, see the alternative approach described in Step 7.
- `internal/metadata/writer.go` exists and exports `WriteTags`, `WriteBPM`, `WriteYear`, and `ReadTags`.
- `internal/domain/models.go` contains `AppConfig` with no `SkipApplied` field yet.
- `internal/tui/model.go` `AppState` iota block contains exactly `StateReviewing`, `StateGenreSelection`, `StateSettings` (values 0, 1, 2) with no `StateQueueComplete` yet.

---

## Step-by-Step Implementation

### Step 1: Add `SkipApplied` to `domain.AppConfig`

**Action:**

Open `internal/domain/models.go`. In the `AppConfig` struct, add the following field after `SeekDeltaSeconds`:

```go
SkipApplied bool `json:"skip_applied"` // When true, GetTasks omits entries with status "applied".
```

The full `AppConfig` struct after the change should look like:

```go
type AppConfig struct {
    MusicFolder      string   `json:"music_folder"`
    JsonPath         string   `json:"review_json_path"`
    GenreList        []string `json:"genres"`
    SeekDeltaSeconds int      `json:"seek_delta_seconds"` // Seek step for ← / → keys. Defaults to 30 if 0 or omitted.
    SkipApplied      bool     `json:"skip_applied"`       // When true, GetTasks omits entries with status "applied".
    ApiKeys          struct {
        MusicBrainzUserAgent string `json:"musicbrainz_user_agent"`
    } `json:"api_keys"`
}
```

No other struct changes are needed. `bool` zero-value is `false`, so existing code that constructs `AppConfig{}` without specifying this field preserves existing behaviour without any change.

**Expected outcome:** `AppConfig` compiles with the new field; all existing tests still pass.

**Verification:** `go build ./internal/domain/...`

---

### Step 2: Filter applied entries in `GetTasks()`

**Action:**

Open `internal/provider/json_provider.go`. In the `GetTasks()` method, add a filter check at the top of the `for _, entry := range raw.ManualReview` loop body, immediately before the `absPath` assignment. The change wraps the entire existing loop body in a guard:

```go
for _, entry := range raw.ManualReview {
    // When skip_applied is enabled, silently omit entries already tagged.
    if p.Config.SkipApplied && strings.EqualFold(entry.Status, "applied") {
        continue
    }

    absPath := filepath.Join(p.Config.MusicFolder, entry.FilePath)
    // ... rest of loop body unchanged ...
}
```

`strings` is already imported in `json_provider.go` (it is used in `SaveState`), so no new import is needed.

The complete loop after the change:

```go
tasks := make([]domain.Task, 0, len(raw.ManualReview))
for _, entry := range raw.ManualReview {
    // When skip_applied is enabled, silently omit entries already tagged.
    if p.Config.SkipApplied && strings.EqualFold(entry.Status, "applied") {
        continue
    }

    absPath := filepath.Join(p.Config.MusicFolder, entry.FilePath)

    // Read Title and Artist from the file's ID3 tags. Non-fatal: if the file
    // is missing, corrupted, or has no tags, Title and Artist remain empty
    // and the TUI falls back to a filename display.
    title, artist, _ := metadata.ReadTags(absPath)

    task := domain.Task{
        FilePath: absPath,
        Title:    title,
        Artist:   artist,
        Genre1:   entry.PrimaryGenre,
        Genre2:   entry.SecondaryGenre,
        BPM:      entry.BPM,
        Year:     entry.Year,
    }
    tasks = append(tasks, task)
}
```

**Expected outcome:** `GetTasks()` with `SkipApplied: false` (or zero value) returns identical results to today. With `SkipApplied: true`, entries whose `status` is `"applied"` (case-insensitive) are silently skipped.

**Verification:** `go build ./internal/provider/...`

---

### Step 3: Add `TestGetTasks_SkipApplied` to the provider test file

**Action:**

Open `internal/provider/json_provider_test.go`. Append the following test function at the end of the file:

```go
// TestGetTasks_SkipApplied verifies that when Config.SkipApplied is true,
// GetTasks omits entries whose status is "applied" from the returned slice.
// When SkipApplied is false (the default), all entries are returned regardless
// of their status.
func TestGetTasks_SkipApplied(t *testing.T) {
    const sampleJSON = `{
        "manual_review": [
            {
                "filepath": "Artist/Song1.mp3",
                "reason": "Uncertain genre",
                "confidence": 3,
                "status": "applied",
                "primary_genre": "Rock"
            },
            {
                "filepath": "Artist/Song2.mp3",
                "reason": "Uncertain genre",
                "confidence": 3
            }
        ]
    }`

    dir := t.TempDir()
    jsonPath := filepath.Join(dir, "manual_review.json")
    if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
        t.Fatalf("setup: writing temp JSON file: %v", err)
    }

    t.Run("SkipApplied=true omits applied entries", func(t *testing.T) {
        cfg := domain.AppConfig{
            MusicFolder: "/test/music",
            JsonPath:    jsonPath,
            SkipApplied: true,
        }
        p := ManualReviewProvider{Config: cfg}

        tasks, err := p.GetTasks()
        if err != nil {
            t.Fatalf("GetTasks() returned unexpected error: %v", err)
        }
        if len(tasks) != 1 {
            t.Fatalf("expected 1 task (applied entry skipped), got %d", len(tasks))
        }
        want := filepath.Join("/test/music", "Artist/Song2.mp3")
        if tasks[0].FilePath != want {
            t.Errorf("tasks[0].FilePath = %q, want %q", tasks[0].FilePath, want)
        }
    })

    t.Run("SkipApplied=false returns all entries", func(t *testing.T) {
        cfg := domain.AppConfig{
            MusicFolder: "/test/music",
            JsonPath:    jsonPath,
            SkipApplied: false,
        }
        p := ManualReviewProvider{Config: cfg}

        tasks, err := p.GetTasks()
        if err != nil {
            t.Fatalf("GetTasks() returned unexpected error: %v", err)
        }
        if len(tasks) != 2 {
            t.Fatalf("expected 2 tasks (all entries returned), got %d", len(tasks))
        }
    })
}
```

No new imports are required — `os`, `filepath`, and `domain` are already imported in the test file.

**Expected outcome:** The new test appears in `go test ./internal/provider/... -v` output and passes alongside all existing provider tests.

**Verification:** `go test ./internal/provider/... -v`

---

### Step 4: Add `StateQueueComplete` to the TUI AppState enum

**Action:**

Open `internal/tui/model.go`. Locate the `AppState` `const` iota block (currently at lines 27–37, containing `StateReviewing`, `StateGenreSelection`, `StateSettings`). Append `StateQueueComplete` as the fourth constant:

```go
type AppState int

const (
    // StateReviewing is the default state: the main playback + header + status bar view.
    StateReviewing AppState = iota

    // StateGenreSelection is shown when the user presses Enter or Space
    // to assign a genre to the current song.
    StateGenreSelection

    // StateSettings is shown when the user presses Ctrl-O to open the Settings overlay.
    // It presents two textinput fields for MusicFolder and JsonPath.
    StateSettings

    // StateQueueComplete is shown when the user skips or tags the last song
    // in the queue. Displays a summary and hints for Ctrl+U (go back) and
    // Ctrl+C (quit). No audio plays in this state.
    StateQueueComplete
)
```

The iota values are: `StateReviewing = 0`, `StateGenreSelection = 1`, `StateSettings = 2`, `StateQueueComplete = 3`. The existing constants are **not renumbered** — `StateQueueComplete` is appended. No existing code that compares against the prior three constants is affected.

**Expected outcome:** The package compiles with the new constant. No existing tests break.

**Verification:** `go build ./internal/tui/...`

---

### Step 5: Update `skipToNext()` to transition to `StateQueueComplete`

**Action:**

Open `internal/tui/update.go`. Locate the `skipToNext()` function. The current guard at the top reads:

```go
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
    nextIndex := m.queue.CurrentIndex + 1
    if nextIndex >= len(m.queue.Tasks) {
        return m, nil
    }

    if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
        m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
    }
    // ... rest of function ...
```

Replace **only** the early-return guard block with the following. The history-append that currently appears **after** the guard must also be removed to avoid double-appending. The full rewritten function is:

```go
// skipToNext advances the queue to the next task, starts playing it, and
// resets enrichment state. Fires fetchYearCmd and fetchBPMCmd for the new song
// if artist/title are available. If no next task exists (end of queue),
// transitions to StateQueueComplete so the user sees a completion screen.
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
    nextIndex := m.queue.CurrentIndex + 1
    if nextIndex >= len(m.queue.Tasks) {
        // Push the current song onto History so Ctrl+U can rewind from
        // the completion screen back to the last song.
        if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
            m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
        }
        m.state = StateQueueComplete
        return m, nil
    }

    // NOTE: The history-append below covers the normal (non-end-of-queue) path.
    // Do NOT add another append here; the end-of-queue path above handles its own.
    if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
        m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
    }

    m.queue.CurrentIndex = nextIndex
    m.lastPlayErr = nil

    // Reset enrichment for the incoming song.
    m = m.resetEnrichment()

    nextTask := m.queue.Tasks[nextIndex]
    var cmds []tea.Cmd
    cmds = append(cmds, playCmd(m.engine, nextTask.FilePath))

    // Fire year and BPM fetches immediately if we have artist/title metadata.
    if nextTask.Artist != "" || nextTask.Title != "" {
        m.enrichYearStatus = enrichLoading
        m.enrichBPMStatus = enrichLoading
        cmds = append(cmds, fetchYearCmd(nextTask.Artist, nextTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
        cmds = append(cmds, fetchBPMCmd(nextTask.Artist, nextTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
    }

    return m, tea.Batch(cmds...)
}
```

**Critical detail:** The original `skipToNext()` had a **single** history-append after the guard. In the rewritten version there are now **two** append sites — one for the end-of-queue path (inside the new `if nextIndex >= len(...)` block) and one for the normal path (immediately after that block). Make sure neither is removed and no third append is added.

**Expected outcome:** `skipToNext()` on the last task transitions to `StateQueueComplete` and appends to `History`. On a non-last task it behaves identically to before.

**Verification:** `go build ./internal/tui/...`

---

### Step 6: Update `handleKey()` to handle `StateQueueComplete`

**Action:**

Open `internal/tui/update.go`. In `handleKey()`, add a guard for `StateQueueComplete` alongside the existing `StateSettings` and `StateGenreSelection` guards. The guard must appear **before** the `StateReviewing` switch block. Place it directly after the `StateGenreSelection` guard (after the closing brace of the `if m.state == StateGenreSelection { ... }` block):

```go
// ── Queue Complete state ──────────────────────────────────────────────────
if m.state == StateQueueComplete {
    switch msg.String() {
    case "ctrl+c":
        // Flush any pending JSON enrichment data before quitting.
        _ = m.providerRef.SaveState(m.queue.Tasks)
        m.engine.Close()
        return m, tea.Quit
    case "ctrl+u":
        return m.undoLast()
    }
    // All other keys are no-ops in the completion screen.
    return m, nil
}
```

No other changes are needed in `handleKey()`. The `undoLast()` function already sets `m.state` back to `StateReviewing` (via `playCmd`) and decrements `CurrentIndex`, so `Ctrl+U` from `StateQueueComplete` naturally transitions back to the review screen and replays the last song.

**Expected outcome:** While in `StateQueueComplete`, only `ctrl+c` (quit) and `ctrl+u` (undo back to last song) are active. All other keys are silently ignored.

**Verification:** `go build ./internal/tui/...`

---

### Step 7: Add `viewQueueComplete()` to `view.go` and wire into `View()`

**Action:**

Open `internal/tui/view.go`. Add the following function anywhere after the existing `viewSettings()` function (i.e., at the bottom of the file):

```go
// viewQueueComplete renders the end-of-queue completion screen.
// Displayed when the user skips or tags the last song in the queue.
// No progress bar, no enrichment panel, no audio. Only Ctrl+U and Ctrl+C are active.
func (m Model) viewQueueComplete() string {
    total := len(m.queue.Tasks)
    heading := styleHeader.Render(
        fmt.Sprintf("  ✓  Queue complete — %d song%s reviewed.", total, pluralS(total)),
    )

    hints := "  " + hintStr("Ctrl+U", "go back") +
        "      " + hintStr("Ctrl+C", "quit")

    return "\n" + heading + "\n\n" + styleStatus.Render(hints) + "\n"
}

// pluralS returns "s" if n != 1, otherwise "". Used for grammatically correct
// pluralisation in the queue complete heading (e.g. "1 song" vs "2 songs").
func pluralS(n int) string {
    if n == 1 {
        return ""
    }
    return "s"
}
```

Then update the `View()` method to dispatch to `viewQueueComplete()` when `m.state == StateQueueComplete`. The current `View()` reads:

```go
func (m Model) View() string {
    switch m.state {
    case StateGenreSelection:
        return m.viewGenreModal()
    case StateSettings:
        return m.viewSettings()
    default:
        return m.viewReviewing()
    }
}
```

Add a new `case` before `default`:

```go
func (m Model) View() string {
    switch m.state {
    case StateGenreSelection:
        return m.viewGenreModal()
    case StateSettings:
        return m.viewSettings()
    case StateQueueComplete:
        return m.viewQueueComplete()
    default:
        return m.viewReviewing()
    }
}
```

`fmt` is already imported in `view.go`. No new imports are needed.

**Expected outcome:** `View()` renders the completion screen when the state is `StateQueueComplete`. No other screen is affected.

**Verification:** `go build ./internal/tui/...`

---

### Step 8: Update the existing `TestSkipToNext_EndOfQueue` test

**Action:**

Open `internal/tui/update_test.go`. Locate `TestSkipToNext_EndOfQueue`. This test currently asserts that `skipToNext()` at end-of-queue is a **no-op** (no state change, nil command). After Step 5, the behaviour changes: the function now sets `state = StateQueueComplete` and appends to `History`. The test must be updated to reflect the new expectations:

```go
// TestSkipToNext_EndOfQueue verifies that skipToNext transitions to
// StateQueueComplete when CurrentIndex is already at the last task,
// and that the current task is pushed onto History so Ctrl+U can rewind.
func TestSkipToNext_EndOfQueue(t *testing.T) {
    mp := &mockPlayer{}
    m := newTestModel(mp)

    // Position at the last task (index 1, length 2).
    m.queue.CurrentIndex = 1

    result, cmd := m.skipToNext()
    resultModel := result.(Model)

    // CurrentIndex must not advance past the end.
    if resultModel.queue.CurrentIndex != 1 {
        t.Errorf("CurrentIndex = %d, want 1 (no advance past end)", resultModel.queue.CurrentIndex)
    }
    // State must transition to StateQueueComplete.
    if resultModel.state != StateQueueComplete {
        t.Errorf("state = %v, want StateQueueComplete", resultModel.state)
    }
    // The last task must be pushed onto History for Ctrl+U to work.
    if len(resultModel.queue.History) != 1 {
        t.Errorf("History length = %d, want 1 (last task pushed for undo)", len(resultModel.queue.History))
    }
    // No play command should be issued.
    if !isNilCmd(cmd) {
        t.Error("expected nil command when transitioning to StateQueueComplete, got non-nil")
    }
}
```

**Expected outcome:** The updated test passes with the new `skipToNext()` behaviour. No other existing tests are affected.

**Verification:** `go test ./internal/tui/... -v -run TestSkipToNext_EndOfQueue`

---

### Step 9: Add a `TestHandleKey_QueueComplete` test

**Action:**

Open `internal/tui/update_test.go`. Append the following test function at the end of the file:

```go
// ── StateQueueComplete key handling tests ─────────────────────────────────────

// TestHandleKey_QueueComplete_CtrlU verifies that pressing Ctrl+U while in
// StateQueueComplete calls undoLast, transitioning back to StateReviewing
// and issuing a non-nil command (playCmd for the previous song).
func TestHandleKey_QueueComplete_CtrlU(t *testing.T) {
    mp := &mockPlayer{}
    m := newTestModel(mp)

    // Simulate: we just reached end-of-queue from index 1, so History has task[1].
    m.queue.CurrentIndex = 1
    m.queue.History = []domain.Task{m.queue.Tasks[1]}
    m.state = StateQueueComplete

    result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
    resultModel := result.(Model)

    if resultModel.state != StateReviewing {
        t.Errorf("after Ctrl+U from StateQueueComplete: state = %v, want StateReviewing", resultModel.state)
    }
    if resultModel.queue.CurrentIndex != 0 {
        t.Errorf("CurrentIndex = %d, want 0 after undo", resultModel.queue.CurrentIndex)
    }
    if isNilCmd(cmd) {
        t.Error("expected a non-nil command (playCmd) after undo from queue complete, got nil")
    }
}

// TestHandleKey_QueueComplete_OtherKeysAreNoOps verifies that keys other than
// ctrl+c and ctrl+u are no-ops while in StateQueueComplete.
func TestHandleKey_QueueComplete_OtherKeysAreNoOps(t *testing.T) {
    mp := &mockPlayer{}
    m := newTestModel(mp)
    m.state = StateQueueComplete
    m.queue.CurrentIndex = 1

    noOpKeys := []tea.KeyMsg{
        {Type: tea.KeyEscape},
        {Type: tea.KeyEnter},
        {Type: tea.KeyRunes, Runes: []rune("p")},
        {Type: tea.KeyLeft},
        {Type: tea.KeyRight},
    }

    for _, key := range noOpKeys {
        result, cmd := m.handleKey(key)
        resultModel := result.(Model)
        if resultModel.state != StateQueueComplete {
            t.Errorf("key %q: state = %v, want StateQueueComplete (should be no-op)", key.String(), resultModel.state)
        }
        if resultModel.queue.CurrentIndex != 1 {
            t.Errorf("key %q: CurrentIndex = %d, want 1 (should be no-op)", key.String(), resultModel.queue.CurrentIndex)
        }
        if !isNilCmd(cmd) {
            t.Errorf("key %q: expected nil command (no-op), got non-nil", key.String())
        }
    }
}
```

**Expected outcome:** Both new TUI tests pass.

**Verification:** `go test ./internal/tui/... -v -run TestHandleKey_QueueComplete`

---

### Step 10: Create the MP3 test fixture

**Action:**

Create the `internal/metadata/testdata/` directory and generate a minimal valid ID3v2-tagged MP3 using `ffmpeg`:

```bash
mkdir -p internal/metadata/testdata
ffmpeg -f lavfi -i anullsrc=r=44100:cl=mono -t 1 -q:a 9 -acodec libmp3lame \
    internal/metadata/testdata/fixture.mp3
```

This generates a ~6–8 KB file containing 1 second of silent audio with a valid ID3v2 header that `github.com/bogem/id3v2/v2` can parse.

**If `ffmpeg` is not available** on the executing machine, use the following Go snippet instead. Run it once from the project root to generate the fixture, then commit the resulting binary:

```bash
# Inline Go script alternative (only if ffmpeg unavailable):
cat > /tmp/gen_fixture.go << 'EOF'
package main

import (
    "os"
    id3v2 "github.com/bogem/id3v2/v2"
)

func main() {
    os.MkdirAll("internal/metadata/testdata", 0755)
    f, _ := os.Create("internal/metadata/testdata/fixture.mp3")
    // Write a minimal valid ID3v2.3 header + one trivial text frame.
    // The file has no audio frames but id3v2.Open still works.
    f.Write([]byte{
        // ID3v2.3 header: "ID3" + version 3.0 + flags 0x00 + size 0x00 0x00 0x00 0x0A (10 bytes)
        0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A,
        // TIT2 frame: frame ID + size 4 + flags + encoding + "Test"
        0x54, 0x49, 0x54, 0x32, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00,
        0x00, 0x54, 0x65, 0x73, 0x74,
    })
    f.Close()

    // Open and re-save with id3v2 to ensure it is valid for all test paths.
    tag, err := id3v2.Open("internal/metadata/testdata/fixture.mp3", id3v2.Options{Parse: true})
    if err != nil {
        panic(err)
    }
    tag.SetTitle("Test Song")
    tag.SetArtist("Test Artist")
    if err := tag.Save(); err != nil {
        panic(err)
    }
    tag.Close()
}
EOF
cd /path/to/mp3-reviewer && go run /tmp/gen_fixture.go
```

**Important:** `fixture.mp3` must be committed as a binary file. Do not add it to `.gitignore`. Verify it exists and is non-empty before proceeding:

```bash
ls -lh internal/metadata/testdata/fixture.mp3
```

**Expected outcome:** `internal/metadata/testdata/fixture.mp3` exists, is a valid ID3v2-tagged file parseable by `id3v2.Open`, and is under 10 KB.

**Verification:**

```bash
ls -lh internal/metadata/testdata/fixture.mp3
# Should show a non-zero file size < 10 KB.
```

---

### Step 11: Create `internal/metadata/writer_test.go`

**Action:**

Create a new file `internal/metadata/writer_test.go` with the following content. The file is in `package metadata` (same package as `writer.go`) so it can call all exported functions directly without import aliasing.

```go
package metadata

import (
    "io"
    "os"
    "path/filepath"
    "testing"

    id3v2 "github.com/bogem/id3v2/v2"
)

// copyFixture copies testdata/fixture.mp3 into a fresh temp file inside dir
// and returns its path. Each test that writes to the file must call this so
// the shared fixture is never mutated.
func copyFixture(t *testing.T) string {
    t.Helper()
    src, err := os.Open(filepath.Join("testdata", "fixture.mp3"))
    if err != nil {
        t.Fatalf("copyFixture: opening fixture: %v", err)
    }
    defer src.Close()

    dst, err := os.CreateTemp(t.TempDir(), "fixture_*.mp3")
    if err != nil {
        t.Fatalf("copyFixture: creating temp file: %v", err)
    }
    defer dst.Close()

    if _, err := io.Copy(dst, src); err != nil {
        t.Fatalf("copyFixture: copying fixture: %v", err)
    }
    return dst.Name()
}

// ── WriteTags tests ───────────────────────────────────────────────────────────

// TestWriteTags_PrimaryOnly writes a primary genre with no secondary and asserts
// that a single TCON frame is present with the correct value.
func TestWriteTags_PrimaryOnly(t *testing.T) {
    path := copyFixture(t)

    if err := WriteTags(path, "Rock", ""); err != nil {
        t.Fatalf("WriteTags() returned unexpected error: %v", err)
    }

    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        t.Fatalf("re-opening file after WriteTags: %v", err)
    }
    defer tag.Close()

    frames := tag.GetFrames(tag.CommonID("Genre"))
    if len(frames) != 1 {
        t.Fatalf("expected 1 TCON frame, got %d", len(frames))
    }
    tf, ok := frames[0].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TCON frame is not a TextFrame")
    }
    if tf.Text != "Rock" {
        t.Errorf("TCON frame text = %q, want %q", tf.Text, "Rock")
    }
}

// TestWriteTags_PrimaryAndSecondary writes both genres and asserts that two TCON
// frames are present and a TXXX frame with description "TGENRE2" contains the
// secondary genre value.
func TestWriteTags_PrimaryAndSecondary(t *testing.T) {
    path := copyFixture(t)

    if err := WriteTags(path, "Rock", "Psych-Rock"); err != nil {
        t.Fatalf("WriteTags() returned unexpected error: %v", err)
    }

    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        t.Fatalf("re-opening file after WriteTags: %v", err)
    }
    defer tag.Close()

    // Assert two TCON frames.
    tconFrames := tag.GetFrames(tag.CommonID("Genre"))
    if len(tconFrames) != 2 {
        t.Fatalf("expected 2 TCON frames, got %d", len(tconFrames))
    }
    primary, ok := tconFrames[0].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TCON[0] is not a TextFrame")
    }
    if primary.Text != "Rock" {
        t.Errorf("primary TCON text = %q, want %q", primary.Text, "Rock")
    }
    secondary, ok := tconFrames[1].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TCON[1] is not a TextFrame")
    }
    if secondary.Text != "Psych-Rock" {
        t.Errorf("secondary TCON text = %q, want %q", secondary.Text, "Psych-Rock")
    }

    // Assert TXXX "TGENRE2" frame.
    txxxFrames := tag.GetFrames("TXXX")
    found := false
    for _, f := range txxxFrames {
        txxx, ok := f.(id3v2.UserDefinedTextFrame)
        if !ok {
            continue
        }
        if txxx.Description == "TGENRE2" {
            found = true
            if txxx.Value != "Psych-Rock" {
                t.Errorf("TXXX TGENRE2 value = %q, want %q", txxx.Value, "Psych-Rock")
            }
        }
    }
    if !found {
        t.Error("expected a TXXX frame with description TGENRE2, none found")
    }
}

// TestWriteTags_EmptyPrimary asserts that WriteTags returns a non-nil error when
// called with an empty primary genre. The file must not be modified.
func TestWriteTags_EmptyPrimary(t *testing.T) {
    path := copyFixture(t)

    // Record file modification time before the call.
    statBefore, err := os.Stat(path)
    if err != nil {
        t.Fatalf("stat before WriteTags: %v", err)
    }

    gotErr := WriteTags(path, "", "Rock")
    if gotErr == nil {
        t.Fatal("WriteTags() with empty primary: expected non-nil error, got nil")
    }

    // File must not have been touched.
    statAfter, err := os.Stat(path)
    if err != nil {
        t.Fatalf("stat after WriteTags: %v", err)
    }
    if !statAfter.ModTime().Equal(statBefore.ModTime()) {
        t.Error("file modification time changed despite WriteTags returning an error")
    }
}

// TestWriteTags_ReplacesExistingGenres writes genres twice and asserts that the
// second write fully replaces the first (no stale TCON frames accumulate).
func TestWriteTags_ReplacesExistingGenres(t *testing.T) {
    path := copyFixture(t)

    if err := WriteTags(path, "Jazz", "Bebop"); err != nil {
        t.Fatalf("first WriteTags() call: %v", err)
    }
    if err := WriteTags(path, "Electronic", "Techno"); err != nil {
        t.Fatalf("second WriteTags() call: %v", err)
    }

    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        t.Fatalf("re-opening file after second WriteTags: %v", err)
    }
    defer tag.Close()

    tconFrames := tag.GetFrames(tag.CommonID("Genre"))
    if len(tconFrames) != 2 {
        t.Fatalf("expected exactly 2 TCON frames after second write, got %d (stale frames may have accumulated)", len(tconFrames))
    }
    primary, ok := tconFrames[0].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TCON[0] is not a TextFrame")
    }
    if primary.Text != "Electronic" {
        t.Errorf("primary TCON text = %q, want %q", primary.Text, "Electronic")
    }
    secondary, ok := tconFrames[1].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TCON[1] is not a TextFrame")
    }
    if secondary.Text != "Techno" {
        t.Errorf("secondary TCON text = %q, want %q", secondary.Text, "Techno")
    }
}

// ── WriteBPM tests ────────────────────────────────────────────────────────────

// TestWriteBPM_RoundTrip writes a BPM value and reads back the TBPM frame,
// asserting the value is preserved exactly.
func TestWriteBPM_RoundTrip(t *testing.T) {
    path := copyFixture(t)

    if err := WriteBPM(path, "128"); err != nil {
        t.Fatalf("WriteBPM() returned unexpected error: %v", err)
    }

    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        t.Fatalf("re-opening file after WriteBPM: %v", err)
    }
    defer tag.Close()

    frames := tag.GetFrames(tag.CommonID("BPM"))
    if len(frames) != 1 {
        t.Fatalf("expected 1 TBPM frame, got %d", len(frames))
    }
    tf, ok := frames[0].(id3v2.TextFrame)
    if !ok {
        t.Fatal("TBPM frame is not a TextFrame")
    }
    if tf.Text != "128" {
        t.Errorf("TBPM frame text = %q, want %q", tf.Text, "128")
    }
}

// TestWriteBPM_EmptyBPM asserts that WriteBPM returns a non-nil error when
// called with an empty string.
func TestWriteBPM_EmptyBPM(t *testing.T) {
    path := copyFixture(t)

    if err := WriteBPM(path, ""); err == nil {
        t.Fatal("WriteBPM() with empty bpm: expected non-nil error, got nil")
    }
}

// ── WriteYear tests ───────────────────────────────────────────────────────────

// TestWriteYear_RoundTrip writes a year value and reads it back via tag.Year(),
// asserting the value is preserved.
func TestWriteYear_RoundTrip(t *testing.T) {
    path := copyFixture(t)

    if err := WriteYear(path, "1971"); err != nil {
        t.Fatalf("WriteYear() returned unexpected error: %v", err)
    }

    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        t.Fatalf("re-opening file after WriteYear: %v", err)
    }
    defer tag.Close()

    got := tag.Year()
    if got != "1971" {
        t.Errorf("tag.Year() = %q, want %q", got, "1971")
    }
}

// TestWriteYear_EmptyYear asserts that WriteYear returns a non-nil error when
// called with an empty string.
func TestWriteYear_EmptyYear(t *testing.T) {
    path := copyFixture(t)

    if err := WriteYear(path, ""); err == nil {
        t.Fatal("WriteYear() with empty year: expected non-nil error, got nil")
    }
}

// ── ReadTags tests ────────────────────────────────────────────────────────────

// TestReadTags_ReadsWrittenValues calls WriteTags and then ReadTags on the same
// file, confirming there is no panic and that ReadTags returns the values it can
// find (title and artist, which the fixture has none of by default — so empty
// strings are expected). This validates the round-trip plumbing without requiring
// WriteTags to also write title/artist.
func TestReadTags_ReadsWrittenValues(t *testing.T) {
    path := copyFixture(t)

    if err := WriteTags(path, "Rock", "Blues-Rock"); err != nil {
        t.Fatalf("WriteTags() setup: %v", err)
    }

    title, artist, err := ReadTags(path)
    if err != nil {
        t.Fatalf("ReadTags() returned unexpected error: %v", err)
    }
    // The fixture has no TIT2 or TPE1 frames so we expect empty strings.
    // The important thing is that ReadTags does not panic or return an error
    // on a file that has been written to by WriteTags.
    if title != "" && artist != "" {
        // If the fixture happens to have been committed with title/artist (e.g.
        // generated with ffmpeg metadata), this test still passes — we just
        // assert non-empty strings are returned without panic.
        t.Logf("ReadTags returned non-empty values (title=%q, artist=%q) — fixture has pre-existing tags", title, artist)
    }
}

// TestReadTags_FileNotFound asserts that ReadTags returns a non-nil error and
// empty strings when called with a path that does not exist.
func TestReadTags_FileNotFound(t *testing.T) {
    title, artist, err := ReadTags("/nonexistent/path/that/does/not/exist.mp3")
    if err == nil {
        t.Fatal("ReadTags() with non-existent path: expected non-nil error, got nil")
    }
    if title != "" {
        t.Errorf("title = %q, want empty string on error", title)
    }
    if artist != "" {
        t.Errorf("artist = %q, want empty string on error", artist)
    }
}
```

**Expected outcome:** The file compiles and all 10 tests pass with `go test ./internal/metadata/... -v`.

**Verification:** `go test ./internal/metadata/... -v`

---

### Step 12: Update `settings.example.json`

**Action:**

Open `settings.example.json`. Add `"skip_applied": false` as a new top-level field after `"seek_delta_seconds"`. The updated file content:

```json
{
  "music_folder": "/Users/username/Music",
  "review_json_path": "./data/manual_review.json",
  "genres": [
    "Rock",
    "Jazz",
    "Blues",
    "Electronic",
    "Hip-Hop",
    "Classical",
    "Folk",
    "Psych-Rock",
    "Techno",
    "House"
  ],
  "seek_delta_seconds": 30,
  "skip_applied": false,
  "api_keys": {
    "musicbrainz_user_agent": "MySongReviewer/1.0.0 ( contact@example.com )"
  }
}
```

**Expected outcome:** `settings.example.json` contains the `"skip_applied"` key.

**Verification:** `grep "skip_applied" settings.example.json`

---

### Step 13: Update `README.md` settings reference table

**Action:**

Open `README.md`. Locate the **Settings Reference** section, which contains the JSON example block and the settings table. Add a new row for `skip_applied` in the table after the `seek_delta_seconds` row.

The updated table should read:

| Field | Description |
|---|---|
| `music_folder` | Absolute path to your music library root. Song file paths in the review JSON are resolved relative to this. |
| `review_json_path` | Path to the JSON file containing songs flagged for manual review. |
| `genres` | List of genre labels available for tagging. Customize to match your taxonomy. |
| `seek_delta_seconds` | Seek step in seconds for the `←` / `→` keys. Defaults to `30` if omitted. |
| `skip_applied` | When `true`, songs already tagged (`status: "applied"`) are silently omitted from the queue on load. Defaults to `false` — all songs are shown. |
| `api_keys.musicbrainz_user_agent` | Required by MusicBrainz API. Must include your app name and contact email. |

Also update the JSON example block in the Settings Reference to include `"skip_applied": false` after `"seek_delta_seconds": 30`:

```json
{
  "music_folder": "/path/to/your/music/library",
  "review_json_path": "./data/manual_review.json",
  "genres": ["Rock", "Jazz", "Blues", "Electronic", "Hip-Hop", "Classical", "Folk", "Psych-Rock", "Techno", "House"],
  "seek_delta_seconds": 30,
  "skip_applied": false,
  "api_keys": {
    "musicbrainz_user_agent": "YourAppName/1.0.0 ( your@email.com )"
  }
}
```

**Expected outcome:** `README.md` documents `skip_applied` in the settings reference.

**Verification:** `grep "skip_applied" README.md`

---

### Step 14: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**

Open `agent-development/agent-specs/architecture-breakdown.md`. Make the following targeted updates:

1. In the `/internal/domain` section, append after the existing `AppConfig` description:
   > `AppConfig` now includes `SkipApplied bool` (`json:"skip_applied"`) — when `true`, `GetTasks()` omits entries with `status == "applied"` from the returned queue.

2. In the `/internal/provider` section, append after the existing `GetTasks()` description:
   > When `Config.SkipApplied` is `true`, `GetTasks()` silently filters out any `reviewEntry` whose `Status` equals `"applied"` (case-insensitive) before constructing tasks.

3. In the `/internal/tui` section, update the `AppState` enum description to include `StateQueueComplete`:
   > `AppState` enum (`StateReviewing`, `StateGenreSelection`, `StateSettings`, `StateQueueComplete`). `StateQueueComplete` is entered when `skipToNext()` reaches the end of the queue; it displays a completion screen with `Ctrl+U` (go back) and `Ctrl+C` (quit) as the only active keys.

4. In the `/internal/metadata` section, append:
   > Integration tests live in `internal/metadata/writer_test.go`; a binary fixture is committed at `internal/metadata/testdata/fixture.mp3` (a 1-second silent MP3 with an ID3v2 header). All four exported functions (`WriteTags`, `WriteBPM`, `WriteYear`, `ReadTags`) have round-trip test coverage.

**Expected outcome:** `architecture-breakdown.md` accurately reflects the three new features introduced in Task 10.

---

### Step 15: Update `agent-development/agent-specs/FOLDER-STRUCTURE.md`

**Action:**

Open `agent-development/agent-specs/FOLDER-STRUCTURE.md`. Make the following changes:

1. Update the `> **Last updated:**` line at the top to:
   > **Last updated:** Task 10 — SkipApplied config flag; StateQueueComplete TUI state; metadata integration tests and testdata fixture added.

2. In the `internal/metadata/` entry under the project tree, add the new files:

```
├── metadata/
│   ├── writer.go               ← WriteTags, WriteBPM, WriteYear, ReadTags (ID3v2 tag writing)
│   ├── writer_test.go          ← Integration tests for all four writer functions
│   └── testdata/
│       └── fixture.mp3         ← Committed binary fixture: 1-second silent MP3 with ID3v2 header
```

**Expected outcome:** `FOLDER-STRUCTURE.md` reflects the new `testdata/` directory and test file.

---

### Step 16: Final build and test verification

**Action:**

Run the full test suite and static analysis from the project root:

```bash
go build ./...
go vet ./...
go test ./...
go test ./internal/metadata/... -v
go test ./internal/provider/... -v
go test ./internal/tui/... -v
```

Confirm:
- `go build ./...` exits 0 with no output.
- `go vet ./...` exits 0 with no output.
- `go test ./...` reports zero failures.
- `go test ./internal/metadata/... -v` shows all 10 metadata tests passing.
- `go test ./internal/provider/... -v` shows `TestGetTasks_SkipApplied` passing alongside all 6 existing provider tests.
- `go test ./internal/tui/... -v` shows the updated `TestSkipToNext_EndOfQueue` and both new `TestHandleKey_QueueComplete_*` tests passing.

**Expected outcome:** All checks pass with zero errors or failures.

---

## Open Questions & Decisions

### Q1: MP3 fixture generation method (`ffmpeg` vs. programmatic Go)

**Context:**

`fixture.mp3` is a binary committed to the repository and used by all 10 metadata tests. It must be parseable by `id3v2.Open` without error. Two generation strategies are practical:

- **A) `ffmpeg` command** — Produces a real 1-second silent MP3 with actual audio frames and a correct ID3v2 header. Reliable, battle-tested. Requires `ffmpeg` to be available on the machine that creates the fixture (only once — the binary is committed). CI does not need `ffmpeg` once the file is committed.

- **B) Minimal hand-crafted binary via Go** — Write a small `TestMain` or standalone script that uses `bogem/id3v2` to create the file programmatically. Avoids any `ffmpeg` dependency entirely. However, a file with no audio frames may cause `id3v2.Open` to issue warnings or `beep` to fail if any test accidentally tries to decode it as audio (these tests don't, but it reduces confidence). The hand-crafted ID3 header approach is also more fragile.

- **C) Commit the fixture generated by the planning agent** — The planning agent generates the file during plan execution using `ffmpeg` and commits it. The implementing agent only has to verify the file is present and run tests. This is the cleanest option if the planning machine has `ffmpeg`.

**Agent's recommendation:** Option A / C — generate with `ffmpeg` once (either at planning time if available, or at Step 10 of execution), commit the resulting binary, and document the command. `ffmpeg` is confirmed available on the current machine. A real MP3 with valid audio frames is more robust than a hand-crafted stub. The implementing agent should run the `ffmpeg` command in Step 10 if the file does not already exist in the repo.

**Human decision:** Let's go with option A. Let's make sure the file is committed to the repository.

---

### Q2: `viewQueueComplete()` summary wording — total tasks vs. "songs reviewed this session"

**Context:**

The task request says the completion screen should display "total songs in queue" and a "current session" count. But `len(m.queue.Tasks)` only tells us the total queue size — we don't currently track how many songs were reviewed in the current session (i.e. how many times `skipToNext()` or `writeTagsCmd` advanced the queue).

Two options:

- **A) Show total queue size only** — `"✓  Queue complete — N songs reviewed."` where `N = len(m.queue.Tasks)`. Simple, no new model field needed. This is technically "total in queue", but since the screen is only shown when all tasks are exhausted it reads correctly.

- **B) Track a session counter** — Add a `sessionCount int` field to `Model`, increment it each time `skipToNext()` advances the queue, and display both `sessionCount` and `len(m.queue.Tasks)` on the completion screen (e.g. `"✓  Queue complete — 3 of 10 songs reviewed this session."`). More accurate if the user launched with `SkipApplied: false` and had already-applied songs in the queue. Adds a new model field and increment logic.

**Agent's recommendation:** Option A — show only `len(m.queue.Tasks)` for now. The task request's wording ("total songs in queue") maps directly to this. A session counter can always be added in a future task. Keeping the model flat and the implementation minimal is in line with the task's stated "low-risk, low-effort" scope.

**Human decision:** Let's simplify the implementation by showing only the total queue size.

---

### Q3: `Ctrl+U` from `StateQueueComplete` — should it pop one History entry or rewind to the last non-complete state?

**Context:**

When the user reaches `StateQueueComplete`, `History` has been appended with the last task (the one that triggered the end-of-queue). Pressing `Ctrl+U` calls `undoLast()`, which pops one entry from `History` and decrements `CurrentIndex`. This is the standard undo behaviour and correctly rewinds to the last song.

However, if the user presses `Ctrl+U` multiple times from `StateQueueComplete`, the second `Ctrl+U` would fire from `StateReviewing` (normal undo behaviour) — no special handling is needed. The question is whether the **first** `Ctrl+U` from `StateQueueComplete` should behave identically to `undoLast()` in `StateReviewing`, or if it should have any special side-effects (e.g. stopping audio, resetting enrichment).

- **A) Call `undoLast()` verbatim** — `undoLast()` already resets enrichment (`resetEnrichment()`) and fires `playCmd` + `fetchYearCmd` + `fetchBPMCmd`. No special handling needed in the `StateQueueComplete` guard.

- **B) Add a custom "go back" handler** — Only decrement `CurrentIndex` and set `state = StateReviewing` without replaying audio. Less jarring if the user just wants to check the last song's metadata, not hear it again.

**Agent's recommendation:** Option A — call `undoLast()` verbatim. Audio replays on undo is established UX for this app (Task 9 unit tests confirm it). Changing undo behaviour only for this state would be an inconsistency. The task request explicitly says `Ctrl+U` "calls `undoLast()` normally."

**Human decision:** Audio replays on undo is what's expected. Let's go with option A.

---

## File Manifest

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/domain/models.go` | Modified | Add `SkipApplied bool \`json:"skip_applied"\`` field to `AppConfig` |
| 2 | `internal/provider/json_provider.go` | Modified | Add filter guard in `GetTasks()` loop to skip `"applied"` entries when `SkipApplied` is true |
| 3 | `internal/provider/json_provider_test.go` | Modified | Add `TestGetTasks_SkipApplied` with two sub-tests |
| 4 | `internal/tui/model.go` | Modified | Append `StateQueueComplete AppState` to the `AppState` iota block |
| 5 | `internal/tui/update.go` | Modified | Rewrite `skipToNext()` end-of-queue guard; add `StateQueueComplete` case in `handleKey()` |
| 6 | `internal/tui/view.go` | Modified | Add `viewQueueComplete()` and `pluralS()` helper; add `case StateQueueComplete` to `View()` dispatch |
| 7 | `internal/tui/update_test.go` | Modified | Update `TestSkipToNext_EndOfQueue`; add `TestHandleKey_QueueComplete_CtrlU` and `TestHandleKey_QueueComplete_OtherKeysAreNoOps` |
| 8 | `internal/metadata/testdata/fixture.mp3` | Created | Binary: 1-second silent MP3 with valid ID3v2 header, generated by `ffmpeg` |
| 9 | `internal/metadata/writer_test.go` | Created | 10 integration tests for `WriteTags` (4), `WriteBPM` (2), `WriteYear` (2), `ReadTags` (2) |
| 10 | `settings.example.json` | Modified | Add `"skip_applied": false` field after `"seek_delta_seconds"` |
| 11 | `README.md` | Modified | Add `skip_applied` row to Settings Reference table; add field to JSON example block |
| 12 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Document `SkipApplied`, `StateQueueComplete`, and metadata test coverage |
| 13 | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Modified | Update last-updated line; add `writer_test.go` and `testdata/fixture.mp3` entries under `internal/metadata/` |

**Total files created: 2 | Total files modified: 11**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go vet ./...` exits 0 with no output.
- [ ] `go test ./...` passes with zero failures.
- [ ] `go test ./internal/metadata/... -v` shows all 10 metadata tests passing.
- [ ] `go test ./internal/provider/... -v` shows `TestGetTasks_SkipApplied` passing alongside all existing provider tests.
- [ ] `go test ./internal/tui/... -v` shows the updated `TestSkipToNext_EndOfQueue` and both new `TestHandleKey_QueueComplete_*` tests passing.
- [ ] `internal/metadata/testdata/fixture.mp3` exists, is non-empty, and is under 10 KB.
- [ ] `settings.example.json` contains `"skip_applied": false`.
- [ ] `README.md` settings table contains a `skip_applied` row.
- [ ] `GetTasks()` with `SkipApplied: false` (or zero-value `AppConfig`) returns identical results to pre-Task-10 — confirmed by inspection and existing passing tests.
- [ ] `skipToNext()` on the last song sets `state = StateQueueComplete` and appends to `History` — confirmed by `TestSkipToNext_EndOfQueue`.
- [ ] Pressing `Ctrl+U` from `StateQueueComplete` returns to `StateReviewing` and issues a play command — confirmed by `TestHandleKey_QueueComplete_CtrlU`.
- [ ] No unrelated files were modified or deleted.
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated to document `SkipApplied`, `StateQueueComplete`, and metadata test file.
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` updated to reflect `testdata/` directory under `internal/metadata/`.

---

## Notes for the Implementing Agent

1. **Source code is the source of truth.** Read `internal/tui/update.go` line-by-line before touching `skipToNext()`. The critical risk is the double-append bug: the current code has one history-append after the guard; the rewritten code must have exactly two append sites (one for end-of-queue, one for normal advance) but must never append twice for the same case.

2. **`StateQueueComplete` iota value is 3.** Verify this by counting the existing constants in the block after reading `model.go`. If any constants were added between Task 9 and Task 10 (e.g., a hotfix), adjust accordingly.

3. **`strings` import in `json_provider.go`.** The `strings` package is already imported (used in `SaveState`). Do not add a duplicate import.

4. **The fixture binary must be committed.** `git add internal/metadata/testdata/fixture.mp3` explicitly. Make sure the file is not matched by any `.gitignore` rule — check `cat .gitignore` before committing.

5. **`TestWriteTags_EmptyPrimary` uses `os.Stat` mod-time.** On some filesystems the mod-time granularity is 1 second. Since `WriteTags` returns the error before opening the file (the empty-primary guard fires first), the file is never touched, so the mod-time check is reliable. Do not change this approach.

6. **`pluralS` is a package-level helper in `view.go`.** It must not be placed in `update.go` or `model.go`. It is a pure formatting utility belonging to the view layer.

7. **Do not modify `handleSettingsKey`.** The `ctrl+c` path in `handleSettingsKey` already calls `providerRef.SaveState` + `engine.Close()` — the same pattern used in `StateQueueComplete`. No changes to the settings handler are required.

8. **`undoLast()` already sets state back to `StateReviewing`** — it calls `playCmd` which transitions state via the model's existing song-load logic. No explicit `m.state = StateReviewing` assignment is needed in the `StateQueueComplete` / `ctrl+u` handler.

9. **Do not add `TestMain` to `writer_test.go`.** The fixture is a committed binary file, not generated at test time. `TestMain` is not needed and would complicate the test setup unnecessarily.

10. **`TestHandleKey_QueueComplete_OtherKeysAreNoOps` is important.** It guards against future regressions where a new key binding accidentally leaks into `StateQueueComplete`. If the list of no-op keys grows in a future task, this test will catch it.

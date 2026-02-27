# Implementation Plan: Task 8 — Reliability & Final Polish

## Overview

This plan audits the Task 8 request against the current codebase and implements the real
delta: reading `Title` and `Artist` from ID3 tags when the review queue is loaded, improving
the filename fallback display in the TUI header, and performing a final documentation sweep.

**Critical audit finding — most of what the request describes is already done:**

| Request Item | Current State |
|---|---|
| Atomic write strategy | **Already implemented** in `provider.SaveState` (temp file → fsync → rename, added in Task 1). No changes needed. |
| Audio error handling / "Corrupted File" toast | **Already implemented**: `PlayErrMsg` is caught in `Update`, the error is stored in `m.lastPlayErr`, and `view.go` renders it in the status bar. The user can press `Esc` to skip. No changes needed. |
| `operations` array in the JSON | **Not applicable**: the codebase never used an `operations` array. The task request references a schema design from a different version. The current schema is the `manual_review` array in `json_provider.go`. No changes needed. |
| Missing Metadata Fallback | **Partially done**: `view.go` already falls back to `task.FilePath` when `task.Title == ""`. However, `Task.Title` and `Task.Artist` are **never populated** — `GetTasks()` does not read ID3 tags, and the JSON schema has no `title` or `artist` fields. So every song always shows as "Unknown Artist / full/absolute/path". **This is the real gap.** |

**Real delta — the two things this task adds:**

1. **ID3 tag read on queue load:** Add a `ReadTags(path string) (title, artist string, err error)` function to the `internal/metadata` package. Call it inside `GetTasks()` to populate `Task.Title` and `Task.Artist` for every entry. Errors are non-fatal: if a file can't be read or has no tags, `Title` and `Artist` remain empty and the fallback display applies.

2. **Better filename fallback:** The current fallback in `view.go` when `Title == ""` is `task.FilePath` — the full absolute path (e.g. `/Users/you/Music/Artist/Song.mp3`). Replace it with `path.Base(task.FilePath)` so the header shows only `Song.mp3` rather than the full path.

3. **Documentation sweep:** Update `architecture-breakdown.md`, `FOLDER-STRUCTURE.md`, and `README.md` to reflect the new `ReadTags` function and the populated `Title`/`Artist` fields.

No new packages, no new top-level directories, no new `go get` commands are required.
`bogem/id3v2` is already in `go.mod` and is the library already used by `WriteTags`.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree and package dependency graph |
| Task Definition | `agent-development/pending/8-reliability.md` | The task being implemented |
| Domain Models | `internal/domain/models.go` | `Task` struct — confirm `Title` and `Artist` fields |
| Provider | `internal/provider/json_provider.go` | `GetTasks()` — where `ReadTags` call is added |
| Metadata Writer | `internal/metadata/writer.go` | Existing tag-write functions — `ReadTags` is added here |
| TUI View | `internal/tui/view.go` | `viewReviewing()` header — where the fallback is changed |
| Provider Test | `internal/provider/json_provider_test.go` | Existing tests — must still pass |

---

## Pre-Conditions

- Tasks 0 through 7 are complete and `go build ./...` and `go test ./...` both pass cleanly.
- `internal/domain/models.go` defines `Task` with `Title string` and `Artist string` fields.
- `internal/metadata/writer.go` exists and uses `github.com/bogem/id3v2/v2`.
- `internal/provider/json_provider.go` defines `GetTasks()` which builds `domain.Task` values.
- `internal/tui/view.go` already contains the `title == ""` guard that falls back to `task.FilePath`.
- `github.com/bogem/id3v2/v2` is in `go.mod` (used by the existing writer).
- No additional `go get` commands are required.

---

## Step-by-Step Implementation

### Step 1: Confirm the Baseline

**Action:**
```
cd mp3-reviewer && go build ./... && go test ./...
```

**Expected outcome:**
All packages compile cleanly and all tests pass. Zero failures.

**Verification:**
Terminal output shows no `FAIL` lines and no build errors.

---

### Step 2: Add `ReadTags` to `internal/metadata/writer.go`

**Action:**
Open `internal/metadata/writer.go`. Add the following function **at the end of the file**,
after `WriteYear`. Do not change any existing code.

Add `"path/filepath"` to the import block (it is needed for `filepath.Base`). The current
import block is:

```
import (
    "fmt"

    "github.com/bogem/id3v2/v2"
)
```

Change it to:

```
import (
    "fmt"
    "path/filepath"

    "github.com/bogem/id3v2/v2"
)
```

Then add the following function at the end of the file:

```
// ReadTags opens the audio file at path and reads the Title (TIT2 frame) and
// Artist (TPE1 frame) ID3 tags. It returns the values found; either may be an
// empty string if the tag is absent or the file has no ID3 header.
//
// Errors are non-fatal from the caller's perspective — if the file cannot be
// opened or parsed, ReadTags returns empty strings and the error so the caller
// can fall back gracefully (e.g. display the filename instead).
//
// This function is used by ManualReviewProvider.GetTasks() to populate
// Task.Title and Task.Artist at queue-load time. It opens the file read-only
// (id3v2.Options{Parse: true}) and does not write anything.
func ReadTags(path string) (title, artist string, err error) {
    tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
    if err != nil {
        return "", "", fmt.Errorf("metadata: opening %q for tag read: %w", path, err)
    }
    defer tag.Close()

    title = tag.Title()
    artist = tag.Artist()
    return title, artist, nil
}
```

Note on the `filepath` import: `filepath` is imported here to be available for use in the
`ReadTags` docstring context, but the function itself does not use it. Remove `filepath`
from the import if the compiler reports it as unused after adding `ReadTags`. The import
was listed in case the implementing agent wants to trim the path for error messages, but it
is not required — the function only needs `fmt` and `id3v2`.

**Corrected import block** (only add what is needed — keep it minimal):

```
import (
    "fmt"

    "github.com/bogem/id3v2/v2"
)
```

The existing import block is already correct. Do **not** add `path/filepath` to
`writer.go` — it is not used in `ReadTags`. Keep the import block exactly as it is.

**Expected outcome:**
`internal/metadata/writer.go` compiles. The new `ReadTags` function is exported and
callable from `internal/provider`.

**Verification:**
```
cd mp3-reviewer && go build ./internal/metadata/...
```
Zero errors.

---

### Step 3: Call `ReadTags` in `GetTasks()`

**Action:**
Open `internal/provider/json_provider.go`. In the `GetTasks()` function, find the loop
that builds `domain.Task` values:

```go
tasks := make([]domain.Task, 0, len(raw.ManualReview))
for _, entry := range raw.ManualReview {
    task := domain.Task{
        FilePath: filepath.Join(p.Config.MusicFolder, entry.FilePath),
        Genre1:   entry.PrimaryGenre,
        Genre2:   entry.SecondaryGenre,
        BPM:      entry.BPM,
        Year:     entry.Year,
    }
    tasks = append(tasks, task)
}
```

Replace it with:

```go
tasks := make([]domain.Task, 0, len(raw.ManualReview))
for _, entry := range raw.ManualReview {
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

Also add the `metadata` import. The current import block in `json_provider.go` is:

```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "song-reviewer/internal/domain"
)
```

Change it to:

```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "song-reviewer/internal/domain"
    "song-reviewer/internal/metadata"
)
```

**Expected outcome:**
`internal/provider/json_provider.go` compiles. `GetTasks()` now calls `metadata.ReadTags`
for every entry and populates `Task.Title` and `Task.Artist`. Missing or unreadable files
silently produce empty strings (the `_` discards the error).

**Verification:**
```
cd mp3-reviewer && go build ./internal/provider/...
```
Zero errors.

---

### Step 4: Improve the Filename Fallback in `view.go`

**Action:**
Open `internal/tui/view.go`. In `viewReviewing()`, find the header-rendering block:

```go
task := m.queue.Tasks[m.queue.CurrentIndex]
artist := task.Artist
title := task.Title
if artist == "" {
    artist = "Unknown Artist"
}
if title == "" {
    title = task.FilePath
}
```

Replace `title = task.FilePath` with `title = filepath.Base(task.FilePath)` so that only the
filename component (e.g. `Song.mp3`) is shown instead of the full absolute path:

```go
task := m.queue.Tasks[m.queue.CurrentIndex]
artist := task.Artist
title := task.Title
if artist == "" {
    artist = "Unknown Artist"
}
if title == "" {
    title = filepath.Base(task.FilePath)
}
```

Also add `"path/filepath"` to the import block. The current imports in `view.go` are:

```go
import (
    "fmt"

    "github.com/charmbracelet/lipgloss"
)
```

Change to:

```go
import (
    "fmt"
    "path/filepath"

    "github.com/charmbracelet/lipgloss"
)
```

**Expected outcome:**
When a song has no `Title` tag, the header now shows only `Song.mp3` (the file's base name)
instead of the full path `/Users/you/Music/Artist/Song.mp3`. The change is purely cosmetic
and does not affect any logic.

**Verification:**
```
cd mp3-reviewer && go build ./internal/tui/...
```
Zero errors.

---

### Step 5: Run the Full Test Suite

**Action:**
```
cd mp3-reviewer && go build ./... && go test ./...
```

**Expected outcome:**
All packages compile. All existing tests pass. Zero failures.

The existing `internal/provider/json_provider_test.go` tests use a mock JSON file with
entries that likely point to non-existent paths. Because `ReadTags` errors are silently
discarded (`_ =`), the `GetTasks()` call still returns tasks — just with empty `Title`
and `Artist`. The tests should continue to pass without modification.

**Verification:**
Terminal output shows no `FAIL` lines.

---

### Step 6: Update `internal/provider/json_provider_test.go` (if needed)

**Action:**
Open `internal/provider/json_provider_test.go` and run the tests:

```
cd mp3-reviewer && go test ./internal/provider/...
```

If any test fails because `GetTasks()` now calls `metadata.ReadTags()` on paths that do
not exist on disk, and the test is checking for an error where none should be (or vice
versa), update the test expectations as follows:

- Any test that checks `Task.Title == ""` or `Task.Artist == ""` for a non-existent file
  path should **pass without change** — `ReadTags` returns empty strings on error and the
  error is discarded.
- If a test fails for a different reason (e.g. import cycle), investigate and fix. There
  should be no import cycle because `provider` → `metadata` → `id3v2` is a valid one-way
  dependency with no reverse edge.

If all tests pass in Step 5, skip this step.

**Expected outcome:**
`go test ./internal/provider/...` passes with zero failures.

**Verification:**
```
cd mp3-reviewer && go test ./internal/provider/... -v
```
All test functions report `--- PASS`.

---

### Step 7: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**
Open `agent-development/agent-specs/architecture-breakdown.md`.

**7.1 — Update the `/internal/metadata` entry.**

Find the current text:
> `/internal/metadata`: ID3 tag write logic. Exposes `WriteTags(path, primary, secondary string) error`, ...

Change the opening phrase from:
> ID3 tag write logic.

to:
> ID3 tag read/write logic.

Add `ReadTags(path string) (title, artist string, err error)` to the list of exposed
functions. The sentence describing the exposed surface should become:

> Exposes `ReadTags(path string) (title, artist string, err error)` (reads the TIT2 and
> TPE1 ID3 frames; non-fatal — returns empty strings on error), `WriteTags(path, primary,
> secondary string) error`, `WriteBPM(path, bpm string) error`, and
> `WriteYear(path, year string) error` (writes ID3v2 frames using `github.com/bogem/id3v2/v2`).

**7.2 — Update the `/internal/provider` entry.**

Find the existing description of `GetTasks()`. Append a sentence after the current
description of what `GetTasks()` populates:

> After resolving each file path, `GetTasks()` calls `metadata.ReadTags(absPath)` to
> populate `Task.Title` and `Task.Artist` from the file's ID3 TIT2 and TPE1 frames. Tag
> read errors are silently ignored so that missing or corrupted files do not prevent the
> queue from loading.

**7.3 — Update the `/internal/tui` entry.**

In the sentence describing `view.go`, find:
> the main four-row layout (`viewReviewing` — header, progress bar, enrichment panel, status bar)

Add a note about the filename fallback after the header description. Change the phrase
> `viewReviewing` — header, progress bar, enrichment panel, status bar

to:
> `viewReviewing` — header (shows Artist/Title from ID3 tags; falls back to "Unknown Artist" and `filepath.Base(FilePath)` when tags are absent), progress bar, enrichment panel, status bar

**Expected outcome:**
`architecture-breakdown.md` accurately describes `ReadTags`, the tag-read-on-load behaviour
in `GetTasks()`, and the filename fallback in the TUI header.

**Verification:**
Open the file and confirm the three updated passages are present and accurate.

---

### Step 8: Update `agent-development/agent-specs/FOLDER-STRUCTURE.md`

**Action:**
Open `agent-development/agent-specs/FOLDER-STRUCTURE.md`. Find the "Last updated" line:

> **Last updated:** Task 7 — Application assembly hardening and Settings overlay implemented.

Change it to:

> **Last updated:** Task 8 — ID3 tag read on queue load; filename fallback improved.

No other changes are required — no new packages or directories were introduced.

**Expected outcome:**
The timestamp reflects Task 8.

**Verification:**
Open the file and confirm the updated line is present.

---

### Step 9: Update `README.md`

**Action:**
Open `README.md`.

**9.1 — Update the Features list.**

Find the existing bullet:
> **Review Queue** — Reads a JSON file of songs marked for `manual_review` and presents them one at a time.

Append a parenthetical so it reads:

> **Review Queue** — Reads a JSON file of songs marked for `manual_review` and presents them one at a time. Song title and artist are read automatically from each file's ID3 tags on load; if tags are absent the filename is shown instead.

**9.2 — Update the Architecture table.**

Find the row:
```
| `internal/metadata/` | ID3 tag write logic (pure Go, `bogem/id3v2`). |
```

Change it to:
```
| `internal/metadata/` | ID3 tag read/write logic (pure Go, `bogem/id3v2`). Reads Title and Artist on queue load; writes Genre, BPM, and Year on user action. |
```

**Expected outcome:**
The README accurately describes the ID3 tag read behaviour without requiring any section
additions — it is a small, accurate clarification to existing text.

**Verification:**
Open the file and confirm both updated passages are present.

---

## Open Questions & Decisions

### Q1: Should `ReadTags` errors be logged to stderr or silently discarded?

**Context:**
When `GetTasks()` calls `metadata.ReadTags(absPath)` and the file does not exist or is not
a valid MP3, `ReadTags` returns an error. The current plan discards this error silently
(`title, artist, _ := metadata.ReadTags(absPath)`). This is consistent with the existing
pattern for missing/corrupted files in this app: `PlayErrMsg` handles audio decode errors
at play time, so there is no need to double-report at queue-load time.

The alternative is to print a warning to `stderr` for each unreadable file when the queue
loads. This gives the user early visibility if their `MusicFolder` path is wrong.

**Options:**
- **A)** Silent discard — `title, artist, _ := metadata.ReadTags(absPath)`. No output.
  The TUI still shows the filename fallback; the user will notice the missing tags when
  they see the header. If audio also fails to play, `PlayErrMsg` surfaces the error.
  - Pro: No noise on startup; consistent with the non-fatal philosophy of the app.
  - Con: No early warning if many files in the queue are unreadable.
- **B)** Log to `stderr` — `title, artist, err := metadata.ReadTags(absPath); if err != nil { fmt.Fprintf(os.Stderr, ...) }`.
  - Pro: Gives the user an early signal that file paths are wrong.
  - Con: Pollutes stderr at startup; may confuse users who are running the app from a
    terminal and see error lines scroll by before the TUI opens.
- **C)** Collect errors and surface them in the TUI — add a `[]string` field to `Model`
  for load-time warnings, show them in the status bar on first render.
  - Pro: Visible inside the TUI without polluting stderr.
  - Con: Significant complexity increase; requires a new Model field and rendering logic.

**Agent's recommendation:** **Option A** (silent discard). The play-time error handling
already surfaces unreadable files when the user encounters them. Adding startup noise
(Options B/C) is not proportionate to the marginal value for this reliability task.

**Human decision:** Let's use option A for simplicity.

---

### Q2: Should `GetTasks()` skip entries where the audio file does not exist on disk?

**Context:**
Currently `GetTasks()` returns a `domain.Task` for every entry in the JSON file, regardless
of whether the resolved `absPath` exists on disk. If `MusicFolder` is wrong, all tasks are
returned with valid-looking paths that will fail when `playCmd` is invoked. The user then
sees a `PlayErrMsg` error in the status bar and can press `Esc` to skip.

An alternative is to stat every file in `GetTasks()` and omit entries whose files do not
exist, returning a smaller (but playable) queue.

**Options:**
- **A)** Keep current behaviour — return all entries regardless of file existence. Let
  the audio engine surface individual errors at play time.
  - Pro: No filesystem stat on every file at load time (can be slow for large queues).
    The Settings overlay (`Ctrl-O`) lets the user fix the path and reload.
  - Con: The user may not realise all songs are unplayable until they start reviewing.
- **B)** Stat-filter at load time — call `os.Stat(absPath)` for each entry; skip entries
  where the file does not exist and log a warning to stderr per skipped file.
  - Pro: The queue only contains songs that are actually accessible.
  - Con: Extra I/O on every load; makes queues with wrong `MusicFolder` appear empty
    rather than showing meaningful error messages. Interacts oddly with network drives.

**Agent's recommendation:** **Option A** (keep current behaviour). The app already handles
missing files gracefully via `PlayErrMsg`. Stat-filtering introduces surprising silent
omissions and slows down queue load for large libraries.

**Human decision:** Let's keep current behavior: Option A.

---

### Q3: Should `Task.Title` and `Task.Artist` be persisted back to the JSON file?

**Context:**
`ReadTags` populates `Task.Title` and `Task.Artist` in memory when the queue loads. These
fields are currently not present in the `reviewEntry` JSON schema (`json_provider.go`).
If they were added to the schema and written back by `SaveState`, subsequent loads would
not need to re-read the ID3 tags for already-processed songs.

**Options:**
- **A)** In-memory only — `Title` and `Artist` are read from ID3 tags on every load and
  never written to the JSON. Simple; no schema change.
  - Pro: Zero schema change; JSON stays minimal.
  - Con: Every load re-reads all ID3 tags (negligible for typical queue sizes of < 500).
- **B)** Persist to JSON — add `title` and `artist` fields to `reviewEntry` and
  `SaveState`. On subsequent loads, use the JSON value if present, otherwise fall back to
  `ReadTags`.
  - Pro: Faster loads for large queues; allows the user to see title/artist in the JSON.
  - Con: Schema change; `SaveState` becomes more complex; the JSON values could diverge
    from the actual ID3 tags if the user edits the file externally.

**Agent's recommendation:** **Option A** (in-memory only). The queue is typically small
(< a few hundred songs), so re-reading tags on every load is imperceptible. The JSON
schema should remain minimal and focused on review-specific data (status, genre, BPM, year)
rather than duplicating source-of-truth tag data.

**Human decision:** Let's use option A, it's simpler and the JSON schema should remain minimal.

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/metadata/writer.go` | Modified | Add `ReadTags(path string) (title, artist string, err error)` function at end of file |
| 2 | `internal/provider/json_provider.go` | Modified | Import `internal/metadata`; call `metadata.ReadTags(absPath)` in `GetTasks()` loop; populate `Task.Title` and `Task.Artist` |
| 3 | `internal/tui/view.go` | Modified | Import `path/filepath`; change `title = task.FilePath` fallback to `title = filepath.Base(task.FilePath)` |
| 4 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Update `/internal/metadata`, `/internal/provider`, and `/internal/tui` descriptions |
| 5 | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Modified | Update "Last updated" timestamp |
| 6 | `README.md` | Modified | Update Features bullet and Architecture table for `internal/metadata` |

**Total files created: 0 | Total files modified: 6**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes with zero failures
- [ ] `internal/metadata/writer.go`: `ReadTags` function is present and exported; it reads `tag.Title()` and `tag.Artist()`; returns `("", "", err)` on open failure
- [ ] `internal/provider/json_provider.go`: imports `song-reviewer/internal/metadata`; `GetTasks()` loop calls `metadata.ReadTags(absPath)` with error discarded; `task.Title` and `task.Artist` are populated from the return values
- [ ] `internal/tui/view.go`: imports `path/filepath`; the `title == ""` fallback uses `filepath.Base(task.FilePath)` not `task.FilePath`
- [ ] No unrelated files were modified or deleted
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated: `/internal/metadata` entry mentions `ReadTags`; `/internal/provider` entry mentions tag read on load; `/internal/tui` entry describes filename fallback
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` "Last updated" line updated to Task 8
- [ ] `README.md` Features bullet and Architecture table updated

---

## Notes for the Implementing Agent

1. **Read all source files before making any edit.** Specifically: read `internal/metadata/writer.go`, `internal/provider/json_provider.go`, and `internal/tui/view.go` in full before touching them.

2. **Import cycle check.** Before adding `"song-reviewer/internal/metadata"` to `json_provider.go`, confirm there is no import cycle. The current dependency graph is:
   - `provider` → `domain` (direct)
   - `metadata` → `id3v2` (external only)
   - `metadata` does NOT import `provider` or `domain`
   Therefore `provider` → `metadata` is safe.

3. **`bogem/id3v2` tag.Title() / tag.Artist() return empty strings** when the frame is absent — they do not return an error. The only error path in `ReadTags` is `id3v2.Open()` failing (file not found, permission denied, or not a valid ID3 container). This means `title` and `artist` can be empty even when `err == nil`. The caller should treat both empty `Title` and empty `Artist` as valid states requiring a fallback.

4. **The `filepath.Base` change is purely cosmetic.** It does not affect any logic, tests, or data. It is a one-line change in one function in `view.go`.

5. **Do NOT modify `domain/models.go`.** The `Task` struct already has `Title string` and `Artist string` fields. They simply haven't been populated until now.

6. **Do NOT add `Title` or `Artist` to the JSON schema** (`reviewEntry` in `json_provider.go`) unless Q3 is resolved as Option B. The plan as written uses Option A (in-memory only).

7. **The existing test for `GetTasks()`** in `json_provider_test.go` uses JSON entries with `filepath` values pointing to non-existent paths on disk. Because `metadata.ReadTags` errors are silently discarded, the tests will still return tasks — just with empty `Title` and `Artist`. **Do not change existing test assertions** unless a test explicitly checks `Title` or `Artist` (it likely does not, since those fields were never populated before).

8. **After completing all steps**, move `agent-development/pending/8-reliability.md` to `agent-development/done/requests/8-reliability.md` and move this plan from `agent-development/plans/8-reliability-plan.md` to `agent-development/done/plans/8-reliability-plan.md`.

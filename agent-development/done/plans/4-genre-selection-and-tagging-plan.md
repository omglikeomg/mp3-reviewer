# Implementation Plan: Task 4 — Genre Selection & Tagging

## Overview

This plan implements the core user-facing feature of the Song Reviewer CLI: the two-step genre selection modal, ID3 tag writing, JSON state persistence, and automatic queue advancement. When the user presses `Enter` or `Space` on the main review screen, the TUI transitions to `StateGenreSelection` and presents a scrollable, filterable list (via `bubbles/list`) of all genres from `AppConfig.GenreList`. After a Primary Genre is selected, the list refreshes and a Secondary Genre (or `[NONE]`) is chosen. The app then writes the ID3 tags to the MP3 file, updates the source JSON on disk to mark the entry as `"applied"`, and automatically advances to the next song.

This task touches five areas:

1. **`internal/metadata/writer.go`** — Implements `WriteTags(path, primary, secondary string) error` using `dhowden/tag` (or a suitable pure-Go ID3v2 writing library — see Open Questions).
2. **`internal/provider/json_provider.go`** — Adds `SaveState(tasks []domain.Task) error` to persist genre assignments back into the source JSON file.
3. **`internal/tui/model.go`** — Extends `Model` with genre-selection state fields and a new `TagWrittenMsg` / `SaveStateMsg` message type; adds Cmd factories for async writing.
4. **`internal/tui/update.go`** — Wires genre selection key handling, `bubbles/list` delegation, `TagWrittenMsg` / `SaveStateErrMsg` handling, and automatic queue advancement.
5. **`internal/tui/view.go`** — Replaces the "coming soon" genre placeholder with the real two-step genre selection modal.

After this task, users can complete a full end-to-end review cycle: play → seek → tag → advance.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand the dual-tier tagging workflow |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, MVU rules |
| Task Definition | `agent-development/pending/4-genre-selection-and-tagging.md` | The task being implemented |
| Completed Plan 3 | `agent-development/done/plans/3-tui-foundations-plan.md` | TUI shape established in the previous task |
| Domain Models | `internal/domain/models.go` | `Task`, `AppConfig`, `ReviewQueue` structs |
| Provider | `internal/provider/json_provider.go` | Existing JSON adapter — will be extended here |
| TUI Model | `internal/tui/model.go` | Current model fields, Cmd helpers, constructor |
| TUI Update | `internal/tui/update.go` | Current message dispatch and key handling |
| TUI View | `internal/tui/view.go` | Current rendering logic and style variables |
| Metadata Stub | `internal/metadata/writer.go` | Currently only `package metadata` — to be filled |
| Main Entry Point | `cmd/reviewer/main.go` | Wire-up reference — no changes expected here |
| Review JSON Example | `manual_review.example.json` | Canonical JSON format for write-back reference |

---

## Pre-Conditions

- Tasks 0–3 must be fully complete. Specifically:
  - `go.mod` declares `module song-reviewer` and all charmbracelet/bubbletea/bubbles/lipgloss/faiface/beep dependencies are already resolved.
  - `internal/domain/models.go` defines `Task`, `AppConfig`, and `ReviewQueue` as documented.
  - `internal/provider/json_provider.go` defines `ManualReviewProvider` with `GetTasks()`.
  - `internal/tui/model.go`, `update.go`, and `view.go` compile and run correctly, with `StateGenreSelection` showing only a placeholder.
  - `internal/metadata/writer.go` exists (currently only `package metadata`).
  - `go build ./...` and `go test ./...` both pass with zero errors.
- The `dhowden/tag` library is **not** yet in `go.mod` — it must be added in Step 1 (see Open Questions Q1 for important caveat).
- `data/manual_review.json` exists and is writable.
- `config/settings.json` exists and contains a non-empty `genres` array.

---

## Step-by-Step Implementation

### Step 1: Add `dhowden/tag` (or Alternative) to the Module

**Action:**

Run the following command from the project root to add the metadata library. Note: `dhowden/tag` supports **reading** ID3 tags but has **no write support**. See Open Question Q1 for the resolution of this critical ambiguity. The command below uses the library resolved in Q1.

If the human decision in Q1 is **Option A** (`go-id3` via `bogem/id3v2`):

```
cd mp3-reviewer && go get github.com/bogem/id3v2/v2
```

If the human decision in Q1 is **Option B** (keep `dhowden/tag` for reads, use `bogem/id3v2` for writes — same practical outcome as A):

```
cd mp3-reviewer && go get github.com/bogem/id3v2/v2
```

If the human decision in Q1 is **Option C** (shell out to `id3v2` CLI tool — no new Go dependency):

No `go get` needed. The implementation in Step 4 will use `os/exec`.

> **IMPORTANT:** Wait for the human to resolve Q1 before executing this step. The rest of this plan assumes **Option A/B: `github.com/bogem/id3v2/v2`** because `dhowden/tag` cannot write tags. If Q1 is resolved differently, adapt Step 4 accordingly.

**Expected outcome:** `go.mod` and `go.sum` updated to include the chosen ID3 writing library.

**Verification:**

```
cd mp3-reviewer && cat go.mod | grep id3
```

Should print the new dependency line.

---

### Step 2: Extend the JSON Provider with `SaveState`

**Action:**

Modify `internal/provider/json_provider.go` to:

1. Extend the `reviewEntry` struct with fields for `status`, `primary_genre`, and `secondary_genre`.
2. Add a `SaveState(tasks []domain.Task) error` method to `ManualReviewProvider` that reads the current JSON file, updates only the entries that have been tagged (matching by relative `filepath`), and writes the file back atomically.

The full updated file content must be:

```go
package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"song-reviewer/internal/domain"
)

// TaskProvider is the interface for all review queue sources.
// Any type that can supply a list of Tasks to review implements this interface.
type TaskProvider interface {
	GetTasks() ([]domain.Task, error)
}

// manualReviewFile mirrors the top-level structure of the manual_review JSON file.
type manualReviewFile struct {
	ManualReview []reviewEntry `json:"manual_review"`
}

// reviewEntry mirrors a single item in the "manual_review" array.
// Fields added in Task 4: Status, PrimaryGenre, SecondaryGenre.
type reviewEntry struct {
	FilePath       string `json:"filepath"`
	Reason         string `json:"reason"`
	Confidence     int    `json:"confidence"`
	Status         string `json:"status,omitempty"`
	PrimaryGenre   string `json:"primary_genre,omitempty"`
	SecondaryGenre string `json:"secondary_genre,omitempty"`
}

// ManualReviewProvider reads a JSON file containing a "manual_review" array
// and converts each entry into a domain.Task.
type ManualReviewProvider struct {
	Config domain.AppConfig
}

// GetTasks reads the JSON file at Config.JsonPath, parses the "manual_review"
// array, and returns a slice of domain.Task values. Each task's FilePath is
// resolved to an absolute path by joining Config.MusicFolder with the relative
// path stored in the JSON.
func (p ManualReviewProvider) GetTasks() ([]domain.Task, error) {
	data, err := os.ReadFile(p.Config.JsonPath)
	if err != nil {
		return nil, fmt.Errorf("provider: reading review file %q: %w", p.Config.JsonPath, err)
	}

	var raw manualReviewFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("provider: parsing review file %q: %w", p.Config.JsonPath, err)
	}

	tasks := make([]domain.Task, 0, len(raw.ManualReview))
	for _, entry := range raw.ManualReview {
		task := domain.Task{
			FilePath: filepath.Join(p.Config.MusicFolder, entry.FilePath),
			Genre1:   entry.PrimaryGenre,
			Genre2:   entry.SecondaryGenre,
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// SaveState persists genre assignments back into the source JSON file on disk.
// For each task in the provided slice, if Genre1 is non-empty the corresponding
// JSON entry is updated with status="applied", primary_genre, and secondary_genre.
// Matching is done by relative filepath (task path with MusicFolder prefix stripped).
//
// The write is done atomically: the file is first written to a temp file in the
// same directory, then renamed over the original. This prevents data loss if the
// process is killed mid-write.
func (p ManualReviewProvider) SaveState(tasks []domain.Task) error {
	// Read current file content.
	data, err := os.ReadFile(p.Config.JsonPath)
	if err != nil {
		return fmt.Errorf("provider: SaveState reading %q: %w", p.Config.JsonPath, err)
	}

	var raw manualReviewFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("provider: SaveState parsing %q: %w", p.Config.JsonPath, err)
	}

	// Build a lookup: relative filepath -> task
	taskMap := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		// Derive the relative path by stripping the music folder prefix.
		rel := t.FilePath
		if strings.HasPrefix(rel, p.Config.MusicFolder) {
			rel = strings.TrimPrefix(rel, p.Config.MusicFolder)
			rel = strings.TrimPrefix(rel, string(filepath.Separator))
		}
		taskMap[rel] = t
	}

	// Update matching entries in the raw struct.
	for i, entry := range raw.ManualReview {
		t, ok := taskMap[entry.FilePath]
		if !ok || t.Genre1 == "" {
			continue
		}
		raw.ManualReview[i].Status = "applied"
		raw.ManualReview[i].PrimaryGenre = t.Genre1
		raw.ManualReview[i].SecondaryGenre = t.Genre2
	}

	// Marshal back to JSON with indentation for human readability.
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("provider: SaveState marshalling: %w", err)
	}

	// Atomic write via temp file + rename.
	dir := filepath.Dir(p.Config.JsonPath)
	tmp, err := os.CreateTemp(dir, "manual_review_*.json.tmp")
	if err != nil {
		return fmt.Errorf("provider: SaveState creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, p.Config.JsonPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState renaming temp file: %w", err)
	}

	return nil
}
```

**Expected outcome:** `internal/provider/json_provider.go` compiles cleanly. The `reviewEntry` struct now has `Status`, `PrimaryGenre`, and `SecondaryGenre` fields. `SaveState` is a method on `ManualReviewProvider`.

**Verification:**

```
cd mp3-reviewer && go build ./internal/provider/...
```

---

### Step 3: Add `SaveState` Tests to the Provider Test File

**Action:**

Append the following two test functions to `internal/provider/json_provider_test.go` (do not remove existing tests):

```go
func TestSaveState_UpdatesAppliedEntries(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			},
			{
				"filepath": "Miles Davis/Kind of Blue.mp3",
				"reason": "Uncertain subgenre",
				"confidence": 2
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	musicFolder := "/test/music"
	cfg := domain.AppConfig{
		MusicFolder: musicFolder,
		JsonPath:    jsonPath,
	}
	p := ManualReviewProvider{Config: cfg}

	tasks := []domain.Task{
		{
			FilePath: filepath.Join(musicFolder, "Cream/Cream - Strange Brew.mp3"),
			Genre1:   "Rock",
			Genre2:   "Blues-Rock",
		},
		// Miles Davis intentionally left un-tagged (Genre1 == "").
		{
			FilePath: filepath.Join(musicFolder, "Miles Davis/Kind of Blue.mp3"),
			Genre1:   "",
		},
	}

	if err := p.SaveState(tasks); err != nil {
		t.Fatalf("SaveState() returned unexpected error: %v", err)
	}

	// Re-read and verify.
	data, _ := os.ReadFile(jsonPath)
	var raw struct {
		ManualReview []struct {
			FilePath       string `json:"filepath"`
			Status         string `json:"status"`
			PrimaryGenre   string `json:"primary_genre"`
			SecondaryGenre string `json:"secondary_genre"`
		} `json:"manual_review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parsing saved JSON: %v", err)
	}

	if raw.ManualReview[0].Status != "applied" {
		t.Errorf("entry[0].status = %q, want %q", raw.ManualReview[0].Status, "applied")
	}
	if raw.ManualReview[0].PrimaryGenre != "Rock" {
		t.Errorf("entry[0].primary_genre = %q, want %q", raw.ManualReview[0].PrimaryGenre, "Rock")
	}
	if raw.ManualReview[0].SecondaryGenre != "Blues-Rock" {
		t.Errorf("entry[0].secondary_genre = %q, want %q", raw.ManualReview[0].SecondaryGenre, "Blues-Rock")
	}

	// The un-tagged Miles Davis entry must be unchanged.
	if raw.ManualReview[1].Status != "" {
		t.Errorf("entry[1].status = %q, want empty (untagged)", raw.ManualReview[1].Status)
	}
}

func TestSaveState_NoneSecondaryGenre(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	p := ManualReviewProvider{Config: cfg}

	tasks := []domain.Task{
		{
			FilePath: filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3"),
			Genre1:   "Rock",
			Genre2:   "", // [NONE] chosen — secondary should be omitted from JSON.
		},
	}

	if err := p.SaveState(tasks); err != nil {
		t.Fatalf("SaveState() unexpected error: %v", err)
	}

	data, _ := os.ReadFile(jsonPath)
	var raw struct {
		ManualReview []struct {
			SecondaryGenre string `json:"secondary_genre"`
		} `json:"manual_review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parsing saved JSON: %v", err)
	}
	if raw.ManualReview[0].SecondaryGenre != "" {
		t.Errorf("secondary_genre = %q, want empty string (omitempty)", raw.ManualReview[0].SecondaryGenre)
	}
}
```

Note: the test file already imports `encoding/json`, `os`, `path/filepath`, `testing`, and `song-reviewer/internal/domain` — no new imports are needed.

**Expected outcome:** Two new test functions are appended. All four existing tests and the two new tests compile and pass.

**Verification:**

```
cd mp3-reviewer && go test ./internal/provider/...
```

---

### Step 4: Implement `internal/metadata/writer.go`

**Action:**

Replace the stub content of `internal/metadata/writer.go` with the full implementation below. This assumes Q1 was resolved as **Option A/B** (using `github.com/bogem/id3v2/v2`). If Q1 was resolved as Option C, replace the body with an `os/exec` call to the `id3v2` CLI instead.

```go
package metadata

import (
	"fmt"

	"github.com/bogem/id3v2/v2"
)

// WriteTags opens the MP3 file at path and writes the genre tags using ID3v2.
// primary must be non-empty. secondary may be empty (representing [NONE]).
//
// Genre encoding strategy (see Q2 in the plan for the human decision):
//   - The ID3v2 "TCON" (Content Type / Genre) frame is written with the
//     primary genre value.
//   - If secondary is non-empty, the TCON frame is set to "Primary/Secondary"
//     (forward-slash separator) so standard players display the full value
//     while custom tooling can split on "/" to recover both tiers.
//   - If secondary is empty, TCON is set to primary only.
//
// The file is opened, modified in-place, and saved. Returns a wrapped error
// on any failure.
func WriteTags(path string, primary string, secondary string) error {
	if primary == "" {
		return fmt.Errorf("metadata: WriteTags called with empty primary genre")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for tag writing: %w", path, err)
	}
	defer tag.Close()

	// Build the genre string.
	genreValue := primary
	if secondary != "" {
		genreValue = primary + "/" + secondary
	}

	// Set the TCON (Content Type) frame — this is the standard ID3v2 genre field.
	tag.SetGenre(genreValue)

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving tags to %q: %w", path, err)
	}

	return nil
}
```

**Expected outcome:** `internal/metadata/writer.go` compiles cleanly. It exports exactly one function: `WriteTags(path, primary, secondary string) error`.

**Verification:**

```
cd mp3-reviewer && go build ./internal/metadata/...
```

---

### Step 5: Extend `internal/tui/model.go` with Genre Selection State

**Action:**

Replace the full content of `internal/tui/model.go` with the version below. Key changes from the existing file:

- Add `"github.com/charmbracelet/bubbles/list"` import.
- Add `"song-reviewer/internal/provider"` import.
- Add three new fields to `Model`: `genreList list.Model`, `genreStep int`, `selectedPrimary string`.
- Add a `providerRef ManualReviewProvider` field so the model can call `SaveState` via a `tea.Cmd`.
- Add two new message types: `TagWrittenMsg` and `SaveStateErrMsg`.
- Add two new Cmd factory functions: `writeTagsCmd` and `saveStateCmd`.
- Add a helper `makeGenreList(genres []string, includeNone bool) list.Model` that builds the `bubbles/list` model from the genre slice.
- Update `New(...)` to accept `ManualReviewProvider` as an additional parameter, build the initial genre list, and store it on the model.

The full updated file content:

```go
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/metadata"
	"song-reviewer/internal/provider"
)

// AppState represents which screen the TUI is currently showing.
type AppState int

const (
	// StateReviewing is the default state: the main playback + header + status bar view.
	StateReviewing AppState = iota

	// StateGenreSelection is shown when the user presses Enter or Space
	// to assign a genre to the current song.
	StateGenreSelection
)

// genreStep tracks which selection step the genre modal is on.
const (
	stepPrimary   = 0
	stepSecondary = 1
)

// TickMsg is sent by tickCmd every 100ms to trigger a progress bar refresh.
type TickMsg time.Time

// PlayErrMsg is returned by playCmd after attempting to play a file.
// If Err is nil, the play succeeded. If Err is non-nil, the TUI should
// display the error message instead of crashing.
type PlayErrMsg struct {
	Err error
}

// TagWrittenMsg is returned by writeTagsCmd after a tag write attempt.
// If Err is nil, the write succeeded. Primary and Secondary carry the values
// that were written so the model can update its queue.
type TagWrittenMsg struct {
	Primary   string
	Secondary string
	Err       error
}

// SaveStateErrMsg is returned by saveStateCmd if the JSON file could not be
// updated. A non-nil Err is shown in the status bar; it does NOT block queue
// advancement (the tag write already succeeded).
type SaveStateErrMsg struct {
	Err error
}

// genreItem is a single entry in the bubbles/list for genre selection.
// It implements the list.Item interface.
type genreItem struct {
	title string
}

func (g genreItem) Title() string       { return g.title }
func (g genreItem) Description() string { return "" }
func (g genreItem) FilterValue() string { return g.title }

// Model is the root Bubble Tea model for the Song Reviewer TUI.
// It owns all mutable state; Update returns a new copy on each message.
type Model struct {
	// queue holds all tasks and the current position / undo history.
	queue domain.ReviewQueue

	// engine is the audio playback engine. It is shared by reference because
	// it owns an OS audio device handle that must not be duplicated.
	engine *audio.Engine

	// providerRef is a reference to the ManualReviewProvider used at startup.
	// It is stored here so saveStateCmd can call SaveState without capturing
	// a pointer to a changing local variable.
	providerRef provider.ManualReviewProvider

	// progress is the bubbles progress bar component.
	progress progress.Model

	// playbackState is a cached snapshot of the audio engine state, updated
	// on every TickMsg. View() reads from this field instead of calling
	// engine.GetState() directly, keeping View() a pure function.
	playbackState audio.PlaybackState

	// seekDelta is the seek step used for the ← / → keys.
	seekDelta time.Duration

	// state tracks which screen is currently active.
	state AppState

	// genreList is the bubbles/list component used for genre selection.
	// It is rebuilt fresh each time the genre modal opens.
	genreList list.Model

	// genreStep indicates which step of genre selection the user is on:
	// stepPrimary (0) = choosing Primary Genre.
	// stepSecondary (1) = choosing Secondary Genre.
	genreStep int

	// selectedPrimary holds the primary genre chosen in step 1.
	// It is used when writing tags after step 2 completes.
	selectedPrimary string

	// cfg holds the application configuration, needed to rebuild genre lists.
	cfg domain.AppConfig

	// lastPlayErr holds the most recent audio error, shown in the status bar.
	lastPlayErr error

	// lastSaveErr holds the most recent JSON save error, shown in the status bar.
	// It does not block queue advancement.
	lastSaveErr error

	// width and height are the current terminal dimensions.
	width  int
	height int

	// pendingInit holds the initial command batch returned by Init().
	pendingInit tea.Cmd
}

// New constructs the initial Model from the given queue, engine, config, and provider.
// It stores the startup command batch (auto-play + first tick) in pendingInit
// so that Init() can return them inside the Bubble Tea event loop.
func New(queue domain.ReviewQueue, engine *audio.Engine, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	seekSecs := cfg.SeekDeltaSeconds
	if seekSecs <= 0 {
		seekSecs = 30
	}

	m := Model{
		queue:         queue,
		engine:        engine,
		providerRef:   p,
		progress:      prog,
		seekDelta:     time.Duration(seekSecs) * time.Second,
		state:         StateReviewing,
		cfg:           cfg,
		playbackState: engine.GetState(),
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	if len(queue.Tasks) > 0 {
		cmds = append(cmds, playCmd(engine, queue.Tasks[0].FilePath))
	}

	m.pendingInit = tea.Batch(cmds...)
	return m
}

// openGenreModal transitions the model into StateGenreSelection at stepPrimary,
// building a fresh genre list from the config. The list does NOT include [NONE]
// at the primary step.
func (m Model) openGenreModal() Model {
	m.state = StateGenreSelection
	m.genreStep = stepPrimary
	m.selectedPrimary = ""
	m.genreList = makeGenreList(m.cfg.GenreList, false, m.width)
	return m
}

// makeGenreList constructs a bubbles/list model populated with genre items.
// If includeNone is true, a "[NONE]" item is prepended to the list.
// width is used to size the list component; a height of 10 is used as a
// reasonable default for the modal overlay.
func makeGenreList(genres []string, includeNone bool, width int) list.Model {
	items := make([]list.Item, 0, len(genres)+1)
	if includeNone {
		items = append(items, genreItem{title: "[NONE]"})
	}
	for _, g := range genres {
		items = append(items, genreItem{title: g})
	}

	// Use a compact delegate (no descriptions) for a dense list.
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	listWidth := width - 4
	if listWidth < 20 {
		listWidth = 40 // sane minimum for narrow terminals
	}

	l := list.New(items, delegate, listWidth, 14)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return l
}

// tickCmd returns a Bubble Tea command that sends a TickMsg after 100ms.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// playCmd returns a Bubble Tea command that calls engine.Play(path).
func playCmd(engine *audio.Engine, path string) tea.Cmd {
	return func() tea.Msg {
		err := engine.Play(path)
		return PlayErrMsg{Err: err}
	}
}

// writeTagsCmd returns a Bubble Tea command that calls metadata.WriteTags
// on a background goroutine. The result is returned as a TagWrittenMsg.
func writeTagsCmd(path, primary, secondary string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteTags(path, primary, secondary)
		return TagWrittenMsg{Primary: primary, Secondary: secondary, Err: err}
	}
}

// saveStateCmd returns a Bubble Tea command that calls provider.SaveState
// to persist the current queue state to disk. Errors are returned as
// SaveStateErrMsg and shown in the status bar but do not block the UI.
func saveStateCmd(p provider.ManualReviewProvider, tasks []domain.Task) tea.Cmd {
	return func() tea.Msg {
		err := p.SaveState(tasks)
		if err != nil {
			return SaveStateErrMsg{Err: err}
		}
		return SaveStateErrMsg{Err: nil}
	}
}
```

**Expected outcome:** `internal/tui/model.go` compiles cleanly with all new fields and Cmd factories present.

**Verification:**

```
cd mp3-reviewer && go build ./internal/tui/...
```

(This will fail until `update.go` and `view.go` are also updated — that is expected at this step. Full build verification is deferred to Step 8.)

---

### Step 6: Update `internal/tui/update.go`

**Action:**

Replace the full content of `internal/tui/update.go` with the version below. Key changes:

- `StateGenreSelection` key handling now delegates most keys to `m.genreList.Update(msg)` and handles `Enter` as a selection confirmation.
- New handlers for `TagWrittenMsg` and `SaveStateErrMsg`.
- After tag writing succeeds, the queue task is updated and the app auto-advances with `skipToNext()`.

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies the tea.Model interface. It returns the pendingInit command
// batch that was stored by New() — this is the canonical Bubble Tea pattern
// for dispatching startup commands (auto-play + first tick) inside the event loop.
func (m Model) Init() tea.Cmd {
	cmd := m.pendingInit
	m.pendingInit = nil
	return cmd
}

// Update is the central message dispatcher.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 4
		// Resize genre list if it is currently showing.
		if m.state == StateGenreSelection {
			m.genreList.SetWidth(msg.Width - 4)
		}
		return m, nil

	case TickMsg:
		m.playbackState = m.engine.GetState()
		return m, tickCmd()

	case PlayErrMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
		} else {
			m.lastPlayErr = nil
		}
		return m, nil

	case TagWrittenMsg:
		if msg.Err != nil {
			// Tag write failed — show the error but stay on the review screen
			// so the user can retry (e.g., re-open genre modal).
			m.lastPlayErr = msg.Err
			m.state = StateReviewing
			return m, nil
		}

		// Tag write succeeded — update the task in the queue.
		idx := m.queue.CurrentIndex
		if idx >= 0 && idx < len(m.queue.Tasks) {
			m.queue.Tasks[idx].Genre1 = msg.Primary
			m.queue.Tasks[idx].Genre2 = msg.Secondary
		}

		// Persist the updated queue to disk asynchronously.
		saveCmd := saveStateCmd(m.providerRef, m.queue.Tasks)

		// Advance to the next song.
		nextModel, nextCmd := m.skipToNext()
		return nextModel, tea.Batch(saveCmd, nextCmd)

	case SaveStateErrMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
		} else {
			m.lastSaveErr = nil
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// In StateGenreSelection, pass unhandled messages to the list component.
	if m.state == StateGenreSelection {
		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	return m, nil
}

// handleKey dispatches keyboard input based on current AppState.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Genre Selection state ─────────────────────────────────────────────────
	if m.state == StateGenreSelection {
		switch msg.String() {
		case "ctrl+c":
			m.engine.Close()
			return m, tea.Quit

		case "esc":
			// Cancel genre selection and return to the review screen without skipping.
			m.state = StateReviewing
			return m, nil

		case "enter":
			// Confirm the currently highlighted genre item.
			return m.confirmGenreSelection()
		}

		// Delegate all other keys (arrow keys, filter typing, etc.) to the list.
		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	// ── StateReviewing — full keybinding set ──────────────────────────────────
	switch msg.String() {

	case "ctrl+c":
		m.engine.Close()
		return m, tea.Quit

	case "left":
		_ = m.engine.Seek(-m.seekDelta)
		return m, nil

	case "right":
		_ = m.engine.Seek(m.seekDelta)
		return m, nil

	case "p":
		m.engine.TogglePause()
		return m, nil

	case "enter", " ":
		// Open the genre modal for Primary Genre selection.
		m = m.openGenreModal()
		return m, nil

	case "esc":
		return m.skipToNext()

	case "ctrl+u":
		return m.undoLast()
	}

	return m, nil
}

// confirmGenreSelection handles the user pressing Enter on a genre list item.
// On stepPrimary it records the selection and transitions to stepSecondary.
// On stepSecondary it fires writeTagsCmd with both selections.
func (m Model) confirmGenreSelection() (tea.Model, tea.Cmd) {
	selected, ok := m.genreList.SelectedItem().(genreItem)
	if !ok {
		// No item highlighted — ignore.
		return m, nil
	}

	if m.genreStep == stepPrimary {
		// Record primary and transition to secondary step.
		m.selectedPrimary = selected.title
		m.genreStep = stepSecondary
		m.genreList = makeGenreList(m.cfg.GenreList, true, m.width)
		return m, nil
	}

	// stepSecondary — we have both selections; fire the tag write.
	secondary := selected.title
	if secondary == "[NONE]" {
		secondary = ""
	}

	// Close the modal immediately for snappy UX.
	m.state = StateReviewing

	if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
		return m, nil
	}
	path := m.queue.Tasks[m.queue.CurrentIndex].FilePath

	return m, writeTagsCmd(path, m.selectedPrimary, secondary)
}

// skipToNext advances the queue to the next task, starts playing it, and
// returns the updated model and a playCmd. If no next task exists (end of
// queue), it is a no-op.
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
	nextIndex := m.queue.CurrentIndex + 1
	if nextIndex >= len(m.queue.Tasks) {
		return m, nil
	}

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
		return m, nil
	}

	lastIdx := len(m.queue.History) - 1
	m.queue.History = m.queue.History[:lastIdx]

	if m.queue.CurrentIndex > 0 {
		m.queue.CurrentIndex--
	}

	m.lastPlayErr = nil
	return m, playCmd(m.engine, m.queue.Tasks[m.queue.CurrentIndex].FilePath)
}
```

**Expected outcome:** `internal/tui/update.go` compiles. The genre selection flow is fully wired. Tag writing and JSON persistence are issued as non-blocking `tea.Cmd` calls.

**Verification:** Deferred to Step 8.

---

### Step 7: Update `internal/tui/view.go`

**Action:**

Replace the full content of `internal/tui/view.go` with the version below. Key changes:

- Replace the `StateGenreSelection` placeholder with a real two-panel modal that shows the `bubbles/list` component.
- Add a `styleModal`, `styleModalTitle`, and `styleModalFooter` style variable.
- Display the correct heading ("Select Primary Genre" vs "Select Secondary Genre") based on `m.genreStep`.
- Show `lastSaveErr` in the status bar alongside `lastPlayErr`.

```go
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2)

	styleArtist = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8DADC")).
			Bold(true)

	styleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	styleHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	styleHintKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0")).
			Bold(true)

	// styleModal wraps the genre selection overlay box.
	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#A8DADC")).
			Padding(1, 2)

	// styleModalTitle is the heading line inside the genre selection modal.
	styleModalTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				MarginBottom(1)

	// styleModalFooter shows hints inside the modal (Esc to cancel, Enter to confirm).
	styleModalFooter = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				MarginTop(1)
)

// View renders the full TUI frame. It is a pure function of the model — no side
// effects, no calls to engine.GetState().
func (m Model) View() string {
	if m.state == StateGenreSelection {
		return m.viewGenreModal()
	}
	return m.viewReviewing()
}

// viewReviewing renders the main playback screen.
func (m Model) viewReviewing() string {
	// ── Header ────────────────────────────────────────────────────────────────
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
	progressBar := "  " + m.progress.ViewAs(m.playbackState.Progress)

	// ── Status bar ────────────────────────────────────────────────────────────
	total := len(m.queue.Tasks)
	pending := total - m.queue.CurrentIndex
	if total == 0 {
		pending = 0
	}
	queueStr := fmt.Sprintf("Pending: %d / %d", pending, total)
	posStr := m.playbackState.Position

	var errStr string
	if m.lastPlayErr != nil {
		errStr = "  " + styleError.Render("Error: "+m.lastPlayErr.Error())
	} else if m.lastSaveErr != nil {
		errStr = "  " + styleError.Render("Save error: "+m.lastSaveErr.Error())
	}

	seekSecs := int(m.seekDelta.Seconds())
	seekLabel := fmt.Sprintf("seek %ds", seekSecs)
	hints := hintStr("← →", seekLabel) +
		"   " + hintStr("p", "pause") +
		"   " + hintStr("Enter", "tag") +
		"   " + hintStr("Esc", "skip") +
		"   " + hintStr("Ctrl+U", "undo") +
		"   " + hintStr("Ctrl+C", "quit")

	statusLine := styleStatus.Render(
		queueStr + "   " + posStr + errStr + "\n  " + hints,
	)

	return "\n" + headerLine + "\n\n" + progressBar + "\n\n" + statusLine + "\n"
}

// viewGenreModal renders the two-step genre selection overlay.
func (m Model) viewGenreModal() string {
	var heading string
	if m.genreStep == stepPrimary {
		heading = "Step 1 of 2 — Select Primary Genre"
	} else {
		heading = fmt.Sprintf("Step 2 of 2 — Select Secondary Genre  (primary: %s)", m.selectedPrimary)
	}

	footer := styleModalFooter.Render(
		hintStr("↑ ↓", "navigate") +
			"   " + hintStr("Enter", "confirm") +
			"   " + hintStr("Esc", "cancel"),
	)

	inner := styleModalTitle.Render(heading) + "\n" +
		m.genreList.View() + "\n" +
		footer

	return "\n" + styleModal.Render(inner) + "\n"
}

// hintStr formats a single keybinding hint as "Key  description".
func hintStr(key, description string) string {
	return styleHintKey.Render(key) + styleHint.Render("  "+description)
}
```

**Expected outcome:** `internal/tui/view.go` compiles. The genre modal renders the `bubbles/list` component with a step heading and footer hints. The main review screen shows `lastSaveErr` in the error slot.

**Verification:** Deferred to Step 8.

---

### Step 8: Update `cmd/reviewer/main.go` to Pass Provider to `tui.New`

**Action:**

`tui.New(...)` now requires a `provider.ManualReviewProvider` as a fourth argument (added in Step 5). Update `cmd/reviewer/main.go` to pass `p` (the already-constructed provider) to `tui.New`.

Find the line:

```go
model := tui.New(queue, engine, cfg)
```

Replace it with:

```go
model := tui.New(queue, engine, cfg, p)
```

No other changes to `main.go` are needed.

**Expected outcome:** `cmd/reviewer/main.go` compiles cleanly.

**Verification:**

```
cd mp3-reviewer && go build ./...
```

Should produce zero errors.

---

### Step 9: Full Build and Test Verification

**Action:**

Run the complete build and test suite:

```
cd mp3-reviewer && go build ./...
cd mp3-reviewer && go test ./...
```

If any test fails, fix it before proceeding. Common failure causes at this stage:

- The `json` import missing from the test file after appending the new tests.
- The `bogem/id3v2` module not yet downloaded — run `go mod tidy` if needed.

Also run `go vet ./...` to catch any static analysis issues:

```
cd mp3-reviewer && go vet ./...
```

**Expected outcome:** All packages compile. All tests pass. `go vet` reports zero issues.

**Verification:**

```
cd mp3-reviewer && go build ./... && go test ./... && go vet ./...
```

All three commands exit with code 0.

---

### Step 10: Update `agent-specs/architecture-breakdown.md`

**Action:**

Open `agent-specs/architecture-breakdown.md` and make the following targeted additions:

1. Under the `## Folder Structure` section, update the `/internal/metadata` line from:

   > `/internal/metadata`: ID3 tag read/write logic using pure Go libraries.

   To:

   > `/internal/metadata`: ID3 tag write logic. Exposes `WriteTags(path, primary, secondary string) error`, which opens the MP3 file and writes the `TCON` (genre) ID3v2 frame. Genre is stored as `"Primary"` or `"Primary/Secondary"` (forward-slash separator). Uses `github.com/bogem/id3v2/v2`.

2. Under `## Folder Structure`, update the `/internal/provider` line from:

   > `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`, which parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`.

   To:

   > `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`. `GetTasks()` parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`. `SaveState(tasks []domain.Task) error` writes genre assignments back into the source JSON atomically (temp file + rename), setting `status="applied"`, `primary_genre`, and `secondary_genre` on matched entries.

3. Under `## Folder Structure`, update the `/internal/tui` description to mention the genre modal:

   Append to the existing TUI line:
   > `genre_selection.go` is **not** used; genre selection is implemented inline in `model.go`/`update.go`/`view.go`. The modal is a two-step `bubbles/list` overlay: Step 1 = Primary Genre (no [NONE] option), Step 2 = Secondary Genre ([NONE] prepended). Confirmation fires `writeTagsCmd` then `saveStateCmd`; both run as non-blocking `tea.Cmd`s.

4. Under `## Technology Stack`, update the metadata row:

   | Metadata (ID3) | `bogem/id3v2` (pure Go, write support) |

**Expected outcome:** `agent-specs/architecture-breakdown.md` reflects the tagging strategy, the updated provider capability, and the corrected technology stack entry.

**Verification:**

```
grep -n "bogem\|SaveState\|TCON\|Primary/Secondary" agent-specs/architecture-breakdown.md
```

Should print at least 4 matching lines.

---

### Step 11: Update `README.md`

**Action:**

Open `README.md` and make the following targeted changes:

1. In the **Technology Stack** table, update the Metadata row:

   | Metadata (ID3) | `bogem/id3v2` |

2. In the **Review JSON Format** section, add a row to the field reference table for `primary_genre` and `secondary_genre`:

   | `primary_genre` | Primary genre written by the app after tagging. |
   | `secondary_genre` | Secondary genre written by the app. Empty string if [NONE] was selected. |

3. In the **Keybindings** table, verify (and correct if wrong) that the `Enter / Space` row reads:

   | `Enter` / `Space` | Open genre selection menu (two-step: Primary then Secondary) |

4. Add a new **Genre Tagging** section after the Keybindings section:

   ```
   ## Genre Tagging

   Pressing `Enter` or `Space` opens a two-step genre selection modal:

   1. **Step 1 — Primary Genre:** Scroll or type to filter. Press `Enter` to confirm.
   2. **Step 2 — Secondary Genre:** A `[NONE]` option is available at the top. Press `Enter` to confirm.

   After both steps, the app:
   - Writes the genre to the MP3's `TCON` ID3v2 frame as `"Primary"` or `"Primary/Secondary"`.
   - Updates the source JSON (`data/manual_review.json`) to set `status: "applied"` and records both genre fields.
   - Automatically advances to the next song in the queue.

   Press `Esc` at any point during genre selection to cancel and return to the review screen without making any changes.
   ```

**Expected outcome:** `README.md` reflects the new tagging workflow, the corrected technology stack, and the updated JSON format.

**Verification:**

```
grep -n "bogem\|primary_genre\|two-step\|TCON\|Status.*applied\|Genre Tagging" README.md
```

Should print at least 5 matching lines.

---

## Open Questions & Decisions

### Q1: ID3 Tag Writing Library — `dhowden/tag` Cannot Write Tags

**Context:** The task request specifies using `dhowden/tag` for `WriteTags`. However, `dhowden/tag` is a **read-only** library — it provides no API for writing or modifying ID3 tags. If `dhowden/tag` is used, the `WriteTags` function cannot be implemented. A different pure-Go library must be chosen. The leading options are:

- `github.com/bogem/id3v2/v2` — a well-maintained pure-Go ID3v2 read/write library. Supports ID3v2.3 and v2.4, including `TCON` (genre), `TIT2` (title), `TPE1` (artist), and all standard text frames. Has no CGO dependencies. This is the most common choice for Go ID3 writing.
- Shell out to the `id3v2` CLI tool via `os/exec` — avoids adding a Go dependency but requires the tool to be installed on the host OS, introduces platform fragility, and is harder to test. This violates the spirit of "pure Go" from the architecture spec.
- Use `dhowden/tag` for reading and find a way to write raw ID3 frames manually using `encoding/binary` — extremely fragile, not recommended.

**Options:**
- **A)** Use `github.com/bogem/id3v2/v2` as the sole metadata library (read + write). Clean, idiomatic, pure Go. This plan is written assuming this option.
- **B)** Keep `dhowden/tag` listed in `go.mod` (for potential future read use) and add `github.com/bogem/id3v2/v2` for write. Functionally identical to A but with an extra unused dependency. No practical benefit.
- **C)** Shell out to the `id3v2` CLI tool. Requires the tool installed; not pure Go; harder to test. Not recommended.

**Agent's recommendation:** **Option A** — use `github.com/bogem/id3v2/v2` exclusively. It is pure Go, actively maintained, covers all the ID3v2 frames needed, and avoids the phantom `dhowden/tag` dependency. The architecture spec says "pure Go libraries for audio/tags" which `bogem/id3v2` satisfies fully. `dhowden/tag` was likely specified by mistake (it is well-known as a read-only parser).

**Human decision:** Let's use option A, it's the best choice to manage metadata and tags. We can change the dependency in the `go.mod` file.

---

### Q2: Genre String Format Written to the ID3 `TCON` Frame

**Context:** Standard ID3v2 players (VLC, MusicBee, Rekordbox) only recognise a single `TCON` (Content Type) text frame per file. The app assigns two tiers of genre. The task says: "If so, concatenate as `Primary/Secondary` or use the specific ID3v2 field if supported." There is no standard ID3v2 multi-genre frame beyond the single `TCON`. Options:

- **A)** Single `TCON` frame: `"Primary/Secondary"` (forward-slash separator). Simple, human-readable, widely compatible. Splitting on `/` in downstream tooling is trivial. This is the format used in this plan's Step 4 default implementation.
- **B)** Single `TCON` frame: `"Primary; Secondary"` (semicolon separator). Some players interpret semicolons as genre separators (e.g., Rekordbox). Slightly more compatible with multi-genre-aware players.
- **C)** Two separate `TCON` frames. ID3v2.4 technically allows multiple frames with the same ID, but most players only read the first one. This would make the secondary genre invisible in most players.
- **D)** Write primary to `TCON` and secondary to a custom `TXXX` (user-defined text) frame with description `"Secondary Genre"`. Keeps standard player compatibility for primary, and preserves secondary for tools that read `TXXX`. More complex to implement.

**Agent's recommendation:** **Option A** (`"Primary/Secondary"`) — it is the simplest, most explicit, and easy to parse programmatically. The forward-slash is already hinted at in the task spec and is broadly understood. Option D is appealing for long-term data fidelity but adds complexity outside the task scope.

**Human decision:** Let's use option C and D: let's append both genres to the `TCON` and also create a custom `TGENRE2` frame with the secondary genre.

---

### Q3: Behaviour When the End of the Queue is Reached After Tagging

**Context:** After the last song in the queue is tagged, `skipToNext()` is called but `nextIndex >= len(m.queue.Tasks)` so it returns a no-op. The TUI would remain on the last (already-tagged) song with no indication that the queue is complete. Options:

- **A)** Display a "Queue complete — all songs tagged!" message in the header area and stop playback. Requires a new `StateQueueComplete` app state.
- **B)** Remain on the last song's review screen silently; the user can `Ctrl+C` to exit. Minimal change.
- **C)** Automatically quit the app with `tea.Quit` after the last song is tagged. Fast exit but potentially disorienting.

**Agent's recommendation:** **Option A** — a clear "Queue complete" state is good UX and aligns with the application overview's emphasis on a polished experience. However, since this is a new state (`StateQueueComplete`) and the task spec does not mention it, it is flagged as a question. **Option B** is an acceptable fallback if the human prefers minimal scope for Task 4.

**Human decision:** Let's follow option B, minimal change, the message that ctrl-c to exit should always be visible anyways.

---

### Q4: Genre List Height in the Modal Overlay

**Context:** The `makeGenreList` function in Step 5 uses a hardcoded height of `14` rows for the `bubbles/list`. The actual number of genres configured by the user varies (the example has 10; real deployments could have 50+). Options:

- **A)** Hardcode a height of `14` rows. Simple, consistent. Works fine for typical genre lists (10–20 entries). `bubbles/list` auto-enables scrolling when the list exceeds the height.
- **B)** Derive height from `m.height` (terminal height), leaving margin for the modal border and the main screen behind it. More adaptive but requires passing height into `makeGenreList` and recalculating on resize.
- **C)** Use `m.height - 8` as the list height (leaves 8 rows for borders, heading, footer). A reasonable heuristic that scales with the terminal.

**Agent's recommendation:** **Option C** — using `m.height - 8` (with a minimum floor of 8) is more robust than a hardcoded constant and doesn't require significant additional complexity. The plan's Step 5 uses a constant `14` as a safe default; the implementing agent should apply `m.height - 8` (min 8) if terminal height is available, or stick with `14` if not.

**Human decision:** Let's use option A, since we want to keep things simple, also the modal will have a fuzzy finder that will shorten the list height.

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/provider/json_provider.go` | Modified | Added `Status`, `PrimaryGenre`, `SecondaryGenre` to `reviewEntry`; added `SaveState(tasks []domain.Task) error` method to `ManualReviewProvider` with atomic write via temp file + rename |
| 2 | `internal/provider/json_provider_test.go` | Modified | Appended `TestSaveState_UpdatesAppliedEntries` and `TestSaveState_NoneSecondaryGenre` test functions |
| 3 | `internal/metadata/writer.go` | Modified | Replaced stub with full `WriteTags(path, primary, secondary string) error` implementation using `bogem/id3v2/v2` |
| 4 | `internal/tui/model.go` | Modified | Added `genreList`, `genreStep`, `selectedPrimary`, `providerRef`, `cfg`, `lastSaveErr` fields; added `TagWrittenMsg`, `SaveStateErrMsg`, `genreItem` types; added `writeTagsCmd`, `saveStateCmd`, `makeGenreList`, `openGenreModal` helpers; updated `New(...)` signature to accept `provider.ManualReviewProvider` |
| 5 | `internal/tui/update.go` | Modified | Wired `StateGenreSelection` key handling with `bubbles/list` delegation; added `confirmGenreSelection()`; added handlers for `TagWrittenMsg` and `SaveStateErrMsg`; retained `skipToNext()`, `undoLast()` |
| 6 | `internal/tui/view.go` | Modified | Replaced genre modal placeholder with real `viewGenreModal()` rendering `bubbles/list`; added `styleModal`, `styleModalTitle`, `styleModalFooter` styles; extracted `viewReviewing()` helper; added `lastSaveErr` display in status bar |
| 7 | `cmd/reviewer/main.go` | Modified | Updated single `tui.New(...)` call to pass `p` (the `ManualReviewProvider`) as the fourth argument |
| 8 | `go.mod` | Modified | Added `github.com/bogem/id3v2/v2` dependency (after `go get`) |
| 9 | `go.sum` | Modified | Updated automatically by `go get` |
| 10 | `agent-specs/architecture-breakdown.md` | Modified | Updated `/internal/metadata`, `/internal/provider`, `/internal/tui` descriptions; updated Technology Stack metadata row |
| 11 | `README.md` | Modified | Updated Technology Stack table; added `primary_genre`/`secondary_genre` to JSON format table; added **Genre Tagging** section; updated `Enter/Space` keybinding description |

**Total files created: 0 | Total files modified: 11**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes — all existing tests plus the two new `SaveState` tests
- [ ] `go vet ./...` reports zero issues
- [ ] `internal/metadata/writer.go` exports `WriteTags(path, primary, secondary string) error` and nothing else
- [ ] `internal/provider/json_provider.go` has `SaveState` on `ManualReviewProvider`; writes atomically via temp file
- [ ] `internal/tui/model.go` compiles; `tui.New` accepts four arguments (queue, engine, cfg, provider)
- [ ] `internal/tui/update.go`: pressing `Enter`/`Space` on `StateReviewing` opens genre modal; `Esc` in modal returns to `StateReviewing` without skipping; confirming secondary genre fires `writeTagsCmd` then `saveStateCmd` then `skipToNext`
- [ ] `internal/tui/view.go`: `viewGenreModal()` renders a `bubbles/list` with step heading; `viewReviewing()` shows `lastSaveErr` in status bar
- [ ] `cmd/reviewer/main.go` compiles; `tui.New` call passes `p` as fourth argument
- [ ] After a successful tag+save cycle, the JSON file on disk has `status: "applied"`, `primary_genre`, and `secondary_genre` populated for that entry
- [ ] `[NONE]` secondary selection results in an empty `secondary_genre` in JSON and `TCON` frame contains only the primary genre (no trailing slash)
- [ ] No unrelated files were modified or deleted
- [ ] `agent-specs/architecture-breakdown.md` updated with tagging strategy, `SaveState` description, and corrected metadata library
- [ ] `README.md` updated with Genre Tagging section, corrected technology stack, and JSON format additions

---

## Notes for the Implementing Agent

1. **`dhowden/tag` cannot write tags.** The task spec mentions it, but it is a read-only parser. Do not attempt to use it for `WriteTags`. Use `github.com/bogem/id3v2/v2` (pending Q1 human decision). The `go get` command in Step 1 must be run before implementing Step 4.

2. **`tui.New` signature change is a breaking change.** `cmd/reviewer/main.go` must be updated in Step 8 before the project will build. Do not run `go build ./...` between Steps 5 and 8 expecting success — build verification is intentionally deferred to Step 8.

3. **`bubbles/list` filter mode.** When the user types characters in `StateGenreSelection`, those keystrokes are forwarded to `m.genreList.Update(msg)` for filtering. The `Enter` key is intercepted **before** delegation to the list so that confirmation always triggers `confirmGenreSelection()`. Be careful: if the list is in active-filter mode, `Enter` in `bubbles/list` normally commits the filter, not the selection. The current plan intercepts `Enter` at the `handleKey` level so `confirmGenreSelection()` always fires — this is intentional and correct.

4. **`[NONE]` is represented as an empty string internally.** The `genreItem{title: "[NONE]"}` is only for display. `confirmGenreSelection()` must convert `"[NONE]"` → `""` before passing to `writeTagsCmd`. The `secondary` parameter of `WriteTags` accepts `""` to mean "no secondary genre."

5. **Atomic JSON write.** `SaveState` uses `os.CreateTemp` + `os.Rename` to avoid corrupting the JSON file if the process is killed during write. This is important because the JSON file is also read by `GetTasks` on startup.

6. **`SaveStateErrMsg` is non-fatal.** If the JSON persistence fails (e.g., disk full), the ID3 tags have already been written successfully. The model shows the error in the status bar but still advances the queue. Do not block queue advancement on a save failure.

7. **Model is a value type.** All methods (`skipToNext`, `undoLast`, `confirmGenreSelection`, `openGenreModal`) must operate on a copy of `m` and return the modified copy. Never use pointer receivers on `Model`.

8. **`bubbles/list` import path.** The correct import is `"github.com/charmbracelet/bubbles/list"` — this package is already transitively available since `charmbracelet/bubbles` is in `go.mod`. No extra `go get` is needed for the list component itself.

9. **Genre list height.** The plan Step 5 hardcodes height `14` in `makeGenreList`. If the human resolves Q4 as Option C, use `m.height - 8` (with a minimum of 8) instead. This requires passing the terminal height into `makeGenreList` as a second int parameter and updating all call sites (`openGenreModal`, the `stepSecondary` transition in `confirmGenreSelection`, and the `WindowSizeMsg` handler).

10. **The `TaskProvider` interface** in `json_provider.go` does not need a `SaveState` method added to it. `SaveState` is a concrete method on `ManualReviewProvider` only — it is not part of the abstraction boundary. The `tui` package stores `provider.ManualReviewProvider` (concrete type) not the `TaskProvider` interface, because only the concrete type has `SaveState`.

# Implementation Plan: Task 5 — External API & Advanced Metadata Enrichment

## Overview

This plan implements the metadata enrichment layer of the Song Reviewer CLI. When a song loads, the app immediately fires two non-blocking background `tea.Cmd` calls: one to the MusicBrainz API (to fetch the original release year) and one implementing a Tap Tempo feature (to let the user tap a key to the beat and calculate BPM — the recommended fallback over unreliable/paid external BPM APIs). Results are cached on the `Model` and displayed in a new metadata panel on the main review screen using `lipgloss` styling. Loading state is communicated via a `bubbles/spinner`. Two new commit keys are added: `Ctrl+1` writes the fetched BPM to the `TBPM` ID3v2 frame, and `Ctrl+2` writes the fetched year to the `TDRC`/`TYER` ID3v2 frame. Once committed, the tag value turns green in the UI.

This task touches six areas:

1. **`internal/api/musicbrainz.go`** — Implements `FetchYear(artist, title, userAgent string) (string, error)` using the MusicBrainz JSON search + release-group lookup API.
2. **`internal/metadata/writer.go`** — Adds `WriteBPM(path, bpm string) error` and `WriteYear(path, year string) error`.
3. **`internal/tui/model.go`** — Adds enrichment state fields (`enrichYear`, `enrichBPM`, `tapTempoTaps`, `spinner`), new message types (`YearFetchedMsg`, `TapTempoMsg`), and new Cmd factories (`fetchYearCmd`, `spinnerTickCmd`).
4. **`internal/tui/update.go`** — Wires `Ctrl+1` / `Ctrl+2` commit keys, tap-tempo key (`t`), auto-fetch trigger on song load, message handlers for `YearFetchedMsg`, `spinner.TickMsg`, and enrichment write messages.
5. **`internal/tui/view.go`** — Adds a metadata enrichment panel below the progress bar showing Year and BPM with colour-coded loading/found/committed states.
6. **`agent-development/agent-specs/architecture-breakdown.md`** and **`README.md`** — Documentation updates.

After this task, the complete enrichment flow works end-to-end: load → fetch → display → commit → write ID3.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand the enrichment workflow goals |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, MVU rules, no-I/O-in-View |
| Diagrams | `diagrams/data-structures.mmd`, `diagrams/software-architecture.mmd`, `diagrams/ui-state-machine.mmd`, `diagrams/task-lifecycle.mmd`, `diagrams/component-data-flow.mmd` | Visual reference for architecture, data model, UI states, task lifecycle, and component data flow |
| Folder Structure | `diagrams/FOLDER-STRUCTURE.md` | Quick-reference project directory tree and package dependency graph |
| Task Definition | `agent-development/pending/5-api-integration-and-enrichment.md` | The task being implemented |
| Completed Plan 4 | `agent-development/done/plans/4-genre-selection-and-tagging-plan.md` | TUI shape and message conventions established in prior task |
| Domain Models | `internal/domain/models.go` | `Task`, `AppConfig` — `Year` and `BPM` fields are already declared |
| TUI Model | `internal/tui/model.go` | Current model fields, Cmd helpers, constructor |
| TUI Update | `internal/tui/update.go` | Current message dispatch, key handling, `skipToNext` |
| TUI View | `internal/tui/view.go` | Current rendering, existing lipgloss styles |
| Metadata Writer | `internal/metadata/writer.go` | Existing `WriteTags` — new functions will be added here |
| API Stub | `internal/api/musicbrainz.go` | Currently only `package api` — to be filled |
| Settings Example | `settings.example.json` | `api_keys.musicbrainz_user_agent` field reference |
| Main Entry Point | `cmd/reviewer/main.go` | No changes expected — included for reference only |

---

## Pre-Conditions

- Tasks 0–4 must be fully complete. Specifically:
  - `go build ./...` and `go test ./...` both pass with zero errors.
  - `internal/tui/model.go` defines `Model` with all fields from Task 4 (genre selection, `providerRef`, `cfg`, `lastSaveErr`, etc.).
  - `internal/tui/update.go` implements `handleKey` with `StateReviewing` and `StateGenreSelection` branches.
  - `internal/metadata/writer.go` exports `WriteTags(path, primary, secondary string) error`.
  - `internal/api/musicbrainz.go` exists but contains only `package api`.
  - `internal/domain/models.go` has `Task.Year string` and `Task.BPM string` fields.
  - `AppConfig.ApiKeys.MusicBrainzUserAgent` is already defined in `domain.AppConfig`.
  - `github.com/bogem/id3v2/v2` is already in `go.mod`.
  - `github.com/charmbracelet/bubbles` v1.0.0 is already in `go.mod` (includes `bubbles/spinner`).
- No additional `go get` commands are required — all necessary libraries are already present.

---

## Step-by-Step Implementation

### Step 1: Implement `internal/api/musicbrainz.go`

**Action:**

Replace the stub content of `internal/api/musicbrainz.go` with the full implementation below.

The strategy:
1. Search the MusicBrainz recording endpoint with `artist` + `title` to get the top recording hit.
2. From the recording hit, extract its linked release groups.
3. Pick the release group with the earliest `first-release-date`.
4. Return just the 4-digit year portion of that date.

The function signature is:

```go
FetchYear(artist, title, userAgent string) (string, error)
```

The full file content:

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const musicBrainzBaseURL = "https://musicbrainz.org/ws/2"

// mbRecordingSearchResponse mirrors the top-level JSON returned by
// GET /ws/2/recording?query=...&fmt=json
type mbRecordingSearchResponse struct {
	Recordings []mbRecording `json:"recordings"`
}

// mbRecording mirrors a single recording entry in the search results.
type mbRecording struct {
	ReleaseGroups []mbReleaseGroupRef `json:"release-groups"`
}

// mbReleaseGroupRef mirrors the release-group object embedded in a recording.
type mbReleaseGroupRef struct {
	ID               string `json:"id"`
	FirstReleaseDate string `json:"first-release-date"`
}

// FetchYear queries the MusicBrainz JSON API to find the earliest original
// release year for the given artist and title combination.
//
// It performs a recording search, inspects the release groups of the top
// result, and returns the 4-digit year extracted from the earliest
// first-release-date found.
//
// userAgent must be a non-empty string in the form "AppName/Version ( email )"
// as required by MusicBrainz rate-limiting policy.
//
// Returns ("", nil) if no matching recording or release year is found.
// Returns ("", err) on network errors or unexpected response formats.
func FetchYear(artist, title, userAgent string) (string, error) {
	if artist == "" && title == "" {
		return "", fmt.Errorf("musicbrainz: FetchYear called with empty artist and title")
	}
	if userAgent == "" {
		return "", fmt.Errorf("musicbrainz: FetchYear requires a non-empty userAgent")
	}

	// Build a Lucene query: artist:"..." AND recording:"..."
	queryParts := []string{}
	if artist != "" {
		queryParts = append(queryParts, fmt.Sprintf(`artist:"%s"`, artist))
	}
	if title != "" {
		queryParts = append(queryParts, fmt.Sprintf(`recording:"%s"`, title))
	}
	query := strings.Join(queryParts, " AND ")

	searchURL := fmt.Sprintf(
		"%s/recording?query=%s&limit=5&fmt=json&inc=release-groups",
		musicBrainzBaseURL,
		url.QueryEscape(query),
	)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("musicbrainz: building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("musicbrainz: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("musicbrainz: unexpected HTTP status %d", resp.StatusCode)
	}

	var result mbRecordingSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("musicbrainz: decoding response: %w", err)
	}

	if len(result.Recordings) == 0 {
		return "", nil
	}

	// Collect all first-release-dates from the release groups of the top result.
	topRecording := result.Recordings[0]
	var dates []string
	for _, rg := range topRecording.ReleaseGroups {
		if rg.FirstReleaseDate != "" {
			dates = append(dates, rg.FirstReleaseDate)
		}
	}

	if len(dates) == 0 {
		return "", nil
	}

	// Sort lexicographically — ISO date strings (YYYY, YYYY-MM, YYYY-MM-DD)
	// sort correctly in lexicographic order when all are the same length or
	// prefixed by year. We extract the year from the earliest.
	sort.Strings(dates)
	earliest := dates[0]

	// Extract the 4-digit year prefix.
	if len(earliest) < 4 {
		return "", nil
	}
	year := earliest[:4]

	return year, nil
}
```

**Expected outcome:** `internal/api/musicbrainz.go` compiles cleanly and exports `FetchYear`.

**Verification:**

```
cd mp3-reviewer && go build ./internal/api/...
```

---

### Step 2: Add `WriteBPM` and `WriteYear` to `internal/metadata/writer.go`

**Action:**

Append the two new exported functions below to the **end** of `internal/metadata/writer.go`. Do NOT remove or modify the existing `WriteTags` function.

```go
// WriteBPM opens the MP3 file at path and writes the BPM value to the TBPM
// (Beats Per Minute) ID3v2 text frame. bpm must be a non-empty string
// containing a numeric value (e.g. "128" or "120").
//
// Any existing TBPM frame is replaced. Returns a wrapped error on failure.
func WriteBPM(path string, bpm string) error {
	if bpm == "" {
		return fmt.Errorf("metadata: WriteBPM called with empty bpm")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for BPM write: %w", path, err)
	}
	defer tag.Close()

	// Replace any existing TBPM frame.
	tag.DeleteFrames(tag.CommonID("BPM"))
	tag.AddTextFrame(tag.CommonID("BPM"), tag.DefaultEncoding(), bpm)

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving BPM to %q: %w", path, err)
	}

	return nil
}

// WriteYear opens the MP3 file at path and writes the release year to the
// appropriate ID3v2 frame. For ID3v2.3 tags this is TYER; for ID3v2.4 it is
// TDRC. bogem/id3v2 maps the CommonID "Year" to the correct frame for the
// tag version, so we use tag.SetYear() which handles both versions.
//
// year must be a 4-digit string (e.g. "1971"). Returns a wrapped error on
// failure.
func WriteYear(path string, year string) error {
	if year == "" {
		return fmt.Errorf("metadata: WriteYear called with empty year")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for year write: %w", path, err)
	}
	defer tag.Close()

	// SetYear uses CommonID("Year") which maps to TYER (v2.3) or TDRC (v2.4)
	// depending on the tag version already present in the file.
	tag.SetYear(year)

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving year to %q: %w", path, err)
	}

	return nil
}
```

**Expected outcome:** `internal/metadata/writer.go` now exports three functions: `WriteTags`, `WriteBPM`, and `WriteYear`. The file compiles cleanly.

**Verification:**

```
cd mp3-reviewer && go build ./internal/metadata/...
```

---

### Step 3: Extend `internal/tui/model.go`

**Action:**

Replace the full content of `internal/tui/model.go` with the version below. Key additions over the Task 4 version:

- New import: `"github.com/charmbracelet/bubbles/spinner"`.
- New import: `"song-reviewer/internal/api"` (used by `fetchYearCmd`).
- Three new enrichment status constants: `enrichIdle`, `enrichLoading`, `enrichFound`, `enrichCommitted`.
- Three new message types: `YearFetchedMsg`, `BPMWrittenMsg`, `YearWrittenMsg`.
- New enrichment fields on `Model`: `enrichYearStatus`, `enrichYearValue`, `enrichBPMStatus`, `enrichBPMValue`, `tapTimes`, `spinner`.
- New Cmd factories: `fetchYearCmd`, `writeBPMCmd`, `writeYearCmd`.
- Updated `New(...)` — starts the spinner and fires `fetchYearCmd` for the first song.
- New helper `resetEnrichment()` — clears all enrichment state; called on song advance.

The full updated file content:

```go
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/api"
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

// enrichStatus represents the state of an enrichment field (Year or BPM).
type enrichStatus int

const (
	// enrichIdle means no fetch has been attempted yet for this field.
	enrichIdle enrichStatus = iota

	// enrichLoading means a background fetch is in progress.
	enrichLoading

	// enrichFound means a value was successfully fetched and is ready to commit.
	enrichFound

	// enrichCommitted means the value has been written to the ID3 tag.
	enrichCommitted

	// enrichError means the fetch failed or write failed.
	enrichError
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

// YearFetchedMsg is returned by fetchYearCmd after a MusicBrainz lookup.
// Year will be a 4-digit string on success, or empty string if not found.
// Err is non-nil on network or parse failure.
type YearFetchedMsg struct {
	Year string
	Err  error
}

// BPMWrittenMsg is returned by writeBPMCmd after writing the TBPM ID3 frame.
// BPM carries the value that was written so the model can update the task.
// Err is non-nil on write failure.
type BPMWrittenMsg struct {
	BPM string
	Err error
}

// YearWrittenMsg is returned by writeYearCmd after writing the year ID3 frame.
// Year carries the value that was written so the model can update the task.
// Err is non-nil on write failure.
type YearWrittenMsg struct {
	Year string
	Err  error
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
	genreList list.Model

	// genreStep indicates which step of genre selection the user is on.
	genreStep int

	// selectedPrimary holds the primary genre chosen in step 1.
	selectedPrimary string

	// cfg holds the application configuration.
	cfg domain.AppConfig

	// lastPlayErr holds the most recent audio error, shown in the status bar.
	lastPlayErr error

	// lastSaveErr holds the most recent JSON save error, shown in the status bar.
	lastSaveErr error

	// ── Enrichment fields ─────────────────────────────────────────────────────

	// enrichYearStatus tracks the fetch/commit state for the Year field.
	enrichYearStatus enrichStatus

	// enrichYearValue holds the year string fetched from MusicBrainz.
	// Empty until a successful fetch.
	enrichYearValue string

	// enrichBPMStatus tracks the tap-tempo calculation and commit state.
	enrichBPMStatus enrichStatus

	// enrichBPMValue holds the BPM string derived from tap tempo.
	// Empty until the user has tapped enough beats.
	enrichBPMValue string

	// tapTimes holds the timestamps of each tap-tempo keypress.
	// At least 3 taps are required to calculate a stable BPM average.
	tapTimes []time.Time

	// spinner is the bubbles spinner component shown while data is loading.
	spinner spinner.Model

	// ── Layout fields ─────────────────────────────────────────────────────────

	// width and height are the current terminal dimensions.
	width  int
	height int

	// pendingInit holds the initial command batch returned by Init().
	pendingInit tea.Cmd
}

// New constructs the initial Model from the given queue, engine, config, and provider.
// It stores the startup command batch (auto-play + first tick + spinner tick +
// initial year fetch) in pendingInit so that Init() can return them inside the
// Bubble Tea event loop.
func New(queue domain.ReviewQueue, engine *audio.Engine, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	seekSecs := cfg.SeekDeltaSeconds
	if seekSecs <= 0 {
		seekSecs = 30
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	m := Model{
		queue:         queue,
		engine:        engine,
		providerRef:   p,
		progress:      prog,
		seekDelta:     time.Duration(seekSecs) * time.Second,
		state:         StateReviewing,
		cfg:           cfg,
		playbackState: engine.GetState(),
		spinner:       spin,
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	cmds = append(cmds, m.spinner.Tick)

	if len(queue.Tasks) > 0 {
		task := queue.Tasks[0]
		cmds = append(cmds, playCmd(engine, task.FilePath))
		// Fire enrichment fetch immediately for the first song.
		if task.Artist != "" || task.Title != "" {
			m.enrichYearStatus = enrichLoading
			cmds = append(cmds, fetchYearCmd(task.Artist, task.Title, cfg.ApiKeys.MusicBrainzUserAgent))
		}
	}

	m.pendingInit = tea.Batch(cmds...)
	return m
}

// resetEnrichment clears all enrichment state for the current song.
// It is called whenever the queue advances to a new song.
func (m Model) resetEnrichment() Model {
	m.enrichYearStatus = enrichIdle
	m.enrichYearValue = ""
	m.enrichBPMStatus = enrichIdle
	m.enrichBPMValue = ""
	m.tapTimes = nil
	return m
}

// openGenreModal transitions the model into StateGenreSelection at stepPrimary.
func (m Model) openGenreModal() Model {
	m.state = StateGenreSelection
	m.genreStep = stepPrimary
	m.selectedPrimary = ""
	m.genreList = makeGenreList(m.cfg.GenreList, false, m.width)
	return m
}

// makeGenreList constructs a bubbles/list model populated with genre items.
// If includeNone is true, a "[NONE]" item is prepended to the list.
// Height is fixed at 14 rows (Option A from Task 4 plan).
func makeGenreList(genres []string, includeNone bool, width int) list.Model {
	items := make([]list.Item, 0, len(genres)+1)
	if includeNone {
		items = append(items, genreItem{title: "[NONE]"})
	}
	for _, g := range genres {
		items = append(items, genreItem{title: g})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	listWidth := width - 4
	if listWidth < 20 {
		listWidth = 40
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

// writeTagsCmd returns a Bubble Tea command that calls metadata.WriteTags.
func writeTagsCmd(path, primary, secondary string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteTags(path, primary, secondary)
		return TagWrittenMsg{Primary: primary, Secondary: secondary, Err: err}
	}
}

// saveStateCmd returns a Bubble Tea command that calls provider.SaveState.
func saveStateCmd(p provider.ManualReviewProvider, tasks []domain.Task) tea.Cmd {
	return func() tea.Msg {
		err := p.SaveState(tasks)
		return SaveStateErrMsg{Err: err}
	}
}

// fetchYearCmd returns a Bubble Tea command that calls api.FetchYear on a
// background goroutine and returns the result as a YearFetchedMsg.
func fetchYearCmd(artist, title, userAgent string) tea.Cmd {
	return func() tea.Msg {
		year, err := api.FetchYear(artist, title, userAgent)
		return YearFetchedMsg{Year: year, Err: err}
	}
}

// writeBPMCmd returns a Bubble Tea command that calls metadata.WriteBPM on a
// background goroutine and returns the result as a BPMWrittenMsg.
func writeBPMCmd(path, bpm string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteBPM(path, bpm)
		return BPMWrittenMsg{BPM: bpm, Err: err}
	}
}

// writeYearCmd returns a Bubble Tea command that calls metadata.WriteYear on a
// background goroutine and returns the result as a YearWrittenMsg.
func writeYearCmd(path, year string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteYear(path, year)
		return YearWrittenMsg{Year: year, Err: err}
	}
}
```

**Expected outcome:** `internal/tui/model.go` compiles. All new types and Cmd factories are present. Build will fail until `update.go` and `view.go` are also updated — that is expected at this stage.

**Verification:** Deferred to Step 6.

---

### Step 4: Update `internal/tui/update.go`

**Action:**

Replace the full content of `internal/tui/update.go` with the version below. Key changes over the Task 4 version:

- New case for `spinner.TickMsg` — advances the spinner animation and re-queues the tick.
- New case for `YearFetchedMsg` — updates `enrichYearStatus` and `enrichYearValue`.
- New case for `BPMWrittenMsg` — updates `enrichBPMStatus` and writes BPM back to the queue task.
- New case for `YearWrittenMsg` — updates `enrichYearStatus` and writes year back to the queue task.
- `Ctrl+1` handler: commits BPM (calls `writeBPMCmd`) if status is `enrichFound`.
- `Ctrl+2` handler: commits Year (calls `writeYearCmd`) if status is `enrichFound`.
- `t` key handler: records a tap-tempo timestamp; calculates BPM from average inter-tap interval when ≥ 3 taps have been recorded.
- `skipToNext` is updated to call `resetEnrichment()` and fire a new `fetchYearCmd` for the next song.

```go
package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies the tea.Model interface. It returns the pendingInit command
// batch that was stored by New() — auto-play + first tick + spinner + year fetch.
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
		if m.state == StateGenreSelection {
			m.genreList.SetWidth(msg.Width - 4)
		}
		return m, nil

	case TickMsg:
		m.playbackState = m.engine.GetState()
		return m, tickCmd()

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		return m, spinCmd

	case PlayErrMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
		} else {
			m.lastPlayErr = nil
		}
		return m, nil

	case TagWrittenMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
			m.state = StateReviewing
			return m, nil
		}

		idx := m.queue.CurrentIndex
		if idx >= 0 && idx < len(m.queue.Tasks) {
			m.queue.Tasks[idx].Genre1 = msg.Primary
			m.queue.Tasks[idx].Genre2 = msg.Secondary
		}

		saveCmd := saveStateCmd(m.providerRef, m.queue.Tasks)
		nextModel, nextCmd := m.skipToNext()
		return nextModel, tea.Batch(saveCmd, nextCmd)

	case SaveStateErrMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
		} else {
			m.lastSaveErr = nil
		}
		return m, nil

	case YearFetchedMsg:
		if msg.Err != nil {
			m.enrichYearStatus = enrichError
			m.enrichYearValue = ""
		} else if msg.Year == "" {
			// Successful request but no year found in MusicBrainz.
			m.enrichYearStatus = enrichIdle
			m.enrichYearValue = ""
		} else {
			m.enrichYearStatus = enrichFound
			m.enrichYearValue = msg.Year
		}
		return m, nil

	case BPMWrittenMsg:
		if msg.Err != nil {
			m.enrichBPMStatus = enrichError
		} else {
			m.enrichBPMStatus = enrichCommitted
			idx := m.queue.CurrentIndex
			if idx >= 0 && idx < len(m.queue.Tasks) {
				m.queue.Tasks[idx].BPM = msg.BPM
			}
		}
		return m, nil

	case YearWrittenMsg:
		if msg.Err != nil {
			m.enrichYearStatus = enrichError
		} else {
			m.enrichYearStatus = enrichCommitted
			idx := m.queue.CurrentIndex
			if idx >= 0 && idx < len(m.queue.Tasks) {
				m.queue.Tasks[idx].Year = msg.Year
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// In StateGenreSelection, pass unhandled non-key messages to the list.
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
			m.state = StateReviewing
			return m, nil

		case "enter":
			return m.confirmGenreSelection()
		}

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
		m = m.openGenreModal()
		return m, nil

	case "esc":
		return m.skipToNext()

	case "ctrl+u":
		return m.undoLast()

	// ── Commit BPM (Ctrl+1) ───────────────────────────────────────────────────
	case "ctrl+1":
		if m.enrichBPMStatus != enrichFound {
			// Nothing ready to commit — ignore.
			return m, nil
		}
		if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
			return m, nil
		}
		path := m.queue.Tasks[m.queue.CurrentIndex].FilePath
		return m, writeBPMCmd(path, m.enrichBPMValue)

	// ── Commit Year (Ctrl+2) ──────────────────────────────────────────────────
	case "ctrl+2":
		if m.enrichYearStatus != enrichFound {
			// Nothing ready to commit — ignore.
			return m, nil
		}
		if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
			return m, nil
		}
		path := m.queue.Tasks[m.queue.CurrentIndex].FilePath
		return m, writeYearCmd(path, m.enrichYearValue)

	// ── Tap Tempo (t) ─────────────────────────────────────────────────────────
	case "t":
		return m.recordTap()
	}

	return m, nil
}

// recordTap records a tap-tempo keypress timestamp and, when at least 3 taps
// have been collected, calculates the BPM from the average inter-tap interval.
// Taps older than 3 seconds are discarded to allow the user to restart a tap
// sequence by pausing.
func (m Model) recordTap() (tea.Model, tea.Cmd) {
	now := time.Now()

	// Discard stale taps (gap > 3 seconds since the last tap means a new sequence).
	if len(m.tapTimes) > 0 {
		last := m.tapTimes[len(m.tapTimes)-1]
		if now.Sub(last) > 3*time.Second {
			m.tapTimes = nil
		}
	}

	m.tapTimes = append(m.tapTimes, now)

	// Need at least 3 taps (2 intervals) for a meaningful average.
	if len(m.tapTimes) < 3 {
		m.enrichBPMStatus = enrichLoading
		return m, nil
	}

	// Calculate average interval between consecutive taps.
	var totalInterval time.Duration
	for i := 1; i < len(m.tapTimes); i++ {
		totalInterval += m.tapTimes[i].Sub(m.tapTimes[i-1])
	}
	avgInterval := totalInterval / time.Duration(len(m.tapTimes)-1)

	if avgInterval <= 0 {
		return m, nil
	}

	// BPM = 60 seconds / average interval in seconds.
	bpm := math.Round(60.0 / avgInterval.Seconds())
	m.enrichBPMValue = fmt.Sprintf("%d", int(bpm))
	m.enrichBPMStatus = enrichFound

	return m, nil
}

// confirmGenreSelection handles the user pressing Enter on a genre list item.
func (m Model) confirmGenreSelection() (tea.Model, tea.Cmd) {
	selected, ok := m.genreList.SelectedItem().(genreItem)
	if !ok {
		return m, nil
	}

	if m.genreStep == stepPrimary {
		m.selectedPrimary = selected.title
		m.genreStep = stepSecondary
		m.genreList = makeGenreList(m.cfg.GenreList, true, m.width)
		return m, nil
	}

	secondary := selected.title
	if secondary == "[NONE]" {
		secondary = ""
	}

	m.state = StateReviewing

	if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
		return m, nil
	}
	path := m.queue.Tasks[m.queue.CurrentIndex].FilePath

	return m, writeTagsCmd(path, m.selectedPrimary, secondary)
}

// skipToNext advances the queue to the next task, starts playing it, and
// resets enrichment state. Fires a fetchYearCmd for the new song if artist/title
// are available. If no next task exists (end of queue), it is a no-op.
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

	// Reset enrichment for the incoming song.
	m = m.resetEnrichment()

	nextTask := m.queue.Tasks[nextIndex]
	var cmds []tea.Cmd
	cmds = append(cmds, playCmd(m.engine, nextTask.FilePath))

	// Fire year fetch immediately if we have artist/title metadata.
	if nextTask.Artist != "" || nextTask.Title != "" {
		m.enrichYearStatus = enrichLoading
		cmds = append(cmds, fetchYearCmd(nextTask.Artist, nextTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
	}

	return m, tea.Batch(cmds...)
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

	// Reset enrichment when rewinding.
	m = m.resetEnrichment()

	prevTask := m.queue.Tasks[m.queue.CurrentIndex]
	var cmds []tea.Cmd
	cmds = append(cmds, playCmd(m.engine, prevTask.FilePath))

	if prevTask.Artist != "" || prevTask.Title != "" {
		m.enrichYearStatus = enrichLoading
		cmds = append(cmds, fetchYearCmd(prevTask.Artist, prevTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
	}

	return m, tea.Batch(cmds...)
}
```

**Expected outcome:** `internal/tui/update.go` compiles. The full keybinding set including `Ctrl+1`, `Ctrl+2`, and `t` is wired. Spinner ticks advance the animation. Enrichment messages update the model state. Build still fails until `view.go` is updated — that is expected.

**Verification:** Deferred to Step 6.

---

### Step 5: Update `internal/tui/view.go`

**Action:**

Replace the full content of `internal/tui/view.go` with the version below. Key changes over the Task 4 version:

- Add four new lipgloss style variables: `styleEnrichLabel`, `styleEnrichLoading`, `styleEnrichFound`, `styleEnrichCommitted`, `styleEnrichError`.
- `viewReviewing()` gains a new **enrichment panel** row rendered between the progress bar and the status bar.
- The enrichment panel shows Year and BPM rows with colour-coded status indicators.
- Updated keybind hints to include `Ctrl+1`, `Ctrl+2`, and `t`.
- The existing `viewGenreModal()` and `hintStr()` helpers are unchanged.

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

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#A8DADC")).
			Padding(1, 2)

	styleModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			MarginBottom(1)

	styleModalFooter = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				MarginTop(1)

	// ── Enrichment panel styles ───────────────────────────────────────────────

	// styleEnrichLabel styles the "Year:" and "BPM:" prefix labels.
	styleEnrichLabel = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	// styleEnrichLoading styles the loading indicator (spinner + "Loading...").
	styleEnrichLoading = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Italic(true)

	// styleEnrichFound styles a fetched value that is ready to be committed.
	// Yellow/amber — "you have something, press the key to commit it".
	styleEnrichFound = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true)

	// styleEnrichCommitted styles a value that has been successfully written to
	// the ID3 tag. Green — "done".
	styleEnrichCommitted = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B")).
				Bold(true)

	// styleEnrichError styles a fetch or write error.
	styleEnrichError = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Italic(true)

	// styleEnrichIdle styles the idle/not-found state.
	styleEnrichIdle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				Italic(true)
)

// View renders the full TUI frame. Pure function — no side effects.
func (m Model) View() string {
	if m.state == StateGenreSelection {
		return m.viewGenreModal()
	}
	return m.viewReviewing()
}

// viewReviewing renders the main playback screen:
// header → progress bar → enrichment panel → status bar.
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

	// ── Enrichment panel ──────────────────────────────────────────────────────
	enrichPanel := m.viewEnrichmentPanel()

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
		"   " + hintStr("t", "tap BPM") +
		"   " + hintStr("Ctrl+1", "commit BPM") +
		"   " + hintStr("Ctrl+2", "commit year") +
		"   " + hintStr("Esc", "skip") +
		"   " + hintStr("Ctrl+U", "undo") +
		"   " + hintStr("Ctrl+C", "quit")

	statusLine := styleStatus.Render(
		queueStr + "   " + posStr + errStr + "\n  " + hints,
	)

	return "\n" + headerLine + "\n\n" + progressBar + "\n\n" + enrichPanel + "\n" + statusLine + "\n"
}

// viewEnrichmentPanel renders the Year and BPM enrichment rows.
// Each row shows: label + status indicator + value (if available).
func (m Model) viewEnrichmentPanel() string {
	yearRow := styleEnrichLabel.Render("  Year: ") + m.enrichFieldView(
		m.enrichYearStatus,
		m.enrichYearValue,
		m.spinner.View(),
		"[not found]",
		"press Ctrl+2 to commit",
	)

	var bpmHint string
	tapCount := len(m.tapTimes)
	switch {
	case tapCount == 0:
		bpmHint = "press t to tap"
	case tapCount < 3:
		bpmHint = fmt.Sprintf("tap %d more...", 3-tapCount)
	default:
		bpmHint = "press Ctrl+1 to commit"
	}

	bpmRow := styleEnrichLabel.Render("  BPM:  ") + m.enrichFieldView(
		m.enrichBPMStatus,
		m.enrichBPMValue,
		m.spinner.View()+" tapping...",
		bpmHint,
		bpmHint,
	)

	return yearRow + "\n" + bpmRow
}

// enrichFieldView returns the styled display string for a single enrichment
// field given its status, value, loading indicator text, idle text, and
// found-but-uncommitted hint text.
func (m Model) enrichFieldView(
	status enrichStatus,
	value string,
	loadingText string,
	idleText string,
	foundHint string,
) string {
	switch status {
	case enrichLoading:
		return styleEnrichLoading.Render(loadingText)
	case enrichFound:
		return styleEnrichFound.Render(value) +
			styleHint.Render("  "+foundHint)
	case enrichCommitted:
		return styleEnrichCommitted.Render(value + "  ✓")
	case enrichError:
		return styleEnrichError.Render("error — retry on next load")
	default: // enrichIdle
		return styleEnrichIdle.Render(idleText)
	}
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

**Expected outcome:** `internal/tui/view.go` compiles. The enrichment panel is visible between the progress bar and status bar. Year and BPM rows show colour-coded state.

**Verification:** Deferred to Step 6.

---

### Step 6: Full Build and Test Verification

**Action:**

Run the complete build and test suite:

```
cd mp3-reviewer && go build ./...
cd mp3-reviewer && go test ./...
cd mp3-reviewer && go vet ./...
```

If `go build ./...` fails with a missing import or undefined symbol, check:

1. The `spinner` import path is `"github.com/charmbracelet/bubbles/spinner"` (already in `go.mod` via `charmbracelet/bubbles` v1.0.0 — no extra `go get` needed).
2. The `"math"` import is present in `update.go` (used by `math.Round`).
3. The `"fmt"` and `"time"` imports are present in `update.go`.
4. `api.FetchYear` is accessible — `internal/tui/model.go` imports `"song-reviewer/internal/api"`.

Run `go mod tidy` if `go.sum` is missing entries:

```
cd mp3-reviewer && go mod tidy
```

**Expected outcome:** All packages compile. All existing tests pass. `go vet` reports zero issues.

**Verification:**

```
cd mp3-reviewer && go build ./... && go test ./... && go vet ./...
```

All three commands exit with code 0.

---

### Step 7: Add a Unit Test for `FetchYear` (Offline / HTTP Mock)

**Action:**

Create a new file `internal/api/musicbrainz_test.go` with the content below. This test uses `net/http/httptest` to stand up a local mock server so no real network call is made during CI.

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchYear_HappyPath(t *testing.T) {
	// Build a fake MusicBrainz response with two release groups.
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				ReleaseGroups: []mbReleaseGroupRef{
					{ID: "rg1", FirstReleaseDate: "1973-03-01"},
					{ID: "rg2", FirstReleaseDate: "1971-11-08"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	// Temporarily override the base URL so FetchYear hits the mock server.
	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Led Zeppelin", "Stairway to Heaven", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	// Earliest date is 1971-11-08, so year should be "1971".
	if year != "1971" {
		t.Errorf("FetchYear() = %q, want %q", year, "1971")
	}
}

func TestFetchYear_NoRecordings(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Unknown Artist", "Unknown Song", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	if year != "" {
		t.Errorf("FetchYear() = %q, want empty string", year)
	}
}

func TestFetchYear_EmptyInputs(t *testing.T) {
	_, err := FetchYear("", "", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for empty artist and title, got nil")
	}
}

func TestFetchYear_MissingUserAgent(t *testing.T) {
	_, err := FetchYear("Artist", "Title", "")
	if err == nil {
		t.Fatal("expected an error for empty userAgent, got nil")
	}
}

func TestFetchYear_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	_, err := FetchYear("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for non-200 HTTP status, got nil")
	}
}
```

**Important:** For the mock server injection to work, `musicBrainzBaseURL` must be a package-level `var` (not `const`) in `musicbrainz.go`. The Step 1 implementation already declares it as `var musicBrainzBaseURL = "https://musicbrainz.org/ws/2"` — confirm this is `var`, not `const`.

**Expected outcome:** Five test functions in the `api` package. All pass with zero network calls.

**Verification:**

```
cd mp3-reviewer && go test ./internal/api/...
```

Should print `ok  song-reviewer/internal/api`.

---

### Step 8: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**

Open `agent-development/agent-specs/architecture-breakdown.md` and make the following targeted changes:

1. Update the `/internal/api` line from:

   > `/internal/api`: External HTTP clients for MusicBrainz/BPM APIs.

   To:

   > `/internal/api`: External HTTP clients. `musicbrainz.go` implements `FetchYear(artist, title, userAgent string) (string, error)`, which searches the MusicBrainz JSON API for the earliest original release year by querying the `/recording` endpoint with a Lucene `artist:"…" AND recording:"…"` query, then picking the earliest `first-release-date` from the embedded release groups of the top result. The package-level `musicBrainzBaseURL` var is overridable in tests via `httptest`.

2. Update the `/internal/metadata` line to append:

   > Additionally exposes `WriteBPM(path, bpm string) error` (writes `TBPM` frame) and `WriteYear(path, year string) error` (writes `TYER`/`TDRC` via `tag.SetYear()`).

3. Update the `/internal/tui` line to mention the enrichment panel and new keys:

   After the existing description, append:

   > The main review screen now includes a metadata enrichment panel between the progress bar and the status bar. It shows Year (fetched from MusicBrainz on song load) and BPM (calculated via Tap Tempo: press `t` to the beat, 3+ taps required). Values are colour-coded by state: loading (grey italic), found/ready (yellow — press `Ctrl+1` to commit BPM or `Ctrl+2` to commit year), committed (green ✓), error (red italic). Background fetches use `tea.Cmd`; the spinner from `bubbles/spinner` animates while a fetch is in progress.

4. Under `## Design Patterns`, extend the Concurrency bullet:

   Change:

   > Audio playback, ID3 tag writing, and JSON persistence must run in background goroutines to prevent UI freezing. Use `tea.Cmd` to communicate results back to the TUI.

   To:

   > Audio playback, ID3 tag writing, JSON persistence, and external API calls (MusicBrainz) must run in background goroutines to prevent UI freezing. Use `tea.Cmd` to communicate results back to the TUI. The `spinner.TickMsg` from `bubbles/spinner` drives the loading animation independently of the 100ms progress bar ticker.

**Expected outcome:** `agent-development/agent-specs/architecture-breakdown.md` documents the new API client, metadata functions, enrichment panel, and Tap Tempo feature.

**Verification:**

```
grep -n "FetchYear\|WriteBPM\|WriteYear\|Tap Tempo\|enrichment\|TBPM\|TDRC" agent-development/agent-specs/architecture-breakdown.md
```

Should print at least 6 matching lines.

---

### Step 9: Update `README.md`

**Action:**

Open `README.md` and make the following targeted additions:

1. In the **Features** section, update the **Data Enrichment** bullet:

   Old:
   > **Data Enrichment** — Fetch original release year from MusicBrainz and BPM from external APIs.

   New:
   > **Data Enrichment** — Fetches the original release year automatically from MusicBrainz when a song loads. BPM is calculated via **Tap Tempo** (press `t` to the beat; 3+ taps required). Both values can be committed to the ID3 tags with `Ctrl+1` (BPM) and `Ctrl+2` (Year).

2. In the **Keybindings** table, add three new rows (insert before the `Esc` row):

   | `t` | Tap to the beat — calculates BPM (3+ taps required) |
   | `Ctrl+1` | Commit suggested BPM to the MP3's TBPM tag (only active when BPM is ready) |
   | `Ctrl+2` | Commit suggested Year to the MP3's year tag (only active when year is found) |

3. Add a new **Metadata Enrichment** section after the **Genre Tagging** section:

   ```
   ## Metadata Enrichment

   When a song loads, the app immediately fetches metadata in the background:

   ### Release Year (MusicBrainz)

   The app queries the [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API) to find the **original** release year for the current song (not a remaster date). It searches by Artist and Title, finds the earliest release group's `first-release-date`, and displays the 4-digit year.

   - **Yellow** — year found, not yet committed. Press `Ctrl+2` to write it to the ID3 tag.
   - **Green ✓** — year committed to the file's year tag.
   - **Grey italic** — loading or not found.

   > **Note:** MusicBrainz requires a descriptive `User-Agent` header. Set `api_keys.musicbrainz_user_agent` in `config/settings.json` to a string like `"MySongReviewer/1.0 ( your@email.com )"`.

   ### BPM (Tap Tempo)

   Press `t` repeatedly to the beat of the song. After 3 or more taps, the app calculates the average BPM from the inter-tap intervals.

   - Tapping pauses > 3 seconds apart reset the sequence automatically.
   - **Yellow** — BPM calculated, not yet committed. Press `Ctrl+1` to write it to the ID3 `TBPM` tag.
   - **Green ✓** — BPM committed to the file.
   ```

4. In the **Settings Reference** table, add a row for the user agent:

   (The `api_keys.musicbrainz_user_agent` field is already mentioned in the config example but may lack a table row — add it if missing):

   | `api_keys.musicbrainz_user_agent` | Required by MusicBrainz API. Must include your app name and contact email per their [rate limiting policy](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting). |

**Expected outcome:** `README.md` documents the enrichment feature, new keybindings, and MusicBrainz user-agent requirement.

**Verification:**

```
grep -n "Tap Tempo\|MusicBrainz\|Ctrl+1\|Ctrl+2\|TBPM\|Enrichment" README.md
```

Should print at least 8 matching lines.

---

## Open Questions & Decisions

### Q1: BPM Source — Tap Tempo vs External API

**Context:** The task request mentions both an external BPM API (SongBPM or AcoustID) and a Tap Tempo fallback. This plan implements **Tap Tempo only** (no external BPM API call). The reasoning:

- **SongBPM API** requires a paid subscription for meaningful usage.
- **AcoustID** is a fingerprint-matching service; it requires the `fpcalc` Chromaprint CLI tool (CGO-free use is impossible without it), which violates the "DON'T use Cgo unless absolutely necessary" guideline.
- Tap Tempo is 100% offline, always accurate for the song the user is actually listening to, and requires no API keys, no extra dependencies, and no rate-limiting concerns.

**Options:**
- **A)** Tap Tempo only (this plan's default). Simple, offline, no new dependencies.
- **B)** AcoustID fingerprint lookup. Requires `fpcalc` binary on the host and significant extra complexity.
- **C)** SongBPM API. Requires a paid API key and a network call per song.
- **D)** Both: attempt an external BPM API call first, fall back to Tap Tempo if it fails or is unavailable.

**Agent's recommendation:** **Option A** — Tap Tempo is the best fit for the project's "pure Go, no CGO, keyboard-driven" philosophy. Option D could be added in a future task if a reliable free BPM API becomes available.

**Human decision:** We should try fetching the song's BPM from the MusicBrainz API first, falling back to Tap Tempo if it fails or is unavailable. Any other paid API is out of scope for this project.

---

### Q2: MusicBrainz Search Strategy — Recording vs Release Group

**Context:** The task request says "Look for the Release Group to find the earliest `first-release-date`." There are two ways to achieve this:

- **Option A (this plan):** Search the `/recording` endpoint with `artist + title`, request `inc=release-groups` to get embedded release groups with their `first-release-date` in a single API call. The top recording result's release groups are sorted to find the earliest date. This requires one HTTP request.
- **Option B:** Search the `/release-group` endpoint directly. This requires an additional lookup step (recording search → get MBID → release group lookup) and therefore two HTTP requests, adding latency.
- **Option C:** Search the `/recording` endpoint and follow up with a `/release-group/<mbid>` lookup to get the full `first-release-date`. Same two-request cost as B.

**Agent's recommendation:** **Option A** — one request is always better for UX. The embedded `release-groups` field in a recording search response includes `first-release-date` directly when `inc=release-groups` is appended, making a second lookup unnecessary.

**Human decision:** We should use option A. But keep in mind the previous answer: if we can also include bpm-related information in the same request, we should do so.

---

### Q3: Minimum Tap Count for BPM Calculation

**Context:** The plan requires at least **3 taps** (2 intervals) before calculating BPM. The task request does not specify a minimum.

- **2 taps (1 interval):** Very fast to get a reading but highly inaccurate (a single tap interval is easily off by 100+ ms from hand latency).
- **3 taps (2 intervals):** Minimum for any meaningful average. Good balance of speed and accuracy.
- **5 taps (4 intervals):** Considerably more accurate but requires the user to wait through 4 beats.
- **8 taps (7 intervals):** Near-professional accuracy but slow UX for a quick-review tool.

**Agent's recommendation:** **3 taps** — the minimum viable average. The user sees the BPM update live after tap 3 and can keep tapping to refine it. Tap count is implemented as a simple `len(m.tapTimes) < 3` guard and is trivial to change.

**Human decision:** I'd like to try using 8 taps (7 intervals) for a more accurate reading, we should add a feature to display the user how many taps they have left before the BPM is calculated. Additionally, if the tapped tempo is too irregular, the count should reset (user can always use ESC key to go back)

---

### Q4: Stale Tap Reset Window

**Context:** When the user pauses between taps (e.g., they look away and come back), the old taps should be discarded. The plan uses a **3-second** gap threshold.

- **2 seconds:** Aggressive. At 60 BPM, one beat = 1 second, so a 2s gap only allows ~2 beats of pause before reset. Risky for slow songs.
- **3 seconds:** Allows up to ~180 BPM to have one beat gap and still not reset. Safe for most music.
- **5 seconds:** Very forgiving, but means the user has to wait 5 seconds to intentionally restart a tap sequence.

**Agent's recommendation:** **3 seconds** — a good middle ground. Changing this is a single constant in `recordTap()`.

**Human decision:** Let's try using option B: 3 seconds.

---

### Q5: Should `WriteBPM` and `WriteYear` Also Call `provider.SaveState`?

**Context:** After `Ctrl+1` (BPM) or `Ctrl+2` (Year) commits a value, the in-memory queue task is updated (`Task.BPM` or `Task.Year`), but `SaveState` is **not** called in this plan. The task request says to update the JSON, but the existing JSON schema (`manual_review.json`) only has `status`, `primary_genre`, and `secondary_genre` fields — there is no `bpm` or `year` field in the schema yet.

**Options:**
- **A)** Do NOT call `SaveState` for BPM/Year commits — the ID3 tag is the source of truth; the JSON only tracks genre assignment status. This is simpler and avoids extending the JSON schema in this task.
- **B)** Extend the `reviewEntry` struct in `json_provider.go` with `bpm` and `year` fields and call `SaveState` after every commit. This keeps the JSON and ID3 tags in sync but adds scope.
- **C)** Add `SaveState` calls but only if `bpm`/`year` fields are already present in the schema (which they currently are not) — partial approach, not clean.

**Agent's recommendation:** **Option A** for this task — the ID3 tag write is the canonical persistence operation. If the human wants full JSON sync, that is a clean scope for a follow-up task. This keeps Task 5 focused on the enrichment UX.

**Human decision:** Let's go with option B: we can extend the struct for `reviewEntry` with `bpm` and `year` fields and call `SaveState` after every commit. Since we're committing these changes, we should also add them to the JSON data source to ensure consistency.

---

### Q6: Enrichment State After `Ctrl+U` (Undo)

**Context:** When the user presses `Ctrl+U` to go back to the previous song, should the enrichment state be:

- **A)** Reset and re-fetched — the cleanest UX, ensures fresh data. Costs one MusicBrainz API call per undo. This is the plan's current implementation.
- **B)** Preserved from when the song was first loaded — avoids the extra API call. But since undo goes backwards, we don't have the cached state stored anywhere (the model doesn't preserve per-song enrichment history).

**Agent's recommendation:** **Option A** — reset and re-fetch. It is simple, consistent with the forward navigation behaviour, and the extra API call is trivial. The alternative (caching per-song enrichment results) would require storing a map on the model, which is added complexity.

**Human decision:** Let's do option A, the API call is simple and trivial.

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/api/musicbrainz.go` | Modified | Full implementation of `FetchYear(artist, title, userAgent string) (string, error)` using MusicBrainz JSON recording search with `inc=release-groups`; package-level `var musicBrainzBaseURL` for test injection |
| 2 | `internal/api/musicbrainz_test.go` | Created | Five unit tests using `net/http/httptest` mock server: happy path, no recordings, empty inputs, missing user agent, HTTP error |
| 3 | `internal/metadata/writer.go` | Modified | Appended `WriteBPM(path, bpm string) error` (TBPM frame) and `WriteYear(path, year string) error` (`tag.SetYear()`) |
| 4 | `internal/tui/model.go` | Modified | Added `enrichStatus` type + constants; `YearFetchedMsg`, `BPMWrittenMsg`, `YearWrittenMsg` types; `enrichYearStatus`, `enrichYearValue`, `enrichBPMStatus`, `enrichBPMValue`, `tapTimes`, `spinner` fields; `fetchYearCmd`, `writeBPMCmd`, `writeYearCmd` Cmd factories; `resetEnrichment()` helper; updated `New()` to start spinner and fire initial `fetchYearCmd` |
| 5 | `internal/tui/update.go` | Modified | Added `spinner.TickMsg` handler; `YearFetchedMsg`, `BPMWrittenMsg`, `YearWrittenMsg` handlers; `Ctrl+1`, `Ctrl+2`, `t` key handlers; `recordTap()` tap-tempo function; updated `skipToNext()` and `undoLast()` to call `resetEnrichment()` and fire `fetchYearCmd` |
| 6 | `internal/tui/view.go` | Modified | Added enrichment panel styles (`styleEnrichLoading`, `styleEnrichFound`, `styleEnrichCommitted`, `styleEnrichError`, `styleEnrichIdle`, `styleEnrichLabel`); added `viewEnrichmentPanel()` and `enrichFieldView()` helpers; updated `viewReviewing()` to include enrichment panel between progress bar and status bar; updated keybind hints |
| 7 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Updated `/internal/api`, `/internal/metadata`, `/internal/tui` descriptions; updated Concurrency pattern bullet |
| 8 | `README.md` | Modified | Updated Data Enrichment feature bullet; added `t`, `Ctrl+1`, `Ctrl+2` to keybindings table; added **Metadata Enrichment** section; noted MusicBrainz user-agent requirement |

**Total files created: 1 | Total files modified: 7**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes — all existing tests plus the five new API tests
- [ ] `go vet ./...` reports zero issues
- [ ] `internal/api/musicbrainz.go` exports `FetchYear` and declares `musicBrainzBaseURL` as a `var` (not `const`)
- [ ] `internal/api/musicbrainz_test.go` exists and all five tests pass with no real network calls (mock server used)
- [ ] `internal/metadata/writer.go` exports `WriteTags`, `WriteBPM`, and `WriteYear` (all three functions present)
- [ ] `internal/tui/model.go`: `Model` has `enrichYearStatus`, `enrichYearValue`, `enrichBPMStatus`, `enrichBPMValue`, `tapTimes`, `spinner` fields; `New()` fires `fetchYearCmd` for the first song; `resetEnrichment()` method exists
- [ ] `internal/tui/update.go`: `Ctrl+1` fires `writeBPMCmd` only when `enrichBPMStatus == enrichFound`; `Ctrl+2` fires `writeYearCmd` only when `enrichYearStatus == enrichFound`; `t` key calls `recordTap()`; `skipToNext()` calls `resetEnrichment()` and fires `fetchYearCmd`
- [ ] `internal/tui/view.go`: enrichment panel is rendered between progress bar and status bar; Year row shows spinner when loading; BPM row shows tap count hint; committed values are styled green with ✓
- [ ] The `bubbles/spinner` import compiles without needing a new `go get` (already available via `charmbracelet/bubbles` v1.0.0)
- [ ] No unrelated files were modified or deleted
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated with new API client, enrichment panel, and Tap Tempo feature
- [ ] `README.md` updated with Metadata Enrichment section and new keybindings
- [ ] Relevant diagrams in `diagrams/` updated to reflect any structural or behavioral changes

---

## Notes for the Implementing Agent

1. **`musicBrainzBaseURL` must be a `var`, not a `const`.** The test file in Step 7 overrides it at runtime to point at a local `httptest` server. If it is declared as `const`, the tests will fail to compile.

2. **No new `go get` is required for this task.** `bubbles/spinner` is already part of `charmbracelet/bubbles` v1.0.0 which is in `go.mod`. The `net/http`, `net/url`, `encoding/json`, `sort`, `math`, `time`, `fmt` packages are all from the Go standard library.

3. **The spinner tick is independent of the 100ms progress bar tick.** The spinner uses its own `spinner.TickMsg` type (from `github.com/charmbracelet/bubbles/spinner`) which is distinct from the TUI's `TickMsg` (defined as `type TickMsg time.Time`). Both can be in flight simultaneously. The `Update` switch handles them as separate cases.

4. **`enrichLoading` for BPM has a different trigger than for Year.** Year loading is triggered automatically on song load (via `fetchYearCmd`). BPM loading is triggered manually by the first tap (`enrichBPMStatus = enrichLoading` inside `recordTap()` when `len(m.tapTimes) < 3`). The spinner is shared but its display context (Year row vs BPM row) is determined by the `enrichStatus` of each field.

5. **MusicBrainz rate limiting.** The API requires no more than one request per second. Since `fetchYearCmd` is fired only once per song load (not in a loop), the rate limit is never hit in normal usage. The `http.Client` timeout is set to 10 seconds to avoid hanging the background goroutine indefinitely.

6. **The `inc=release-groups` parameter.** Without this, the recording search result does not include release-group data. The URL in `FetchYear` must include `&inc=release-groups` to get embedded `first-release-date` values. The plan's Step 1 URL construction includes this parameter — do not remove it.

7. **`tag.SetYear()` compatibility.** `bogem/id3v2` maps `tag.CommonID("Year")` to `TYER` for ID3v2.3 tags and `TDRC` for ID3v2.4 tags. `tag.SetYear()` calls `AddTextFrame(tag.CommonID("Year"), ...)` internally, so it handles both versions automatically. Do not hardcode `"TDRC"` or `"TYER"` directly.

8. **Tap times slice mutation.** `m.tapTimes` is a `[]time.Time`. Since `Model` is a value type, appending to `m.tapTimes` inside `recordTap()` creates a new slice header on the local copy `m` and is then returned. This is correct Go value-semantics behaviour for Bubble Tea models. Do not use a pointer to the slice.

9. **`Ctrl+1` and `Ctrl+2` key strings.** In Bubble Tea, `tea.KeyMsg.String()` returns `"ctrl+1"` and `"ctrl+2"` for those key combinations (lowercase, with plus sign, no space). Verify this matches the `case` strings in `handleKey`.

10. **Artist and Title fields on `domain.Task`.** The `Task` struct has `Artist` and `Title` fields, but they are not currently populated by `ManualReviewProvider.GetTasks()` (the JSON schema only has `filepath`, `reason`, `confidence`, `status`, `primary_genre`, `secondary_genre`). This means `fetchYearCmd` will be called with empty strings for most songs in the current data file. The `FetchYear` function returns `("", error)` for empty both-fields input — the guard at the top of `FetchYear` catches this. The enrichment status in this case will stay `enrichIdle` (since the `if task.Artist != "" || task.Title != ""` guard in `New()` and `skipToNext()` prevents the fetch). This is expected and correct behaviour. Populating `Title` and `Artist` from actual ID3 tags is a future task (reading existing tags with `bogem/id3v2` at song load time). **Do not implement tag reading in this task** — it is out of scope.

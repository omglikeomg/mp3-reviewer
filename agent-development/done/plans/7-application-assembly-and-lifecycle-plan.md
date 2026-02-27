# Implementation Plan: Task 7 — Application Assembly & Lifecycle

## Overview

This plan audits and completes the application assembly layer, then adds a runtime Settings
overlay (`Ctrl-O`) and hardens the application lifecycle (defaults when `settings.json` is
missing, graceful shutdown with a final `SaveState` flush on quit).

**Critical audit finding:** Most of what the task request describes as "to be built" is
already fully implemented from prior tasks:

- `cmd/reviewer/main.go` — fully implemented: loads JSON config, initialises `audio.Engine`,
  loads tasks via `ManualReviewProvider`, builds `domain.ReviewQueue`, calls `tui.New()`,
  runs `tea.NewProgram` with alt-screen, and calls `engine.Close()` on both error and clean
  exit paths.
- `tui.New(queue, engine, cfg, provider)` — fully wired.
- Graceful audio shutdown — `engine.Close()` is called in the `ctrl+c` key handler AND
  after `prog.Run()` returns (defence-in-depth, already documented in
  `agent-instructions.md`).
- All config fields (`MusicFolder`, `JsonPath`, `GenreList`, `SeekDeltaSeconds`,
  `ApiKeys`) already flow from `domain.AppConfig` into the TUI.

**Real delta — the three things this task adds:**

1. **`main.go` hardening:** Provide safe defaults when `settings.json` is missing or
   unreadable, so the app starts instead of hard-exiting.
2. **Settings overlay (`Ctrl-O`):** A new `StateSettings` TUI screen with two
   `bubbles/textinput` fields (Music Folder Path, JSON Path). Saving triggers a live reload
   of the `TaskQueue` without restarting the process.
3. **Final `SaveState` flush on quit:** When the user presses `Ctrl-C`, call `SaveState`
   synchronously before `tea.Quit` so any enrichment data committed since the last
   auto-save is never lost.

No new packages or top-level directories are introduced. All changes are contained in
`cmd/reviewer/main.go`, `internal/tui/model.go`, `internal/tui/update.go`, and
`internal/tui/view.go`.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, MVU rules, TUI best practices |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree |
| Task Definition | `agent-development/pending/7-application-assembly-and-lifecycle.md` | The task being implemented |
| Entry Point | `cmd/reviewer/main.go` | Current assembly code — read before editing |
| Domain Models | `internal/domain/models.go` | `Task`, `AppConfig`, `ReviewQueue` — read before editing |
| TUI Model | `internal/tui/model.go` | Current model fields, Cmd factories, constructor |
| TUI Update | `internal/tui/update.go` | Current message dispatch and key handling |
| TUI View | `internal/tui/view.go` | Current rendering and lipgloss styles |
| Provider | `internal/provider/json_provider.go` | `ManualReviewProvider`, `GetTasks`, `SaveState` |
| Settings Example | `settings.example.json` | Reference for config field names and defaults |

---

## Pre-Conditions

- Tasks 0 through 6 are complete and `go build ./...` and `go test ./...` both pass cleanly.
- `cmd/reviewer/main.go` exists with the `loadConfig` helper.
- `internal/tui/model.go` defines `Model`, `AppState` (with `StateReviewing` and
  `StateGenreSelection`), all enrichment message types, and `tui.New()`.
- `internal/tui/update.go` implements `Init`, `Update`, and `handleKey` with state-aware
  dispatch.
- `github.com/charmbracelet/bubbles v1.0.0` is in `go.mod` (provides `bubbles/textinput`).
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

### Step 2: Harden `main.go` — Provide Defaults When `settings.json` Is Missing

**Action:**
Open `cmd/reviewer/main.go`. Replace the current `loadConfig` call site and `loadConfig`
function so that a missing or unreadable `settings.json` produces a warning on `stderr` but
does **not** exit the process — instead it continues with safe defaults. The hard exit on
missing config is currently the only lifecycle gap; the rest of `main.go` is correct and
must not be changed.

Change the `loadConfig` helper signature to:

```
func loadConfig(path string) (domain.AppConfig, error)
```

(no signature change — it already returns `(domain.AppConfig, error)`)

Change the call site in `main()` from:

```
cfg, err := loadConfig("config/settings.json")
if err != nil {
    fmt.Fprintf(os.Stderr, "song-reviewer: failed to load config: %v\n", err)
    os.Exit(1)
}
```

to:

```
cfg, err := loadConfig("config/settings.json")
if err != nil {
    // Non-fatal: warn but continue with defaults so the app starts even without
    // a settings file (the user can configure paths via the Settings overlay).
    fmt.Fprintf(os.Stderr, "song-reviewer: warning: could not load config (%v) — using defaults\n", err)
    cfg = defaultConfig()
}
```

Add a new `defaultConfig()` helper function to `main.go` (below `loadConfig`):

```
// defaultConfig returns a safe AppConfig used when settings.json is missing or
// unreadable. The user can update MusicFolder and JsonPath via the Ctrl-O
// Settings overlay at runtime.
func defaultConfig() domain.AppConfig {
    return domain.AppConfig{
        MusicFolder:      "",
        JsonPath:         "./data/manual_review.json",
        GenreList:        []string{},
        SeekDeltaSeconds: 30,
    }
}
```

Also change the `GetTasks` failure from a hard exit to a warning + empty queue, so the app
can still open and let the user fix paths via the Settings overlay. Change:

```
tasks, err := p.GetTasks()
if err != nil {
    fmt.Fprintf(os.Stderr, "song-reviewer: failed to load review queue: %v\n", err)
    os.Exit(1)
}
```

to:

```
tasks, err := p.GetTasks()
if err != nil {
    // Non-fatal: warn and start with an empty queue. The user can reload via
    // the Settings overlay (Ctrl-O) after correcting the paths.
    fmt.Fprintf(os.Stderr, "song-reviewer: warning: could not load review queue (%v) — starting with empty queue\n", err)
    tasks = []domain.Task{}
}
```

No other lines in `main.go` need to change.

**Expected outcome:**
`cmd/reviewer/main.go` compiles. Running the binary without a `config/settings.json`
prints a warning to stderr and opens the TUI with an empty queue instead of crashing.

**Verification:**
```
cd mp3-reviewer && go build ./...
```
Zero errors.

---

### Step 3: Add `StateSettings` and Settings Message Types to `model.go`

**Action:**
Open `internal/tui/model.go`. Make the following additions — do not remove or change any
existing code, only add.

#### 3.1 — Extend the `AppState` enum

Add `StateSettings` as a third constant after `StateGenreSelection`:

```
const (
    StateReviewing     AppState = iota
    StateGenreSelection
    StateSettings
)
```

#### 3.2 — Add new message types (place these after `YearWrittenMsg`)

```
// QueueReloadedMsg is returned by reloadQueueCmd after re-reading the JSON file.
// On success Tasks contains the newly loaded slice; Err is nil.
// On failure Tasks is nil and Err describes what went wrong.
type QueueReloadedMsg struct {
    Tasks []domain.Task
    Err   error
}
```

#### 3.3 — Add `bubbles/textinput` import

The file already imports `"github.com/charmbracelet/bubbles/list"`,
`"github.com/charmbracelet/bubbles/progress"`, and
`"github.com/charmbracelet/bubbles/spinner"`.  Add:

```
"github.com/charmbracelet/bubbles/textinput"
```

to the import block.

#### 3.4 — Add Settings fields to `Model`

Add the following fields to the `Model` struct, after the `pendingInit tea.Cmd` field at the
bottom:

```
// ── Settings overlay fields ───────────────────────────────────────────────

// settingsMusicFolder is the textinput component for the Music Folder Path field.
settingsMusicFolder textinput.Model

// settingsJsonPath is the textinput component for the JSON Review Path field.
settingsJsonPath textinput.Model

// settingsFocusIndex indicates which settings field (0 = MusicFolder, 1 = JsonPath)
// currently has focus.
settingsFocusIndex int
```

#### 3.5 — Initialise settings inputs in `New()`

Add the following block to `New()`, directly before the line that sets `m.pendingInit`:

```
// Initialise settings textinput components.
musicFolderInput := textinput.New()
musicFolderInput.Placeholder = "e.g. /Users/you/Music"
musicFolderInput.CharLimit = 512
musicFolderInput.Width = 60
musicFolderInput.SetValue(cfg.MusicFolder)

jsonPathInput := textinput.New()
jsonPathInput.Placeholder = "e.g. ./data/manual_review.json"
jsonPathInput.CharLimit = 512
jsonPathInput.Width = 60
jsonPathInput.SetValue(cfg.JsonPath)

m.settingsMusicFolder = musicFolderInput
m.settingsJsonPath    = jsonPathInput
m.settingsFocusIndex  = 0
```

#### 3.6 — Add `reloadQueueCmd` Cmd factory (place after `writeYearCmd`)

```
// reloadQueueCmd returns a Bubble Tea command that re-reads the review JSON file
// using the provided ManualReviewProvider and returns the result as a QueueReloadedMsg.
func reloadQueueCmd(p provider.ManualReviewProvider) tea.Cmd {
    return func() tea.Msg {
        tasks, err := p.GetTasks()
        return QueueReloadedMsg{Tasks: tasks, Err: err}
    }
}
```

**Expected outcome:**
`internal/tui/model.go` compiles cleanly. `StateSettings` is defined. `Model` has
`settingsMusicFolder`, `settingsJsonPath`, and `settingsFocusIndex` fields.
`QueueReloadedMsg` and `reloadQueueCmd` are declared.

**Verification:**
```
cd mp3-reviewer && go build ./internal/tui/...
```
Zero errors.

---

### Step 4: Wire Settings Logic into `update.go`

**Action:**
Open `internal/tui/update.go`. Make the following additions and modifications.

#### 4.1 — Handle `QueueReloadedMsg` in `Update`

Add a new `case` to the `switch msg := msg.(type)` block in `Update`, after the
`YearWrittenMsg` case:

```
case QueueReloadedMsg:
    if msg.Err != nil {
        m.lastSaveErr = msg.Err
        return m, nil
    }
    // Replace the task list in-place, reset position.
    m.queue.Tasks        = msg.Tasks
    m.queue.CurrentIndex = 0
    m.queue.History      = []domain.Task{}
    m.lastPlayErr        = nil
    m.lastSaveErr        = nil
    m = m.resetEnrichment()

    // Auto-play the first song of the newly loaded queue.
    var cmds []tea.Cmd
    if len(msg.Tasks) > 0 {
        first := msg.Tasks[0]
        cmds = append(cmds, playCmd(m.engine, first.FilePath))
        if first.Artist != "" || first.Title != "" {
            m.enrichYearStatus = enrichLoading
            m.enrichBPMStatus  = enrichLoading
            cmds = append(cmds, fetchYearCmd(first.Artist, first.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
            cmds = append(cmds, fetchBPMCmd(first.Artist, first.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
        }
    }
    return m, tea.Batch(cmds...)
```

#### 4.2 — Forward `tea.WindowSizeMsg` to settings inputs

In the `tea.WindowSizeMsg` case, after the existing lines that update
`m.progress.Width` and `m.genreList.SetWidth`, add:

```
m.settingsMusicFolder.Width = msg.Width - 10
m.settingsJsonPath.Width    = msg.Width - 10
```

#### 4.3 — Add `StateSettings` branch to `handleKey`

At the top of `handleKey`, before the existing `StateGenreSelection` block, add:

```
// ── Settings state ────────────────────────────────────────────────────────
if m.state == StateSettings {
    return m.handleSettingsKey(msg)
}
```

#### 4.4 — Add `Ctrl-O` key to the `StateReviewing` block

In the `StateReviewing` `switch msg.String()` block, add a new case after `"ctrl+u"`:

```
case "ctrl+o":
    return m.openSettings()
```

#### 4.5 — Add `handleSettingsKey` helper

Add the following method to `update.go` (at the end of the file):

```
// openSettings transitions the model into StateSettings and focuses the first
// input field (Music Folder Path).
func (m Model) openSettings() (tea.Model, tea.Cmd) {
    m.state = StateSettings
    m.settingsFocusIndex = 0
    m.settingsMusicFolder.SetValue(m.cfg.MusicFolder)
    m.settingsJsonPath.SetValue(m.cfg.JsonPath)
    // Focus the first field; textinput.Focus() returns a Cmd for cursor blink.
    cmd := m.settingsMusicFolder.Focus()
    m.settingsJsonPath.Blur()
    return m, cmd
}

// handleSettingsKey dispatches keyboard input while StateSettings is active.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {

    case "ctrl+c":
        // Flush any pending changes then quit.
        _ = m.providerRef.SaveState(m.queue.Tasks)
        m.engine.Close()
        return m, tea.Quit

    case "esc":
        // Discard changes and return to the review screen.
        m.state = StateReviewing
        m.settingsMusicFolder.Blur()
        m.settingsJsonPath.Blur()
        return m, nil

    case "tab", "shift+tab":
        // Toggle focus between the two input fields.
        if m.settingsFocusIndex == 0 {
            m.settingsFocusIndex = 1
            m.settingsMusicFolder.Blur()
            cmd := m.settingsJsonPath.Focus()
            return m, cmd
        }
        m.settingsFocusIndex = 0
        m.settingsJsonPath.Blur()
        cmd := m.settingsMusicFolder.Focus()
        return m, cmd

    case "enter":
        // Save: update cfg, update providerRef, dismiss overlay, reload queue.
        m.cfg.MusicFolder = m.settingsMusicFolder.Value()
        m.cfg.JsonPath    = m.settingsJsonPath.Value()
        m.providerRef.Config.MusicFolder = m.cfg.MusicFolder
        m.providerRef.Config.JsonPath    = m.cfg.JsonPath
        m.state = StateReviewing
        m.settingsMusicFolder.Blur()
        m.settingsJsonPath.Blur()
        return m, reloadQueueCmd(m.providerRef)
    }

    // Forward all other keys to the focused input.
    var cmd tea.Cmd
    if m.settingsFocusIndex == 0 {
        m.settingsMusicFolder, cmd = m.settingsMusicFolder.Update(msg)
    } else {
        m.settingsJsonPath, cmd = m.settingsJsonPath.Update(msg)
    }
    return m, cmd
}
```

#### 4.6 — Flush `SaveState` synchronously on `Ctrl-C` in `StateReviewing`

The existing `StateReviewing` `ctrl+c` handler is:

```
case "ctrl+c":
    m.engine.Close()
    return m, tea.Quit
```

Replace it with:

```
case "ctrl+c":
    // Flush any pending JSON enrichment data before quitting.
    _ = m.providerRef.SaveState(m.queue.Tasks)
    m.engine.Close()
    return m, tea.Quit
```

Do the same in the `StateGenreSelection` `ctrl+c` handler (it has the same two-line body):

```
case "ctrl+c":
    // Flush any pending JSON enrichment data before quitting.
    _ = m.providerRef.SaveState(m.queue.Tasks)
    m.engine.Close()
    return m, tea.Quit
```

**Expected outcome:**
`internal/tui/update.go` compiles cleanly. `Ctrl-O` opens the settings overlay.
`Enter` in settings saves and fires a queue reload. `Esc` discards. `Ctrl-C` from any
state flushes `SaveState` before quitting.

**Verification:**
```
cd mp3-reviewer && go build ./internal/tui/...
```
Zero errors.

---

### Step 5: Render the Settings Overlay in `view.go`

**Action:**
Open `internal/tui/view.go`. Make the following additions — do not change any existing
rendering code.

#### 5.1 — Route `StateSettings` in `View()`

The current `View()` function is:

```
func (m Model) View() string {
    if m.state == StateGenreSelection {
        return m.viewGenreModal()
    }
    return m.viewReviewing()
}
```

Update it to:

```
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

#### 5.2 — Add two new lipgloss styles (add to the `var (...)` block after `styleEnrichIdle`)

```
// styleSettingsLabel styles the field labels in the Settings overlay.
styleSettingsLabel = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#A8DADC")).
        Bold(true)

// styleSettingsHint styles the keybinding hints at the bottom of the Settings overlay.
styleSettingsHint = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#555555"))
```

#### 5.3 — Add `viewSettings()` rendering method (add at end of file)

```
// viewSettings renders the Settings overlay.
// The user can edit MusicFolder and JsonPath using bubbles/textinput.
// Tab / Shift-Tab switches focus. Enter saves and reloads the queue. Esc cancels.
func (m Model) viewSettings() string {
    title := styleModalTitle.Render("  Settings")

    musicLabel := styleSettingsLabel.Render("  Music Folder Path:")
    musicField := "  " + m.settingsMusicFolder.View()

    jsonLabel := styleSettingsLabel.Render("  Review JSON Path:")
    jsonField := "  " + m.settingsJsonPath.View()

    hints := "  " + styleSettingsHint.Render(
        hintStr("Tab", "next field")+
            "   "+hintStr("Enter", "save & reload")+
            "   "+hintStr("Esc", "cancel"),
    )

    inner := title + "\n\n" +
        musicLabel + "\n" + musicField + "\n\n" +
        jsonLabel + "\n" + jsonField + "\n\n" +
        hints

    return "\n" + styleModal.Render(inner) + "\n"
}
```

#### 5.4 — Add `Ctrl-O` hint to the main review screen status bar

In `viewReviewing()`, find the `hints` variable assignment and add `"   " + hintStr("Ctrl+O", "settings")` at the end of the hint string, before `"   " + hintStr("Ctrl+C", "quit")`.

The updated hints line should read:

```
hints := hintStr("← →", seekLabel) +
    "   " + hintStr("p", "pause") +
    "   " + hintStr("Enter", "tag") +
    "   " + hintStr("t", "tap BPM") +
    "   " + hintStr("Ctrl+1", "commit BPM") +
    "   " + hintStr("Ctrl+2", "commit year") +
    "   " + hintStr("Esc", "skip") +
    "   " + hintStr("Ctrl+U", "undo") +
    "   " + hintStr("Ctrl+O", "settings") +
    "   " + hintStr("Ctrl+C", "quit")
```

**Expected outcome:**
`internal/tui/view.go` compiles cleanly. `View()` routes `StateSettings` to
`viewSettings()`. The settings overlay renders two labelled textinput fields inside the
existing `styleModal` border.

**Verification:**
```
cd mp3-reviewer && go build ./internal/tui/...
```
Zero errors.

---

### Step 6: Run the Full Test Suite

**Action:**
```
cd mp3-reviewer && go build ./... && go test ./...
```

**Expected outcome:**
All packages compile. All existing tests pass. Zero failures.

**Verification:**
Terminal output shows no `FAIL` lines.

---

### Step 7: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**
In `architecture-breakdown.md`, find the `/internal/tui` paragraph. Extend the
`AppState` enum description to include `StateSettings`. Specifically, change the existing
phrase:

> `AppState` enum (`StateReviewing`, `StateGenreSelection`)

to:

> `AppState` enum (`StateReviewing`, `StateGenreSelection`, `StateSettings`)

Also add `QueueReloadedMsg` to the message types list, changing:

> all message types (`TickMsg`, `PlayErrMsg`, `TagWrittenMsg`, `SaveStateErrMsg`,
> `YearFetchedMsg`, `BPMFetchedMsg`, `BPMWrittenMsg`, `YearWrittenMsg`)

to:

> all message types (`TickMsg`, `PlayErrMsg`, `TagWrittenMsg`, `SaveStateErrMsg`,
> `YearFetchedMsg`, `BPMFetchedMsg`, `BPMWrittenMsg`, `YearWrittenMsg`,
> `QueueReloadedMsg`)

Also add `reloadQueueCmd` to the Cmd factories list, changing:

> all `tea.Cmd` factories (`tickCmd`, `playCmd`, `writeTagsCmd`, `saveStateCmd`,
> `fetchYearCmd`, `fetchBPMCmd`, `writeBPMCmd`, `writeYearCmd`)

to:

> all `tea.Cmd` factories (`tickCmd`, `playCmd`, `writeTagsCmd`, `saveStateCmd`,
> `fetchYearCmd`, `fetchBPMCmd`, `writeBPMCmd`, `writeYearCmd`, `reloadQueueCmd`)

Add a sentence describing the settings overlay at the end of the `/internal/tui` paragraph:

> A third TUI screen, `StateSettings` (toggled via `Ctrl-O`), presents two
> `bubbles/textinput` fields for `MusicFolder` and `JsonPath`. Saving (Enter) updates
> `cfg` and `providerRef` in-place and fires `reloadQueueCmd` to reload the task list
> without restarting the process. `Ctrl-C` from any state calls `providerRef.SaveState`
> synchronously before closing the audio engine and quitting, guaranteeing all committed
> enrichment data is persisted.

Also update the `/cmd/reviewer` entry to add the defaults behaviour:

> Entry point. Loads `config/settings.json` using `encoding/json`; if the file is missing or
> unreadable, a warning is printed to stderr and safe defaults are used (empty MusicFolder,
> `./data/manual_review.json` JsonPath, 30-second seek delta) so the app opens and lets the
> user configure paths via `Ctrl-O`.

**Expected outcome:**
The architecture doc accurately reflects the new `StateSettings` screen, `QueueReloadedMsg`,
`reloadQueueCmd`, the settings textinput fields, the synchronous `SaveState`-on-quit
behaviour, and the graceful-default startup.

**Verification:**
Open the file and confirm all four updated passages are present.

---

### Step 8: Update `agent-development/agent-specs/FOLDER-STRUCTURE.md`

**Action:**
No new packages or top-level directories were introduced. The only update needed is the
"Last updated" timestamp line at the top of the file. Change:

> **Last updated:** Task 5 — API integration and enrichment features implemented.

to:

> **Last updated:** Task 7 — Application assembly hardening and Settings overlay implemented.

**Expected outcome:**
`FOLDER-STRUCTURE.md` reflects the current task.

**Verification:**
Open the file and confirm the updated timestamp is present.

---

### Step 9: Update `README.md`

**Action:**
In `README.md`, make the following two updates.

#### 9.1 — Add Settings overlay to the Keybindings table

In the **Keybindings** section, add a new row to the keybinding table after the `Ctrl+U`
row:

```
| `Ctrl+O` | Open Settings overlay (edit Music Folder and JSON paths) |
```

#### 9.2 — Add a Settings section after the Metadata Enrichment section

Add the following new section after the existing **Metadata Enrichment** section and before
**Review JSON Format**:

```
## Settings

Press `Ctrl+O` at any time during review to open the Settings overlay. This lets you change
the **Music Folder Path** and **Review JSON Path** without restarting the app.

- Press `Tab` / `Shift-Tab` to switch between fields.
- Press `Enter` to save and immediately reload the review queue from the new path.
- Press `Esc` to cancel without saving.

> **Note:** Changes made via the Settings overlay are applied in memory only for the current
> session. To make them permanent, update `config/settings.json` directly.
```

**Expected outcome:**
The README accurately describes the Settings overlay and updated keybindings.

**Verification:**
Open the file and confirm the new keybinding row and the new Settings section are present.

---

## Open Questions & Decisions

### Q1: Should settings changes be persisted to `settings.json` on disk?

**Context:**
When the user edits `MusicFolder` or `JsonPath` in the Settings overlay and presses Enter,
the plan currently applies the change in memory only for the current session. The
`settings.json` file is gitignored (it's a user-local file), so writing back to it is safe.
However, the task request does not explicitly call for persistence of settings changes to
disk — it only says "Saving these options should trigger a reload of the TaskQueue."

There are two viable approaches:

**Options:**
- **A)** In-memory only — changes apply for the current session but are lost on restart.
  The note in the README tells the user to edit `settings.json` manually if they want
  permanence. This is the minimal-scope interpretation of the task.
  - Pro: No new file-write code path; no risk of corrupting the settings file.
  - Con: Slightly surprising UX — changes don't survive a restart.
- **B)** Write-back to disk — on Enter, also overwrite `config/settings.json` with the
  updated `MusicFolder` and `JsonPath` (leaving all other fields unchanged).
  Uses the same atomic temp-file + rename pattern as `SaveState`.
  - Pro: Changes are permanent; consistent with how `SaveState` works.
  - Con: Requires a new `saveSettingsCmd` Cmd factory and a new `SettingsSavedMsg`
    message type, adding moderate complexity. Also, if the user points to a bad path it
    gets written to disk.

**Agent's recommendation:** **Option A** (in-memory only) for this task. The task request
scopes the requirement narrowly to "trigger a reload of the TaskQueue" — it does not say
"persist to disk." Writing back to `settings.json` can always be added in a later task once
the UX is validated. A README note makes the limitation transparent.

**Human decision:** Let's go with Option B - this script will have a responsible use so there is very low risk. Persisting on disk is the way the script is intended to work.

---

### Q2: Should `Ctrl-O` be available during genre selection (inside the genre modal)?

**Context:**
The genre selection modal (`StateGenreSelection`) currently only handles `ctrl+c`, `esc`,
`enter`, and list navigation keys. If the user presses `Ctrl-O` while in genre selection,
the key falls through to the `bubbles/list` delegate, which ignores it silently. The
question is whether `Ctrl-O` should dismiss the genre modal and open Settings, or be
silently ignored while genre selection is active.

**Options:**
- **A)** Ignore `Ctrl-O` during `StateGenreSelection` — the user must press `Esc` first to
  leave genre selection, then press `Ctrl-O`.
  - Pro: Simpler state machine; no edge-case interaction between the two overlays.
  - Con: Slightly less discoverable.
- **B)** Allow `Ctrl-O` to dismiss the genre modal and open Settings directly.
  - Pro: Feels more ergonomic for power users.
  - Con: Introduces a path where the user may accidentally lose their genre-step progress.

**Agent's recommendation:** **Option A** (ignore in genre selection). Requiring an explicit
`Esc` before changing context is safer and cleaner. The two overlays should not nest or
interrupt each other.

**Human decision:** Yes we can go with Option A - we don't need to have the settings always accessible.

---

### Q3: Should the Settings overlay show a confirmation / error after a failed queue reload?

**Context:**
When the user saves new paths in the Settings overlay, `reloadQueueCmd` fires and eventually
returns a `QueueReloadedMsg`. If `Err` is non-nil (e.g. bad JSON path), the plan stores the
error in `m.lastSaveErr`, which is shown in the main review screen's status bar. However,
by the time the user sees the error they've already been returned to `StateReviewing`. An
alternative is to stay in `StateSettings` and show an inline error.

**Options:**
- **A)** Return to `StateReviewing` immediately when Enter is pressed; show any reload error
  in the status bar (via `m.lastSaveErr`). This is what the plan currently specifies.
  - Pro: Simple — reuses the existing error display mechanism.
  - Con: The user is taken back to the review screen even if the reload failed.
- **B)** Stay in `StateSettings` until the reload succeeds; show an inline error message on
  `QueueReloadedMsg{Err: ...}` and allow the user to correct the path.
  - Pro: Better UX — the user can fix the path without reopening the overlay.
  - Con: Requires adding an error field to the settings state and a new rendering path.

**Agent's recommendation:** **Option A** for simplicity. The status bar error is visible and
non-blocking. The user can press `Ctrl-O` again to reopen Settings and correct the path.
Option B can be added as a UX polish task later.

**Human decision:** Yes let's do option A for simplicity.

---

## File Manifest

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `cmd/reviewer/main.go` | Modified | Demote config-load and queue-load failures from `os.Exit` to warnings + safe defaults; add `defaultConfig()` helper |
| 2 | `internal/tui/model.go` | Modified | Add `StateSettings` to enum; add `QueueReloadedMsg` type; add `textinput` import; add `settingsMusicFolder`, `settingsJsonPath`, `settingsFocusIndex` fields to `Model`; initialise inputs in `New()`; add `reloadQueueCmd` factory |
| 3 | `internal/tui/update.go` | Modified | Handle `QueueReloadedMsg`; add `Ctrl-O` key to `StateReviewing`; add `StateSettings` branch to `handleKey`; add `openSettings()` and `handleSettingsKey()` helpers; flush `SaveState` in both `ctrl+c` handlers |
| 4 | `internal/tui/view.go` | Modified | Route `StateSettings` in `View()`; add `styleSettingsLabel` and `styleSettingsHint` styles; add `viewSettings()` renderer; add `Ctrl+O` hint to the status bar |
| 5 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Update `/internal/tui` and `/cmd/reviewer` descriptions |
| 6 | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Modified | Update "Last updated" timestamp |
| 7 | `README.md` | Modified | Add `Ctrl+O` keybinding row; add Settings section |

**Total files created: 0 | Total files modified: 7**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes with zero failures
- [ ] `cmd/reviewer/main.go`: running the binary without `config/settings.json` prints a
  warning to stderr and opens the TUI (does not crash)
- [ ] `cmd/reviewer/main.go`: `defaultConfig()` helper is present and returns non-zero
  `SeekDeltaSeconds` (30)
- [ ] `internal/tui/model.go`: `StateSettings` constant is declared; `QueueReloadedMsg` type
  is declared; `reloadQueueCmd` function is present; `Model` has `settingsMusicFolder`,
  `settingsJsonPath`, and `settingsFocusIndex` fields
- [ ] `internal/tui/update.go`: `QueueReloadedMsg` is handled in `Update`; `ctrl+c` in both
  `StateReviewing` and `StateGenreSelection` calls `m.providerRef.SaveState` before quitting;
  `ctrl+o` opens the Settings overlay; `handleSettingsKey` dispatches Tab/Enter/Esc correctly
- [ ] `internal/tui/view.go`: `View()` routes `StateSettings` to `viewSettings()`; the hint
  bar includes `Ctrl+O`
- [ ] No unrelated files were modified or deleted
- [ ] `agent-development/agent-specs/architecture-breakdown.md` updated with `StateSettings`,
  `QueueReloadedMsg`, `reloadQueueCmd`, settings overlay description, and updated
  `/cmd/reviewer` entry
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` "Last updated" line updated
- [ ] `README.md` updated with `Ctrl+O` keybinding and Settings section

---

## Notes for the Implementing Agent

1. **Read all source files before making any edit.** The existing `main.go`, `model.go`,
   `update.go`, and `view.go` are well-structured. Many things the task request describes
   as missing are already present. Read first, then make only the additions specified in
   this plan.

2. **Do not introduce Viper.** The task request mentions Viper, but the project already uses
   plain `encoding/json` for config loading — it is simpler, has no external dependency, and
   fully covers the use case. Do not add Viper to `go.mod`.

3. **`providerRef` is a value type (`ManualReviewProvider`), not a pointer.** When
   `handleSettingsKey` updates `m.providerRef.Config.MusicFolder` and
   `m.providerRef.Config.JsonPath`, it is updating a copy on the model value. This is
   correct — the model is a value type and the updated `providerRef` travels forward through
   Bubble Tea's message loop as part of the returned model.

4. **`SaveState` on `Ctrl-C` is synchronous** (called directly in `handleKey`, not via
   `tea.Cmd`). This is intentional: the process is about to exit immediately after
   `tea.Quit` is returned, so there is no event loop to dispatch a Cmd into. The error is
   silently discarded (`_`) because there is nothing useful to display at shutdown time.

5. **Do not add a `StateSettings` branch to the `StateGenreSelection` `ctrl+c` handler.**
   That handler already follows the same pattern — just add the `SaveState` call to it as
   described in Step 4.6.

6. **The `textinput` cursor-blink command.** `textinput.Focus()` returns a `tea.Cmd` for
   cursor blinking. Always return this Cmd from `openSettings()` and from the Tab-switching
   path in `handleSettingsKey`. If the Cmd is dropped, the cursor will still work but won't
   blink.

7. **`go test ./...` should still pass after this task.** No existing tests are affected —
   all changes are additive. If a test breaks, re-read the relevant source file to find the
   conflict.

8. **After completing all steps**, move `agent-development/pending/7-application-assembly-and-lifecycle.md`
   to `agent-development/done/requests/7-application-assembly-and-lifecycle.md` and move
   this plan from `agent-development/plans/7-application-assembly-and-lifecycle-plan.md` to
   `agent-development/done/plans/7-application-assembly-and-lifecycle-plan.md`.

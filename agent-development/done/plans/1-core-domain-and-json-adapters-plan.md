# Implementation Plan: Task 1 — Core Domain Models, JSON Adapter & Review Queue

## Overview

This plan implements the foundational data layer of the Song Reviewer CLI. It builds on top of the stub files created in Task 0 (project bootstrapping) and fills in three interconnected pieces:

1. **Domain models** (`internal/domain/models.go`) — defines the `Task`, `AppConfig`, and `ReviewQueue` structs that every other package in the application will depend on.
2. **JSON adapter** (`internal/provider/json_provider.go`) — defines the `TaskProvider` interface and implements `ManualReviewProvider`, which reads the user's `manual_review.json` file and maps its entries into the canonical `[]Task` slice.
3. **Unit tests** (`internal/provider/json_provider_test.go`) — verifies the adapter using a hard-coded sample JSON snippet that mirrors the real file format.

After this task, every subsequent task (TUI, audio, metadata) has stable, importable types to build against. No Bubble Tea, audio, or I/O wiring happens here — this is purely the data foundation.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does and the review queue concept |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, Adapter pattern, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/1-core-domain-and-json-adapters.md` | The task being implemented |
| Completed Plan 0 | `agent-development/done/0-initialization-plan.md` | Understand what stubs exist and what was already built |
| Settings Example | `settings.example.json` | Canonical shape of the app configuration JSON |
| Review JSON Example | `manual_review.example.json` | Canonical shape of the review queue JSON |
| README | `README.md` | Existing user-facing docs to be extended in this task |

---

## Pre-Conditions

- Task 0 (project bootstrapping) must be fully complete. Specifically:
  - `go.mod` exists at the project root declaring `module song-reviewer` and `go 1.24.4`.
  - `internal/domain/models.go` exists and contains exactly `package domain` (stub only).
  - `internal/provider/json_provider.go` exists and contains exactly `package provider` (stub only).
  - `config/settings.json` exists (a copy of `settings.example.json`).
  - `data/manual_review.json` exists (a copy of `manual_review.example.json`).
  - `go build ./...` currently succeeds with zero errors.
- No Go dependencies beyond the standard library are required for this task. All code in this task uses only `encoding/json`, `fmt`, `os`, `path/filepath`, and `testing`.

---

## Step-by-Step Implementation

### Step 1: Implement `internal/domain/models.go`

**Action:** Replace the stub content of `internal/domain/models.go` with the full implementation below. This file must remain free of any imports from outside the standard library — it is a pure data package.

The file must define three types:

**1a. `Task` struct**

Represents a single song in the review queue. Every field maps either to a JSON source field or to a value derived during loading (e.g., `FilePath` is the resolved absolute path).

```go
// Task represents a single song to be reviewed and tagged.
type Task struct {
    FilePath string // Absolute path to the audio file on disk.
    Title    string // Song title (from ID3 tags or filename, populated later).
    Artist   string // Artist name (from ID3 tags, populated later).
    Album    string // Album name (from ID3 tags, populated later).
    Genre1   string // Primary genre — set by the user during review.
    Genre2   string // Secondary genre — set by the user during review. May be empty.
    Year     string // Release year — fetched from MusicBrainz or set manually.
    BPM      string // Beats per minute — fetched from BPM API or set manually.
}
```

**1b. `AppConfig` struct**

Represents the parsed contents of `config/settings.json`. Field names use `json` struct tags that match the exact keys in `settings.example.json`.

```go
// AppConfig holds the application-wide configuration loaded from settings.json.
type AppConfig struct {
    MusicFolder    string   `json:"music_folder"`
    JsonPath       string   `json:"review_json_path"`
    GenreList      []string `json:"genres"`
}
```

**1c. `ReviewQueue` struct**

Manages the ordered list of tasks and supports the Undo feature described in the application overview. The undo mechanism is a simple slice-based history stack — per agent instructions, no over-engineering.

```go
// ReviewQueue tracks the current position in the review queue and supports undo.
type ReviewQueue struct {
    Tasks        []Task // Ordered list of all tasks loaded from the JSON file.
    CurrentIndex int    // Index of the task currently being reviewed.
    History      []Task // Stack of tasks that have been completed; used for Undo.
}
```

**Full file content to write:**

```go
package domain

// Task represents a single song to be reviewed and tagged.
type Task struct {
	FilePath string // Absolute path to the audio file on disk.
	Title    string // Song title (from ID3 tags or filename; populated later).
	Artist   string // Artist name (from ID3 tags; populated later).
	Album    string // Album name (from ID3 tags; populated later).
	Genre1   string // Primary genre — set by the user during review.
	Genre2   string // Secondary genre — set by the user during review. May be empty.
	Year     string // Release year — fetched from MusicBrainz or set manually.
	BPM      string // Beats per minute — fetched from BPM API or set manually.
}

// AppConfig holds the application-wide configuration loaded from settings.json.
type AppConfig struct {
	MusicFolder string   `json:"music_folder"`
	JsonPath    string   `json:"review_json_path"`
	GenreList   []string `json:"genres"`
}

// ReviewQueue tracks the current position in the review queue and supports undo.
type ReviewQueue struct {
	Tasks        []Task // Ordered list of all tasks loaded from the JSON file.
	CurrentIndex int    // Index of the task currently being reviewed.
	History      []Task // Stack of completed tasks; used for Undo (Ctrl+U).
}
```

**Expected outcome:** `internal/domain/models.go` contains the three types above — no imports, no functions, just the struct declarations.

**Verification:** Run `go build ./internal/domain/` from the project root. It must succeed with zero errors and zero warnings.

---

### Step 2: Implement `internal/provider/json_provider.go`

**Action:** Replace the stub content of `internal/provider/json_provider.go` with the full implementation below.

This file has three responsibilities:
1. Define the `TaskProvider` interface.
2. Define the `ManualReviewProvider` struct that holds the config needed to do the parsing.
3. Implement `GetTasks()` on `ManualReviewProvider` — read the JSON file, parse its `manual_review` array, and map each entry to a `domain.Task`, resolving `filepath` to a full absolute path by joining it with `AppConfig.MusicFolder`.

**JSON schema being parsed** (from `manual_review.example.json` / `data/manual_review.json`):

```json
{
  "manual_review": [
    {
      "filepath": "Cream/Cream - Strange Brew.mp3",
      "reason": "Genre not in taxonomy",
      "confidence": 4
    }
  ]
}
```

The `reason` and `confidence` fields are informational — they are present in the JSON for human readability but do not map into `domain.Task`. They must be parsed correctly (so the JSON decoder does not error), but they do not need to be stored in any exported struct.

**Internal helper type for JSON decoding:**

A private struct `manualReviewFile` (unexported) is used to deserialize the raw JSON. It must not be exported — callers interact only with `[]domain.Task`.

A private struct `reviewEntry` (unexported) represents a single element in the `manual_review` array.

**Full file content to write:**

```go
package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
type reviewEntry struct {
	FilePath   string  `json:"filepath"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
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
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
```

**Expected outcome:** `internal/provider/json_provider.go` contains the `TaskProvider` interface, the unexported JSON helper structs, the `ManualReviewProvider` struct, and its `GetTasks()` method. The file compiles cleanly.

**Verification:** Run `go build ./internal/provider/` from the project root. It must succeed with zero errors.

---

### Step 3: Write `internal/provider/json_provider_test.go`

**Action:** Create a new file `internal/provider/json_provider_test.go`. This file does not exist yet — it must be created from scratch.

The test must use a hard-coded JSON string (not read from disk) so it is self-contained and reproducible in any environment. Use `os.WriteFile` to write the JSON to a temporary file (via `os.MkdirTemp` / `t.TempDir()`), construct a `ManualReviewProvider` pointing at it, call `GetTasks()`, and assert the results.

**Test cases to cover:**

1. **`TestGetTasks_HappyPath`** — a valid JSON with two entries. Verifies:
   - The returned slice has exactly 2 elements.
   - `tasks[0].FilePath` equals `filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3")`.
   - `tasks[1].FilePath` equals `filepath.Join("/test/music", "Miles Davis/Kind of Blue.mp3")`.
   - All other `Task` fields (`Title`, `Artist`, `Album`, `Genre1`, `Genre2`, `Year`, `BPM`) are empty strings (they are not populated by the provider).

2. **`TestGetTasks_EmptyArray`** — a valid JSON with an empty `manual_review` array. Verifies:
   - The returned slice is non-nil and has length 0.
   - No error is returned.

3. **`TestGetTasks_FileNotFound`** — `Config.JsonPath` points to a path that does not exist. Verifies:
   - An error is returned (non-nil).
   - The returned slice is nil.

4. **`TestGetTasks_MalformedJSON`** — the JSON file contains invalid JSON (`{bad json`). Verifies:
   - An error is returned (non-nil).
   - The returned slice is nil.

**Full file content to write:**

```go
package provider

import (
	"os"
	"path/filepath"
	"testing"

	"song-reviewer/internal/domain"
)

func TestGetTasks_HappyPath(t *testing.T) {
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
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() returned unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	want0 := filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3")
	if tasks[0].FilePath != want0 {
		t.Errorf("tasks[0].FilePath = %q, want %q", tasks[0].FilePath, want0)
	}

	want1 := filepath.Join("/test/music", "Miles Davis/Kind of Blue.mp3")
	if tasks[1].FilePath != want1 {
		t.Errorf("tasks[1].FilePath = %q, want %q", tasks[1].FilePath, want1)
	}

	// Fields not populated by the provider must be empty strings.
	emptyFields := []struct {
		name  string
		value string
	}{
		{"Title", tasks[0].Title},
		{"Artist", tasks[0].Artist},
		{"Album", tasks[0].Album},
		{"Genre1", tasks[0].Genre1},
		{"Genre2", tasks[0].Genre2},
		{"Year", tasks[0].Year},
		{"BPM", tasks[0].BPM},
	}
	for _, f := range emptyFields {
		if f.value != "" {
			t.Errorf("tasks[0].%s = %q, want empty string", f.name, f.value)
		}
	}
}

func TestGetTasks_EmptyArray(t *testing.T) {
	const sampleJSON = `{"manual_review": []}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() returned unexpected error: %v", err)
	}
	if tasks == nil {
		t.Fatal("expected non-nil slice for empty array, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestGetTasks_FileNotFound(t *testing.T) {
	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    "/nonexistent/path/that/does/not/exist.json",
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err == nil {
		t.Fatal("expected an error for missing file, got nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
}

func TestGetTasks_MalformedJSON(t *testing.T) {
	const badJSON = `{bad json`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(jsonPath, []byte(badJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
}
```

**Expected outcome:** `internal/provider/json_provider_test.go` exists and all four test functions are defined.

**Verification:** Run `go test ./internal/provider/` from the project root. All four tests must pass with output similar to:

```
ok  	song-reviewer/internal/provider	0.XXXs
```

---

### Step 4: Update `agent-specs/architecture-breakdown.md`

**Action:** Open `agent-specs/architecture-breakdown.md` and update the description of the `/internal/provider` entry under **Folder Structure** to document the newly defined `TaskProvider` interface. Also add a note under **Design Patterns → Adapter Pattern** to name the concrete type.

**Locate the existing line:**
```
- `/internal/provider`: Implementation of JSON parsers (Adapters).
```

**Replace it with:**
```
- `/internal/provider`: Defines the `TaskProvider` interface (`GetTasks() ([]Task, error)`) and implements `ManualReviewProvider`, which parses the `manual_review` JSON schema and resolves file paths against `MusicFolder`.
```

**Locate the existing Adapter Pattern paragraph:**
```
2. **Adapter Pattern:** The `provider` package must use an interface to allow the app to read different JSON schemas (e.g., `manual_review` vs. a simple file list) without changing the TUI code.
```

**Replace it with:**
```
2. **Adapter Pattern:** The `provider` package exposes a `TaskProvider` interface (`GetTasks() ([]domain.Task, error)`). The concrete implementation `ManualReviewProvider` parses the `manual_review` JSON schema. New schemas (e.g., a flat file list) can be added by implementing the same interface without touching any TUI code.
```

**Expected outcome:** `agent-specs/architecture-breakdown.md` contains the updated descriptions above. No other content in the file is changed.

**Verification:** Read the file and confirm both updated passages are present.

---

### Step 5: Update `README.md`

**Action:** The existing `README.md` already documents the review JSON format and the settings file from Task 0. Two targeted additions are needed:

**5a. Add a "Review JSON Fields" table that includes `status`.**

The application overview states that the source JSON is updated with "Applied" status after a successful tag write. The README's **Review JSON Format** section should be expanded to note this. Find the existing field table under `## Review JSON Format` and add a row for `status`:

Existing table:
```markdown
| Field | Description |
|---|---|
| `filepath` | Path to the audio file, relative to `music_folder`. |
| `reason` | Why this song was flagged for manual review. |
| `confidence` | Numeric confidence score from the automated classifier (informational). |
```

Replace with:
```markdown
| Field | Description |
|---|---|
| `filepath` | Path to the audio file, relative to `music_folder`. |
| `reason` | Why this song was flagged for manual review. |
| `confidence` | Numeric confidence score from the automated classifier (informational). |
| `status` | Written back by the app after a successful tag write. Value will be `"Applied"`. Omit or leave blank for pending songs. |
```

**5b. Add a `settings.json` fields table that documents all keys.**

The existing settings reference table already exists. No change is needed — it was already written correctly in Task 0. Verify the table is present and contains `music_folder`, `review_json_path`, `genres`, and `api_keys.musicbrainz_user_agent`. If any of these four rows are missing, add them. Otherwise leave it unchanged.

**Expected outcome:** `README.md` contains the four-row `status` field table under **Review JSON Format**, and the settings reference table contains all four required fields.

**Verification:** Read the relevant sections of `README.md` and confirm the additions are present. No other sections of the README should be modified.

---

### Step 6: Final Build and Test Verification

**Action:** Run the following commands from the project root in order. Each must succeed before proceeding to the next.

**6.1 — Full build:**
```bash
go build ./...
```
Expected: exits with code 0, no output.

**6.2 — Full test suite:**
```bash
go test ./...
```
Expected: all tests pass. Output must include a passing line for `song-reviewer/internal/provider`. Other packages that have no test files will show `[no test files]`, which is acceptable.

**6.3 — Verify domain models file is non-empty:**
```bash
grep -c "type Task struct" internal/domain/models.go
```
Expected: prints `1`.

**6.4 — Verify interface is defined in provider:**
```bash
grep -c "TaskProvider" internal/provider/json_provider.go
```
Expected: prints a number ≥ 2 (declaration + method signature).

**6.5 — Verify test file exists:**
```bash
ls internal/provider/json_provider_test.go
```
Expected: the file path is printed with no error.

---

## Open Questions & Decisions

### Q1: Should `AppConfig` include `api_keys` fields?

**Context:** The `settings.example.json` file contains an `api_keys` object with a `musicbrainz_user_agent` string. The task request defines `AppConfig` as having only `MusicFolder`, `JsonPath`, and `GenreList []string`. However, `api_keys` is a real field in the settings file and the JSON decoder will silently ignore unknown fields by default — meaning that if `AppConfig` never defines it, the value will be permanently inaccessible from the config struct. A later task (the MusicBrainz API integration) will need this value. There are two options:

**Options:**
- **A)** Add `ApiKeys` to `AppConfig` now — include a nested `ApiKeys struct { MusicBrainzUserAgent string \`json:"musicbrainz_user_agent"\` } \`json:"api_keys"\`` field. This makes the config complete and prevents a future task from needing to re-open `models.go` to add it.
- **B)** Strictly implement only what the task request specifies — omit `api_keys` from `AppConfig` now. A future task adds it when needed. This keeps the scope of this task tight.

**Agent's recommendation:** **A** — Adding the `ApiKeys` nested struct now is low cost (4 extra lines), prevents a later plan from having to modify the same struct, and ensures `go vet` / `json.Unmarshal` behaves predictably against the real settings file from day one. A partial config struct that silently drops known fields is a maintenance hazard.

**Human decision:** `PENDING`

---

### Q2: Should `ReviewQueue` live in `domain/models.go` or a separate file `domain/queue.go`?

**Context:** The task request groups `ReviewQueue` with `Task` and `AppConfig` in a single `models.go`. All three are pure data structures with no methods in this task. However, in later tasks `ReviewQueue` may gain methods (e.g., `Advance()`, `Undo()`, `Current()`). If that happens, keeping it in the same file as `Task` and `AppConfig` could make `models.go` large and mixed-purpose.

**Options:**
- **A)** Single file `domain/models.go` — all three types in one file. Simple, matches the task request literally, and is the right call while the types are all just data.
- **B)** Separate file `domain/queue.go` — `ReviewQueue` (and any future methods on it) in its own file. Better separation of concerns if methods are added later, but adds a file for what is currently a 5-line struct.

**Agent's recommendation:** **A** — Keep everything in `models.go` for now. Go's convention is to split files when content justifies it, not preemptively. If `ReviewQueue` gains methods in a later task, that task's plan can split the file at that point. Premature splitting adds navigational overhead for no current gain.

**Human decision:** `PENDING`

---

### Q3: Should `GetTasks()` return an error when the `manual_review` array is empty?

**Context:** An empty `manual_review` array (`[]`) is valid JSON and may be an intentional state (all songs have been reviewed). However, from the application's perspective, launching the reviewer with zero tasks is a degenerate case that could cause a crash or confusing UI if not handled. There are two reasonable behaviors:

**Options:**
- **A)** Return `([]domain.Task{}, nil)` — treat an empty array as valid. The caller (TUI or main) is responsible for detecting the empty state and showing a "Nothing to review" message. This is the most correct and composable behavior for a library function.
- **B)** Return `(nil, error)` — treat an empty array as an error condition, letting the provider signal "no work to do." This conflates an application-level condition with a data-layer error, which is an anti-pattern.

**Agent's recommendation:** **A** — Empty is not an error. The provider's job is to parse and map; business-logic decisions about what to do with an empty queue belong in the TUI layer. The test plan above already tests for this case with option A behavior.

**Human decision:** `PENDING`

---

### Q4: Should `reviewEntry.Confidence` be typed as `float64` or `int`?

**Context:** In `manual_review.example.json`, `confidence` values are integers (e.g., `4`, `2`). However, JSON numbers are natively floating-point, and `encoding/json` will correctly unmarshal an integer JSON value into a `float64` field. If the field is typed as `int`, a JSON value like `4.5` would cause an unmarshal error. Since `confidence` is informational and never stored in `domain.Task`, the choice has no downstream impact — but it does affect how defensive the parser is.

**Options:**
- **A)** `float64` — accepts both integer and decimal JSON numbers without error. More robust. Slightly less semantically precise.
- **B)** `int` — matches the apparent intent of the data (whole-number confidence scores). Will error on any decimal value in the input.

**Agent's recommendation:** **A** (`float64`) — since `confidence` is only parsed to avoid a JSON decode error and is then discarded, robustness beats precision. This prevents a future breakage if the upstream script ever writes `3.5`.

**Human decision:** `PENDING`

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/domain/models.go` | Modified | Replaces stub. Defines `Task`, `AppConfig`, and `ReviewQueue` structs. |
| 2 | `internal/provider/json_provider.go` | Modified | Replaces stub. Defines `TaskProvider` interface, unexported JSON helper structs, `ManualReviewProvider` struct, and `GetTasks()` method. |
| 3 | `internal/provider/json_provider_test.go` | Created | Four unit tests covering happy path, empty array, file-not-found, and malformed JSON. |
| 4 | `agent-specs/architecture-breakdown.md` | Modified | Updates `/internal/provider` folder description and Adapter Pattern description to name `TaskProvider` interface and `ManualReviewProvider`. |
| 5 | `README.md` | Modified | Adds `status` field row to the Review JSON Format table. |

**Total files created: 1 | Total files modified: 4**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors from the project root.
- [ ] `go test ./...` passes; `song-reviewer/internal/provider` shows `ok` status.
- [ ] `internal/domain/models.go` defines exactly three exported types: `Task`, `AppConfig`, `ReviewQueue`.
- [ ] `internal/provider/json_provider.go` defines the exported `TaskProvider` interface and exported `ManualReviewProvider` struct.
- [ ] `internal/provider/json_provider_test.go` exists and contains all four test functions.
- [ ] `agent-specs/architecture-breakdown.md` updated to document `TaskProvider` interface and `ManualReviewProvider`.
- [ ] `README.md` updated with `status` field documentation in the Review JSON Format table.
- [ ] No unrelated files were modified or deleted.
- [ ] No new Go module dependencies were introduced (`go.mod` and `go.sum` are unchanged).

---

## Notes for the Implementing Agent

1. **Do not add any methods to `ReviewQueue` in this task.** Methods like `Advance()`, `Undo()`, and `Current()` are intentionally deferred to the TUI task, where their interaction with Bubble Tea messages can be designed holistically.

2. **Do not add a `LoadConfig()` function in this task.** Config loading (reading `config/settings.json`) belongs in `cmd/reviewer/main.go` or a dedicated config loader, which is out of scope here. The `AppConfig` struct is defined now so it can be imported, but no file I/O for config belongs in the domain package.

3. **The `domain` package must have zero imports.** It is a pure data package. If you find yourself writing an `import` statement in `internal/domain/models.go`, stop — you have made a design error.

4. **The `provider` package imports `domain`, never the other way around.** The dependency direction is strictly one-way: `provider` → `domain`. The `tui` and other packages will later import both.

5. **`FilePath` in `Task` is always an absolute path by the time it leaves `GetTasks()`.** The `filepath.Join(musicFolder, relativePath)` call in `ManualReviewProvider.GetTasks()` is the single point where relative paths become absolute. Every downstream consumer of `domain.Task` may assume `FilePath` is absolute and ready to pass to `os.Open`.

6. **The test file uses `package provider` (not `package provider_test`).** This is a white-box test that needs access to the unexported `manualReviewFile` and `reviewEntry` types to verify internal parsing behavior if needed. Using the same package name is the standard Go approach for unit tests that need internal access.

7. **Resolve the Open Questions before executing.** All four questions are marked `PENDING` and must be resolved by the human reviewer before this plan moves to `queued/`. In particular, Q1 (whether to include `ApiKeys` in `AppConfig`) directly affects the content of `internal/domain/models.go` Step 1. If the human chooses option A for Q1, add the following nested struct to `AppConfig`:
   ```go
   ApiKeys struct {
       MusicBrainzUserAgent string `json:"musicbrainz_user_agent"`
   } `json:"api_keys"`
   ```

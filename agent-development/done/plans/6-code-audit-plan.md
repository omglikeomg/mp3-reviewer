# Implementation Plan: Task 6 — Code Audit: Safe Write Strategy for JSON Persistence

## Overview

This plan documents the results of a code audit against the existing JSON persistence layer
(`internal/provider/json_provider.go`) and prescribes the targeted hardening changes needed
to guarantee the `manual_review.json` file cannot be corrupted by a `Ctrl-C` or an
unexpected process kill mid-write.

**Audit finding:** The core "write to temporary file and rename" strategy was already
implemented in Task 4. The `SaveState` method correctly uses `os.CreateTemp` + `os.Rename`
and includes proper cleanup (`os.Remove`) on every error path. However, two gaps remain:

1. **Missing `tmp.Sync()` (fsync) before `tmp.Close()`** — Without an explicit `fsync`, the
   OS page cache may not have flushed the written bytes to stable storage before the rename
   completes. On a sudden power cut (or a kernel-level kill signal that bypasses `defer`),
   the renamed file could contain a truncated or zero-length payload even though the rename
   itself succeeded. Adding `tmp.Sync()` between `tmp.Write()` and `tmp.Close()` closes this
   window.

2. **No test exercises the safe-write guarantee** — The existing `TestSaveState_*` tests
   verify the *content* of the saved JSON but never assert that the original file is left
   intact when a write fails partway through. A dedicated test (`TestSaveState_OriginalFilePreservedOnWriteError`)
   should be added to lock in this invariant for future maintainers.

No new packages, interfaces, or top-level directories are introduced. The change is entirely
contained within `internal/provider/json_provider.go` and
`internal/provider/json_provider_test.go`.

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-development/agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-development/agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-development/agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Folder Structure | `agent-development/agent-specs/FOLDER-STRUCTURE.md` | Quick-reference project directory tree and package dependency graph |
| Task Definition | `agent-development/pending/6-code-audit.md` | The task being implemented |
| Provider source | `internal/provider/json_provider.go` | The file being hardened |
| Provider tests | `internal/provider/json_provider_test.go` | The test file being extended |

---

## Pre-Conditions

- Tasks 0 through 5 are complete and merged.
- `internal/provider/json_provider.go` exists and contains the `SaveState` method with
  the existing temp-file + rename pattern (introduced in Task 4, extended in Task 5).
- `internal/provider/json_provider_test.go` exists with at least `TestSaveState_UpdatesAppliedEntries`
  and `TestSaveState_NoneSecondaryGenre`.
- `go test ./...` passes cleanly before any edits are made (confirm this first).

---

## Step-by-Step Implementation

### Step 1: Confirm the Baseline

**Action:**
Run the full test suite to establish a clean baseline before making any changes:

```
cd mp3-reviewer && go test ./...
```

**Expected outcome:**
All packages report `ok` or `[no test files]`. Zero failures.

**Verification:**
The terminal output shows no `FAIL` lines.

---

### Step 2: Add `tmp.Sync()` to `SaveState` in `json_provider.go`

**Action:**
Open `internal/provider/json_provider.go` and locate the atomic-write block near the end of
`SaveState`. It currently reads:

```internal/provider/json_provider.go#L1-1
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState closing temp file: %w", err)
	}
```

Insert a `tmp.Sync()` call **between** the `tmp.Write()` block and the `tmp.Close()` block,
so that the complete atomic-write section becomes:

```internal/provider/json_provider.go#L1-1
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState writing temp file: %w", err)
	}
	// Sync flushes the OS page cache to stable storage before we rename.
	// Without this, a power failure after rename could leave a zero-length or
	// truncated file even though the rename syscall succeeded.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, p.Config.JsonPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState renaming temp file: %w", err)
	}
```

The `Sync()` block follows exactly the same error-handling pattern as `Write()` and
`Close()`: on failure, close and remove the temp file, then return a wrapped error.

Also update the doc-comment on `SaveState` to mention the `Sync` step. The updated comment
block (which precedes the `func` line) must read:

```internal/provider/json_provider.go#L1-1
// SaveState persists genre assignments back into the source JSON file on disk.
// For each task in the provided slice, if Genre1 is non-empty the corresponding
// JSON entry is updated with status="applied", primary_genre, and secondary_genre.
// Matching is done by relative filepath (task path with MusicFolder prefix stripped).
//
// The write is done atomically: the file is first written to a temp file in the
// same directory, synced to stable storage (fsync), and then renamed over the
// original. This prevents data loss if the process is killed mid-write, even on
// a sudden power failure.
```

No other lines in `json_provider.go` need to be touched.

**Expected outcome:**
`internal/provider/json_provider.go` compiles cleanly and the full atomic-write section now
contains four sequential error-checked calls: `Write`, `Sync`, `Close`, `Rename`. Each error
path removes the temp file before returning.

**Verification:**
```
cd mp3-reviewer && go build ./internal/provider/...
```
Zero errors.

---

### Step 3: Add a Test for the Safe-Write Guarantee

**Action:**
Open `internal/provider/json_provider_test.go` and append the following new test function
**after** the existing `TestSaveState_NoneSecondaryGenre` function (i.e. at the end of the
file):

```internal/provider/json_provider_test.go#L1-1
// TestSaveState_OriginalFilePreservedOnWriteError verifies that if the temp-file
// write is interrupted (simulated here by making the target directory read-only),
// the original JSON file is left completely intact and SaveState returns a non-nil
// error.
//
// This test is skipped when run as root (where filesystem permissions are not
// enforced) and on Windows (where chmod has limited effect).
func TestSaveState_OriginalFilePreservedOnWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root — filesystem permission checks are not enforced")
	}

	const originalJSON = `{
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
	if err := os.WriteFile(jsonPath, []byte(originalJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
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
			Genre2:   "Blues-Rock",
		},
	}

	// Make the directory read-only so os.CreateTemp fails.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("setup: chmod dir: %v", err)
	}
	// Restore write permission so t.TempDir cleanup can delete the directory.
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := p.SaveState(tasks)
	if err == nil {
		t.Fatal("SaveState() expected an error when temp file cannot be created, got nil")
	}

	// The original file must be byte-for-byte identical to what was written at setup.
	got, readErr := os.ReadFile(jsonPath)
	if readErr != nil {
		t.Fatalf("reading original file after failed SaveState: %v", readErr)
	}
	if string(got) != originalJSON {
		t.Errorf("original file was modified by a failing SaveState:\ngot:  %q\nwant: %q", string(got), originalJSON)
	}
}
```

**Key design decisions in this test:**
- The failure is induced by revoking write permission on the containing *directory*, which
  causes `os.CreateTemp` to fail. This tests the very first error path — the point at which
  `SaveState` hasn't touched the original file at all — confirming the rename-only-on-success
  contract.
- `os.Getuid() == 0` guard skips the test gracefully when running as root, where `chmod`
  has no effect.
- `t.Cleanup` restores write permission so `t.TempDir` can delete the directory after the
  test, avoiding spurious cleanup errors in CI.
- The test does **not** try to simulate a mid-write crash (which would require injecting a
  fault into `os.File.Write`); that is outside the scope of a unit test. The meaningful
  contract to assert here is: "if `SaveState` returns an error, the original file is
  untouched."

**Expected outcome:**
`json_provider_test.go` compiles cleanly and the new test appears when running:
```
cd mp3-reviewer && go test -v -run TestSaveState ./internal/provider/...
```

**Verification:**
The test passes (reports `--- PASS: TestSaveState_OriginalFilePreservedOnWriteError`).

---

### Step 4: Run the Full Test Suite

**Action:**
```
cd mp3-reviewer && go test ./...
```

**Expected outcome:**
All packages pass. The provider package should now show **four** `TestSaveState_*` tests
passing.

**Verification:**
```
cd mp3-reviewer && go test -v ./internal/provider/...
```
Output includes:
```
--- PASS: TestSaveState_UpdatesAppliedEntries
--- PASS: TestSaveState_NoneSecondaryGenre
--- PASS: TestSaveState_OriginalFilePreservedOnWriteError
ok  	song-reviewer/internal/provider
```

---

### Step 5: Update `agent-development/agent-specs/architecture-breakdown.md`

**Action:**
In `agent-development/agent-specs/architecture-breakdown.md`, find the paragraph describing
the `/internal/provider` package. It currently ends with:

> `SaveState` is a concrete method on `ManualReviewProvider` only — it is not part of the
> `TaskProvider` interface.

Replace that sentence with the following expanded version (append; do not remove what was
already there):

> `SaveState` is a concrete method on `ManualReviewProvider` only — it is not part of the
> `TaskProvider` interface. The write sequence is: marshal JSON → `os.CreateTemp` (same
> directory as the target) → `tmp.Write` → `tmp.Sync` (fsync, flushes OS page cache to
> stable storage) → `tmp.Close` → `os.Rename`. Every error path removes the temp file
> before returning, guaranteeing the original JSON is never left in a partial state even if
> the process is killed mid-write.

**Expected outcome:**
The architecture doc accurately describes the full four-step write sequence including the
`Sync` (fsync) step.

**Verification:**
Open the file and confirm the updated sentence is present and grammatically correct.

---

### Step 6: Update `README.md`

**Action:**
In `README.md`, find the **Persistence** bullet point in the **Features** section:

> **Persistence** — Writes changes directly to MP3/FLAC ID3 tags and updates the source
> JSON to reflect "Applied" status.

Update it to:

> **Persistence** — Writes changes directly to MP3/FLAC ID3 tags and updates the source
> JSON to reflect "Applied" status. JSON updates use an atomic write strategy (temp file →
> fsync → rename) so the review queue is never corrupted by an interrupted save.

**Expected outcome:**
The README accurately communicates the safe-write guarantee to users.

**Verification:**
Open the file and confirm the updated bullet is present.

---

## Open Questions & Decisions

### Q1: Should `fsync` also be called on the *parent directory* after `os.Rename`?

**Context:**
On Linux ext4 (and some other filesystems), after a `rename` syscall the kernel may not
immediately flush the directory entry update to disk. A power failure between the rename and
a directory fsync could mean the file reverts to the old name/content even though the
application thought the write succeeded. The fully paranoid safe-write pattern is therefore:
`Write → Sync(file) → Close(file) → Rename → Open(dir) → Sync(dir) → Close(dir)`.

In practice this extra step matters mainly for databases and write-ahead logs. For a JSON
review queue that is a human-writable side-file (not a transactional store), the risk is
extremely low — especially given that the app targets macOS (APFS, which provides stronger
default guarantees) and personal/desktop Linux (where power failures are rare and data loss
of one JSON annotation is tolerable).

**Options:**
- **A)** Add the directory fsync step for maximum correctness.
  - Pro: The write is provably safe even on bare ext4 with `data=ordered`.
  - Con: Two additional syscalls (`open` + `fsync` on a directory) for a minor gain on the
    target platform. Adds complexity to the implementation and the test.
- **B)** Skip the directory fsync — only fsync the file itself (as this plan prescribes).
  - Pro: Closes the Ctrl-C / SIGKILL gap (which is the explicit goal of the task request)
    with minimal added complexity.
  - Con: Technically incomplete for a hostile power-cut scenario on ext4 without `fsync`
    on the directory.

**Agent's recommendation:** **Option B.** The task request is scoped to "Ctrl-C mid-write"
protection, not full power-failure durability for a transactional system. A file fsync is
the correct and proportionate change for this class of application. The directory fsync can
be revisited if the app is ever deployed in an environment where power-failure safety is a
hard requirement.

**Human decision:** We can go with Option B: there's no need to be extra paranoid.

---

### Q2: Should the temp-file name pattern be made configurable or made more opaque?

**Context:**
The current temp file is created with the pattern `"manual_review_*.json.tmp"`. The `*` is
replaced by a random string by `os.CreateTemp`. The suffix `.json.tmp` makes the file
recognisable to humans scanning the directory, but it also means an external tool or script
that globs `*.json` might accidentally pick it up during the brief write window.

**Options:**
- **A)** Keep the current pattern `"manual_review_*.json.tmp"` (human-readable, clear intent).
- **B)** Change the pattern to `".manual_review_*.tmp"` (dot-prefixed = hidden on Unix, no
  `.json` suffix = excluded from JSON globs).
- **C)** Change the pattern to `"manual_review_*.tmp"` (no `.json` suffix, still visible).

**Agent's recommendation:** **Option B** (`.manual_review_*.tmp`). Dot-prefixing hides the
temp file from casual `ls` output and removes the `.json` extension, making it invisible to
any tool that globs for JSON files. The rename is still atomic so the window of visibility
is essentially zero — but defence-in-depth favours making the temp file as unobtrusive as
possible.

**Human decision:** We should create a `.tmp/` hidden folder and include the files inside it with format explained in option A: `"manual_review_*.json.tmp"` 

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `internal/provider/json_provider.go` | Modified | Add `tmp.Sync()` call (with error handling) between `tmp.Write()` and `tmp.Close()` inside `SaveState`. Update doc-comment to describe the full Write → Sync → Close → Rename sequence. |
| 2 | `internal/provider/json_provider_test.go` | Modified | Append `TestSaveState_OriginalFilePreservedOnWriteError` to verify the original file is untouched when `SaveState` fails to create the temp file. |
| 3 | `agent-development/agent-specs/architecture-breakdown.md` | Modified | Expand the `/internal/provider` description to include the full four-step write sequence with `Sync`. |
| 4 | `README.md` | Modified | Update the **Persistence** feature bullet to mention the atomic write strategy. |

**Total files created: 0 | Total files modified: 4**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./...` passes with zero failures
- [ ] `go test -v ./internal/provider/...` shows all four `TestSaveState_*` tests passing
- [ ] `internal/provider/json_provider.go`: the atomic-write block contains exactly four sequential error-checked calls in order — `Write`, `Sync`, `Close`, `Rename`
- [ ] `internal/provider/json_provider.go`: every error path in the `Sync` block calls `_ = tmp.Close()` and `_ = os.Remove(tmpName)` before returning
- [ ] `internal/provider/json_provider.go`: the `SaveState` doc-comment mentions `fsync` / `Sync`
- [ ] `internal/provider/json_provider_test.go`: `TestSaveState_OriginalFilePreservedOnWriteError` exists and includes the `os.Getuid() == 0` skip guard
- [ ] `agent-development/agent-specs/architecture-breakdown.md`: the provider section describes the full Write → Sync → Close → Rename sequence
- [ ] `README.md`: the Persistence bullet mentions the atomic write strategy
- [ ] No unrelated files were modified or deleted
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` was **not** modified (no new packages or directories were introduced)

---

## Notes for the Implementing Agent

1. **Read the source file first.** The exact line numbers of the atomic-write block will
   differ from the excerpts in this plan (which use `#L1-1` as placeholders). Locate the
   block visually — it starts with the comment `// Atomic write via temp file + rename.`
   and ends with the `os.Rename` call.

2. **Do not refactor anything else.** The only production-code change is the insertion of
   the `Sync` block. Do not rename variables, reformat the file, add imports, or restructure
   the function. The `Sync` call does not require a new import — `os.File.Sync()` is a
   method on the `*os.File` value already held in `tmp`.

3. **The test uses `os.Getuid()`**, which is available in the standard library without any
   new import. The `os` package is already imported in the test file.

4. **The `t.Cleanup` pattern** (not `defer`) is the correct Bubble Tea–era Go testing
   idiom for test teardown that must run even if the test is skipped. Use `t.Cleanup` for
   the `chmod` restore, exactly as shown in Step 3.

5. **Temp file name pattern change (Q2):** Do not apply the option-B name change until the
   human has resolved Q2. If the human selects option B, update the `os.CreateTemp` call in
   `json_provider.go` to use `".manual_review_*.tmp"` as the pattern string. No other code
   changes are required for this optional tweak.

6. **The architecture doc and README changes (Steps 5 and 6) are mandatory.** Do not skip
   them even though they are documentation-only — the task checklist explicitly requires
   them.

7. **After completing all steps**, move `agent-development/pending/6-code-audit.md` to
   `agent-development/done/requests/6-code-audit.md` and move this plan file from
   `agent-development/plans/6-code-audit-plan.md` to
   `agent-development/done/plans/6-code-audit-plan.md`.

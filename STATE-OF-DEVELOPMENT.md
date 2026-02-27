State of Development

### Overall: ~82% complete as a production-ready personal tool

The core loop — load queue → play → tag → persist — is **fully implemented and working**. For the stated purpose (a personal CLI tool to manually review and tag a music library), it's genuinely usable today. The remaining 18% is mostly hardening, edge cases, and polish rather than missing features.

---

## Breakdown by Dimension

### ✅ Core Features — 95%

Everything in the README works:
- Queue loading, ID3 tag reads, atomic JSON persistence
- Audio playback, seeking, pause/resume
- Two-step genre selection modal → writes TCON + TXXX frames
- BPM (MusicBrainz fetch + 8-tap tempo fallback) → writes TBPM
- Year (MusicBrainz) → writes TYER/TDRC
- Undo (Ctrl+U)
- In-app settings overlay with live queue reload

The 5% gap: only MP3 is tested/documented — the README mentions "MP3/FLAC" but `beep` and `id3v2` have different code paths for FLAC. That claim is currently unverified.

### ✅ Code Quality & Architecture — 90%

- Clean MVU separation (model/update/view never bleed into each other)
- Proper interface boundaries (`TaskProvider`, `AudioPlayer`)
- Atomic writes everywhere, no data loss risk
- Idiomatic Go throughout (no CGO, no goroutine leaks, value-type model)
- 45 tests across 4 packages

The 10% gap: `internal/metadata` has **zero tests** for `WriteTags`, `WriteBPM`, `WriteYear`, and `ReadTags` — the most critical data-mutating code in the app writes to real files with no test coverage.

### ⚠️ Test Coverage — 60%

| Package | Tests | Quality |
|---|---|---|
| `internal/api` | 17 tests | Excellent — full httptest mocking |
| `internal/audio` | 5 tests | Thin — only headless/no-device paths |
| `internal/provider` | 7 tests | Good — happy path + error cases |
| `internal/tui` | 10 tests | Good — key handlers + state transitions |
| `internal/metadata` | **0 tests** | ❌ None |
| `cmd/reviewer` | 0 tests | Expected for entry point |

The `audio` tests can't exercise `Play()` in CI (no device), which is acceptable. But `metadata` writing to temp MP3 files is fully testable and currently unprotected.

### ⚠️ Operational Readiness — 55%

- **No binary release / Makefile** — users must `go build` manually
- **No `--help` flag or `-version`** — the binary has no self-documentation
- **No graceful "queue finished" screen** — the app likely just stalls at the last song with no clear signal it's done
- **No progress persistence across restarts** — if you quit halfway through a 500-song queue, `CurrentIndex` resets to 0 on next load (songs with `status: "applied"` are still loaded but re-presented)
- **Single file format** — only MP3 is reliably supported despite FLAC being mentioned

### ✅ Developer Experience (agent pipeline) — 95%

The spec-driven agent workflow, plan templates, and done/pending/queued pipeline are exceptionally well-structured. The architecture docs are kept in sync. This is actually better than most real-world projects.

---

## Suggested Improvements (prioritised)

### High value, low effort

**1. Skip `applied` songs on load**
The most impactful UX fix. Right now, songs with `status: "applied"` are loaded into the queue and re-presented. Filter them out in `GetTasks()` (or let the user toggle with a flag). A 500-song queue where 400 are done currently forces you to skip through them all.

**2. "Queue complete" screen**
When `skipToNext()` is a no-op because you're at the last song and there are no more, the TUI currently just... stays there with no feedback. Show a simple "All songs reviewed!" message and offer `Ctrl+C` to quit.

**3. Tests for `internal/metadata`**
Write integration tests using `os.CreateTemp` to create a minimal MP3 fixture (or embed one as `testdata/`), call `WriteTags`/`ReadTags`/`WriteBPM`/`WriteYear` on it, and verify the round-trip. This is the highest-risk untested code in the codebase.

### Medium value, medium effort

**4. Resume position across restarts**
The JSON already tracks `status: "applied"`. On load, set `CurrentIndex` to the first entry where `status != "applied"`. Zero additional schema changes needed — it's a one-liner filter in `GetTasks()` or the queue constructor.

**5. `--help` and `--version` flags**
Parse `os.Args` in `main.go` before starting the TUI. `--help` prints usage; `--version` prints a build-time version string injected via `go build -ldflags`.

**6. `Space` key behaviour clarification**
The README says `Enter` / `Space` opens the genre modal, but `Space` being mapped to genre selection is unusual and potentially conflicts with "play/pause" expectations from media apps. Worth deciding whether `Space` should be reassigned to pause (like every music player) and `Enter` kept for genre selection.

### Lower priority / nice to have

**7. FLAC support verification** — either test it and document it properly, or remove the claim from the README.

**8. `go install` Makefile / release workflow** — a simple `Makefile` with `build`, `test`, `install`, `lint` targets would make this easier to distribute.

**9. Configurable genre list hot-reload** — currently you must restart to pick up genre list changes in `settings.json`. The Settings overlay already reloads the queue; it could also reload `GenreList`.

**10. `cmd/reviewer` integration test** — a smoke test that constructs the full program with an empty queue and confirms it starts and exits cleanly without a real audio device.

# Agent Instructions & Coding Standards

## Your Role
You are an expert Go Developer specializing in CLI tools and Concurrent Systems. Your goal is to implement the requested tasks while maintaining a clean, "idiomatic Go" codebase.

## Dos and Don'ts
- **DO:** Use `tea.Cmd` for any I/O (File reading, API calls, Tag writing).
- **DO:** Use `lipgloss` for styling. Keep the UI clean and scannable.
- **DO:** Handle errors gracefully. If an MP3 fails to load, show a warning in the TUI instead of crashing.
- **DON'T:** Use Cgo unless absolutely necessary. Prefer pure Go libraries for audio/tags.
- **DON'T:** Over-complicate the Undo system. A simple slice-based stack is sufficient.
- **DON'T:** Block the main loop. The audio streamer must be non-blocking.

## Metadata Guidelines
- When writing ID3 tags, ensure compatibility with standard players (VLC, MusicBee, Rekordbox).
- Always update the "Status" field in the source JSON after a successful tag write.

## Workflow
1. Read the provided Task Markdown (plan or request).
2. Read the relevant source code to understand the areas you'll be working in. Use `agent-development/agent-specs/FOLDER-STRUCTURE.md` for quick orientation if needed.
3. Implement the logic.
4. Update any relevant documentation (e.g., `agent-development/agent-specs/architecture-breakdown.md`) if you introduce new packages.
5. Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.

## Source Code Is the Source of Truth

The Go source files are the canonical reference for how the system works. When working on a task:

1. **Read the source code directly** for the packages and files relevant to your task. The code is small, well-structured, and self-documenting.
2. **Use `agent-development/agent-specs/FOLDER-STRUCTURE.md`** for quick orientation on project layout if you're unfamiliar with the directory structure.
3. **After completing a task**, update `agent-development/agent-specs/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.

## TUI Structure & Best Practices

- **Model file split:** Keep `model.go` (structs + constructor + Cmd factories), `update.go` (message dispatch), and `view.go` (rendering + styles) as separate files. Do not merge them.
- **No I/O in View:** `View()` must be a pure function. It reads from cached model fields (e.g. `m.playbackState`) and must not call `engine.Play()`, `engine.Seek()`, or any function with side effects.
- **Commands for audio calls:** `Play` is always wrapped in a `tea.Cmd` (see `playCmd`). `Seek`, `TogglePause`, and `GetState` are called directly in `Update` because they are fast, non-blocking, and return immediately.
- **Ticker pattern:** The 100ms progress bar ticker is self-perpetuating: each `TickMsg` handler calls `engine.GetState()`, caches the result on the model, and returns a new `tickCmd()`. Never use `time.Sleep` or a background goroutine to drive UI refreshes.
- **Cached playback state:** `Model.playbackState` is an `audio.PlaybackState` snapshot refreshed every 100ms in the `TickMsg` handler. `View()` reads `m.playbackState` — never calls `engine.GetState()` directly — keeping `View()` a pure, lock-free function.
- **Model is a value type:** Bubble Tea models are passed by value. Always return `m` (the local copy) from `Update`, never a pointer.
- **Engine lifetime:** `engine.Close()` is called both in the `ctrl+c` key handler (for clean interactive exit) and in `main()` after `prog.Run()` returns (defence-in-depth for unexpected exits). It is safe to call twice because the engine guards with `speakerInitialized`.
- **Initial commands:** Issue auto-play and the first tick from `tui.New()` via `tea.Batch`, stored in `Model.pendingInit`. `Init()` returns and clears `pendingInit` — this is the canonical Bubble Tea pattern that ensures startup commands are dispatched inside the event loop.
- **State-aware key handling:** `handleKey` checks `m.state` before dispatching. Keys like `Esc` have different meanings in `StateReviewing` (skip song) vs. `StateGenreSelection` (dismiss modal). Always guard state-specific behaviour at the top of `handleKey`.
- **Seek delta from config:** The seek step (`← / →`) is read from `AppConfig.SeekDeltaSeconds`, defaulting to 30 if the field is zero or omitted. Store it as `time.Duration` on the model at construction time.
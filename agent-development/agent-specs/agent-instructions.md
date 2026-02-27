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
1. Read the relevant diagrams in `diagrams/` (see `diagrams/README.md` for the full list) and self-assess understanding on a 1–10 scale. If confidence is **9 or above**, proceed. If **below 9**, go to the source code for the specific areas of uncertainty, then re-assess.
2. Read the provided Task Markdown.
3. Implement the logic.
4. Update any relevant documentation (e.g., `agent-development/agent-specs/architecture-breakdown.md`) if you introduce new packages.
5. Update any relevant diagrams in `diagrams/` to reflect changes made during the task.

## Diagram-First Development

Before writing or modifying any code, agents must follow the **diagram-first** approach:

1. **Read the relevant diagram(s)** in `diagrams/` — this includes `data-structures.mmd`, `software-architecture.mmd`, `ui-state-machine.mmd`, `task-lifecycle.mmd`, and `component-data-flow.mmd`. Focus on the diagrams most relevant to the task at hand, but scan all of them for cross-cutting concerns. Also consult `diagrams/FOLDER-STRUCTURE.md` for a quick orientation on project layout.
2. **Self-assess understanding** on a scale of 1–10. Rate your confidence in understanding the area of the codebase you are about to work on, based solely on the diagrams.
   - **9 or above:** Proceed with the task using diagram knowledge. You have sufficient understanding.
   - **Below 9:** Go to the source code for the specific areas where your understanding is lacking. Read only what is necessary to close the gap, then re-assess.
3. **After completing a task**, update any diagram(s) that were affected by the changes. This includes:
   - Adding new types, fields, methods, or relationships to `data-structures.mmd`.
   - Adding new packages, interfaces, public functions, or dependency arrows to `software-architecture.mmd`.
   - Adding new states or screen transitions to `ui-state-machine.mmd`.
   - Adding new task-lifecycle stages or data-flow steps to `task-lifecycle.mmd`.
   - Adding new tea.Cmd/tea.Msg flows or component interactions to `component-data-flow.mmd`.
   - Updating `diagrams/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.
4. **Diagram updates are mandatory deliverables** — they are as important as code changes. A task is not complete until the diagrams accurately reflect the current state of the code.

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
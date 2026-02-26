# Task 3: TUI & Navigation Logic

## Context
Create the visual shell and link it to the Audio Engine and Queue.

## Implementation Details
1. **The Model (`/internal/tui/model.go`):**
   - Include `AudioEngine`, `ReviewQueue`, and `Viewport` state.
2. **Main Loop (`Update`):**
   - `Left/Right`: Call `audio.Seek`.
   - `Enter/Space`: Switch state to `StateGenreSelection`.
   - `Esc`: Call `queue.Next()`, then `audio.Play()` for the new file.
   - `Ctrl+U`: Pop from `queue.History`, set index back, reload audio.
3. **Rendering (`View`):**
   - Top: Header with Artist/Title.
   - Middle: Progress bar using `bubbles/progress`.
   - Bottom: Status bar with "Pending: X/Y" and keybind hints.

## Agent Checklist
- [ ] Implement `internal/tui/` logic.
- [ ] Implement a `Tick` command to refresh the progress bar every 100ms.
- [ ] Ensure `Ctrl+C` performs a clean shutdown of the audio device.
- [ ] Update `architecture-breakdown.md` if the provider interface changes.
- [ ] Update `agent-instructions.md` with TUI best practices and how to structure the `Model`.
- [ ] Update `README.md` with latest considerations.

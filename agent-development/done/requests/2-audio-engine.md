# Task 2: Audio Engine Implementation

## Context
Build a robust audio controller that the TUI can trigger.

## Implementation Details
1. **The Controller (`/internal/audio/engine.go`):**
   - Use `faiface/beep` and `faiface/beep/mp3`.
   - State: Store `streamer`, `ctrl` (for pausing), and `resampler`.
2. **Key Functions:**
   - `Play(path string)`: Must stop existing audio, close the file handle, and start the new one.
   - `Seek(delta time.Duration)`: Calculate new position. If `newPos > duration` or `newPos < 0`, loop to the start/end.
   - `GetState()`: Returns current playback percentage (0.0 to 1.0) and time strings (e.g., "01:30 / 04:00").
3. **Concurrency:** Wrap calls in a Mutex to prevent the TUI thread and the Speaker thread from accessing the streamer simultaneously during a "Skip" or "Undo".

## Agent Checklist
- [ ] Implement `internal/audio/engine.go`.
- [ ] Verify `Seek` logic handles the "Loop" requirement.
- [ ] Ensure `Close()` is called on file streamers to prevent memory leaks.
- [ ] Update `architecture-breakdown.md` if the provider interface changes.
- [ ] Update `README.md` with latest considerations.

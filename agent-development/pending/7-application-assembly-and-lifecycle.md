# Task 6: Application Assembly & Settings Mode

## Context
We need to wire the Domain, Provider, Audio, and TUI together into a single executable. We also need to implement the "Options" menu (Ctrl-O).

## Implementation Details
1. **The Entry Point (`/cmd/reviewer/main.go`):**
   - Load `settings.json` using `Viper`.
   - Initialize the `AudioController`.
   - Use the `Provider` to load the `TaskQueue`.
   - Initialize the `Bubble Tea` program with the `Model`.
   - Use the `settings.json` values to get the initial states of the model:
      - `Music Folder Path` for loading songs.
      - `JSON Path` for loading/saving metadata.
      - `Genre List` for the genre picker in the UI.
2. **Options Menu (Ctrl-O):**
   - Create a sub-view in the TUI to edit `Music Folder Path` and `JSON Path`.
   - Use `bubbles/textinput` for fields.
   - **Requirement:** Saving these options should trigger a reload of the `TaskQueue`.
3. **Graceful Shutdown:**
   - Ensure `AudioController.Close()` is called to release the hardware device.
   - Sync any pending JSON changes to disk.

## Agent Checklist
- [ ] Implement `main.go`.
- [ ] Implement the `Settings` view and `Ctrl-O` toggle in `update.go`.
- [ ] Ensure the app doesn't crash if the `settings.json` is missing (provide defaults).
- [ ] Update `agent-development/agent-specs/architecture-breakdown.md` to document the tagging strategy.
- [ ] Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.
- [ ] Update `README.md` with latest considerations.

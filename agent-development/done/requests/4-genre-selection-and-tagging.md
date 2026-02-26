# Task 4: Genre Selection & Tagging

## Context
When the user hits `Space` or `Enter`, the app must pause or dim the background and present a searchable list of genres. Once two selections are made (Primary and Secondary), the metadata must be written to the file.

## Implementation Details
1. **Fuzzy Menu (`/internal/tui/list.go`):**
   - Use `bubbles/list`. 
   - **Step 1:** Show all genres from `AppConfig.GenreList`.
   - **Step 2:** Once selected, refresh the list for "Secondary Genre."
   - **Requirement:** The secondary list must include a `[NONE]` option at the top.
2. **Metadata Writer (`/internal/metadata/writer.go`):**
   - Use `dhowden/tag`.
   - Function `WriteTags(path string, primary string, secondary string) error`.
   - Note: Some libraries handle "Genre" as a single string. If so, concatenate as `Primary/Secondary` or use the specific ID3v2 field if supported.
3. **JSON Persistence:**
   - Create a `SaveState()` method in the `provider`.
   - When a song is tagged, update that specific entry in the original JSON:
     - Set `status` to `"applied"`.
     - Fill in `primary_genre` and `secondary_genre`.
4. **State Transition:**
   - After the second genre is picked and saved, automatically trigger `queue.Next()` and load the next audio file.

## Agent Checklist
- [ ] Implement the `bubbles/list` model for genre selection.
- [ ] Implement the `metadata` package for ID3 writing.
- [ ] Ensure the JSON file is updated on disk after every successful tag (don't wait for app exit).
- [ ] Update `architecture-breakdown.md` to document the tagging strategy.
- [ ] Update `README.md` with latest considerations.

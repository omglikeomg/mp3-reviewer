# Task 5: External API & Advanced Metadata

## Context
Provide the user with "Suggested" metadata for Year and BPM via `Ctrl-1` and `Ctrl-2`.

## Implementation Details
1. **MusicBrainz Client (`/internal/api/musicbrainz.go`):**
   - Search by `Artist` and `Title`.
   - **Logic:** Look for the "Release Group" to find the earliest `first-release-date`.
2. **BPM Client:**
   - Use an API like `SongBPM` or `AcoustID`. If no free API is reliable, implement a "Tap Tempo" feature where pressing a key to the beat calculates the BPM.
3. **Async Fetching:**
   - When a song loads, immediately fire a `tea.Cmd` to fetch this data.
   - While the BPM info is being fetched, use a `tea.Tick` or `spinner` to show a loading state in the UI.
   - Update the UI with `Year: [Loading...]` then `Year: 1971 (Found)`.
4. **The "Commit" Keys:**
   - `Ctrl-1`: If BPM was found, write it to the file's BPM tag.
   - `Ctrl-2`: If Year was found, write it to the file's Year/Date tag.
5. **Visual Feedback:** Use `lipgloss` to highlight the text in green once a tag has been "Enriched" via API.

## Agent Checklist
- [ ] Implement MusicBrainz API integration.
- [ ] Implement BPM fetching logic (or Tap Tempo fallback).
- [ ] Update `tui/view.go` to display these extra metadata fields.
- [ ] Ensure API calls are non-blocking and use `tea.Msg` to return data to the UI.
- [ ] Update `architecture-breakdown.md` to document the tagging strategy.
- [ ] Update `README.md` with latest considerations.

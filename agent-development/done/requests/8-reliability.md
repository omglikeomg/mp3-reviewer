# Task 7: Reliability & Final Polish

## Context
Prevent data loss and handle files that don't play or tags that won't write.

## Implementation Details
1. **The "Atomic Write" Strategy:**
   - When updating the JSON file, write to `manual_review.json.tmp` first, then rename it to the original. This prevents file corruption if the app crashes mid-save.
2. **Missing Metadata Fallback:**
   - If a file has no Title/Artist tags, the UI should display the `Filename` instead of empty strings.
3. **Audio Error Handling:**
   - If `beep` fails to decode a file (e.g., a corrupted MP3), catch the error, show a "Corrupted File" toast in the UI, and allow the user to `Skip` (Esc).
4. **The "Progressive" JSON:**
   - Ensure that the `operations` array in the JSON (from your original snippet) is also updated when a song is "Applied," not just the `manual_review` entry.

## Agent Checklist
- [ ] Implement atomic file saving for the JSON.
- [ ] Add error "Toasts" or status messages in the TUI footer.
- [ ] Perform a final sweep of `agent-development/agent-specs/architecture-breakdown.md` to ensure it matches the final implementation.
- [ ] Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.
- [ ] Update `README.md` with latest considerations.

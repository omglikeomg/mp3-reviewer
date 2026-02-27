# Application Overview: Song Reviewer CLI

## Purpose
A high-performance Go-based CLI tool for music enthusiasts to manually categorize song genres and enrich metadata (BPM, Release Year) through a terminal interface. It bridges the gap between automated scripts (which may produce uncertain results) and manual ID3 tagging.

## Core Workflows
1. **The Review Queue:** The app reads a JSON file containing a list of songs marked for "manual_review".
2. **Immersive Playback:** Songs play automatically upon selection. Users can seek ±30s to find the "soul" of the song (e.g., the drop or the chorus) to identify the genre quickly.
3. **Dual-Tier Tagging:**
   - **Primary Genre:** The main bucket (e.g., "Rock").
   - **Secondary Genre:** The sub-style (e.g., "Psych-Rock"). Supports a `[NONE]` value.
4. **Data Enrichment:**
   - **MusicBrainz Integration:** Fetch the *original* release date (not the remaster date).
   - **BPM Fetching:** Pulling tempo data from external APIs.
5. **Persistence:** Changes are written directly to the MP3/FLAC ID3 tags and the source JSON is updated to reflect the "Applied" status.

## Key UX Goals
- **No Mouse Required:** 100% keyboard-driven.
- **Low Latency:** Audio seeking and UI transitions must feel instantaneous.
- **Error Tolerance:** Easy "Undo" (Ctrl-U) for mis-categorizations.

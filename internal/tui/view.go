package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2)

	styleArtist = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8DADC")).
			Bold(true)

	styleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	styleHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	styleHintKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0")).
			Bold(true)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#A8DADC")).
			Padding(1, 2)

	styleModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			MarginBottom(1)

	styleModalFooter = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				MarginTop(1)

	// ── Enrichment panel styles ───────────────────────────────────────────────

	// styleEnrichLabel styles the "Year:" and "BPM:" prefix labels.
	styleEnrichLabel = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	// styleEnrichLoading styles the loading indicator (spinner + "Loading...").
	styleEnrichLoading = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Italic(true)

	// styleEnrichFound styles a fetched value that is ready to be committed.
	// Yellow/amber — "you have something, press the key to commit it".
	styleEnrichFound = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true)

	// styleEnrichCommitted styles a value that has been successfully written to
	// the ID3 tag. Green — "done".
	styleEnrichCommitted = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B")).
				Bold(true)

	// styleEnrichError styles a fetch or write error.
	styleEnrichError = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Italic(true)

	// styleEnrichIdle styles the idle/not-found state.
	styleEnrichIdle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)

	// styleSettingsLabel styles the field labels in the Settings overlay.
	styleSettingsLabel = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A8DADC")).
				Bold(true)

	// styleSettingsHint styles the keybinding hints at the bottom of the Settings overlay.
	styleSettingsHint = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))
)

// View renders the full TUI frame. Pure function — no side effects.
func (m Model) View() string {
	switch m.state {
	case StateGenreSelection:
		return m.viewGenreModal()
	case StateSettings:
		return m.viewSettings()
	default:
		return m.viewReviewing()
	}
}

// viewReviewing renders the main playback screen:
// header → progress bar → enrichment panel → status bar.
func (m Model) viewReviewing() string {
	// ── Header ────────────────────────────────────────────────────────────────
	var headerLine string
	if len(m.queue.Tasks) == 0 {
		headerLine = styleHeader.Render("  No songs in queue. Add entries to data/manual_review.json.")
	} else {
		task := m.queue.Tasks[m.queue.CurrentIndex]
		artist := task.Artist
		title := task.Title
		if artist == "" {
			artist = "Unknown Artist"
		}
		if title == "" {
			title = filepath.Base(task.FilePath)
		}
		headerLine = styleHeader.Render(
			styleArtist.Render(artist) + "  —  " + styleTitle.Render(title),
		)
	}

	// ── Progress bar ──────────────────────────────────────────────────────────
	progressBar := "  " + m.progress.ViewAs(m.playbackState.Progress)

	// ── Enrichment panel ──────────────────────────────────────────────────────
	enrichPanel := m.viewEnrichmentPanel()

	// ── Status bar ────────────────────────────────────────────────────────────
	total := len(m.queue.Tasks)
	pending := total - m.queue.CurrentIndex
	if total == 0 {
		pending = 0
	}
	queueStr := fmt.Sprintf("Pending: %d / %d", pending, total)
	posStr := m.playbackState.Position

	var errStr string
	if m.lastPlayErr != nil {
		errStr = "  " + styleError.Render("Error: "+m.lastPlayErr.Error())
	} else if m.lastSaveErr != nil {
		errStr = "  " + styleError.Render("Save error: "+m.lastSaveErr.Error())
	}

	seekSecs := int(m.seekDelta.Seconds())
	seekLabel := fmt.Sprintf("seek %ds", seekSecs)
	hints := hintStr("← →", seekLabel) +
		"   " + hintStr("p", "pause") +
		"   " + hintStr("Enter", "tag") +
		"   " + hintStr("t", "tap BPM") +
		"   " + hintStr("Ctrl+1", "commit BPM") +
		"   " + hintStr("Ctrl+2", "commit year") +
		"   " + hintStr("Esc", "skip") +
		"   " + hintStr("Ctrl+U", "undo") +
		"   " + hintStr("Ctrl+O", "settings") +
		"   " + hintStr("Ctrl+C", "quit")

	statusLine := styleStatus.Render(
		queueStr + "   " + posStr + errStr + "\n  " + hints,
	)

	return "\n" + headerLine + "\n\n" + progressBar + "\n\n" + enrichPanel + "\n" + statusLine + "\n"
}

// viewEnrichmentPanel renders the Year and BPM enrichment rows.
// Each row shows: label + status indicator + value (if available).
// The BPM row shows the number of taps remaining before a value is calculated
// (Q3 human decision: 8 taps required, display remaining count).
func (m Model) viewEnrichmentPanel() string {
	yearRow := styleEnrichLabel.Render("  Year: ") + m.enrichFieldView(
		m.enrichYearStatus,
		m.enrichYearValue,
		m.spinner.View()+" fetching...",
		"[not found]",
		"press Ctrl+2 to commit",
	)

	var bpmHint string
	tapCount := len(m.tapTimes)
	switch {
	case tapCount == 0:
		bpmHint = "press t to tap"
	case tapCount < minTapCount:
		remaining := minTapCount - tapCount
		bpmHint = fmt.Sprintf("tap %d more... (%d/%d)", remaining, tapCount, minTapCount)
	default:
		bpmHint = "press Ctrl+1 to commit"
	}

	bpmRow := styleEnrichLabel.Render("  BPM:  ") + m.enrichFieldView(
		m.enrichBPMStatus,
		m.enrichBPMValue,
		m.bpmLoadingText(tapCount),
		bpmHint,
		bpmHint,
	)

	return yearRow + "\n" + bpmRow
}

// bpmLoadingText returns the loading indicator text for the BPM row.
// When the BPM is being fetched from MusicBrainz (tapCount == 0 and status
// is loading), it shows a spinner. When the user is actively tapping, it shows
// tap progress.
func (m Model) bpmLoadingText(tapCount int) string {
	if tapCount == 0 {
		// Still waiting for MusicBrainz fetch or user hasn't started tapping yet.
		return m.spinner.View() + " fetching..."
	}
	remaining := minTapCount - tapCount
	if remaining <= 0 {
		remaining = 0
	}
	return fmt.Sprintf("tapping... %d/%d (need %d more)", tapCount, minTapCount, remaining)
}

// enrichFieldView returns the styled display string for a single enrichment
// field given its status, value, loading indicator text, idle text, and
// found-but-uncommitted hint text.
func (m Model) enrichFieldView(
	status enrichStatus,
	value string,
	loadingText string,
	idleText string,
	foundHint string,
) string {
	switch status {
	case enrichLoading:
		return styleEnrichLoading.Render(loadingText)
	case enrichFound:
		return styleEnrichFound.Render(value) +
			styleHint.Render("  "+foundHint)
	case enrichCommitted:
		return styleEnrichCommitted.Render(value + "  ✓")
	case enrichError:
		return styleEnrichError.Render("error — retry on next load")
	default: // enrichIdle
		return styleEnrichIdle.Render(idleText)
	}
}

// viewGenreModal renders the two-step genre selection overlay.
func (m Model) viewGenreModal() string {
	var heading string
	if m.genreStep == stepPrimary {
		heading = "Step 1 of 2 — Select Primary Genre"
	} else {
		heading = fmt.Sprintf("Step 2 of 2 — Select Secondary Genre  (primary: %s)", m.selectedPrimary)
	}

	footer := styleModalFooter.Render(
		hintStr("↑ ↓", "navigate") +
			"   " + hintStr("Enter", "confirm") +
			"   " + hintStr("Esc", "cancel"),
	)

	inner := styleModalTitle.Render(heading) + "\n" +
		m.genreList.View() + "\n" +
		footer

	return "\n" + styleModal.Render(inner) + "\n"
}

// hintStr formats a single keybinding hint as "Key  description".
func hintStr(key, description string) string {
	return styleHintKey.Render(key) + styleHint.Render("  "+description)
}

// viewSettings renders the Settings overlay.
// The user can edit MusicFolder and JsonPath using bubbles/textinput.
// Tab / Shift-Tab switches focus. Enter saves and reloads the queue. Esc cancels.
func (m Model) viewSettings() string {
	title := styleModalTitle.Render("  Settings")

	musicLabel := styleSettingsLabel.Render("  Music Folder Path:")
	musicField := "  " + m.settingsMusicFolder.View()

	jsonLabel := styleSettingsLabel.Render("  Review JSON Path:")
	jsonField := "  " + m.settingsJsonPath.View()

	hints := "  " + styleSettingsHint.Render(
		hintStr("Tab", "next field")+
			"   "+hintStr("Enter", "save & reload")+
			"   "+hintStr("Esc", "cancel"),
	)

	inner := title + "\n\n" +
		musicLabel + "\n" + musicField + "\n\n" +
		jsonLabel + "\n" + jsonField + "\n\n" +
		hints

	return "\n" + styleModal.Render(inner) + "\n"
}

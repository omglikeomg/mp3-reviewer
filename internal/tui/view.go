package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────
// All lipgloss styles are declared at package level so they are constructed once.

var (
	// styleHeader styles the top row: bold white text, full-width, dark background.
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2)

	// styleArtist is used for the artist name inside the header.
	styleArtist = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8DADC")).
			Bold(true)

	// styleTitle is used for the song title inside the header.
	styleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	// styleStatus styles the bottom status bar.
	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	// styleError styles inline error messages in the status bar.
	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	// styleHint styles individual keybinding hint tokens (e.g. "← →  seek").
	styleHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	// styleHintKey styles the key portion of a hint (e.g. "←").
	styleHintKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E0E0")).
			Bold(true)

	// styleModal wraps the genre selection overlay box with a rounded border.
	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#A8DADC")).
			Padding(1, 2)

	// styleModalTitle is the heading line inside the genre selection modal.
	styleModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			MarginBottom(1)

	// styleModalFooter shows keybinding hints inside the modal footer.
	styleModalFooter = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				MarginTop(1)
)

// View renders the full TUI frame. It is a pure function of the model's current
// state — it reads only from m fields and must contain NO side effects.
// Playback position is read from m.playbackState (cached in Update on TickMsg),
// not by calling engine.GetState() directly.
func (m Model) View() string {
	if m.state == StateGenreSelection {
		return m.viewGenreModal()
	}
	return m.viewReviewing()
}

// viewReviewing renders the main playback screen: header, progress bar, status bar.
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
			title = task.FilePath
		}
		headerLine = styleHeader.Render(
			styleArtist.Render(artist) + "  —  " + styleTitle.Render(title),
		)
	}

	// ── Progress bar ──────────────────────────────────────────────────────────
	// m.playbackState is refreshed every 100ms in the TickMsg handler.
	progressBar := "  " + m.progress.ViewAs(m.playbackState.Progress)

	// ── Status bar ────────────────────────────────────────────────────────────

	// Queue counter.
	total := len(m.queue.Tasks)
	pending := total - m.queue.CurrentIndex
	if total == 0 {
		pending = 0
	}
	queueStr := fmt.Sprintf("Pending: %d / %d", pending, total)

	// Time position from cached playback state.
	posStr := m.playbackState.Position

	// Error notice — play error takes precedence; save error shown as fallback.
	var errStr string
	if m.lastPlayErr != nil {
		errStr = "  " + styleError.Render("Error: "+m.lastPlayErr.Error())
	} else if m.lastSaveErr != nil {
		errStr = "  " + styleError.Render("Save error: "+m.lastSaveErr.Error())
	}

	// Keybind hints — seek label reflects the configured delta.
	seekSecs := int(m.seekDelta.Seconds())
	seekLabel := fmt.Sprintf("seek %ds", seekSecs)
	hints := hintStr("← →", seekLabel) +
		"   " + hintStr("p", "pause") +
		"   " + hintStr("Enter", "tag") +
		"   " + hintStr("Esc", "skip") +
		"   " + hintStr("Ctrl+U", "undo") +
		"   " + hintStr("Ctrl+C", "quit")

	statusLine := styleStatus.Render(
		queueStr + "   " + posStr + errStr + "\n  " + hints,
	)

	// ── Compose layout ────────────────────────────────────────────────────────
	return "\n" + headerLine + "\n\n" + progressBar + "\n\n" + statusLine + "\n"
}

// viewGenreModal renders the two-step genre selection overlay.
// Step 1 shows the primary genre list (no [NONE] option).
// Step 2 shows the secondary genre list ([NONE] prepended).
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

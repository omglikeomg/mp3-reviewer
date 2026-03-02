package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/domain"
)

// Init satisfies the tea.Model interface. It returns the pendingInit command
// batch that was stored by New() — auto-play + first tick + spinner + year/BPM fetch.
func (m Model) Init() tea.Cmd {
	cmd := m.pendingInit
	m.pendingInit = nil
	return cmd
}

// Update is the central message dispatcher.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 4
		if m.state == StateGenreSelection {
			m.genreList.SetWidth(msg.Width - 4)
		}
		m.settingsMusicFolder.Width = msg.Width - 10
		m.settingsJsonPath.Width = msg.Width - 10
		return m, nil

	case TickMsg:
		m.playbackState = m.engine.GetState()
		return m, tickCmd()

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		return m, spinCmd

	case PlayErrMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
		} else {
			m.lastPlayErr = nil
		}
		return m, nil

	case TagWrittenMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
			m.state = StateReviewing
			return m, nil
		}

		idx := m.queue.CurrentIndex
		if idx >= 0 && idx < len(m.queue.Tasks) {
			m.queue.Tasks[idx].Genre1 = msg.Primary
			m.queue.Tasks[idx].Genre2 = msg.Secondary
		}

		saveCmd := saveStateCmd(m.providerRef, m.queue.Tasks)
		nextModel, nextCmd := m.skipToNext()
		return nextModel, tea.Batch(saveCmd, nextCmd)

	case SaveStateErrMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
		} else {
			m.lastSaveErr = nil
		}
		return m, nil

	case YearFetchedMsg:
		if msg.Err != nil {
			m.enrichYearStatus = enrichError
			m.enrichYearValue = ""
		} else if msg.Year == "" {
			// Successful request but no year found in MusicBrainz.
			m.enrichYearStatus = enrichIdle
			m.enrichYearValue = ""
		} else {
			m.enrichYearStatus = enrichFound
			m.enrichYearValue = msg.Year
		}
		return m, nil

	case BPMFetchedMsg:
		if msg.Err != nil {
			// MusicBrainz BPM fetch failed — fall back to tap tempo.
			// Set to idle so the user can use tap tempo.
			m.enrichBPMStatus = enrichIdle
			m.enrichBPMValue = ""
		} else if msg.BPM == "" {
			// No BPM found in MusicBrainz — fall back to tap tempo.
			m.enrichBPMStatus = enrichIdle
			m.enrichBPMValue = ""
		} else {
			// BPM found from MusicBrainz — ready to commit.
			m.enrichBPMStatus = enrichFound
			m.enrichBPMValue = msg.BPM
		}
		return m, nil

	case BPMWrittenMsg:
		if msg.Err != nil {
			m.enrichBPMStatus = enrichError
		} else {
			m.enrichBPMStatus = enrichCommitted
			idx := m.queue.CurrentIndex
			if idx >= 0 && idx < len(m.queue.Tasks) {
				m.queue.Tasks[idx].BPM = msg.BPM
			}
			// Q5 human decision: call SaveState after BPM commit to keep
			// the JSON data source in sync with the ID3 tag write.
			return m, saveStateCmd(m.providerRef, m.queue.Tasks)
		}
		return m, nil

	case YearWrittenMsg:
		if msg.Err != nil {
			m.enrichYearStatus = enrichError
		} else {
			m.enrichYearStatus = enrichCommitted
			idx := m.queue.CurrentIndex
			if idx >= 0 && idx < len(m.queue.Tasks) {
				m.queue.Tasks[idx].Year = msg.Year
			}
			// Q5 human decision: call SaveState after Year commit to keep
			// the JSON data source in sync with the ID3 tag write.
			return m, saveStateCmd(m.providerRef, m.queue.Tasks)
		}
		return m, nil

	case QueueReloadedMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
			return m, nil
		}
		// Replace the task list in-place, reset position.
		m.queue.Tasks = msg.Tasks
		m.queue.CurrentIndex = 0
		m.queue.History = []domain.Task{}
		m.lastPlayErr = nil
		m.lastSaveErr = nil
		m = m.resetEnrichment()

		// Auto-play the first song of the newly loaded queue.
		var cmds []tea.Cmd
		if len(msg.Tasks) > 0 {
			first := msg.Tasks[0]
			cmds = append(cmds, playCmd(m.engine, first.FilePath))
			if first.Artist != "" || first.Title != "" {
				m.enrichYearStatus = enrichLoading
				m.enrichBPMStatus = enrichLoading
				cmds = append(cmds, fetchYearCmd(first.Artist, first.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
				cmds = append(cmds, fetchBPMCmd(first.Artist, first.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
			}
		}
		return m, tea.Batch(cmds...)

	case SettingsSavedMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// In StateGenreSelection, pass unhandled non-key messages to the list.
	if m.state == StateGenreSelection {
		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	return m, nil
}

// handleKey dispatches keyboard input based on current AppState.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Settings state ────────────────────────────────────────────────────────
	if m.state == StateSettings {
		return m.handleSettingsKey(msg)
	}

	// ── Genre Selection state ─────────────────────────────────────────────────
	if m.state == StateGenreSelection {
		switch msg.String() {
		case "ctrl+c":
			// Flush any pending JSON enrichment data before quitting.
			_ = m.providerRef.SaveState(m.queue.Tasks)
			m.engine.Close()
			return m, tea.Quit

		case "esc":
			m.state = StateReviewing
			return m, nil

		case "enter":
			return m.confirmGenreSelection()
		}

		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	// ── Queue Complete state ──────────────────────────────────────────────────
	if m.state == StateQueueComplete {
		switch msg.String() {
		case "ctrl+c":
			// Flush any pending JSON enrichment data before quitting.
			_ = m.providerRef.SaveState(m.queue.Tasks)
			m.engine.Close()
			return m, tea.Quit
		case "ctrl+u":
			m.state = StateReviewing
			return m.undoLast()
		}
		// All other keys are no-ops in the completion screen.
		return m, nil
	}

	// ── StateReviewing — full keybinding set ──────────────────────────────────
	switch msg.String() {

	case "ctrl+c":
		// Flush any pending JSON enrichment data before quitting.
		_ = m.providerRef.SaveState(m.queue.Tasks)
		m.engine.Close()
		return m, tea.Quit

	case "left":
		_ = m.engine.Seek(-m.seekDelta)
		return m, nil

	case "right":
		_ = m.engine.Seek(m.seekDelta)
		return m, nil

	case "p":
		m.engine.TogglePause()
		return m, nil

	case "enter", " ":
		m = m.openGenreModal()
		return m, nil

	case "esc":
		return m.skipToNext()

	case "ctrl+u":
		return m.undoLast()

	case "ctrl+o":
		return m.openSettings()

	// ── Commit BPM (Ctrl+1) ───────────────────────────────────────────────────
	case "ctrl+1":
		if m.enrichBPMStatus != enrichFound {
			// Nothing ready to commit — ignore.
			return m, nil
		}
		if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
			return m, nil
		}
		path := m.queue.Tasks[m.queue.CurrentIndex].FilePath
		return m, writeBPMCmd(path, m.enrichBPMValue)

	// ── Commit Year (Ctrl+2) ──────────────────────────────────────────────────
	case "ctrl+2":
		if m.enrichYearStatus != enrichFound {
			// Nothing ready to commit — ignore.
			return m, nil
		}
		if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
			return m, nil
		}
		path := m.queue.Tasks[m.queue.CurrentIndex].FilePath
		return m, writeYearCmd(path, m.enrichYearValue)

	// ── Tap Tempo (t) ─────────────────────────────────────────────────────────
	case "t":
		return m.recordTap()
	}

	return m, nil
}

// recordTap records a tap-tempo keypress timestamp and, when at least minTapCount
// taps (Q3 human decision: 8 taps = 7 intervals) have been collected, calculates
// the BPM from the average inter-tap interval.
//
// Stale tap detection: taps with a gap > staleTapWindow (3s) since the last tap
// reset the sequence (Q4 human decision).
//
// Irregularity detection: if any individual inter-tap interval deviates from
// the running average by more than maxTapDeviation (40%), the sequence is
// considered too irregular and is reset. The user can see the remaining tap count
// in the UI and always use Esc to go back (Q3 human decision).
func (m Model) recordTap() (tea.Model, tea.Cmd) {
	now := time.Now()

	// Discard stale taps (gap > staleTapWindow since the last tap means a new sequence).
	if len(m.tapTimes) > 0 {
		last := m.tapTimes[len(m.tapTimes)-1]
		if now.Sub(last) > staleTapWindow {
			m.tapTimes = nil
		}
	}

	m.tapTimes = append(m.tapTimes, now)

	// Check for irregular tapping once we have at least 2 intervals (3 taps).
	// If any interval deviates too much from the current average, reset the
	// sequence so the user gets a clean start.
	if len(m.tapTimes) >= 3 {
		if m.isTappingIrregular() {
			// Reset: keep only the most recent tap as the start of a new sequence.
			lastTap := m.tapTimes[len(m.tapTimes)-1]
			m.tapTimes = []time.Time{lastTap}
			m.enrichBPMStatus = enrichLoading
			m.enrichBPMValue = ""
			return m, nil
		}
	}

	// Need at least minTapCount taps (minTapCount-1 intervals) for a stable BPM.
	if len(m.tapTimes) < minTapCount {
		m.enrichBPMStatus = enrichLoading
		return m, nil
	}

	// Calculate average interval between consecutive taps.
	var totalInterval time.Duration
	for i := 1; i < len(m.tapTimes); i++ {
		totalInterval += m.tapTimes[i].Sub(m.tapTimes[i-1])
	}
	avgInterval := totalInterval / time.Duration(len(m.tapTimes)-1)

	if avgInterval <= 0 {
		return m, nil
	}

	// BPM = 60 seconds / average interval in seconds.
	bpm := math.Round(60.0 / avgInterval.Seconds())
	m.enrichBPMValue = fmt.Sprintf("%d", int(bpm))
	m.enrichBPMStatus = enrichFound

	return m, nil
}

// isTappingIrregular checks whether the current tap sequence has any interval
// that deviates from the running average by more than maxTapDeviation.
// Returns true if the tapping is too irregular and the sequence should be reset.
// Requires at least 3 taps (2 intervals) to be meaningful.
func (m Model) isTappingIrregular() bool {
	n := len(m.tapTimes)
	if n < 3 {
		return false
	}

	// Calculate average interval.
	var totalInterval time.Duration
	intervals := make([]time.Duration, 0, n-1)
	for i := 1; i < n; i++ {
		d := m.tapTimes[i].Sub(m.tapTimes[i-1])
		intervals = append(intervals, d)
		totalInterval += d
	}
	avgInterval := totalInterval / time.Duration(len(intervals))

	if avgInterval <= 0 {
		return true
	}

	// Check each interval against the average.
	avgSeconds := avgInterval.Seconds()
	for _, d := range intervals {
		deviation := math.Abs(d.Seconds()-avgSeconds) / avgSeconds
		if deviation > maxTapDeviation {
			return true
		}
	}

	return false
}

// confirmGenreSelection handles the user pressing Enter on a genre list item.
func (m Model) confirmGenreSelection() (tea.Model, tea.Cmd) {
	selected, ok := m.genreList.SelectedItem().(genreItem)
	if !ok {
		return m, nil
	}

	if m.genreStep == stepPrimary {
		m.selectedPrimary = selected.title
		m.genreStep = stepSecondary
		m.genreList = makeGenreList(m.cfg.GenreList, true, m.width)
		return m, nil
	}

	secondary := selected.title
	if secondary == "[NONE]" {
		secondary = ""
	}

	m.state = StateReviewing

	if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
		return m, nil
	}
	path := m.queue.Tasks[m.queue.CurrentIndex].FilePath

	return m, writeTagsCmd(path, m.selectedPrimary, secondary)
}

// skipToNext advances the queue to the next task, starts playing it, and
// resets enrichment state. Fires fetchYearCmd and fetchBPMCmd for the new song
// if artist/title are available. If no next task exists (end of queue),
// transitions to StateQueueComplete so the user sees a completion screen.
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
	nextIndex := m.queue.CurrentIndex + 1
	if nextIndex >= len(m.queue.Tasks) {
		// Push the current song onto History so Ctrl+U can rewind from
		// the completion screen back to the last song.
		if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
			m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
		}
		m.state = StateQueueComplete
		return m, nil
	}

	// NOTE: The history-append below covers the normal (non-end-of-queue) path.
	// Do NOT add another append here; the end-of-queue path above handles its own.
	if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
		m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
	}

	m.queue.CurrentIndex = nextIndex
	m.lastPlayErr = nil

	// Reset enrichment for the incoming song.
	m = m.resetEnrichment()

	nextTask := m.queue.Tasks[nextIndex]
	var cmds []tea.Cmd
	cmds = append(cmds, playCmd(m.engine, nextTask.FilePath))

	// Fire year and BPM fetches immediately if we have artist/title metadata.
	if nextTask.Artist != "" || nextTask.Title != "" {
		m.enrichYearStatus = enrichLoading
		m.enrichBPMStatus = enrichLoading
		cmds = append(cmds, fetchYearCmd(nextTask.Artist, nextTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
		cmds = append(cmds, fetchBPMCmd(nextTask.Artist, nextTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
	}

	return m, tea.Batch(cmds...)
}

// undoLast pops the most recent task off the History stack, rewinds
// CurrentIndex by one, and replays that song. Enrichment is reset and
// re-fetched (Q6 human decision: Option A).
func (m Model) undoLast() (tea.Model, tea.Cmd) {
	if len(m.queue.History) == 0 {
		return m, nil
	}

	lastIdx := len(m.queue.History) - 1
	m.queue.History = m.queue.History[:lastIdx]

	if m.queue.CurrentIndex > 0 {
		m.queue.CurrentIndex--
	}

	m.lastPlayErr = nil

	// Reset enrichment when rewinding.
	m = m.resetEnrichment()

	prevTask := m.queue.Tasks[m.queue.CurrentIndex]
	var cmds []tea.Cmd
	cmds = append(cmds, playCmd(m.engine, prevTask.FilePath))

	if prevTask.Artist != "" || prevTask.Title != "" {
		m.enrichYearStatus = enrichLoading
		m.enrichBPMStatus = enrichLoading
		cmds = append(cmds, fetchYearCmd(prevTask.Artist, prevTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
		cmds = append(cmds, fetchBPMCmd(prevTask.Artist, prevTask.Title, m.cfg.ApiKeys.MusicBrainzUserAgent))
	}

	return m, tea.Batch(cmds...)
}

// openSettings transitions the model into StateSettings and focuses the first
// input field (Music Folder Path).
func (m Model) openSettings() (tea.Model, tea.Cmd) {
	m.state = StateSettings
	m.settingsFocusIndex = 0
	m.settingsMusicFolder.SetValue(m.cfg.MusicFolder)
	m.settingsJsonPath.SetValue(m.cfg.JsonPath)
	// Focus the first field; textinput.Focus() returns a Cmd for cursor blink.
	cmd := m.settingsMusicFolder.Focus()
	m.settingsJsonPath.Blur()
	return m, cmd
}

// handleSettingsKey dispatches keyboard input while StateSettings is active.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c":
		// Flush any pending changes then quit.
		_ = m.providerRef.SaveState(m.queue.Tasks)
		m.engine.Close()
		return m, tea.Quit

	case "esc":
		// Discard changes and return to the review screen.
		m.state = StateReviewing
		m.settingsMusicFolder.Blur()
		m.settingsJsonPath.Blur()
		return m, nil

	case "tab", "shift+tab":
		// Toggle focus between the two input fields.
		if m.settingsFocusIndex == 0 {
			m.settingsFocusIndex = 1
			m.settingsMusicFolder.Blur()
			cmd := m.settingsJsonPath.Focus()
			return m, cmd
		}
		m.settingsFocusIndex = 0
		m.settingsJsonPath.Blur()
		cmd := m.settingsMusicFolder.Focus()
		return m, cmd

	case "enter":
		// Save: update cfg, update providerRef, dismiss overlay, persist to disk,
		// and reload queue (Q1 human decision: Option B — persist to disk).
		m.cfg.MusicFolder = m.settingsMusicFolder.Value()
		m.cfg.JsonPath = m.settingsJsonPath.Value()
		m.providerRef.Config.MusicFolder = m.cfg.MusicFolder
		m.providerRef.Config.JsonPath = m.cfg.JsonPath
		m.state = StateReviewing
		m.settingsMusicFolder.Blur()
		m.settingsJsonPath.Blur()
		return m, tea.Batch(saveSettingsCmd(m.cfg), reloadQueueCmd(m.providerRef))
	}

	// Forward all other keys to the focused input.
	var cmd tea.Cmd
	if m.settingsFocusIndex == 0 {
		m.settingsMusicFolder, cmd = m.settingsMusicFolder.Update(msg)
	} else {
		m.settingsJsonPath, cmd = m.settingsJsonPath.Update(msg)
	}
	return m, cmd
}

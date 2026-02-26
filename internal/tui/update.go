package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies the tea.Model interface. It returns the pendingInit command
// batch that was stored by New() — this is the canonical Bubble Tea pattern
// for dispatching startup commands (auto-play + first tick) inside the event loop.
// After Init returns the batch, pendingInit is cleared to avoid re-issuing on
// any future model reconstruction.
func (m Model) Init() tea.Cmd {
	cmd := m.pendingInit
	m.pendingInit = nil
	return cmd
}

// Update is the central message dispatcher. It receives every message Bubble Tea
// delivers — keypresses, timer ticks, window resizes, and custom messages from
// background commands — and returns the next model state plus any new commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave a margin so the progress bar doesn't touch terminal edges.
		m.progress.Width = msg.Width - 4
		// Resize genre list if it is currently showing.
		if m.state == StateGenreSelection {
			m.genreList.SetWidth(msg.Width - 4)
		}
		return m, nil

	case TickMsg:
		// Refresh the cached playback state so View() stays pure (no lock calls).
		m.playbackState = m.engine.GetState()
		// Re-queue the ticker immediately so the next tick arrives in 100ms.
		return m, tickCmd()

	case PlayErrMsg:
		if msg.Err != nil {
			m.lastPlayErr = msg.Err
		} else {
			m.lastPlayErr = nil
		}
		return m, nil

	case TagWrittenMsg:
		if msg.Err != nil {
			// Tag write failed — show the error but stay on the review screen
			// so the user can retry by re-opening the genre modal.
			m.lastPlayErr = msg.Err
			m.state = StateReviewing
			return m, nil
		}

		// Tag write succeeded — update the task in the queue.
		idx := m.queue.CurrentIndex
		if idx >= 0 && idx < len(m.queue.Tasks) {
			m.queue.Tasks[idx].Genre1 = msg.Primary
			m.queue.Tasks[idx].Genre2 = msg.Secondary
		}

		// Persist the updated queue to disk asynchronously.
		saveCmd := saveStateCmd(m.providerRef, m.queue.Tasks)

		// Advance to the next song.
		nextModel, nextCmd := m.skipToNext()
		return nextModel, tea.Batch(saveCmd, nextCmd)

	case SaveStateErrMsg:
		if msg.Err != nil {
			m.lastSaveErr = msg.Err
		} else {
			m.lastSaveErr = nil
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// In StateGenreSelection, pass unhandled non-key messages to the list component.
	if m.state == StateGenreSelection {
		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	return m, nil
}

// handleKey dispatches keyboard input to the appropriate action.
// It is extracted from Update to keep the switch statement readable.
// Key behaviour varies by current AppState (e.g. Esc has different meanings
// in StateReviewing vs StateGenreSelection).
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Genre Selection state — intercept Enter/Esc, delegate the rest ────────
	if m.state == StateGenreSelection {
		switch msg.String() {
		case "ctrl+c":
			m.engine.Close()
			return m, tea.Quit

		case "esc":
			// Cancel genre selection and return to the review screen without skipping.
			m.state = StateReviewing
			return m, nil

		case "enter":
			// Confirm the currently highlighted genre item.
			return m.confirmGenreSelection()
		}

		// Delegate all other keys (arrow keys, filter typing, etc.) to the list.
		var listCmd tea.Cmd
		m.genreList, listCmd = m.genreList.Update(msg)
		return m, listCmd
	}

	// ── StateReviewing — full keybinding set ──────────────────────────────────
	switch msg.String() {

	// ── Quit ──────────────────────────────────────────────────────────────────
	case "ctrl+c":
		// Close the audio device before exiting. engine.Close() is safe to call
		// even if nothing is playing.
		m.engine.Close()
		return m, tea.Quit

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "left":
		_ = m.engine.Seek(-m.seekDelta)
		return m, nil

	case "right":
		_ = m.engine.Seek(m.seekDelta)
		return m, nil

	// ── Pause / Resume ────────────────────────────────────────────────────────
	case "p":
		m.engine.TogglePause()
		return m, nil

	// ── Genre Selection ───────────────────────────────────────────────────────
	case "enter", " ":
		// Open the genre modal for Primary Genre selection.
		m = m.openGenreModal()
		return m, nil

	// ── Skip (Esc) ────────────────────────────────────────────────────────────
	case "esc":
		return m.skipToNext()

	// ── Undo (Ctrl+U) ─────────────────────────────────────────────────────────
	case "ctrl+u":
		return m.undoLast()
	}

	return m, nil
}

// confirmGenreSelection handles the user pressing Enter on a genre list item.
// On stepPrimary it records the selection and transitions to stepSecondary,
// rebuilding the list with a [NONE] option prepended.
// On stepSecondary it fires writeTagsCmd with both genre selections.
func (m Model) confirmGenreSelection() (tea.Model, tea.Cmd) {
	selected, ok := m.genreList.SelectedItem().(genreItem)
	if !ok {
		// No item highlighted (empty list) — ignore.
		return m, nil
	}

	if m.genreStep == stepPrimary {
		// Record primary and transition to secondary step.
		m.selectedPrimary = selected.title
		m.genreStep = stepSecondary
		m.genreList = makeGenreList(m.cfg.GenreList, true, m.width)
		return m, nil
	}

	// stepSecondary — we have both selections; fire the tag write.
	secondary := selected.title
	if secondary == "[NONE]" {
		secondary = ""
	}

	// Close the modal immediately for snappy UX; the write runs in the background.
	m.state = StateReviewing

	if m.queue.CurrentIndex < 0 || m.queue.CurrentIndex >= len(m.queue.Tasks) {
		return m, nil
	}
	path := m.queue.Tasks[m.queue.CurrentIndex].FilePath

	return m, writeTagsCmd(path, m.selectedPrimary, secondary)
}

// skipToNext advances the queue to the next task, starts playing it, and
// returns the updated model and a playCmd. If no next task exists (end of
// queue), it is a no-op (Q3 decision: Option B — stay silently on last song).
func (m Model) skipToNext() (tea.Model, tea.Cmd) {
	nextIndex := m.queue.CurrentIndex + 1
	if nextIndex >= len(m.queue.Tasks) {
		// End of queue — nothing to do.
		return m, nil
	}

	// Push the current task onto the history stack before advancing.
	if m.queue.CurrentIndex >= 0 && m.queue.CurrentIndex < len(m.queue.Tasks) {
		m.queue.History = append(m.queue.History, m.queue.Tasks[m.queue.CurrentIndex])
	}

	m.queue.CurrentIndex = nextIndex
	m.lastPlayErr = nil
	return m, playCmd(m.engine, m.queue.Tasks[nextIndex].FilePath)
}

// undoLast pops the most recent task off the History stack, rewinds
// CurrentIndex by one, and replays that song.
func (m Model) undoLast() (tea.Model, tea.Cmd) {
	if len(m.queue.History) == 0 {
		// Nothing to undo.
		return m, nil
	}

	// Pop from history.
	lastIdx := len(m.queue.History) - 1
	m.queue.History = m.queue.History[:lastIdx]

	// Rewind index (guard against going below 0).
	if m.queue.CurrentIndex > 0 {
		m.queue.CurrentIndex--
	}

	m.lastPlayErr = nil
	return m, playCmd(m.engine, m.queue.Tasks[m.queue.CurrentIndex].FilePath)
}

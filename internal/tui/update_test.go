package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/provider"
)

// newTestModel builds a minimal Model for use in unit tests.
// It uses a *mockPlayer so no OS audio device is required.
// The queue is pre-populated with two fake tasks so that skipToNext and
// undoLast have something to work with.
func newTestModel(mp *mockPlayer) Model {
	tasks := []domain.Task{
		{FilePath: "/music/song1.mp3", Title: "Song One", Artist: "Artist A"},
		{FilePath: "/music/song2.mp3", Title: "Song Two", Artist: "Artist B"},
	}
	queue := domain.ReviewQueue{
		Tasks:        tasks,
		CurrentIndex: 0,
		History:      []domain.Task{},
	}
	cfg := domain.AppConfig{
		SeekDeltaSeconds: 10,
		GenreList:        []string{"Rock", "Jazz", "Electronic"},
	}
	p := provider.ManualReviewProvider{}

	return New(queue, mp, cfg, p)
}

// isNilCmd returns true if the given tea.Cmd is nil.
// We cannot inspect tea.Cmd internals directly, so we check for non-nil as a
// proxy for "a command was issued". Use this to assert that a command was or
// was not returned.
func isNilCmd(cmd tea.Cmd) bool {
	return cmd == nil
}

// ── Seek tests ────────────────────────────────────────────────────────────────

// TestHandleKey_SeekForward verifies that pressing the right-arrow key calls
// Seek with a positive delta equal to the configured seekDelta.
func TestHandleKey_SeekForward(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRight})

	if len(mp.seekDeltas) != 1 {
		t.Fatalf("expected 1 Seek call, got %d", len(mp.seekDeltas))
	}
	want := 10 * time.Second
	if mp.seekDeltas[0] != want {
		t.Errorf("Seek delta = %v, want %v", mp.seekDeltas[0], want)
	}
}

// TestHandleKey_SeekBackward verifies that pressing the left-arrow key calls
// Seek with a negative delta equal to -seekDelta.
func TestHandleKey_SeekBackward(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})

	if len(mp.seekDeltas) != 1 {
		t.Fatalf("expected 1 Seek call, got %d", len(mp.seekDeltas))
	}
	want := -10 * time.Second
	if mp.seekDeltas[0] != want {
		t.Errorf("Seek delta = %v, want %v", mp.seekDeltas[0], want)
	}
}

// ── TogglePause test ──────────────────────────────────────────────────────────

// TestHandleKey_TogglePause verifies that pressing 'p' calls TogglePause once.
func TestHandleKey_TogglePause(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})

	if mp.toggleCount != 1 {
		t.Errorf("TogglePause call count = %d, want 1", mp.toggleCount)
	}
}

// ── Genre modal open test ─────────────────────────────────────────────────────

// TestHandleKey_EnterOpensGenreSelection verifies that pressing Enter while in
// StateReviewing transitions the model to StateGenreSelection.
func TestHandleKey_EnterOpensGenreSelection(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	if m.state != StateReviewing {
		t.Fatalf("precondition: expected StateReviewing, got %v", m.state)
	}

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	resultModel := result.(Model)

	if resultModel.state != StateGenreSelection {
		t.Errorf("after Enter: state = %v, want StateGenreSelection", resultModel.state)
	}
}

// ── Esc behaviour tests ───────────────────────────────────────────────────────

// TestHandleKey_EscInReviewingSkips verifies that pressing Esc while in
// StateReviewing advances CurrentIndex by 1 and issues a non-nil command (play).
func TestHandleKey_EscInReviewingSkips(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	if m.queue.CurrentIndex != 0 {
		t.Fatalf("precondition: CurrentIndex = %d, want 0", m.queue.CurrentIndex)
	}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1", resultModel.queue.CurrentIndex)
	}
	if isNilCmd(cmd) {
		t.Error("expected a non-nil command (playCmd) after skip, got nil")
	}
}

// TestHandleKey_EscInGenreSelectionCancels verifies that pressing Esc while in
// StateGenreSelection returns the model to StateReviewing without advancing the queue.
func TestHandleKey_EscInGenreSelectionCancels(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)
	m.state = StateGenreSelection // Force into genre selection state.

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	resultModel := result.(Model)

	if resultModel.state != StateReviewing {
		t.Errorf("after Esc in genre selection: state = %v, want StateReviewing", resultModel.state)
	}
	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 (no skip should have occurred)", resultModel.queue.CurrentIndex)
	}
	if !isNilCmd(cmd) {
		t.Error("expected nil command after Esc in genre modal (no side effects), got non-nil")
	}
}

// ── Undo tests ────────────────────────────────────────────────────────────────

// TestHandleKey_CtrlU_NoHistory verifies that pressing Ctrl+U with an empty
// History slice is a no-op: CurrentIndex does not change and no command is issued.
func TestHandleKey_CtrlU_NoHistory(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Confirm empty history.
	if len(m.queue.History) != 0 {
		t.Fatalf("precondition: expected empty history, got %d items", len(m.queue.History))
	}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 after no-op undo", resultModel.queue.CurrentIndex)
	}
	if !isNilCmd(cmd) {
		t.Error("expected nil command for no-op undo, got non-nil")
	}
}

// TestHandleKey_CtrlU_WithHistory verifies that pressing Ctrl+U with a non-empty
// History rewinds CurrentIndex by 1, pops the last history entry, and issues a
// non-nil command (playCmd for the previous song).
func TestHandleKey_CtrlU_WithHistory(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Simulate having moved to song 2: CurrentIndex = 1, History = [song1].
	m.queue.CurrentIndex = 1
	m.queue.History = []domain.Task{m.queue.Tasks[0]}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	resultModel := result.(Model)

	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 after undo", resultModel.queue.CurrentIndex)
	}
	if len(resultModel.queue.History) != 0 {
		t.Errorf("History length = %d, want 0 after undo popped the last entry", len(resultModel.queue.History))
	}
	if isNilCmd(cmd) {
		t.Error("expected a non-nil command (playCmd) after undo, got nil")
	}
}

// ── skipToNext end-of-queue test ──────────────────────────────────────────────

// TestSkipToNext_EndOfQueue verifies that skipToNext transitions to
// StateQueueComplete when CurrentIndex is already at the last task,
// and that the current task is pushed onto History so Ctrl+U can rewind.
func TestSkipToNext_EndOfQueue(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Position at the last task (index 1, length 2).
	m.queue.CurrentIndex = 1

	result, cmd := m.skipToNext()
	resultModel := result.(Model)

	// CurrentIndex must not advance past the end.
	if resultModel.queue.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1 (no advance past end)", resultModel.queue.CurrentIndex)
	}
	// State must transition to StateQueueComplete.
	if resultModel.state != StateQueueComplete {
		t.Errorf("state = %v, want StateQueueComplete", resultModel.state)
	}
	// The last task must be pushed onto History for Ctrl+U to work.
	if len(resultModel.queue.History) != 1 {
		t.Errorf("History length = %d, want 1 (last task pushed for undo)", len(resultModel.queue.History))
	}
	// No play command should be issued.
	if !isNilCmd(cmd) {
		t.Error("expected nil command when transitioning to StateQueueComplete, got non-nil")
	}
}

// ── TickMsg playback state caching test ──────────────────────────────────────

// TestTickMsg_CachesPlaybackState verifies that a TickMsg causes the model to
// call engine.GetState() and cache the result in m.playbackState.
func TestTickMsg_CachesPlaybackState(t *testing.T) {
	wantState := audio.PlaybackState{
		Progress: 0.42,
		Elapsed:  "01:00",
		Total:    "02:22",
		Position: "01:00 / 02:22",
	}
	mp := &mockPlayer{state: wantState}
	m := newTestModel(mp)

	// Ensure the initial cached state is the zero value (or whatever New() set).
	// Then fire a TickMsg and verify the cache was updated.
	result, _ := m.Update(TickMsg(time.Now()))
	resultModel := result.(Model)

	got := resultModel.playbackState
	if got.Progress != wantState.Progress {
		t.Errorf("playbackState.Progress = %v, want %v", got.Progress, wantState.Progress)
	}
	if got.Position != wantState.Position {
		t.Errorf("playbackState.Position = %q, want %q", got.Position, wantState.Position)
	}
}

// ── StateQueueComplete key handling tests ─────────────────────────────────────

// TestHandleKey_QueueComplete_CtrlU verifies that pressing Ctrl+U while in
// StateQueueComplete calls undoLast, transitioning back to StateReviewing
// and issuing a non-nil command (playCmd for the previous song).
func TestHandleKey_QueueComplete_CtrlU(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)

	// Simulate: we just reached end-of-queue from index 1, so History has task[1].
	m.queue.CurrentIndex = 1
	m.queue.History = []domain.Task{m.queue.Tasks[1]}
	m.state = StateQueueComplete

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	resultModel := result.(Model)

	if resultModel.state != StateReviewing {
		t.Errorf("after Ctrl+U from StateQueueComplete: state = %v, want StateReviewing", resultModel.state)
	}
	if resultModel.queue.CurrentIndex != 0 {
		t.Errorf("CurrentIndex = %d, want 0 after undo", resultModel.queue.CurrentIndex)
	}
	if isNilCmd(cmd) {
		t.Error("expected a non-nil command (playCmd) after undo from queue complete, got nil")
	}
}

// TestHandleKey_QueueComplete_OtherKeysAreNoOps verifies that keys other than
// ctrl+c and ctrl+u are no-ops while in StateQueueComplete.
func TestHandleKey_QueueComplete_OtherKeysAreNoOps(t *testing.T) {
	mp := &mockPlayer{}
	m := newTestModel(mp)
	m.state = StateQueueComplete
	m.queue.CurrentIndex = 1

	noOpKeys := []tea.KeyMsg{
		{Type: tea.KeyEscape},
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune("p")},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
	}

	for _, key := range noOpKeys {
		result, cmd := m.handleKey(key)
		resultModel := result.(Model)
		if resultModel.state != StateQueueComplete {
			t.Errorf("key %q: state = %v, want StateQueueComplete (should be no-op)", key.String(), resultModel.state)
		}
		if resultModel.queue.CurrentIndex != 1 {
			t.Errorf("key %q: CurrentIndex = %d, want 1 (should be no-op)", key.String(), resultModel.queue.CurrentIndex)
		}
		if !isNilCmd(cmd) {
			t.Errorf("key %q: expected nil command (no-op), got non-nil", key.String())
		}
	}
}

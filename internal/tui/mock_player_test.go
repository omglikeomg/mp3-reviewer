package tui

import (
	"time"

	"song-reviewer/internal/audio"
)

// mockPlayer is a test-only implementation of AudioPlayer.
// All methods record their calls so tests can assert on them.
// Zero value is safe to use; all error fields default to nil (no errors).
type mockPlayer struct {
	// Configuration fields — set these before calling methods.
	playErr error               // Error to return from Play. nil = success.
	seekErr error               // Error to return from Seek. nil = success.
	state   audio.PlaybackState // Value to return from GetState.

	// Observation fields — inspect these after calling methods.
	playCalled  []string        // Paths passed to Play, in order.
	seekDeltas  []time.Duration // Deltas passed to Seek, in order.
	toggleCount int             // Number of times TogglePause was called.
	stopCount   int             // Number of times Stop was called.
	closed      bool            // True if Close was called at least once.
}

func (m *mockPlayer) Play(path string) error {
	m.playCalled = append(m.playCalled, path)
	return m.playErr
}

func (m *mockPlayer) Seek(delta time.Duration) error {
	m.seekDeltas = append(m.seekDeltas, delta)
	return m.seekErr
}

func (m *mockPlayer) TogglePause() {
	m.toggleCount++
}

func (m *mockPlayer) GetState() audio.PlaybackState {
	return m.state
}

func (m *mockPlayer) Stop() {
	m.stopCount++
}

func (m *mockPlayer) Close() {
	m.closed = true
}

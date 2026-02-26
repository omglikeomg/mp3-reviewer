package audio

import (
	"testing"
	"time"
)

// TestFormatDuration verifies the MM:SS formatting helper for typical song lengths.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "00:00",
		},
		{
			name:     "30 seconds",
			input:    30 * time.Second,
			expected: "00:30",
		},
		{
			name:     "1 minute 30 seconds",
			input:    90 * time.Second,
			expected: "01:30",
		},
		{
			name:     "4 minutes exactly",
			input:    4 * time.Minute,
			expected: "04:00",
		},
		{
			name:     "75 minutes 30 seconds (no hour rollover)",
			input:    75*time.Minute + 30*time.Second,
			expected: "75:30",
		},
		{
			name:     "negative duration is clamped to zero",
			input:    -5 * time.Second,
			expected: "00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestNewEngine verifies the constructor returns a non-nil Engine with clean state.
func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine() returned nil")
	}
	if e.streamer != nil {
		t.Error("expected streamer to be nil on a new Engine")
	}
	if e.ctrl != nil {
		t.Error("expected ctrl to be nil on a new Engine")
	}
	if e.fileHandle != nil {
		t.Error("expected fileHandle to be nil on a new Engine")
	}
	if e.speakerInitialized {
		t.Error("expected speakerInitialized to be false on a new Engine")
	}
}

// TestGetState_NoTrack verifies GetState returns a sane zero-value state when
// no track is loaded (no speaker device needed).
func TestGetState_NoTrack(t *testing.T) {
	e := NewEngine()
	state := e.GetState()

	if state.Progress != 0 {
		t.Errorf("expected Progress=0, got %f", state.Progress)
	}
	if state.Elapsed != "00:00" {
		t.Errorf("expected Elapsed=%q, got %q", "00:00", state.Elapsed)
	}
	if state.Total != "00:00" {
		t.Errorf("expected Total=%q, got %q", "00:00", state.Total)
	}
	if state.Position != "00:00 / 00:00" {
		t.Errorf("expected Position=%q, got %q", "00:00 / 00:00", state.Position)
	}
}

// TestSeek_NoTrack verifies Seek returns an error gracefully when no track is loaded.
func TestSeek_NoTrack(t *testing.T) {
	e := NewEngine()
	err := e.Seek(30 * time.Second)
	if err == nil {
		t.Fatal("expected an error when seeking with no track loaded, got nil")
	}
}

// TestClose_Idempotent verifies that calling Close on a never-used Engine does not panic.
func TestClose_Idempotent(t *testing.T) {
	e := NewEngine()
	// Must not panic.
	e.Close()
	// Second close must also not panic.
	e.Close()
}

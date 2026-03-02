package audio

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

const (
	// sampleRate is the fixed sample rate used to initialize the speaker device.
	// All MP3 streams are resampled to this rate via a beep.Resampler.
	sampleRate = beep.SampleRate(44100)

	// resampleQuality is passed to beep.Resample. A value of 3 gives good quality
	// with low CPU cost — appropriate for real-time on-the-fly resampling.
	resampleQuality = 3
)

// PlaybackState is a snapshot of the current audio playback position.
// It is safe to read from any goroutine after GetState returns.
type PlaybackState struct {
	// Progress is the playback position expressed as a fraction in [0.0, 1.0].
	// 0.0 means the beginning; 1.0 means the end. Returns 0 when nothing is playing.
	Progress float64

	// Elapsed is a human-readable string of the current position, e.g. "01:30".
	Elapsed string

	// Total is a human-readable string of the total duration, e.g. "04:00".
	Total string

	// Position is the elapsed/total string combined, e.g. "01:30 / 04:00".
	Position string
}

// Engine manages audio playback for a single file at a time.
// All public methods are safe to call from multiple goroutines.
type Engine struct {
	mu sync.Mutex

	// streamer is the raw MP3 decoder for the currently loaded file.
	// It implements beep.StreamSeekCloser, which gives us Seek, Len, Position, and Close.
	// It is nil when no file is loaded.
	streamer beep.StreamSeekCloser

	// format holds the beep.Format (SampleRate, NumChannels, Precision) of the
	// currently loaded stream. Required to calculate seek positions.
	format beep.Format

	// ctrl wraps the resampled streamer and allows pause/resume without
	// stopping the speaker. ctrl.Paused = true streams silence instead of audio.
	ctrl *beep.Ctrl

	// fileHandle is the *os.File underlying the current stream.
	// We keep a reference so we can close it explicitly when switching tracks.
	fileHandle *os.File

	// speakerInitialized tracks whether speaker.Init has already been called.
	// The speaker must be initialized exactly once for the lifetime of the process.
	speakerInitialized bool
}

// NewEngine creates a new Engine. The underlying speaker device is NOT initialized
// here; it is initialized lazily on the first call to Play. This avoids opening
// the audio device when the binary is run with --help or in headless test environments.
func NewEngine() *Engine {
	return &Engine{}
}

// initSpeaker initializes the speaker device if it has not been initialized yet.
// This must be called while the Engine's mu lock is held.
// The buffer is set to 100ms worth of samples, which provides a good balance
// between UI responsiveness and playback reliability.
func (e *Engine) initSpeaker() error {
	if e.speakerInitialized {
		return nil
	}
	bufferSize := sampleRate.N(time.Second / 10)
	if err := speaker.Init(sampleRate, bufferSize); err != nil {
		return fmt.Errorf("audio: initializing speaker: %w", err)
	}
	e.speakerInitialized = true
	return nil
}

// stopCurrent stops whatever is currently playing and releases all associated
// resources. It is safe to call when nothing is playing (all fields nil/zero).
// This must be called while the Engine's mu lock is held.
func (e *Engine) stopCurrent() {
	// Clear the speaker's queue so it stops pulling from ctrl immediately.
	// speaker.Clear() is safe to call even if nothing is playing.
	speaker.Clear()

	// Nil out ctrl so the speaker (if still running briefly) streams silence.
	if e.ctrl != nil {
		e.ctrl.Streamer = nil
		e.ctrl = nil
	}

	// Close the beep streamer (releases its internal buffers).
	if e.streamer != nil {
		_ = e.streamer.Close()
		e.streamer = nil
	}

	// Close the underlying file handle.
	if e.fileHandle != nil {
		_ = e.fileHandle.Close()
		e.fileHandle = nil
	}

	// Reset format to zero value.
	e.format = beep.Format{}
}

// Play stops any currently playing audio, opens the MP3 file at path, decodes
// it, and starts streaming it through the speaker. The file remains open for the
// duration of playback; it is closed when Play is called again, Seek causes an
// error, or Close is called.
//
// Returns an error if the file cannot be opened or decoded.
func (e *Engine) Play(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize the speaker device on first use.
	if err := e.initSpeaker(); err != nil {
		return err
	}

	// Stop and clean up whatever was playing before.
	e.stopCurrent()

	// Open the file.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("audio: opening file %q: %w", path, err)
	}

	// Decode the MP3 stream.
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("audio: decoding MP3 %q: %w", path, err)
	}

	// Store state.
	e.fileHandle = f
	e.streamer = streamer
	e.format = format

	// Wrap the streamer in a Ctrl so we can pause/resume later.
	// Resample to the fixed speaker sample rate to handle MP3s with non-standard rates.
	resampled := beep.Resample(resampleQuality, format.SampleRate, sampleRate, streamer)
	e.ctrl = &beep.Ctrl{Streamer: resampled, Paused: false}

	// Hand off to the speaker. speaker.Play is non-blocking; it starts a background
	// goroutine that pulls samples from ctrl.
	speaker.Play(e.ctrl)

	return nil
}

// Seek moves the playback position by delta (which may be negative for rewind).
// If the resulting position would exceed the total duration, playback loops back
// to the beginning. If the resulting position would go before the start, playback
// jumps to position 0 (start of track).
//
// Returns an error if no track is loaded or if the underlying seek fails.
func (e *Engine) Seek(delta time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamer == nil {
		return fmt.Errorf("audio: seek called with no track loaded")
	}

	totalSamples := e.streamer.Len()
	currentPos := e.streamer.Position()

	// Convert the time delta into a sample offset using the stream's native sample rate.
	deltaInSamples := e.format.SampleRate.N(delta)
	newPos := currentPos + deltaInSamples

	// Apply wrap-around rules (Q1 human decision: Option C — clamp to 0 in both directions).
	if newPos >= totalSamples {
		// Overshot the end — loop to the very beginning.
		newPos = 0
	} else if newPos < 0 {
		// Undershot the start — stay at position 0 (do not loop backwards).
		newPos = 0
	}

	// Lock the speaker before seeking to prevent the speaker goroutine from
	// reading the streamer's position mid-seek.
	speaker.Lock()
	err := e.streamer.Seek(newPos)
	speaker.Unlock()

	if err != nil {
		return fmt.Errorf("audio: seeking to sample %d: %w", newPos, err)
	}

	return nil
}

// TogglePause pauses playback if currently playing, or resumes if currently paused.
// It is a no-op if no track is loaded.
//
// TODO(tui-task): Wire TogglePause into the TUI keybinding layer. The recommended
// shortcut is 'p' (see Keybindings table in README.md). The TUI agent should call
// engine.TogglePause() in the Update function when tea.KeyMsg.String() == "p".
func (e *Engine) TogglePause() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctrl == nil {
		return
	}

	speaker.Lock()
	e.ctrl.Paused = !e.ctrl.Paused
	speaker.Unlock()
}

// GetState returns a snapshot of the current playback position.
// If no track is loaded, it returns a zero PlaybackState (Progress = 0, empty strings).
// This method is safe to call from any goroutine, including the TUI render loop.
func (e *Engine) GetState() PlaybackState {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamer == nil {
		return PlaybackState{
			Progress: 0,
			Elapsed:  "00:00",
			Total:    "00:00",
			Position: "00:00 / 00:00",
		}
	}

	// Lock the speaker while reading position to avoid a race with the
	// background streaming goroutine advancing e.streamer.Position().
	speaker.Lock()
	currentPos := e.streamer.Position()
	speaker.Unlock()

	totalSamples := e.streamer.Len()

	// Calculate progress fraction.
	var progress float64
	if totalSamples > 0 {
		progress = float64(currentPos) / float64(totalSamples)
		if progress > 1.0 {
			progress = 1.0
		}
	}

	// Convert sample counts to durations using the stream's native sample rate.
	elapsed := e.format.SampleRate.D(currentPos)
	total := e.format.SampleRate.D(totalSamples)

	elapsedStr := formatDuration(elapsed)
	totalStr := formatDuration(total)

	return PlaybackState{
		Progress: progress,
		Elapsed:  elapsedStr,
		Total:    totalStr,
		Position: elapsedStr + " / " + totalStr,
	}
}

// Stop halts playback and releases the current file handle without closing
// the speaker device. Safe to call when nothing is playing (all fields nil).
// Use this before writing ID3 tags to the currently-playing file on Windows,
// where an open file handle blocks os.Rename (the rename that bogem/id3v2
// performs internally during Save).
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopCurrent()
}

// Close stops all playback, releases the audio file handle, and closes the
// speaker device. It should be called exactly once when the application exits
// (e.g., in the Bubble Tea shutdown hook).
//
// After Close returns, the Engine must not be used again.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopCurrent()

	if e.speakerInitialized {
		speaker.Close()
		e.speakerInitialized = false
	}
}

// formatDuration converts a time.Duration into a "MM:SS" string.
// Hours are not displayed; durations >= 60 minutes will show as e.g. "75:30".
// This keeps the display compact for typical song lengths.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

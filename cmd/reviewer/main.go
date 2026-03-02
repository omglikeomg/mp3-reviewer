package main

import (
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/provider"
	"song-reviewer/internal/tui"
)

var version = "dev"

const helpText = `song-reviewer — interactively review and tag MP3 files from a manual-review queue.

Usage: song-reviewer [--help] [--version]

Configuration:
  Copy settings.example.json to config/settings.json and set your paths before first run.
  Key fields:
    music_folder          Absolute path to your music library root.
    review_json_path      Path to the JSON file of songs to review.
    genres                List of genre labels available for tagging.
    seek_delta_seconds    Seek step in seconds (default: 30).
    skip_applied          Omit already-tagged songs from the queue (default: false).
    api_keys.musicbrainz_user_agent
                          Required MusicBrainz User-Agent string.

  Example: config/settings.json  (template: settings.example.json)

Keybindings:
  ← / →         Seek backward / forward (seek_delta_seconds)
  p             Pause / Resume playback
  Enter / Space Open genre selection menu
  t             Tap to the beat — calculates BPM (8 taps required)
  Ctrl+1        Commit BPM to TBPM tag
  Ctrl+2        Commit Year to year tag
  Esc           Skip current song
  Ctrl+U        Undo last genre assignment
  Ctrl+O        Open Settings overlay
  Ctrl+C        Quit

For full documentation see README.md.
`

func main() {
	// ── CLI flags ─────────────────────────────────────────────────────────────
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Print(helpText)
			os.Exit(0)
		case "--version", "-v":
			fmt.Println("song-reviewer", version)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "song-reviewer: unknown flag: %s\nRun 'song-reviewer --help' for usage.\n", os.Args[1])
			os.Exit(1)
		}
	}

	// ── Load configuration ────────────────────────────────────────────────────
	cfg, err := loadConfig("config/settings.json")
	if err != nil {
		// Non-fatal: warn but continue with defaults so the app starts even without
		// a settings file (the user can configure paths via the Settings overlay).
		fmt.Fprintf(os.Stderr, "song-reviewer: warning: could not load config (%v) — using defaults\n", err)
		cfg = defaultConfig()
	}

	// ── Load review queue ─────────────────────────────────────────────────────
	p := provider.ManualReviewProvider{Config: cfg}
	tasks, err := p.GetTasks()
	if err != nil {
		// Non-fatal: warn and start with an empty queue. The user can reload via
		// the Settings overlay (Ctrl-O) after correcting the paths.
		fmt.Fprintf(os.Stderr, "song-reviewer: warning: could not load review queue (%v) — starting with empty queue\n", err)
		tasks = []domain.Task{}
	}

	queue := domain.ReviewQueue{
		Tasks:        tasks,
		CurrentIndex: 0,
		History:      []domain.Task{},
	}

	// ── Construct audio engine ────────────────────────────────────────────────
	engine := audio.NewEngine()

	// ── Construct TUI model ───────────────────────────────────────────────────
	// New() stores the startup command batch (auto-play + first tick) in the
	// model's pendingInit field. Init() will return and dispatch it inside the
	// Bubble Tea event loop — this is the canonical Q1-B pattern.
	model := tui.New(queue, engine, cfg, p)

	// ── Start Bubble Tea program ──────────────────────────────────────────────
	prog := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use the alternate screen buffer (hides shell history).
		tea.WithMouseCellMotion(), // Enable mouse support for future use.
	)

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: program error: %v\n", err)
		// Ensure audio device is released even on unexpected exit.
		engine.Close()
		os.Exit(1)
	}

	// Clean shutdown: close the audio device if the TUI didn't already.
	// engine.Close() is idempotent — safe to call even if ctrl+c already closed it.
	engine.Close()
}

// loadConfig reads and unmarshals the JSON settings file at the given path.
func loadConfig(path string) (domain.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.AppConfig{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var cfg domain.AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.AppConfig{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return cfg, nil
}

// defaultConfig returns a safe AppConfig used when settings.json is missing or
// unreadable. The user can update MusicFolder and JsonPath via the Ctrl-O
// Settings overlay at runtime.
func defaultConfig() domain.AppConfig {
	return domain.AppConfig{
		MusicFolder:      "",
		JsonPath:         "./data/manual_review.json",
		GenreList:        []string{},
		SeekDeltaSeconds: 30,
	}
}

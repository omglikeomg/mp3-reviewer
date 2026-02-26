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

func main() {
	// ── Load configuration ────────────────────────────────────────────────────
	cfg, err := loadConfig("config/settings.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ── Load review queue ─────────────────────────────────────────────────────
	p := provider.ManualReviewProvider{Config: cfg}
	tasks, err := p.GetTasks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "song-reviewer: failed to load review queue: %v\n", err)
		os.Exit(1)
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

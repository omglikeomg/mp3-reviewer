package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"song-reviewer/internal/domain"
)

// TaskProvider is the interface for all review queue sources.
// Any type that can supply a list of Tasks to review implements this interface.
type TaskProvider interface {
	GetTasks() ([]domain.Task, error)
}

// manualReviewFile mirrors the top-level structure of the manual_review JSON file.
type manualReviewFile struct {
	ManualReview []reviewEntry `json:"manual_review"`
}

// reviewEntry mirrors a single item in the "manual_review" array.
type reviewEntry struct {
	FilePath   string `json:"filepath"`
	Reason     string `json:"reason"`
	Confidence int    `json:"confidence"`
}

// ManualReviewProvider reads a JSON file containing a "manual_review" array
// and converts each entry into a domain.Task.
type ManualReviewProvider struct {
	Config domain.AppConfig
}

// GetTasks reads the JSON file at Config.JsonPath, parses the "manual_review"
// array, and returns a slice of domain.Task values. Each task's FilePath is
// resolved to an absolute path by joining Config.MusicFolder with the relative
// path stored in the JSON.
func (p ManualReviewProvider) GetTasks() ([]domain.Task, error) {
	data, err := os.ReadFile(p.Config.JsonPath)
	if err != nil {
		return nil, fmt.Errorf("provider: reading review file %q: %w", p.Config.JsonPath, err)
	}

	var raw manualReviewFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("provider: parsing review file %q: %w", p.Config.JsonPath, err)
	}

	tasks := make([]domain.Task, 0, len(raw.ManualReview))
	for _, entry := range raw.ManualReview {
		task := domain.Task{
			FilePath: filepath.Join(p.Config.MusicFolder, entry.FilePath),
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

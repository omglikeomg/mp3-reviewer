package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"song-reviewer/internal/domain"
	"song-reviewer/internal/metadata"
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
// Fields added in Task 4: Status, PrimaryGenre, SecondaryGenre.
type reviewEntry struct {
	FilePath       string `json:"filepath"`
	Reason         string `json:"reason"`
	Confidence     int    `json:"confidence"`
	Status         string `json:"status,omitempty"`
	PrimaryGenre   string `json:"primary_genre,omitempty"`
	SecondaryGenre string `json:"secondary_genre,omitempty"`
	BPM            string `json:"bpm,omitempty"`
	Year           string `json:"year,omitempty"`
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
		// When skip_applied is enabled, silently omit entries already tagged.
		if p.Config.SkipApplied && strings.EqualFold(entry.Status, "applied") {
			continue
		}

		absPath := filepath.Join(p.Config.MusicFolder, entry.FilePath)

		// Read Title and Artist from the file's ID3 tags. Non-fatal: if the file
		// is missing, corrupted, or has no tags, Title and Artist remain empty
		// and the TUI falls back to a filename display.
		title, artist, _ := metadata.ReadTags(absPath)

		task := domain.Task{
			FilePath: absPath,
			Title:    title,
			Artist:   artist,
			Genre1:   entry.PrimaryGenre,
			Genre2:   entry.SecondaryGenre,
			BPM:      entry.BPM,
			Year:     entry.Year,
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// SaveState persists genre assignments back into the source JSON file on disk.
// For each task in the provided slice, if Genre1 is non-empty the corresponding
// JSON entry is updated with status="applied", primary_genre, and secondary_genre.
// Matching is done by relative filepath (task path with MusicFolder prefix stripped).
//
// The write is done atomically: the file is first written to a temp file inside a
// hidden .tmp/ subdirectory (sibling to the target JSON file), synced to stable
// storage (fsync), and then renamed over the original. This prevents data loss if
// the process is killed mid-write, even on a sudden power failure.
func (p ManualReviewProvider) SaveState(tasks []domain.Task) error {
	// Read current file content.
	data, err := os.ReadFile(p.Config.JsonPath)
	if err != nil {
		return fmt.Errorf("provider: SaveState reading %q: %w", p.Config.JsonPath, err)
	}

	var raw manualReviewFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("provider: SaveState parsing %q: %w", p.Config.JsonPath, err)
	}

	// Build a lookup: relative filepath -> task
	taskMap := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		// Derive the relative path by stripping the music folder prefix.
		rel := t.FilePath
		if strings.HasPrefix(rel, p.Config.MusicFolder) {
			rel = strings.TrimPrefix(rel, p.Config.MusicFolder)
			rel = strings.TrimPrefix(rel, string(filepath.Separator))
		}
		taskMap[rel] = t
	}

	// Update matching entries in the raw struct.
	for i, entry := range raw.ManualReview {
		t, ok := taskMap[entry.FilePath]
		if !ok {
			continue
		}
		// Update genre fields if a genre has been assigned.
		if t.Genre1 != "" {
			raw.ManualReview[i].Status = "applied"
			raw.ManualReview[i].PrimaryGenre = t.Genre1
			raw.ManualReview[i].SecondaryGenre = t.Genre2
		}
		// Update BPM and Year independently — they can be committed
		// without a genre assignment (Q5 human decision).
		if t.BPM != "" {
			raw.ManualReview[i].BPM = t.BPM
		}
		if t.Year != "" {
			raw.ManualReview[i].Year = t.Year
		}
	}

	// Marshal back to JSON with indentation for human readability.
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("provider: SaveState marshalling: %w", err)
	}

	// Atomic write via temp file + rename.
	// Temp files are created inside a hidden .tmp/ subdirectory that is a sibling
	// of the target JSON file. This keeps temp files out of JSON globs and casual
	// directory listings while keeping them on the same filesystem as the target
	// (required for os.Rename to be atomic).
	tmpDir := filepath.Join(filepath.Dir(p.Config.JsonPath), ".tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return fmt.Errorf("provider: SaveState creating temp dir: %w", err)
	}
	tmp, err := os.CreateTemp(tmpDir, "manual_review_*.json.tmp")
	if err != nil {
		return fmt.Errorf("provider: SaveState creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState writing temp file: %w", err)
	}
	// Sync flushes the OS page cache to stable storage before we rename.
	// Without this, a power failure after rename could leave a zero-length or
	// truncated file even though the rename syscall succeeded.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, p.Config.JsonPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("provider: SaveState renaming temp file: %w", err)
	}

	return nil
}

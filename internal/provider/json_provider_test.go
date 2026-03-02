package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"song-reviewer/internal/domain"
)

func TestGetTasks_HappyPath(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			},
			{
				"filepath": "Miles Davis/Kind of Blue.mp3",
				"reason": "Uncertain subgenre",
				"confidence": 2
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() returned unexpected error: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	want0 := filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3")
	if tasks[0].FilePath != want0 {
		t.Errorf("tasks[0].FilePath = %q, want %q", tasks[0].FilePath, want0)
	}

	want1 := filepath.Join("/test/music", "Miles Davis/Kind of Blue.mp3")
	if tasks[1].FilePath != want1 {
		t.Errorf("tasks[1].FilePath = %q, want %q", tasks[1].FilePath, want1)
	}

	// Fields not populated by the provider must be empty strings.
	emptyFields := []struct {
		name  string
		value string
	}{
		{"Title", tasks[0].Title},
		{"Artist", tasks[0].Artist},
		{"Album", tasks[0].Album},
		{"Genre1", tasks[0].Genre1},
		{"Genre2", tasks[0].Genre2},
		{"Year", tasks[0].Year},
		{"BPM", tasks[0].BPM},
	}
	for _, f := range emptyFields {
		if f.value != "" {
			t.Errorf("tasks[0].%s = %q, want empty string", f.name, f.value)
		}
	}
}

func TestGetTasks_EmptyArray(t *testing.T) {
	const sampleJSON = `{"manual_review": []}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() returned unexpected error: %v", err)
	}
	if tasks == nil {
		t.Fatal("expected non-nil slice for empty array, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestGetTasks_FileNotFound(t *testing.T) {
	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    "/nonexistent/path/that/does/not/exist.json",
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err == nil {
		t.Fatal("expected an error for missing file, got nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
}

func TestGetTasks_MalformedJSON(t *testing.T) {
	const badJSON = `{bad json`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(jsonPath, []byte(badJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	provider := ManualReviewProvider{Config: cfg}

	tasks, err := provider.GetTasks()
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
}

func TestSaveState_UpdatesAppliedEntries(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			},
			{
				"filepath": "Miles Davis/Kind of Blue.mp3",
				"reason": "Uncertain subgenre",
				"confidence": 2
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	musicFolder := "/test/music"
	cfg := domain.AppConfig{
		MusicFolder: musicFolder,
		JsonPath:    jsonPath,
	}
	p := ManualReviewProvider{Config: cfg}

	tasks := []domain.Task{
		{
			FilePath: filepath.Join(musicFolder, "Cream/Cream - Strange Brew.mp3"),
			Genre1:   "Rock",
			Genre2:   "Blues-Rock",
		},
		// Miles Davis intentionally left un-tagged (Genre1 == "").
		{
			FilePath: filepath.Join(musicFolder, "Miles Davis/Kind of Blue.mp3"),
			Genre1:   "",
		},
	}

	if err := p.SaveState(tasks); err != nil {
		t.Fatalf("SaveState() returned unexpected error: %v", err)
	}

	// Re-read and verify.
	data, _ := os.ReadFile(jsonPath)
	var raw struct {
		ManualReview []struct {
			FilePath       string `json:"filepath"`
			Status         string `json:"status"`
			PrimaryGenre   string `json:"primary_genre"`
			SecondaryGenre string `json:"secondary_genre"`
		} `json:"manual_review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parsing saved JSON: %v", err)
	}

	if raw.ManualReview[0].Status != "applied" {
		t.Errorf("entry[0].status = %q, want %q", raw.ManualReview[0].Status, "applied")
	}
	if raw.ManualReview[0].PrimaryGenre != "Rock" {
		t.Errorf("entry[0].primary_genre = %q, want %q", raw.ManualReview[0].PrimaryGenre, "Rock")
	}
	if raw.ManualReview[0].SecondaryGenre != "Blues-Rock" {
		t.Errorf("entry[0].secondary_genre = %q, want %q", raw.ManualReview[0].SecondaryGenre, "Blues-Rock")
	}

	// The un-tagged Miles Davis entry must be unchanged.
	if raw.ManualReview[1].Status != "" {
		t.Errorf("entry[1].status = %q, want empty (untagged)", raw.ManualReview[1].Status)
	}
}

func TestSaveState_NoneSecondaryGenre(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	p := ManualReviewProvider{Config: cfg}

	tasks := []domain.Task{
		{
			FilePath: filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3"),
			Genre1:   "Rock",
			Genre2:   "", // [NONE] chosen — secondary should be omitted from JSON.
		},
	}

	if err := p.SaveState(tasks); err != nil {
		t.Fatalf("SaveState() unexpected error: %v", err)
	}

	data, _ := os.ReadFile(jsonPath)
	var raw struct {
		ManualReview []struct {
			SecondaryGenre string `json:"secondary_genre"`
		} `json:"manual_review"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("re-parsing saved JSON: %v", err)
	}
	if raw.ManualReview[0].SecondaryGenre != "" {
		t.Errorf("secondary_genre = %q, want empty string (omitempty)", raw.ManualReview[0].SecondaryGenre)
	}
}

// TestSaveState_OriginalFilePreservedOnWriteError verifies that if the temp-file
// write is interrupted (simulated here by making the target directory read-only so
// that os.MkdirAll cannot create the .tmp/ subdirectory), the original JSON file
// is left completely intact and SaveState returns a non-nil error.
//
// This test is skipped when run as root (where filesystem permissions are not
// enforced).
func TestSaveState_OriginalFilePreservedOnWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root — filesystem permission checks are not enforced")
	}

	const originalJSON = `{
		"manual_review": [
			{
				"filepath": "Cream/Cream - Strange Brew.mp3",
				"reason": "Genre not in taxonomy",
				"confidence": 4
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(originalJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	cfg := domain.AppConfig{
		MusicFolder: "/test/music",
		JsonPath:    jsonPath,
	}
	p := ManualReviewProvider{Config: cfg}

	tasks := []domain.Task{
		{
			FilePath: filepath.Join("/test/music", "Cream/Cream - Strange Brew.mp3"),
			Genre1:   "Rock",
			Genre2:   "Blues-Rock",
		},
	}

	// Make the directory read-only so os.MkdirAll cannot create the .tmp/
	// subdirectory, causing SaveState to fail before touching the original file.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("setup: chmod dir: %v", err)
	}
	// Restore write permission so t.TempDir cleanup can delete the directory.
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := p.SaveState(tasks)
	if err == nil {
		t.Fatal("SaveState() expected an error when temp dir cannot be created, got nil")
	}

	// The original file must be byte-for-byte identical to what was written at setup.
	got, readErr := os.ReadFile(jsonPath)
	if readErr != nil {
		t.Fatalf("reading original file after failed SaveState: %v", readErr)
	}
	if string(got) != originalJSON {
		t.Errorf("original file was modified by a failing SaveState:\ngot:  %q\nwant: %q", string(got), originalJSON)
	}
}

// TestGetTasks_SkipApplied verifies that when Config.SkipApplied is true,
// GetTasks omits entries whose status is "applied" from the returned slice.
// When SkipApplied is false (the default), all entries are returned regardless
// of their status.
func TestGetTasks_SkipApplied(t *testing.T) {
	const sampleJSON = `{
		"manual_review": [
			{
				"filepath": "Artist/Song1.mp3",
				"reason": "Uncertain genre",
				"confidence": 3,
				"status": "applied",
				"primary_genre": "Rock"
			},
			{
				"filepath": "Artist/Song2.mp3",
				"reason": "Uncertain genre",
				"confidence": 3
			}
		]
	}`

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "manual_review.json")
	if err := os.WriteFile(jsonPath, []byte(sampleJSON), 0644); err != nil {
		t.Fatalf("setup: writing temp JSON file: %v", err)
	}

	t.Run("SkipApplied=true omits applied entries", func(t *testing.T) {
		cfg := domain.AppConfig{
			MusicFolder: "/test/music",
			JsonPath:    jsonPath,
			SkipApplied: true,
		}
		p := ManualReviewProvider{Config: cfg}

		tasks, err := p.GetTasks()
		if err != nil {
			t.Fatalf("GetTasks() returned unexpected error: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task (applied entry skipped), got %d", len(tasks))
		}
		want := filepath.Join("/test/music", "Artist/Song2.mp3")
		if tasks[0].FilePath != want {
			t.Errorf("tasks[0].FilePath = %q, want %q", tasks[0].FilePath, want)
		}
	})

	t.Run("SkipApplied=false returns all entries", func(t *testing.T) {
		cfg := domain.AppConfig{
			MusicFolder: "/test/music",
			JsonPath:    jsonPath,
			SkipApplied: false,
		}
		p := ManualReviewProvider{Config: cfg}

		tasks, err := p.GetTasks()
		if err != nil {
			t.Fatalf("GetTasks() returned unexpected error: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks (all entries returned), got %d", len(tasks))
		}
	})
}

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

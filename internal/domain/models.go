package domain

// Task represents a single song to be reviewed and tagged.
type Task struct {
	FilePath string // Absolute path to the audio file on disk.
	Title    string // Song title (from ID3 tags or filename; populated later).
	Artist   string // Artist name (from ID3 tags; populated later).
	Album    string // Album name (from ID3 tags; populated later).
	Genre1   string // Primary genre — set by the user during review.
	Genre2   string // Secondary genre — set by the user during review. May be empty.
	Year     string // Release year — fetched from MusicBrainz or set manually.
	BPM      string // Beats per minute — fetched from BPM API or set manually.
}

// AppConfig holds the application-wide configuration loaded from settings.json.
type AppConfig struct {
	MusicFolder string   `json:"music_folder"`
	JsonPath    string   `json:"review_json_path"`
	GenreList   []string `json:"genres"`
	ApiKeys     struct {
		MusicBrainzUserAgent string `json:"musicbrainz_user_agent"`
	} `json:"api_keys"`
}

// ReviewQueue tracks the current position in the review queue and supports undo.
type ReviewQueue struct {
	Tasks        []Task // Ordered list of all tasks loaded from the JSON file.
	CurrentIndex int    // Index of the task currently being reviewed.
	History      []Task // Stack of completed tasks; used for Undo (Ctrl+U).
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchYear_HappyPath(t *testing.T) {
	// Build a fake MusicBrainz response with two release groups.
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				ReleaseGroups: []mbReleaseGroupRef{
					{ID: "rg1", FirstReleaseDate: "1973-03-01"},
					{ID: "rg2", FirstReleaseDate: "1971-11-08"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	// Temporarily override the base URL so FetchYear hits the mock server.
	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Led Zeppelin", "Stairway to Heaven", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	// Earliest date is 1971-11-08, so year should be "1971".
	if year != "1971" {
		t.Errorf("FetchYear() = %q, want %q", year, "1971")
	}
}

func TestFetchYear_NoRecordings(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Unknown Artist", "Unknown Song", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	if year != "" {
		t.Errorf("FetchYear() = %q, want empty string", year)
	}
}

func TestFetchYear_EmptyInputs(t *testing.T) {
	_, err := FetchYear("", "", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for empty artist and title, got nil")
	}
}

func TestFetchYear_MissingUserAgent(t *testing.T) {
	_, err := FetchYear("Artist", "Title", "")
	if err == nil {
		t.Fatal("expected an error for empty userAgent, got nil")
	}
}

func TestFetchYear_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	_, err := FetchYear("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for non-200 HTTP status, got nil")
	}
}

func TestFetchBPM_HappyPath(t *testing.T) {
	// Build a fake MusicBrainz response with a BPM-related tag.
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				Tags: []mbTag{
					{Name: "rock", Count: 10},
					{Name: "120 bpm", Count: 3},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	bpm, err := FetchBPM("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchBPM() unexpected error: %v", err)
	}
	if bpm != "120" {
		t.Errorf("FetchBPM() = %q, want %q", bpm, "120")
	}
}

func TestFetchBPM_NoBPMTag(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				Tags: []mbTag{
					{Name: "rock", Count: 10},
					{Name: "alternative", Count: 5},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	bpm, err := FetchBPM("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchBPM() unexpected error: %v", err)
	}
	if bpm != "" {
		t.Errorf("FetchBPM() = %q, want empty string (no BPM tag found)", bpm)
	}
}

func TestFetchBPM_NoRecordings(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	bpm, err := FetchBPM("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchBPM() unexpected error: %v", err)
	}
	if bpm != "" {
		t.Errorf("FetchBPM() = %q, want empty string", bpm)
	}
}

func TestFetchBPM_EmptyInputs(t *testing.T) {
	_, err := FetchBPM("", "", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for empty artist and title, got nil")
	}
}

func TestFetchBPM_MissingUserAgent(t *testing.T) {
	_, err := FetchBPM("Artist", "Title", "")
	if err == nil {
		t.Fatal("expected an error for empty userAgent, got nil")
	}
}

func TestFetchBPM_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	_, err := FetchBPM("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err == nil {
		t.Fatal("expected an error for non-200 HTTP status, got nil")
	}
}

func TestFetchBPM_BPMPrefixFormat(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				Tags: []mbTag{
					{Name: "bpm:140", Count: 2},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	bpm, err := FetchBPM("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchBPM() unexpected error: %v", err)
	}
	if bpm != "140" {
		t.Errorf("FetchBPM() = %q, want %q", bpm, "140")
	}
}

func TestExtractBPMFromTag(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"120 bpm", "120"},
		{"bpm:140", "140"},
		{"BPM 90", "90"},
		{"85bpm", "85"},
		{"rock", ""},
		{"alternative", ""},
		{"", ""},
		{"10 bpm", ""},  // below range (< 20)
		{"999 bpm", ""}, // above range (> 300)
		{"bpm:200", "200"},
		{"BPM:60", "60"},
		{" 128 BPM ", "128"},
		{"300", "300"},
		{"20", "20"},
		{"19", ""},
		{"301", ""},
	}

	for _, tt := range tests {
		got := extractBPMFromTag(tt.tag)
		if got != tt.want {
			t.Errorf("extractBPMFromTag(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestFetchYear_UserAgentSent(t *testing.T) {
	var receivedUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mbRecordingSearchResponse{})
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	expectedUA := "TestApp/1.0 ( test@example.com )"
	_, _ = FetchYear("Artist", "Title", expectedUA)

	if receivedUA != expectedUA {
		t.Errorf("User-Agent = %q, want %q", receivedUA, expectedUA)
	}
}

func TestFetchYear_OnlyArtist(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				ReleaseGroups: []mbReleaseGroupRef{
					{ID: "rg1", FirstReleaseDate: "2005"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Artist", "", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	if year != "2005" {
		t.Errorf("FetchYear() = %q, want %q", year, "2005")
	}
}

func TestFetchYear_OnlyTitle(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				ReleaseGroups: []mbReleaseGroupRef{
					{ID: "rg1", FirstReleaseDate: "1999-06"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	if year != "1999" {
		t.Errorf("FetchYear() = %q, want %q", year, "1999")
	}
}

func TestFetchYear_NoReleaseDates(t *testing.T) {
	fakeResponse := mbRecordingSearchResponse{
		Recordings: []mbRecording{
			{
				ReleaseGroups: []mbReleaseGroupRef{
					{ID: "rg1", FirstReleaseDate: ""},
					{ID: "rg2", FirstReleaseDate: ""},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeResponse)
	}))
	defer srv.Close()

	origBase := musicBrainzBaseURL
	musicBrainzBaseURL = srv.URL
	defer func() { musicBrainzBaseURL = origBase }()

	year, err := FetchYear("Artist", "Title", "TestApp/1.0 ( test@example.com )")
	if err != nil {
		t.Fatalf("FetchYear() unexpected error: %v", err)
	}
	if year != "" {
		t.Errorf("FetchYear() = %q, want empty string (no dates available)", year)
	}
}

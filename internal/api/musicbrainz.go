package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// musicBrainzBaseURL is a package-level var (not const) so that tests can
// override it with an httptest server URL.
var musicBrainzBaseURL = "https://musicbrainz.org/ws/2"

// mbRecordingSearchResponse mirrors the top-level JSON returned by
// GET /ws/2/recording?query=...&fmt=json
type mbRecordingSearchResponse struct {
	Recordings []mbRecording `json:"recordings"`
}

// mbRecording mirrors a single recording entry in the search results.
type mbRecording struct {
	ReleaseGroups []mbReleaseGroupRef `json:"release-groups"`
	Tags          []mbTag             `json:"tags"`
}

// mbReleaseGroupRef mirrors the release-group object embedded in a recording.
type mbReleaseGroupRef struct {
	ID               string `json:"id"`
	FirstReleaseDate string `json:"first-release-date"`
}

// mbTag mirrors a user-submitted tag on a MusicBrainz recording.
type mbTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// buildRecordingQuery constructs the Lucene query string for a MusicBrainz
// recording search from the given artist and title.
func buildRecordingQuery(artist, title string) string {
	queryParts := []string{}
	if artist != "" {
		queryParts = append(queryParts, fmt.Sprintf(`artist:"%s"`, artist))
	}
	if title != "" {
		queryParts = append(queryParts, fmt.Sprintf(`recording:"%s"`, title))
	}
	return strings.Join(queryParts, " AND ")
}

// doRecordingSearch performs a MusicBrainz recording search with the given
// Lucene query and returns the parsed response. It requests both release-groups
// and tags in a single API call so that both year and BPM data (if available)
// can be extracted from one request.
func doRecordingSearch(query, userAgent string) (*mbRecordingSearchResponse, error) {
	searchURL := fmt.Sprintf(
		"%s/recording?query=%s&limit=5&fmt=json&inc=release-groups+tags",
		musicBrainzBaseURL,
		url.QueryEscape(query),
	)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz: unexpected HTTP status %d", resp.StatusCode)
	}

	var result mbRecordingSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("musicbrainz: decoding response: %w", err)
	}

	return &result, nil
}

// FetchYear queries the MusicBrainz JSON API to find the earliest original
// release year for the given artist and title combination.
//
// It performs a recording search, inspects the release groups of the top
// result, and returns the 4-digit year extracted from the earliest
// first-release-date found.
//
// userAgent must be a non-empty string in the form "AppName/Version ( email )"
// as required by MusicBrainz rate-limiting policy.
//
// Returns ("", nil) if no matching recording or release year is found.
// Returns ("", err) on network errors or unexpected response formats.
func FetchYear(artist, title, userAgent string) (string, error) {
	if artist == "" && title == "" {
		return "", fmt.Errorf("musicbrainz: FetchYear called with empty artist and title")
	}
	if userAgent == "" {
		return "", fmt.Errorf("musicbrainz: FetchYear requires a non-empty userAgent")
	}

	query := buildRecordingQuery(artist, title)
	result, err := doRecordingSearch(query, userAgent)
	if err != nil {
		return "", err
	}

	if len(result.Recordings) == 0 {
		return "", nil
	}

	// Collect all first-release-dates from the release groups of the top result.
	topRecording := result.Recordings[0]
	var dates []string
	for _, rg := range topRecording.ReleaseGroups {
		if rg.FirstReleaseDate != "" {
			dates = append(dates, rg.FirstReleaseDate)
		}
	}

	if len(dates) == 0 {
		return "", nil
	}

	// Sort lexicographically — ISO date strings (YYYY, YYYY-MM, YYYY-MM-DD)
	// sort correctly in lexicographic order when all are the same length or
	// prefixed by year. We extract the year from the earliest.
	sort.Strings(dates)
	earliest := dates[0]

	// Extract the 4-digit year prefix.
	if len(earliest) < 4 {
		return "", nil
	}
	year := earliest[:4]

	return year, nil
}

// FetchBPM queries the MusicBrainz JSON API to find a BPM value for the given
// artist and title combination. MusicBrainz does not have a dedicated BPM field
// on recordings, but user-contributed tags sometimes include BPM-related values.
// This function checks the tags of the top recording result for any numeric
// tag that could represent a BPM value (typically in the 20–300 range).
//
// This is called before falling back to Tap Tempo. Since MusicBrainz tags
// rarely contain BPM data, this will often return ("", nil) — indicating
// no BPM was found and the user should use Tap Tempo instead.
//
// userAgent must be a non-empty string as required by MusicBrainz.
//
// Returns ("", nil) if no BPM-related tag is found.
// Returns ("", err) on network errors.
func FetchBPM(artist, title, userAgent string) (string, error) {
	if artist == "" && title == "" {
		return "", fmt.Errorf("musicbrainz: FetchBPM called with empty artist and title")
	}
	if userAgent == "" {
		return "", fmt.Errorf("musicbrainz: FetchBPM requires a non-empty userAgent")
	}

	query := buildRecordingQuery(artist, title)
	result, err := doRecordingSearch(query, userAgent)
	if err != nil {
		return "", err
	}

	if len(result.Recordings) == 0 {
		return "", nil
	}

	// Check user-contributed tags for a numeric value in BPM range (20–300).
	// MusicBrainz tags are free-text labels; some users tag BPM values like
	// "120 bpm" or just "120". We look for patterns that suggest a BPM.
	topRecording := result.Recordings[0]
	for _, tag := range topRecording.Tags {
		bpm := extractBPMFromTag(tag.Name)
		if bpm != "" {
			return bpm, nil
		}
	}

	return "", nil
}

// extractBPMFromTag attempts to extract a numeric BPM value from a MusicBrainz
// tag name string. It handles formats like "120", "120 bpm", "bpm:120", etc.
// Returns the numeric string if a plausible BPM value (20–300) is found,
// or empty string otherwise.
func extractBPMFromTag(tagName string) string {
	// Normalize: lowercase, trim whitespace.
	s := strings.ToLower(strings.TrimSpace(tagName))

	// Remove common BPM prefixes/suffixes.
	s = strings.TrimPrefix(s, "bpm:")
	s = strings.TrimPrefix(s, "bpm ")
	s = strings.TrimSuffix(s, " bpm")
	s = strings.TrimSuffix(s, "bpm")
	s = strings.TrimSpace(s)

	if s == "" {
		return ""
	}

	// Check if the remaining string is a pure integer in BPM range.
	var num int
	if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
		return ""
	}

	// Plausible BPM range: 20–300.
	if num < 20 || num > 300 {
		return ""
	}

	return fmt.Sprintf("%d", num)
}

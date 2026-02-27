package metadata

import (
	"fmt"

	"github.com/bogem/id3v2/v2"
)

// WriteTags opens the MP3 file at path and writes the genre tags using ID3v2.
// primary must be non-empty. secondary may be empty (representing [NONE]).
//
// Genre encoding strategy (Q2 human decision: Options C + D):
//   - All existing TCON (Content Type / Genre) frames are replaced with two new
//     ones: the first contains the primary genre, the second the secondary genre
//     (if non-empty). ID3v2.4 allows multiple frames with the same ID; standard
//     players read the first, giving the primary genre prominence.
//   - Additionally, when a secondary genre is provided, a custom TXXX frame with
//     description "TGENRE2" is written containing the secondary genre value. This
//     preserves the secondary genre for tools that read TXXX user-defined frames
//     while keeping standard player compatibility for the primary genre.
//   - If secondary is empty, only a single TCON frame is written and no TXXX
//     frame is created.
//
// The file is opened, modified in-place, and saved. Returns a wrapped error
// on any failure.
func WriteTags(path string, primary string, secondary string) error {
	if primary == "" {
		return fmt.Errorf("metadata: WriteTags called with empty primary genre")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for tag writing: %w", path, err)
	}
	defer tag.Close()

	// Remove any existing TCON frames before writing new ones to avoid
	// accumulating stale genre values across multiple tag operations.
	tag.DeleteFrames(tag.CommonID("Genre"))

	// Write the primary genre as the first TCON frame.
	tag.AddTextFrame(tag.CommonID("Genre"), tag.DefaultEncoding(), primary)

	if secondary != "" {
		// Write the secondary genre as a second TCON frame (ID3v2.4 allows
		// multiple frames with the same ID).
		tag.AddTextFrame(tag.CommonID("Genre"), tag.DefaultEncoding(), secondary)

		// Also write secondary genre to a custom TXXX frame (description = "TGENRE2")
		// for tools that read user-defined text frames.
		tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
			Encoding:    tag.DefaultEncoding(),
			Description: "TGENRE2",
			Value:       secondary,
		})
	}

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving tags to %q: %w", path, err)
	}

	return nil
}

// WriteBPM opens the MP3 file at path and writes the BPM value to the TBPM
// (Beats Per Minute) ID3v2 text frame. bpm must be a non-empty string
// containing a numeric value (e.g. "128" or "120").
//
// Any existing TBPM frame is replaced. Returns a wrapped error on failure.
func WriteBPM(path string, bpm string) error {
	if bpm == "" {
		return fmt.Errorf("metadata: WriteBPM called with empty bpm")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for BPM write: %w", path, err)
	}
	defer tag.Close()

	// Replace any existing TBPM frame.
	tag.DeleteFrames(tag.CommonID("BPM"))
	tag.AddTextFrame(tag.CommonID("BPM"), tag.DefaultEncoding(), bpm)

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving BPM to %q: %w", path, err)
	}

	return nil
}

// WriteYear opens the MP3 file at path and writes the release year to the
// appropriate ID3v2 frame. For ID3v2.3 tags this is TYER; for ID3v2.4 it is
// TDRC. bogem/id3v2 maps the CommonID "Year" to the correct frame for the
// tag version, so we use tag.SetYear() which handles both versions.
//
// year must be a 4-digit string (e.g. "1971"). Returns a wrapped error on
// failure.
func WriteYear(path string, year string) error {
	if year == "" {
		return fmt.Errorf("metadata: WriteYear called with empty year")
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("metadata: opening %q for year write: %w", path, err)
	}
	defer tag.Close()

	// SetYear uses CommonID("Year") which maps to TYER (v2.3) or TDRC (v2.4)
	// depending on the tag version already present in the file.
	tag.SetYear(year)

	if err := tag.Save(); err != nil {
		return fmt.Errorf("metadata: saving year to %q: %w", path, err)
	}

	return nil
}

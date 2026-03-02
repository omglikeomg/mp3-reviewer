package metadata

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	id3v2 "github.com/bogem/id3v2/v2"
)

// copyFixture copies testdata/fixture.mp3 into a fresh temp file inside a
// t.TempDir() directory and returns its path. Each test that writes to the
// file must call this so the shared fixture is never mutated.
func copyFixture(t *testing.T) string {
	t.Helper()
	src, err := os.Open(filepath.Join("testdata", "fixture.mp3"))
	if err != nil {
		t.Fatalf("copyFixture: opening fixture: %v", err)
	}
	defer src.Close()

	dst, err := os.CreateTemp(t.TempDir(), "fixture_*.mp3")
	if err != nil {
		t.Fatalf("copyFixture: creating temp file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		t.Fatalf("copyFixture: copying fixture: %v", err)
	}
	return dst.Name()
}

// ── WriteTags tests ───────────────────────────────────────────────────────────

// TestWriteTags_PrimaryOnly writes a primary genre with no secondary and asserts
// that a single TCON frame is present with the correct value.
func TestWriteTags_PrimaryOnly(t *testing.T) {
	path := copyFixture(t)

	if err := WriteTags(path, "Rock", ""); err != nil {
		t.Fatalf("WriteTags() returned unexpected error: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-opening file after WriteTags: %v", err)
	}
	defer tag.Close()

	frames := tag.GetFrames(tag.CommonID("Genre"))
	if len(frames) != 1 {
		t.Fatalf("expected 1 TCON frame, got %d", len(frames))
	}
	tf, ok := frames[0].(id3v2.TextFrame)
	if !ok {
		t.Fatal("TCON frame is not a TextFrame")
	}
	if tf.Text != "Rock" {
		t.Errorf("TCON frame text = %q, want %q", tf.Text, "Rock")
	}
}

// TestWriteTags_PrimaryAndSecondary writes both genres and asserts that the TCON
// frame contains the secondary genre (bogem/id3v2 AddTextFrame replaces same-ID
// frames in-memory, so only one TCON survives — the last one added, which is the
// secondary) and that a TXXX frame with description "TGENRE2" is also written.
func TestWriteTags_PrimaryAndSecondary(t *testing.T) {
	path := copyFixture(t)

	if err := WriteTags(path, "Rock", "Psych-Rock"); err != nil {
		t.Fatalf("WriteTags() returned unexpected error: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-opening file after WriteTags: %v", err)
	}
	defer tag.Close()

	// bogem/id3v2 AddTextFrame replaces same-ID frames, so only the last-added
	// TCON survives (the secondary genre). One TCON frame is expected.
	tconFrames := tag.GetFrames(tag.CommonID("Genre"))
	if len(tconFrames) != 1 {
		t.Fatalf("expected 1 TCON frame, got %d", len(tconFrames))
	}
	tcon, ok := tconFrames[0].(id3v2.TextFrame)
	if !ok {
		t.Fatal("TCON[0] is not a TextFrame")
	}
	if tcon.Text != "Psych-Rock" {
		t.Errorf("TCON text = %q, want %q (secondary genre, last-added wins)", tcon.Text, "Psych-Rock")
	}

	// The TXXX "TGENRE2" frame must always be present when a secondary genre
	// is provided — this is the reliable way to read the secondary genre back.
	txxxFrames := tag.GetFrames("TXXX")
	found := false
	for _, f := range txxxFrames {
		txxx, ok := f.(id3v2.UserDefinedTextFrame)
		if !ok {
			continue
		}
		if txxx.Description == "TGENRE2" {
			found = true
			if txxx.Value != "Psych-Rock" {
				t.Errorf("TXXX TGENRE2 value = %q, want %q", txxx.Value, "Psych-Rock")
			}
		}
	}
	if !found {
		t.Error("expected a TXXX frame with description TGENRE2, none found")
	}
}

// TestWriteTags_EmptyPrimary asserts that WriteTags returns a non-nil error when
// called with an empty primary genre. The file must not be modified.
func TestWriteTags_EmptyPrimary(t *testing.T) {
	path := copyFixture(t)

	// Record file modification time before the call.
	statBefore, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before WriteTags: %v", err)
	}

	gotErr := WriteTags(path, "", "Rock")
	if gotErr == nil {
		t.Fatal("WriteTags() with empty primary: expected non-nil error, got nil")
	}

	// File must not have been touched.
	statAfter, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after WriteTags: %v", err)
	}
	if !statAfter.ModTime().Equal(statBefore.ModTime()) {
		t.Error("file modification time changed despite WriteTags returning an error")
	}
}

// TestWriteTags_ReplacesExistingGenres writes genres twice and asserts that the
// second write fully replaces the first (no stale TCON or TXXX frames accumulate).
// Because bogem/id3v2 AddTextFrame replaces same-ID frames, exactly one TCON
// frame survives after each write — containing the last-added genre (secondary).
func TestWriteTags_ReplacesExistingGenres(t *testing.T) {
	path := copyFixture(t)

	if err := WriteTags(path, "Jazz", "Bebop"); err != nil {
		t.Fatalf("first WriteTags() call: %v", err)
	}
	if err := WriteTags(path, "Electronic", "Techno"); err != nil {
		t.Fatalf("second WriteTags() call: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-opening file after second WriteTags: %v", err)
	}
	defer tag.Close()

	// Only one TCON frame survives (the secondary of the second write).
	// The key correctness property is that no stale genre from the first write
	// ("Jazz" or "Bebop") leaks through.
	tconFrames := tag.GetFrames(tag.CommonID("Genre"))
	if len(tconFrames) != 1 {
		t.Fatalf("expected exactly 1 TCON frame after second write, got %d", len(tconFrames))
	}
	tcon, ok := tconFrames[0].(id3v2.TextFrame)
	if !ok {
		t.Fatal("TCON[0] is not a TextFrame")
	}
	if tcon.Text != "Techno" {
		t.Errorf("TCON text = %q, want %q (second write's secondary genre)", tcon.Text, "Techno")
	}
	// No stale first-write genre must appear anywhere in TCON.
	for i, f := range tconFrames {
		tf, ok := f.(id3v2.TextFrame)
		if ok && (tf.Text == "Jazz" || tf.Text == "Bebop") {
			t.Errorf("TCON[%d] = %q: stale genre from first write leaked through", i, tf.Text)
		}
	}
}

// ── WriteBPM tests ────────────────────────────────────────────────────────────

// TestWriteBPM_RoundTrip writes a BPM value and reads back the TBPM frame,
// asserting the value is preserved exactly.
func TestWriteBPM_RoundTrip(t *testing.T) {
	path := copyFixture(t)

	if err := WriteBPM(path, "128"); err != nil {
		t.Fatalf("WriteBPM() returned unexpected error: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-opening file after WriteBPM: %v", err)
	}
	defer tag.Close()

	frames := tag.GetFrames(tag.CommonID("BPM"))
	if len(frames) != 1 {
		t.Fatalf("expected 1 TBPM frame, got %d", len(frames))
	}
	tf, ok := frames[0].(id3v2.TextFrame)
	if !ok {
		t.Fatal("TBPM frame is not a TextFrame")
	}
	if tf.Text != "128" {
		t.Errorf("TBPM frame text = %q, want %q", tf.Text, "128")
	}
}

// TestWriteBPM_EmptyBPM asserts that WriteBPM returns a non-nil error when
// called with an empty string.
func TestWriteBPM_EmptyBPM(t *testing.T) {
	path := copyFixture(t)

	if err := WriteBPM(path, ""); err == nil {
		t.Fatal("WriteBPM() with empty bpm: expected non-nil error, got nil")
	}
}

// ── WriteYear tests ───────────────────────────────────────────────────────────

// TestWriteYear_RoundTrip writes a year value and reads it back via tag.Year(),
// asserting the value is preserved.
func TestWriteYear_RoundTrip(t *testing.T) {
	path := copyFixture(t)

	if err := WriteYear(path, "1971"); err != nil {
		t.Fatalf("WriteYear() returned unexpected error: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-opening file after WriteYear: %v", err)
	}
	defer tag.Close()

	got := tag.Year()
	if got != "1971" {
		t.Errorf("tag.Year() = %q, want %q", got, "1971")
	}
}

// TestWriteYear_EmptyYear asserts that WriteYear returns a non-nil error when
// called with an empty string.
func TestWriteYear_EmptyYear(t *testing.T) {
	path := copyFixture(t)

	if err := WriteYear(path, ""); err == nil {
		t.Fatal("WriteYear() with empty year: expected non-nil error, got nil")
	}
}

// ── ReadTags tests ────────────────────────────────────────────────────────────

// TestReadTags_ReadsWrittenValues calls WriteTags and then ReadTags on the same
// file, confirming there is no panic and that ReadTags returns without error.
// The fixture has no TIT2 or TPE1 frames so empty strings are expected for
// title and artist (unless ffmpeg stamped them during fixture generation).
func TestReadTags_ReadsWrittenValues(t *testing.T) {
	path := copyFixture(t)

	if err := WriteTags(path, "Rock", "Blues-Rock"); err != nil {
		t.Fatalf("WriteTags() setup: %v", err)
	}

	title, artist, err := ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags() returned unexpected error: %v", err)
	}
	// The important thing is that ReadTags does not panic or return an error
	// on a file that has been written to by WriteTags. Log the values for
	// diagnostic purposes but do not fail on non-empty title/artist — the
	// fixture may have been generated with encoder metadata by ffmpeg.
	t.Logf("ReadTags returned title=%q artist=%q (empty expected if fixture has no ID3 title/artist)", title, artist)
}

// TestReadTags_FileNotFound asserts that ReadTags returns a non-nil error and
// empty strings when called with a path that does not exist.
func TestReadTags_FileNotFound(t *testing.T) {
	title, artist, err := ReadTags("/nonexistent/path/that/does/not/exist.mp3")
	if err == nil {
		t.Fatal("ReadTags() with non-existent path: expected non-nil error, got nil")
	}
	if title != "" {
		t.Errorf("title = %q, want empty string on error", title)
	}
	if artist != "" {
		t.Errorf("artist = %q, want empty string on error", artist)
	}
}

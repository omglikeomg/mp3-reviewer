package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"song-reviewer/internal/api"
	"song-reviewer/internal/audio"
	"song-reviewer/internal/domain"
	"song-reviewer/internal/metadata"
	"song-reviewer/internal/provider"
)

// AppState represents which screen the TUI is currently showing.
type AppState int

const (
	// StateReviewing is the default state: the main playback + header + status bar view.
	StateReviewing AppState = iota

	// StateGenreSelection is shown when the user presses Enter or Space
	// to assign a genre to the current song.
	StateGenreSelection
)

// genreStep tracks which selection step the genre modal is on.
const (
	stepPrimary   = 0
	stepSecondary = 1
)

// minTapCount is the minimum number of taps required before a BPM value is
// calculated from the average inter-tap interval. 8 taps = 7 intervals,
// providing near-professional accuracy (Q3 human decision).
const minTapCount = 8

// staleTapWindow is the maximum gap between consecutive taps before the
// sequence is considered stale and reset (Q4 human decision: 3 seconds).
const staleTapWindow = 3 * time.Second

// maxTapDeviation is the maximum relative deviation allowed between any single
// inter-tap interval and the running average. If a tap deviates more than this
// fraction (e.g. 0.40 = 40%), the sequence is considered too irregular and is
// reset. This prevents wildly inconsistent tapping from producing a meaningless
// BPM value (Q3 human decision: reset on irregular tapping).
const maxTapDeviation = 0.40

// enrichStatus represents the state of an enrichment field (Year or BPM).
type enrichStatus int

const (
	// enrichIdle means no fetch has been attempted yet for this field.
	enrichIdle enrichStatus = iota

	// enrichLoading means a background fetch or tap-tempo sequence is in progress.
	enrichLoading

	// enrichFound means a value was successfully fetched/calculated and is ready to commit.
	enrichFound

	// enrichCommitted means the value has been written to the ID3 tag.
	enrichCommitted

	// enrichError means the fetch failed or write failed.
	enrichError
)

// TickMsg is sent by tickCmd every 100ms to trigger a progress bar refresh.
type TickMsg time.Time

// PlayErrMsg is returned by playCmd after attempting to play a file.
// If Err is nil, the play succeeded. If Err is non-nil, the TUI should
// display the error message instead of crashing.
type PlayErrMsg struct {
	Err error
}

// TagWrittenMsg is returned by writeTagsCmd after a tag write attempt.
// If Err is nil, the write succeeded. Primary and Secondary carry the values
// that were written so the model can update its queue.
type TagWrittenMsg struct {
	Primary   string
	Secondary string
	Err       error
}

// SaveStateErrMsg is returned by saveStateCmd if the JSON file could not be
// updated. A non-nil Err is shown in the status bar; it does NOT block queue
// advancement (the tag write already succeeded).
type SaveStateErrMsg struct {
	Err error
}

// YearFetchedMsg is returned by fetchYearCmd after a MusicBrainz lookup.
// Year will be a 4-digit string on success, or empty string if not found.
// Err is non-nil on network or parse failure.
type YearFetchedMsg struct {
	Year string
	Err  error
}

// BPMFetchedMsg is returned by fetchBPMCmd after a MusicBrainz lookup for BPM.
// BPM will be a numeric string on success, or empty if not found.
// Err is non-nil on network or parse failure.
type BPMFetchedMsg struct {
	BPM string
	Err error
}

// BPMWrittenMsg is returned by writeBPMCmd after writing the TBPM ID3 frame.
// BPM carries the value that was written so the model can update the task.
// Err is non-nil on write failure.
type BPMWrittenMsg struct {
	BPM string
	Err error
}

// YearWrittenMsg is returned by writeYearCmd after writing the year ID3 frame.
// Year carries the value that was written so the model can update the task.
// Err is non-nil on write failure.
type YearWrittenMsg struct {
	Year string
	Err  error
}

// genreItem is a single entry in the bubbles/list for genre selection.
// It implements the list.Item interface.
type genreItem struct {
	title string
}

func (g genreItem) Title() string       { return g.title }
func (g genreItem) Description() string { return "" }
func (g genreItem) FilterValue() string { return g.title }

// Model is the root Bubble Tea model for the Song Reviewer TUI.
// It owns all mutable state; Update returns a new copy on each message.
type Model struct {
	// queue holds all tasks and the current position / undo history.
	queue domain.ReviewQueue

	// engine is the audio playback engine. It is shared by reference because
	// it owns an OS audio device handle that must not be duplicated.
	engine *audio.Engine

	// providerRef is a reference to the ManualReviewProvider used at startup.
	// It is stored here so saveStateCmd can call SaveState without capturing
	// a pointer to a changing local variable.
	providerRef provider.ManualReviewProvider

	// progress is the bubbles progress bar component.
	progress progress.Model

	// playbackState is a cached snapshot of the audio engine state, updated
	// on every TickMsg. View() reads from this field instead of calling
	// engine.GetState() directly, keeping View() a pure function.
	playbackState audio.PlaybackState

	// seekDelta is the seek step used for the ← / → keys.
	seekDelta time.Duration

	// state tracks which screen is currently active.
	state AppState

	// genreList is the bubbles/list component used for genre selection.
	genreList list.Model

	// genreStep indicates which step of genre selection the user is on.
	genreStep int

	// selectedPrimary holds the primary genre chosen in step 1.
	selectedPrimary string

	// cfg holds the application configuration.
	cfg domain.AppConfig

	// lastPlayErr holds the most recent audio error, shown in the status bar.
	lastPlayErr error

	// lastSaveErr holds the most recent JSON save error, shown in the status bar.
	lastSaveErr error

	// ── Enrichment fields ─────────────────────────────────────────────────────

	// enrichYearStatus tracks the fetch/commit state for the Year field.
	enrichYearStatus enrichStatus

	// enrichYearValue holds the year string fetched from MusicBrainz.
	// Empty until a successful fetch.
	enrichYearValue string

	// enrichBPMStatus tracks the BPM fetch/tap-tempo/commit state.
	enrichBPMStatus enrichStatus

	// enrichBPMValue holds the BPM string — either fetched from MusicBrainz
	// or derived from tap tempo. Empty until a value is available.
	enrichBPMValue string

	// tapTimes holds the timestamps of each tap-tempo keypress.
	// At least minTapCount taps are required to calculate a stable BPM average.
	tapTimes []time.Time

	// spinner is the bubbles spinner component shown while data is loading.
	spinner spinner.Model

	// ── Layout fields ─────────────────────────────────────────────────────────

	// width and height are the current terminal dimensions.
	width  int
	height int

	// pendingInit holds the initial command batch returned by Init().
	pendingInit tea.Cmd
}

// New constructs the initial Model from the given queue, engine, config, and provider.
// It stores the startup command batch (auto-play + first tick + spinner tick +
// initial year/BPM fetch) in pendingInit so that Init() can return them inside the
// Bubble Tea event loop.
func New(queue domain.ReviewQueue, engine *audio.Engine, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	seekSecs := cfg.SeekDeltaSeconds
	if seekSecs <= 0 {
		seekSecs = 30
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	m := Model{
		queue:         queue,
		engine:        engine,
		providerRef:   p,
		progress:      prog,
		seekDelta:     time.Duration(seekSecs) * time.Second,
		state:         StateReviewing,
		cfg:           cfg,
		playbackState: engine.GetState(),
		spinner:       spin,
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	cmds = append(cmds, m.spinner.Tick)

	if len(queue.Tasks) > 0 {
		task := queue.Tasks[0]
		cmds = append(cmds, playCmd(engine, task.FilePath))
		// Fire enrichment fetches immediately for the first song.
		if task.Artist != "" || task.Title != "" {
			m.enrichYearStatus = enrichLoading
			m.enrichBPMStatus = enrichLoading
			cmds = append(cmds, fetchYearCmd(task.Artist, task.Title, cfg.ApiKeys.MusicBrainzUserAgent))
			cmds = append(cmds, fetchBPMCmd(task.Artist, task.Title, cfg.ApiKeys.MusicBrainzUserAgent))
		}
	}

	m.pendingInit = tea.Batch(cmds...)
	return m
}

// resetEnrichment clears all enrichment state for the current song.
// It is called whenever the queue advances to a new song.
func (m Model) resetEnrichment() Model {
	m.enrichYearStatus = enrichIdle
	m.enrichYearValue = ""
	m.enrichBPMStatus = enrichIdle
	m.enrichBPMValue = ""
	m.tapTimes = nil
	return m
}

// openGenreModal transitions the model into StateGenreSelection at stepPrimary.
func (m Model) openGenreModal() Model {
	m.state = StateGenreSelection
	m.genreStep = stepPrimary
	m.selectedPrimary = ""
	m.genreList = makeGenreList(m.cfg.GenreList, false, m.width)
	return m
}

// makeGenreList constructs a bubbles/list model populated with genre items.
// If includeNone is true, a "[NONE]" item is prepended to the list.
// Height is fixed at 14 rows (Option A from Task 4 plan).
func makeGenreList(genres []string, includeNone bool, width int) list.Model {
	items := make([]list.Item, 0, len(genres)+1)
	if includeNone {
		items = append(items, genreItem{title: "[NONE]"})
	}
	for _, g := range genres {
		items = append(items, genreItem{title: g})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	listWidth := width - 4
	if listWidth < 20 {
		listWidth = 40
	}

	l := list.New(items, delegate, listWidth, 14)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return l
}

// tickCmd returns a Bubble Tea command that sends a TickMsg after 100ms.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// playCmd returns a Bubble Tea command that calls engine.Play(path).
func playCmd(engine *audio.Engine, path string) tea.Cmd {
	return func() tea.Msg {
		err := engine.Play(path)
		return PlayErrMsg{Err: err}
	}
}

// writeTagsCmd returns a Bubble Tea command that calls metadata.WriteTags.
func writeTagsCmd(path, primary, secondary string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteTags(path, primary, secondary)
		return TagWrittenMsg{Primary: primary, Secondary: secondary, Err: err}
	}
}

// saveStateCmd returns a Bubble Tea command that calls provider.SaveState.
func saveStateCmd(p provider.ManualReviewProvider, tasks []domain.Task) tea.Cmd {
	return func() tea.Msg {
		err := p.SaveState(tasks)
		return SaveStateErrMsg{Err: err}
	}
}

// fetchYearCmd returns a Bubble Tea command that calls api.FetchYear on a
// background goroutine and returns the result as a YearFetchedMsg.
func fetchYearCmd(artist, title, userAgent string) tea.Cmd {
	return func() tea.Msg {
		year, err := api.FetchYear(artist, title, userAgent)
		return YearFetchedMsg{Year: year, Err: err}
	}
}

// fetchBPMCmd returns a Bubble Tea command that calls api.FetchBPM on a
// background goroutine and returns the result as a BPMFetchedMsg.
// Per Q1 human decision: try MusicBrainz for BPM first, fall back to Tap Tempo.
func fetchBPMCmd(artist, title, userAgent string) tea.Cmd {
	return func() tea.Msg {
		bpm, err := api.FetchBPM(artist, title, userAgent)
		return BPMFetchedMsg{BPM: bpm, Err: err}
	}
}

// writeBPMCmd returns a Bubble Tea command that calls metadata.WriteBPM on a
// background goroutine and returns the result as a BPMWrittenMsg.
func writeBPMCmd(path, bpm string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteBPM(path, bpm)
		return BPMWrittenMsg{BPM: bpm, Err: err}
	}
}

// writeYearCmd returns a Bubble Tea command that calls metadata.WriteYear on a
// background goroutine and returns the result as a YearWrittenMsg.
func writeYearCmd(path, year string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteYear(path, year)
		return YearWrittenMsg{Year: year, Err: err}
	}
}

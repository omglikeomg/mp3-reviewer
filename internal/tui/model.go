package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

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

	// progress is the bubbles progress bar component. It is updated in Update
	// using the fraction from playbackState.Progress.
	progress progress.Model

	// playbackState is a cached snapshot of the audio engine state, updated
	// on every TickMsg. View() reads from this field instead of calling
	// engine.GetState() directly, keeping View() a pure function.
	playbackState audio.PlaybackState

	// seekDelta is the seek step used for the ← / → keys. It is loaded from
	// AppConfig.SeekDeltaSeconds (defaulting to 30s when the config value is 0).
	seekDelta time.Duration

	// state tracks which screen is currently active.
	state AppState

	// genreList is the bubbles/list component used for genre selection.
	// It is rebuilt fresh each time the genre modal opens.
	genreList list.Model

	// genreStep indicates which step of genre selection the user is on:
	// stepPrimary (0) = choosing Primary Genre.
	// stepSecondary (1) = choosing Secondary Genre.
	genreStep int

	// selectedPrimary holds the primary genre chosen in step 1.
	// It is used when writing tags after step 2 completes.
	selectedPrimary string

	// cfg holds the application configuration, needed to rebuild genre lists.
	cfg domain.AppConfig

	// lastPlayErr holds the most recent audio error, shown in the status bar.
	// It is reset to nil when a new track starts successfully.
	lastPlayErr error

	// lastSaveErr holds the most recent JSON save error, shown in the status bar.
	// It does not block queue advancement.
	lastSaveErr error

	// width and height are the current terminal dimensions, updated on
	// tea.WindowSizeMsg so that the layout can fill the terminal correctly.
	width  int
	height int

	// pendingInit holds the initial command batch that must be returned by
	// Init(). Using Init() to dispatch startup commands is the canonical
	// Bubble Tea pattern — it ensures commands run inside the event loop.
	// New() stores the batch here; Init() returns and clears it.
	pendingInit tea.Cmd
}

// New constructs the initial Model from the given queue, engine, config, and provider.
// It stores the startup command batch (auto-play + first tick) in pendingInit
// so that Init() can return them inside the Bubble Tea event loop.
//
// The caller (main.go) is responsible for constructing the ReviewQueue, the
// *audio.Engine, and the ManualReviewProvider and passing them in. New does
// not perform any I/O itself.
func New(queue domain.ReviewQueue, engine *audio.Engine, cfg domain.AppConfig, p provider.ManualReviewProvider) Model {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	// Resolve seek delta: use config value, falling back to 30s if unset.
	seekSecs := cfg.SeekDeltaSeconds
	if seekSecs <= 0 {
		seekSecs = 30
	}

	m := Model{
		queue:       queue,
		engine:      engine,
		providerRef: p,
		progress:    prog,
		seekDelta:   time.Duration(seekSecs) * time.Second,
		state:       StateReviewing,
		cfg:         cfg,
		// Populate initial playback state so the first render is not empty.
		playbackState: engine.GetState(),
	}

	var cmds []tea.Cmd

	// Start the progress bar ticker immediately.
	cmds = append(cmds, tickCmd())

	// Auto-play the first song if the queue is non-empty.
	if len(queue.Tasks) > 0 {
		cmds = append(cmds, playCmd(engine, queue.Tasks[0].FilePath))
	}

	m.pendingInit = tea.Batch(cmds...)
	return m
}

// openGenreModal transitions the model into StateGenreSelection at stepPrimary,
// building a fresh genre list from the config. The list does NOT include [NONE]
// at the primary step.
func (m Model) openGenreModal() Model {
	m.state = StateGenreSelection
	m.genreStep = stepPrimary
	m.selectedPrimary = ""
	m.genreList = makeGenreList(m.cfg.GenreList, false, m.width)
	return m
}

// makeGenreList constructs a bubbles/list model populated with genre items.
// If includeNone is true, a "[NONE]" item is prepended to the list.
// width is used to size the list component. Height is fixed at 14 rows (Q4: Option A).
func makeGenreList(genres []string, includeNone bool, width int) list.Model {
	items := make([]list.Item, 0, len(genres)+1)
	if includeNone {
		items = append(items, genreItem{title: "[NONE]"})
	}
	for _, g := range genres {
		items = append(items, genreItem{title: g})
	}

	// Use a compact delegate (no descriptions) for a dense list.
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	listWidth := width - 4
	if listWidth < 20 {
		listWidth = 40 // sane minimum for narrow terminals
	}

	l := list.New(items, delegate, listWidth, 14)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return l
}

// tickCmd returns a Bubble Tea command that sends a TickMsg after 100ms.
// It must be re-issued after every TickMsg to maintain a continuous ticker.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// playCmd returns a Bubble Tea command that calls engine.Play(path).
// Bubble Tea Cmd functions run off the main loop, so this does not block the UI.
// It wraps the result in a PlayErrMsg so Update can handle errors without panicking.
func playCmd(engine *audio.Engine, path string) tea.Cmd {
	return func() tea.Msg {
		err := engine.Play(path)
		return PlayErrMsg{Err: err}
	}
}

// writeTagsCmd returns a Bubble Tea command that calls metadata.WriteTags on a
// background goroutine. The result is returned as a TagWrittenMsg.
func writeTagsCmd(path, primary, secondary string) tea.Cmd {
	return func() tea.Msg {
		err := metadata.WriteTags(path, primary, secondary)
		return TagWrittenMsg{Primary: primary, Secondary: secondary, Err: err}
	}
}

// saveStateCmd returns a Bubble Tea command that calls provider.SaveState to
// persist the current queue state to disk. Errors are returned as SaveStateErrMsg
// and shown in the status bar but do not block the UI.
func saveStateCmd(p provider.ManualReviewProvider, tasks []domain.Task) tea.Cmd {
	return func() tea.Msg {
		err := p.SaveState(tasks)
		return SaveStateErrMsg{Err: err}
	}
}

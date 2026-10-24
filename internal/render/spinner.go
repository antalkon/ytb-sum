package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Spinner shows the current pipeline stage on stderr. On a non-terminal it
// prints each stage once (only when verbose); it is safe for concurrent use.
type Spinner struct {
	w       io.Writer
	tty     bool
	verbose bool
	mu      sync.Mutex
	stage   string
	started time.Time // start of the current stage
	stop    chan struct{}
	done    chan struct{}
}

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a spinner writing to stderr.
func NewSpinner(verbose bool) *Spinner {
	return &Spinner{
		w:       os.Stderr,
		tty:     term.IsTerminal(int(os.Stderr.Fd())),
		verbose: verbose,
		started: time.Now(),
	}
}

// Set changes the stage text, starting the animation on first use.
func (s *Spinner) Set(stage string) {
	s.mu.Lock()
	prev := s.stage
	s.stage = stage
	// Progress updates within one stage ("Transcribing (2/3)") don't log.
	if stageKey(prev) != stageKey(stage) {
		if prev != "" {
			s.logLocked(stageKey(prev))
		}
		s.started = time.Now()
	}
	if !s.tty {
		if s.verbose {
			fmt.Fprintf(s.w, "… %s\n", stage)
		}
		s.mu.Unlock()
		return
	}
	if s.stop == nil {
		s.stop, s.done = make(chan struct{}), make(chan struct{})
		go s.loop()
	}
	s.mu.Unlock()
}

func (s *Spinner) logLocked(stage string) {
	if !s.tty {
		return
	}
	fmt.Fprintf(s.w, "\r\033[K  %s %s %s\n",
		lipgloss.NewStyle().Foreground(good).Render("✔"), stage,
		lipgloss.NewStyle().Foreground(muted).Render(time.Since(s.started).Round(100*time.Millisecond).String()))
}

func (s *Spinner) loop() {
	defer close(s.done)
	t := time.NewTicker(90 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		s.mu.Lock()
		fmt.Fprintf(s.w, "\r\033[K  %s %s %s",
			lipgloss.NewStyle().Foreground(accent).Render(frames[i%len(frames)]), s.stage,
			lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("%.0fs", time.Since(s.started).Seconds())))
		s.mu.Unlock()
		select {
		case <-s.stop:
			return
		case <-t.C:
		}
	}
}

// Finish marks the current stage as done and stops the spinner.
func (s *Spinner) Finish() { s.stopSpinner(true) }

// Stop clears the spinner line without marking the stage as done (used on
// errors). It is safe to call more than once.
func (s *Spinner) Stop() { s.stopSpinner(false) }

func (s *Spinner) stopSpinner(ok bool) {
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop = nil
	if ok && s.stage != "" {
		s.logLocked(stageKey(s.stage))
	}
	s.stage = ""
	s.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
		fmt.Fprint(s.w, "\r\033[K")
	}
}

func stageKey(stage string) string {
	if i := strings.Index(stage, " ("); i > 0 {
		return stage[:i]
	}
	return stage
}

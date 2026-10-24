// Package render prints summaries to the terminal with colors and layout.
package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/antalkon/ytb-sum/internal/llm"
	"github.com/antalkon/ytb-sum/internal/mdfile"
	"github.com/antalkon/ytb-sum/internal/summarize"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#C4302B", Dark: "#FF5F57"}
	muted  = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#9A9A9A"}
	good   = lipgloss.AdaptiveColor{Light: "#1E8E3E", Dark: "#5AF78E"}
)

// Stats describes the run for the footer line.
type Stats struct {
	Model   string
	Caption string
	Usage   llm.Usage
	Elapsed time.Duration
	File    string
	HTML    string
	Chunks  int
}

// Printer writes to w, using color only when Color is true.
type Printer struct {
	W     io.Writer
	Color bool
	Width int
}

// NewPrinter configures a Printer for stdout.
func NewPrinter(noColor bool) *Printer {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	width := 100
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 20 && w < width {
		width = w
	}
	if !isTTY || noColor {
		lipgloss.SetColorProfile(0) // Ascii: no escape codes
	}
	return &Printer{W: os.Stdout, Color: isTTY && !noColor, Width: width}
}

// Summary prints the header card, the rendered body and a footer.
func (p *Printer) Summary(v *youtube.Video, s *summarize.Summary, lang string, st Stats) error {
	l := mdfile.LabelsFor(lang)
	if !p.Color {
		_, err := fmt.Fprint(p.W, mdfile.Document(v, s, lang)+"\n"+p.footer(st)+"\n")
		return err
	}
	fmt.Fprintln(p.W, p.card(v, l))

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyle(lipgloss.HasDarkBackground())),
		glamour.WithWordWrap(p.Width-4),
	)
	if err != nil {
		return err
	}
	out, err := r.Render(mdfile.Body(v, s, l, false))
	if err != nil {
		return err
	}
	fmt.Fprint(p.W, out)
	fmt.Fprintln(p.W, p.footer(st))
	return nil
}

// markdownStyle tweaks glamour's built-in theme: no "##" heading prefixes,
// accent-colored section headings.
func markdownStyle(dark bool) ansi.StyleConfig {
	cfg, h2, h3 := styles.LightStyleConfig, "#C4302B", "#1F6FEB"
	if dark {
		cfg, h2, h3 = styles.DarkStyleConfig, "#FF5F57", "#79C0FF"
	}
	cfg.H2.Prefix = "▍"
	cfg.H2.Color = &h2
	cfg.H2.BackgroundColor = nil
	cfg.H3.Prefix = "› "
	cfg.H3.Color = &h3
	return cfg
}

func (p *Printer) card(v *youtube.Video, l mdfile.Labels) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("▶ " + v.Title)
	var meta []string
	meta = append(meta, v.Author())
	if v.Duration > 0 {
		meta = append(meta, youtube.FormatTimestamp(v.Duration))
	}
	if d := v.Date(); !d.IsZero() {
		meta = append(meta, d.Format("2006-01-02"))
	}
	if v.ViewCount > 0 {
		meta = append(meta, humanCount(v.ViewCount)+" views")
	}
	info := lipgloss.NewStyle().Foreground(muted).Render(strings.Join(meta, "  ·  "))
	link := lipgloss.NewStyle().Underline(true).Foreground(muted).Render(youtube.WatchURL(v.ID))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Margin(1, 0, 0, 2).
		Width(min(p.Width-6, 96))
	return box.Render(title + "\n" + info + "\n" + link)
}

func (p *Printer) footer(st Stats) string {
	var parts []string
	if st.Model != "" {
		parts = append(parts, st.Model)
	}
	if st.Caption != "" {
		parts = append(parts, "subs: "+st.Caption)
	}
	if st.Usage.PromptTokens > 0 {
		parts = append(parts, fmt.Sprintf("%s→%s tok", humanCount(int64(st.Usage.PromptTokens)), humanCount(int64(st.Usage.CompletionTokens))))
	}
	if st.Usage.Cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", st.Usage.Cost))
	}
	parts = append(parts, st.Elapsed.Round(100*time.Millisecond).String())
	line := strings.Join(parts, "  ·  ")
	if !p.Color {
		if st.File != "" {
			line += "\nSaved: " + st.File
		}
		if st.HTML != "" {
			line += "\nSaved: " + st.HTML
		}
		return line
	}
	out := "  " + lipgloss.NewStyle().Foreground(muted).Render(line)
	if st.File != "" {
		out += "\n  " + lipgloss.NewStyle().Foreground(good).Render("✔ Saved ") +
			lipgloss.NewStyle().Bold(true).Render(st.File)
	}
	if st.HTML != "" {
		out += "\n  " + lipgloss.NewStyle().Foreground(good).Render("✔ Saved ") +
			lipgloss.NewStyle().Bold(true).Render(st.HTML)
	}
	return out + "\n"
}

func humanCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%dk", n/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

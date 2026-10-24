// Package mdfile builds the Markdown document and writes it to disk.
package mdfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/antalkon/ytb-sum/internal/summarize"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

// Labels are the headings used in the document, in the summary language.
type Labels struct {
	Channel, Duration, Published, Link, Summary, KeyPoints, Sections, Takeaways, Mentioned string
}

// LabelsFor returns headings for lang (Russian or English; others use English).
func LabelsFor(lang string) Labels {
	if strings.EqualFold(lang, "ru") || strings.EqualFold(lang, "russian") {
		return Labels{"Канал", "Длительность", "Опубликовано", "Ссылка", "Кратко", "Ключевые тезисы", "Содержание", "Выводы", "Упоминается"}
	}
	return Labels{"Channel", "Duration", "Published", "Link", "TL;DR", "Key points", "Contents", "Takeaways", "Mentioned"}
}

// Header renders the title and metadata block.
func Header(v *youtube.Video, l Labels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", escapeInline(v.Title))
	fmt.Fprintf(&b, "- **%s:** %s\n", l.Channel, escapeInline(v.Author()))
	if v.Duration > 0 {
		fmt.Fprintf(&b, "- **%s:** %s\n", l.Duration, youtube.FormatTimestamp(v.Duration))
	}
	if d := v.Date(); !d.IsZero() {
		fmt.Fprintf(&b, "- **%s:** %s\n", l.Published, d.Format("2006-01-02"))
	}
	fmt.Fprintf(&b, "- **%s:** %s\n", l.Link, youtube.WatchURL(v.ID))
	return b.String()
}

// Body renders the summary content without the header. With links,
// section timestamps become clickable YouTube links; the terminal view
// passes false to keep headings short.
func Body(v *youtube.Video, s *summarize.Summary, l Labels, links bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n%s\n\n", l.Summary, strings.TrimSpace(s.TLDR))
	list(&b, l.KeyPoints, s.KeyPoints)
	if len(s.Sections) > 0 {
		fmt.Fprintf(&b, "## %s\n\n", l.Sections)
		for _, sec := range s.Sections {
			title := escapeInline(sec.Title)
			sec2, ok := sec.Seconds()
			switch {
			case ok && links:
				fmt.Fprintf(&b, "### [%s](%s) %s\n\n", sec.Timestamp, youtube.TimestampURL(v.ID, sec2), title)
			case ok:
				fmt.Fprintf(&b, "### %s · %s\n\n", sec.Timestamp, title)
			default:
				fmt.Fprintf(&b, "### %s\n\n", title)
			}
			for _, p := range sec.Points {
				fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(p))
			}
			b.WriteString("\n")
		}
	}
	list(&b, l.Takeaways, s.Takeaways)
	list(&b, l.Mentioned, s.Mentioned)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// Document is the full Markdown file content.
func Document(v *youtube.Video, s *summarize.Summary, lang string) string {
	l := LabelsFor(lang)
	return Header(v, l) + "\n" + Body(v, s, l, true)
}

func list(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", strings.TrimSpace(it))
	}
	b.WriteString("\n")
}

func escapeInline(s string) string {
	return strings.NewReplacer("\n", " ", "[", "\\[", "]", "\\]").Replace(strings.TrimSpace(s))
}

// MaxNameBytes keeps file names well below the 255-byte filesystem limit.
const MaxNameBytes = 180

// SafeFilename turns a video title into a portable file name stem.
func SafeFilename(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case strings.ContainsRune(`/\:*?"<>|`, r) || unicode.IsControl(r):
			b.WriteRune(' ')
		case r == utf8.RuneError:
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	name = strings.Trim(name, ". ")
	for len(name) > MaxNameBytes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	name = strings.TrimRight(name, ". ")
	if name == "" {
		name = "video"
	}
	return name
}

// Write saves content as "<title>.md" in dir without overwriting existing
// files ("<title> (2).md" and so on). It returns the path written.
func Write(dir, title, content string) (string, error) {
	return WriteExt(dir, title, ".md", content)
}

// WriteExt is Write with a custom extension such as ".html".
func WriteExt(dir, title, ext, content string) (string, error) {
	stem := SafeFilename(title)
	for i := 1; i < 1000; i++ {
		name := stem + ext
		if i > 1 {
			name = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			os.Remove(path)
			return "", err
		}
		return path, f.Close()
	}
	return "", fmt.Errorf("too many files named %q in %s", stem, dir)
}

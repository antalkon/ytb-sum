package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antalkon/ytb-sum/internal/proc"
)

// ErrNoSubtitles means the video has neither manual nor auto captions.
var ErrNoSubtitles = errors.New("video has no subtitles")

// Segment is a piece of transcript text starting at Start seconds.
type Segment struct {
	Start float64
	Text  string
}

// Caption identifies the chosen subtitle track.
type Caption struct {
	Lang   string
	Auto   bool
	Tracks []Track
}

// Source describes the caption for humans, e.g. "en (auto)".
func (c Caption) Source() string {
	if c.Auto {
		return c.Lang + " (auto)"
	}
	return c.Lang
}

// PickCaption chooses the best subtitle track. Order: manual in preferred
// language, manual in the video's language, original-language auto captions,
// auto in preferred language, any manual track. Auto-translated captions are
// avoided because the LLM translates better.
func PickCaption(v *Video, preferred string) (Caption, error) {
	orig := baseLang(v.Language)
	pref := baseLang(preferred)

	find := func(m map[string][]Track, lang string, auto bool) (Caption, bool) {
		if lang == "" {
			return Caption{}, false
		}
		if t := m[lang]; len(t) > 0 {
			return Caption{Lang: lang, Auto: auto, Tracks: t}, true
		}
		for _, k := range sortedKeys(m) {
			t := m[k]
			if baseLang(k) == lang && len(t) > 0 && !strings.Contains(k, "-orig") {
				return Caption{Lang: k, Auto: auto, Tracks: t}, true
			}
		}
		return Caption{}, false
	}

	if c, ok := find(v.Subtitles, pref, false); ok {
		return c, nil
	}
	if c, ok := find(v.Subtitles, orig, false); ok {
		return c, nil
	}
	// The *-orig auto track is the speech recognition in the spoken language.
	for _, k := range sortedKeys(v.AutoCaps) {
		t := v.AutoCaps[k]
		if strings.HasSuffix(k, "-orig") && len(t) > 0 {
			return Caption{Lang: strings.TrimSuffix(k, "-orig"), Auto: true, Tracks: t}, nil
		}
	}
	if c, ok := find(v.AutoCaps, orig, true); ok {
		return c, nil
	}
	if c, ok := find(v.AutoCaps, pref, true); ok {
		return c, nil
	}
	if c, ok := find(v.Subtitles, "en", false); ok {
		return c, nil
	}
	for _, k := range sortedKeys(v.Subtitles) {
		if t := v.Subtitles[k]; len(t) > 0 && k != "live_chat" {
			return Caption{Lang: k, Tracks: t}, nil
		}
	}
	return Caption{}, ErrNoSubtitles
}

// FetchTranscript downloads the caption into memory. If YouTube rejects the
// direct request it falls back to letting yt-dlp fetch it into tmpDir.
func FetchTranscript(ctx context.Context, hc *http.Client, v *Video, c Caption, tmpDir string, o Options) ([]Segment, error) {
	segs, err := fetchDirect(ctx, hc, c)
	if err == nil && len(segs) > 0 {
		return segs, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	segs, err2 := fetchViaYtdlp(ctx, v.ID, c, tmpDir, o)
	if err2 != nil {
		return nil, fmt.Errorf("download subtitles: %v; yt-dlp fallback: %w", err, err2)
	}
	return segs, nil
}

func fetchDirect(ctx context.Context, hc *http.Client, c Caption) ([]Segment, error) {
	u := ""
	for _, t := range c.Tracks {
		if t.Ext == "json3" {
			u = t.URL
			break
		}
	}
	if u == "" && len(c.Tracks) > 0 {
		pu, err := url.Parse(c.Tracks[0].URL)
		if err != nil {
			return nil, err
		}
		q := pu.Query()
		q.Set("fmt", "json3")
		pu.RawQuery = q.Encode()
		u = pu.String()
	}
	if u == "" {
		return nil, ErrNoSubtitles
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subtitles HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return ParseJSON3(data)
}

func fetchViaYtdlp(ctx context.Context, id string, c Caption, tmpDir string, o Options) ([]Segment, error) {
	dir, err := os.MkdirTemp(tmpDir, "subs-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	lang := c.Lang
	flag := "--write-subs"
	if c.Auto {
		flag = "--write-auto-subs"
		lang = c.Lang + "-orig," + c.Lang
	}
	args := append(o.BaseArgs(), "--skip-download", flag, "--sub-langs", lang,
		"--sub-format", "json3", "-P", dir, "-o", "subs.%(ext)s", "--", WatchURL(id))
	if _, err := proc.Run(ctx, Env(tmpDir), "yt-dlp", args...); err != nil {
		return nil, err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json3"))
	if len(files) == 0 {
		return nil, ErrNoSubtitles
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		return nil, err
	}
	return ParseJSON3(data)
}

type json3 struct {
	Events []struct {
		TStartMs int64 `json:"tStartMs"`
		Segs     []struct {
			UTF8 string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

// ParseJSON3 parses YouTube's json3 caption format.
func ParseJSON3(data []byte) ([]Segment, error) {
	var j json3
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("parse json3 subtitles: %w", err)
	}
	var out []Segment
	for _, e := range j.Events {
		var b strings.Builder
		for _, s := range e.Segs {
			b.WriteString(s.UTF8)
		}
		text := strings.Join(strings.Fields(b.String()), " ")
		if text == "" {
			continue
		}
		out = append(out, Segment{Start: float64(e.TStartMs) / 1000, Text: text})
	}
	return out, nil
}

// Group merges segments into blocks of roughly window seconds so the
// transcript keeps timestamps without one marker per caption line.
func Group(segs []Segment, window float64) []Segment {
	var out []Segment
	for _, s := range segs {
		if n := len(out); n > 0 && s.Start-out[n-1].Start < window {
			out[n-1].Text += " " + s.Text
			continue
		}
		out = append(out, s)
	}
	return out
}

// FormatTimestamp renders seconds as M:SS or H:MM:SS.
func FormatTimestamp(sec float64) string {
	t := int(sec)
	h, m, s := t/3600, t%3600/60, t%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// FormatTranscript renders segments as "[M:SS] text" lines for the LLM.
func FormatTranscript(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		fmt.Fprintf(&b, "[%s] %s\n", FormatTimestamp(s.Start), s.Text)
	}
	return b.String()
}

func baseLang(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	if i := strings.IndexAny(l, "-_"); i > 0 {
		l = l[:i]
	}
	return l
}

func sortedKeys(m map[string][]Track) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

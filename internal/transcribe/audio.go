// Package transcribe produces a transcript from a video's audio track using
// an audio-capable model. Audio lives only in a temp dir that is removed
// before returning.
package transcribe

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/antalkon/ytb-sum/internal/llm"
	"github.com/antalkon/ytb-sum/internal/proc"
	"github.com/antalkon/ytb-sum/internal/summarize"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

// ChunkSeconds is the length of each audio piece sent to the model.
const ChunkSeconds = 20 * 60

// Transcriber transcribes audio through an OpenAI-compatible API.
type Transcriber struct {
	Client   summarize.Chatter
	Model    string
	Progress func(stage string)
}

// Transcribe downloads the audio of id into a temp dir under tmpRoot,
// compresses and splits it, and transcribes every piece.
func (t *Transcriber) Transcribe(ctx context.Context, id, tmpRoot string, o youtube.Options) ([]youtube.Segment, llm.Usage, error) {
	var usage llm.Usage
	dir, err := os.MkdirTemp(tmpRoot, "audio-")
	if err != nil {
		return nil, usage, err
	}
	defer os.RemoveAll(dir)

	t.progress("Downloading audio")
	args := append(o.BaseArgs(), "-f", "bestaudio/best", "--no-part", "-P", dir, "-o", "src.%(ext)s", "--", youtube.WatchURL(id))
	if _, err := proc.Run(ctx, youtube.Env(tmpRoot), "yt-dlp", args...); err != nil {
		return nil, usage, fmt.Errorf("download audio: %w", err)
	}
	src, _ := filepath.Glob(filepath.Join(dir, "src.*"))
	if len(src) == 0 {
		return nil, usage, fmt.Errorf("download audio: no file produced")
	}

	t.progress("Compressing audio")
	// Mono 16 kHz 32 kbps is plenty for speech and keeps requests small.
	if _, err := proc.Run(ctx, nil, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", src[0], "-vn", "-ac", "1", "-ar", "16000", "-b:a", "32k",
		"-f", "segment", "-segment_time", fmt.Sprint(ChunkSeconds), "-reset_timestamps", "1",
		filepath.Join(dir, "part_%03d.mp3")); err != nil {
		return nil, usage, fmt.Errorf("compress audio: %w", err)
	}
	os.Remove(src[0])
	parts, _ := filepath.Glob(filepath.Join(dir, "part_*.mp3"))
	sort.Strings(parts)
	if len(parts) == 0 {
		return nil, usage, fmt.Errorf("compress audio: no output")
	}

	var all []youtube.Segment
	for i, p := range parts {
		t.progress(fmt.Sprintf("Transcribing audio (%d/%d)", i+1, len(parts)))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, usage, err
		}
		os.Remove(p)
		resp, err := t.Client.Chat(ctx, llm.Request{
			Model: t.Model,
			Messages: []llm.Message{{Role: "user", Content: []llm.Part{
				{Type: "text", Text: prompt},
				{Type: "input_audio", InputAudio: &llm.InputAudio{Data: base64.StdEncoding.EncodeToString(data), Format: "mp3"}},
			}}},
			MaxTokens: 32000,
		})
		if err != nil {
			return nil, usage, fmt.Errorf("transcribe part %d: %w", i+1, err)
		}
		usage.Add(resp.Usage)
		all = append(all, ParseTimestamped(resp.Content, float64(i*ChunkSeconds))...)
	}
	if len(all) == 0 {
		return nil, usage, fmt.Errorf("no speech found in the audio (music-only video?)")
	}
	return all, usage, nil
}

func (t *Transcriber) progress(s string) {
	if t.Progress != nil {
		t.Progress(s)
	}
}

const prompt = `Transcribe this audio verbatim in its original language. Do not translate or summarize.
Output plain text lines, one per sentence or short phrase, each starting with its start time as [MM:SS], for example:
[00:00] First words.
[00:07] Next phrase.`

var lineRe = regexp.MustCompile(`^\s*\[?(\d{1,2}(?::\d{2}){1,2})\]?\s*[-–:]?\s*(.*)$`)

// ParseTimestamped parses "[MM:SS] text" lines, shifting times by offset.
// Lines without a timestamp are appended to the previous segment.
func ParseTimestamped(text string, offset float64) []youtube.Segment {
	var out []youtube.Segment
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		if m := lineRe.FindStringSubmatch(line); m != nil {
			if sec, ok := summarize.ParseTimestamp(m[1]); ok {
				if txt := strings.TrimSpace(m[2]); txt != "" {
					out = append(out, youtube.Segment{Start: offset + float64(sec), Text: txt})
				}
				continue
			}
		}
		if n := len(out); n > 0 {
			out[n-1].Text += " " + line
		} else {
			out = append(out, youtube.Segment{Start: offset, Text: line})
		}
	}
	return out
}

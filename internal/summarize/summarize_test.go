package summarize

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/antalkon/ytb-sum/internal/llm"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

type fakeChat struct {
	mu       sync.Mutex
	replies  []string
	requests []llm.Request
	mapCalls int
}

func (f *fakeChat) Chat(_ context.Context, req llm.Request) (*llm.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if req.ResponseFormat == nil {
		f.mapCalls++
		return &llm.Response{Content: "[0:00] notes", Usage: llm.Usage{PromptTokens: 1}}, nil
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	return &llm.Response{Model: "test-model", Content: r, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10, Cost: 0.01}}, nil
}

const validJSON = `{"tldr":"Gist of the whole video in one full sentence here.","key_points":["a","b"],"sections":[{"title":"Intro","timestamp":"0:00","points":["p"]}],"takeaways":[],"mentioned":[]}`

var video = &youtube.Video{ID: "aircAruvnKk", Title: "T", Channel: "C", Duration: 90}

func segs() []youtube.Segment {
	return []youtube.Segment{{Start: 0, Text: "hello"}, {Start: 30, Text: "world"}}
}

func TestSummarizeHappyPath(t *testing.T) {
	f := &fakeChat{replies: []string{"```json\n" + validJSON + "\n```"}}
	s := &Summarizer{Client: f, Model: "m", Fallback: "fb", Lang: "ru", Length: "medium"}
	sum, res, err := s.Summarize(context.Background(), video, segs())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sum.TLDR, "Gist") || len(sum.Sections) != 1 || res.Model != "test-model" || res.Usage.Cost != 0.01 {
		t.Errorf("sum=%+v res=%+v", sum, res)
	}
	req := f.requests[0]
	if strings.Join(req.Models, ",") != "m,fb" {
		t.Errorf("fallback models = %v", req.Models)
	}
	sys := req.Messages[0].Content.(string)
	if !strings.Contains(sys, "Russian") {
		t.Error("system prompt does not set the language")
	}
	user := req.Messages[1].Content.(string)
	if !strings.Contains(user, "[0:30] world") || !strings.Contains(user, "Title: T") {
		t.Errorf("user prompt missing transcript/metadata:\n%s", user)
	}
}

func TestSummarizeRepairsInvalidJSON(t *testing.T) {
	f := &fakeChat{replies: []string{"sorry, here you go: {broken", validJSON}}
	s := &Summarizer{Client: f, Model: "m", Lang: "en", Length: "short"}
	sum, res, err := s.Summarize(context.Background(), video, segs())
	if err != nil || sum == nil {
		t.Fatalf("err=%v", err)
	}
	if len(f.requests) != 2 || len(f.requests[1].Messages) != 4 {
		t.Errorf("expected one repair round, got %d requests", len(f.requests))
	}
	if res.Usage.PromptTokens != 200 {
		t.Errorf("usage not accumulated: %+v", res.Usage)
	}
}

func TestSummarizeGivesUpAfterRepair(t *testing.T) {
	f := &fakeChat{replies: []string{"nope", `{"tldr":""}`}}
	s := &Summarizer{Client: f, Model: "m", Lang: "en", Length: "short"}
	if _, _, err := s.Summarize(context.Background(), video, segs()); err == nil {
		t.Fatal("want error")
	}
}

func TestSummarizeMapReduceForLongTranscript(t *testing.T) {
	var long []youtube.Segment
	for i := 0; i < 300; i++ {
		long = append(long, youtube.Segment{Start: float64(i * 10), Text: strings.Repeat("слово ", 20)})
	}
	f := &fakeChat{replies: []string{validJSON}}
	s := &Summarizer{Client: f, Model: "m", Lang: "ru", Length: "medium", MaxInputTokens: 2000, ChunkTokens: 1000}
	_, res, err := s.Summarize(context.Background(), video, long)
	if err != nil {
		t.Fatal(err)
	}
	if res.Chunks < 2 || f.mapCalls != res.Chunks {
		t.Errorf("chunks=%d mapCalls=%d", res.Chunks, f.mapCalls)
	}
}

func TestSplitLines(t *testing.T) {
	text := strings.Repeat("abcdefghi\n", 10) // ~4 tokens per line
	chunks := SplitLines(text, 10)
	if strings.Join(chunks, "") != text {
		t.Fatal("chunks lose text")
	}
	for _, c := range chunks {
		if EstimateTokens(c) > 12 {
			t.Errorf("chunk too big: %q", c)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	ok := map[string]int{"0:00": 0, "12:34": 754, "1:02:03": 3723, "[5:07]": 307}
	for in, want := range ok {
		if got, valid := ParseTimestamp(in); !valid || got != want {
			t.Errorf("ParseTimestamp(%q) = %d,%v want %d", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "12", "1:60", "a:bc", "1:2:3:4", "-1:00"} {
		if _, valid := ParseTimestamp(in); valid {
			t.Errorf("ParseTimestamp(%q) should be invalid", in)
		}
	}
}

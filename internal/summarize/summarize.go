package summarize

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/antalkon/ytb-sum/internal/llm"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

// Chatter is the subset of llm.Client the summarizer needs.
type Chatter interface {
	Chat(ctx context.Context, req llm.Request) (*llm.Response, error)
}

// Summarizer produces a Summary from a transcript.
type Summarizer struct {
	Client   Chatter
	Model    string
	Fallback string
	Lang     string
	Length   string
	// MaxInputTokens is the largest transcript sent in one request; longer
	// ones are condensed chunk by chunk first (map-reduce).
	MaxInputTokens int
	ChunkTokens    int
	Progress       func(stage string)
}

// Result carries bookkeeping about the run.
type Result struct {
	Model  string
	Usage  llm.Usage
	Chunks int
}

// EstimateTokens is a deliberately pessimistic token estimate
// (~3 characters per token, which holds for Cyrillic too).
func EstimateTokens(s string) int { return utf8.RuneCountInString(s)/3 + 1 }

// Summarize runs the pipeline and returns the parsed summary.
func (s *Summarizer) Summarize(ctx context.Context, v *youtube.Video, segs []youtube.Segment) (*Summary, Result, error) {
	var res Result
	transcript := youtube.FormatTranscript(segs)
	if strings.TrimSpace(transcript) == "" {
		return nil, res, fmt.Errorf("transcript is empty")
	}

	if s.MaxInputTokens > 0 && EstimateTokens(transcript) > s.MaxInputTokens {
		chunks := SplitLines(transcript, s.ChunkTokens)
		res.Chunks = len(chunks)
		notes, usage, err := s.condense(ctx, chunks)
		res.Usage.Add(usage)
		if err != nil {
			return nil, res, err
		}
		transcript = notes
	}

	s.progress("Summarizing with " + s.Model)
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt(s.Lang, s.Length)},
		{Role: "user", Content: userPrompt(v, transcript)},
	}
	// One repair round if the model returns malformed JSON.
	for attempt := 0; ; attempt++ {
		resp, err := s.Client.Chat(ctx, s.request(messages, true))
		if err != nil {
			return nil, res, err
		}
		res.Usage.Add(resp.Usage)
		res.Model = resp.Model
		sum, perr := Parse(resp.Content)
		if perr == nil {
			return sum, res, nil
		}
		if attempt == 1 {
			if resp.FinishReason == "length" {
				perr = fmt.Errorf("%w (output was cut off; try --length short)", perr)
			}
			return nil, res, perr
		}
		messages = append(messages,
			llm.Message{Role: "assistant", Content: resp.Content},
			llm.Message{Role: "user", Content: "That was not valid: " + perr.Error() + ". Reply with only the corrected JSON object."},
		)
	}
}

func (s *Summarizer) condense(ctx context.Context, chunks []string) (string, llm.Usage, error) {
	notes := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	usages := make([]llm.Usage, len(chunks))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	s.progress(fmt.Sprintf("Condensing long transcript (0/%d parts)", len(chunks)))
	for i, chunk := range chunks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			defer func() { <-sem }()
			req := s.request([]llm.Message{
				{Role: "system", Content: mapPrompt(s.Lang, i+1, len(chunks))},
				{Role: "user", Content: chunk},
			}, false)
			resp, err := s.Client.Chat(ctx, req)
			if err != nil {
				errs[i] = err
				return
			}
			notes[i], usages[i] = resp.Content, resp.Usage
			mu.Lock()
			done++
			s.progress(fmt.Sprintf("Condensing long transcript (%d/%d parts)", done, len(chunks)))
			mu.Unlock()
		}()
	}
	wg.Wait()
	var total llm.Usage
	for i := range chunks {
		total.Add(usages[i])
		if errs[i] != nil {
			return "", total, fmt.Errorf("condense part %d: %w", i+1, errs[i])
		}
	}
	return strings.Join(notes, "\n"), total, nil
}

func (s *Summarizer) request(msgs []llm.Message, structured bool) llm.Request {
	// No temperature: current Anthropic/OpenAI models on OpenRouter reject it.
	req := llm.Request{Model: s.Model, Messages: msgs, MaxTokens: 8000}
	if s.Fallback != "" && s.Fallback != s.Model {
		req.Models = []string{s.Model, s.Fallback}
	}
	if structured {
		req.ResponseFormat = &llm.ResponseFormat{
			Type:       "json_schema",
			JSONSchema: &llm.JSONSchema{Name: "video_summary", Strict: true, Schema: Schema()},
		}
	}
	return req
}

func (s *Summarizer) progress(stage string) {
	if s.Progress != nil {
		s.Progress(stage)
	}
}

// SplitLines splits text on line boundaries into chunks of at most maxTokens
// (estimated). A single oversized line becomes its own chunk.
func SplitLines(text string, maxTokens int) []string {
	var chunks []string
	var b strings.Builder
	cur := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		if line == "" {
			continue
		}
		t := EstimateTokens(line)
		if cur > 0 && cur+t > maxTokens {
			chunks = append(chunks, b.String())
			b.Reset()
			cur = 0
		}
		b.WriteString(line)
		cur += t
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}

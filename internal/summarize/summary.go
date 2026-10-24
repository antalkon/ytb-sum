// Package summarize turns a transcript plus metadata into a structured
// Summary using an LLM.
package summarize

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Summary is the structured result rendered to the terminal and Markdown.
type Summary struct {
	TLDR      string    `json:"tldr"`
	KeyPoints []string  `json:"key_points"`
	Sections  []Section `json:"sections"`
	Takeaways []string  `json:"takeaways"`
	Mentioned []string  `json:"mentioned"`
}

// Section is a chronological part of the video.
type Section struct {
	Title     string   `json:"title"`
	Timestamp string   `json:"timestamp"`
	Points    []string `json:"points"`
}

// Seconds parses Timestamp ("M:SS" or "H:MM:SS"); ok is false if invalid.
func (s Section) Seconds() (int, bool) {
	return ParseTimestamp(s.Timestamp)
}

// ParseTimestamp parses "M:SS" or "H:MM:SS" (optionally in brackets).
func ParseTimestamp(ts string) (int, bool) {
	ts = strings.Trim(strings.TrimSpace(ts), "[]")
	parts := strings.Split(ts, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	total := 0
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (i > 0 && n >= 60) {
			return 0, false
		}
		total = total*60 + n
	}
	return total, true
}

// Schema is the strict JSON schema the model must follow.
func Schema() map[string]any {
	strs := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		// tldr goes last so the model writes it after working through the content.
		"required": []string{"key_points", "sections", "takeaways", "mentioned", "tldr"},
		"properties": map[string]any{
			"tldr":       map[string]any{"type": "string", "description": "2-3 full sentences with the gist of the whole video, written after the other fields"},
			"key_points": strs,
			"sections": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"title", "timestamp", "points"},
					"properties": map[string]any{
						"title":     map[string]any{"type": "string"},
						"timestamp": map[string]any{"type": "string", "description": "start time copied from a transcript marker, e.g. 12:34 or 1:02:03"},
						"points":    strs,
					},
				},
			},
			"takeaways": strs,
			"mentioned": strs,
		},
	}
}

// Parse decodes model output into a Summary, tolerating code fences and
// surrounding prose, and checks the required content is present.
func Parse(content string) (*Summary, error) {
	s := strings.TrimSpace(content)
	if i, j := strings.IndexByte(s, '{'), strings.LastIndexByte(s, '}'); i >= 0 && j > i {
		s = s[i : j+1]
	}
	var sum Summary
	if err := json.Unmarshal([]byte(s), &sum); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	// Guards against placeholder values such as "x" seen in practice.
	if utf8.RuneCountInString(strings.TrimSpace(sum.TLDR)) < 40 {
		return nil, errors.New("tldr must be 2-3 full sentences")
	}
	if len(sum.KeyPoints) == 0 {
		return nil, errors.New("model returned no key points")
	}
	return &sum, nil
}

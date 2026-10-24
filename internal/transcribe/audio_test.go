package transcribe

import (
	"testing"

	"github.com/antalkon/ytb-sum/internal/youtube"
)

func TestParseTimestamped(t *testing.T) {
	in := "```\n[00:00] Hello there.\n[00:07] - Next phrase\ncontinued line\n\n[1:02:03] late\n```"
	got := ParseTimestamped(in, 1200)
	want := []youtube.Segment{
		{Start: 1200, Text: "Hello there."},
		{Start: 1207, Text: "Next phrase continued line"},
		{Start: 1200 + 3723, Text: "late"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("seg %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseTimestampedNoMarkers(t *testing.T) {
	got := ParseTimestamped("just text\nmore", 60)
	if len(got) != 1 || got[0].Start != 60 || got[0].Text != "just text more" {
		t.Errorf("got %+v", got)
	}
}

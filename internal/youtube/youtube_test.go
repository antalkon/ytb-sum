package youtube

import (
	"testing"
)

func TestParseVideoID(t *testing.T) {
	ok := map[string]string{
		"aircAruvnKk": "aircAruvnKk",
		"https://www.youtube.com/watch?v=aircAruvnKk":         "aircAruvnKk",
		"https://www.youtube.com/watch?v=aircAruvnKk&t=42s":   "aircAruvnKk",
		"youtube.com/watch?v=aircAruvnKk":                     "aircAruvnKk",
		"https://m.youtube.com/watch?v=aircAruvnKk":           "aircAruvnKk",
		"https://youtu.be/aircAruvnKk?si=abc":                 "aircAruvnKk",
		"https://www.youtube.com/shorts/aircAruvnKk":          "aircAruvnKk",
		"https://www.youtube.com/live/aircAruvnKk?feature=sh": "aircAruvnKk",
		"https://www.youtube.com/embed/aircAruvnKk":           "aircAruvnKk",
		"https://music.youtube.com/watch?v=aircAruvnKk":       "aircAruvnKk",
		"  https://youtu.be/aircAruvnKk  ":                    "aircAruvnKk",
	}
	for in, want := range ok {
		got, err := ParseVideoID(in)
		if err != nil || got != want {
			t.Errorf("ParseVideoID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{"", "hello", "https://vimeo.com/123456", "https://www.youtube.com/@channel",
		"https://www.youtube.com/watch?v=short", "https://evil.com/watch?v=aircAruvnKk"}
	for _, in := range bad {
		if got, err := ParseVideoID(in); err == nil {
			t.Errorf("ParseVideoID(%q) = %q, want error", in, got)
		}
	}
}

func TestParseJSON3(t *testing.T) {
	data := []byte(`{"events":[
		{"tStartMs":0,"dDurationMs":1000},
		{"tStartMs":1200,"segs":[{"utf8":"Hello"},{"utf8":" world"}]},
		{"tStartMs":2500,"aAppend":1,"segs":[{"utf8":"\n"}]},
		{"tStartMs":3000,"segs":[{"utf8":"Привет,\n  мир"}]}
	]}`)
	segs, err := ParseJSON3(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Segment{{1.2, "Hello world"}, {3, "Привет, мир"}}
	if len(segs) != len(want) {
		t.Fatalf("got %d segments %+v", len(segs), segs)
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Errorf("seg %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
	if _, err := ParseJSON3([]byte("<html>")); err == nil {
		t.Error("want error for non-JSON input")
	}
}

func TestGroupAndFormat(t *testing.T) {
	segs := []Segment{{0, "a"}, {5, "b"}, {21, "c"}, {3700, "d"}}
	g := Group(segs, 20)
	if len(g) != 3 || g[0].Text != "a b" || g[1].Text != "c" {
		t.Fatalf("Group = %+v", g)
	}
	got := FormatTranscript(g)
	want := "[0:00] a b\n[0:21] c\n[1:01:40] d\n"
	if got != want {
		t.Errorf("FormatTranscript = %q, want %q", got, want)
	}
}

func tr(n int) []Track { return make([]Track, n) }

func TestPickCaption(t *testing.T) {
	cases := []struct {
		name     string
		v        Video
		pref     string
		wantLang string
		wantAuto bool
		wantErr  bool
	}{
		{
			name:     "manual in video language beats auto",
			v:        Video{Language: "en-US", Subtitles: map[string][]Track{"en": tr(1), "ru": tr(1)}, AutoCaps: map[string][]Track{"en-orig": tr(1)}},
			wantLang: "en",
		},
		{
			name:     "preferred manual language wins",
			v:        Video{Language: "en", Subtitles: map[string][]Track{"en": tr(1), "ru": tr(1)}},
			pref:     "ru",
			wantLang: "ru",
		},
		{
			name:     "original auto captions, not auto-translated",
			v:        Video{Language: "ru", AutoCaps: map[string][]Track{"de": tr(1), "ru-orig": tr(1), "ru": tr(1)}},
			pref:     "de",
			wantLang: "ru", wantAuto: true,
		},
		{
			name:     "manual with region code matches base language",
			v:        Video{Language: "pt", Subtitles: map[string][]Track{"pt-BR": tr(1)}},
			wantLang: "pt-BR",
		},
		{
			name:    "no subtitles",
			v:       Video{Language: "en", Subtitles: map[string][]Track{"live_chat": tr(1)}},
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := PickCaption(&c.v, c.pref)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Lang != c.wantLang || got.Auto != c.wantAuto {
				t.Errorf("got %s auto=%v, want %s auto=%v", got.Lang, got.Auto, c.wantLang, c.wantAuto)
			}
		})
	}
}

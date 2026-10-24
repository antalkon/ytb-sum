package mdfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antalkon/ytb-sum/internal/summarize"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

func TestSafeFilename(t *testing.T) {
	cases := map[string]string{
		"What is AI? / Part 1: Intro":   "What is AI Part 1 Intro",
		"Нейросети: как это работает 🤖": "Нейросети как это работает 🤖",
		"  ...hidden.  ":   "hidden",
		`a\b*c"d<e>f|g`:    "a b c d e f g",
		"line\nbreak\ttab": "line break tab",
		"???":              "video",
	}
	for in, want := range cases {
		if got := SafeFilename(in); got != want {
			t.Errorf("SafeFilename(%q) = %q, want %q", in, got, want)
		}
	}
	long := SafeFilename(strings.Repeat("Ж", 500))
	if len(long) > MaxNameBytes || !strings.HasPrefix(long, "ЖЖ") || strings.ContainsRune(long, '\uFFFD') {
		t.Errorf("long name not truncated on a rune boundary: %d bytes", len(long))
	}
}

func TestWriteDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	p1, err := Write(dir, "Title: one", "first")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Write(dir, "Title: one", "second")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p1) != "Title one.md" || filepath.Base(p2) != "Title one (2).md" {
		t.Errorf("paths %s, %s", p1, p2)
	}
	if b, _ := os.ReadFile(p1); string(b) != "first" {
		t.Error("first file was overwritten")
	}
}

func TestDocument(t *testing.T) {
	v := &youtube.Video{ID: "aircAruvnKk", Title: "Neural [nets]", Channel: "3B1B", Duration: 1120, UploadDate: "20171005"}
	s := &summarize.Summary{
		TLDR:      "Gist.",
		KeyPoints: []string{"k1"},
		Sections: []summarize.Section{
			{Title: "Intro", Timestamp: "2:05", Points: []string{"p1"}},
			{Title: "Bad ts", Timestamp: "soon", Points: []string{"p2"}},
		},
		Mentioned: []string{"Book — ref"},
	}
	doc := Document(v, s, "ru")
	for _, want := range []string{
		"# Neural \\[nets\\]",
		"- **Канал:** 3B1B",
		"- **Длительность:** 18:40",
		"- **Опубликовано:** 2017-10-05",
		"## Кратко\n\nGist.",
		"### [2:05](https://www.youtube.com/watch?v=aircAruvnKk&t=125s) Intro",
		"### Bad ts",
		"## Упоминается\n\n- Book — ref",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "Выводы") {
		t.Error("empty takeaways section should be omitted")
	}
}

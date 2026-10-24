package htmlpage

import (
	"strings"
	"testing"
)

const doc = `# Neural \[nets\] & <tricks>

- **Канал:** 3B1B
- **Ссылка:** https://www.youtube.com/watch?v=aircAruvnKk

## Кратко

Gist <script>alert(1)</script> here.

## Содержание

### [2:05](https://www.youtube.com/watch?v=aircAruvnKk&t=125s) Intro

- point
`

func TestRender(t *testing.T) {
	out, err := Render(doc, "ru")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<html lang="ru">`,
		`<title>Neural [nets] &amp; &lt;tricks&gt;</title>`,
		`https://i.ytimg.com/vi/aircAruvnKk/hqdefault.jpg`,
		`<a href="https://www.youtube.com/watch?v=aircAruvnKk">youtu.be/aircAruvnKk</a>`,
		`<a href="#%d0%ba%d1%80%d0%b0%d1%82%d0%ba%d0%be">Кратко</a>`,
		`<h2 id="кратко">Кратко</h2>`,
		`href="https://www.youtube.com/watch?v=aircAruvnKk&amp;t=125s">2:05</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(out, "<script>alert(1)") {
		t.Error("raw HTML from markdown must not be rendered")
	}
	if strings.Count(out, "<h1>") != 1 {
		t.Error("expected exactly one <h1>")
	}
}

func TestRenderWithoutVideo(t *testing.T) {
	out, err := Render("## Only body\n\ntext\n", "en")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "i.ytimg.com") || !strings.Contains(out, "<title>YouTube summary</title>") {
		t.Error("page without a video should have no thumbnail and a default title")
	}
}

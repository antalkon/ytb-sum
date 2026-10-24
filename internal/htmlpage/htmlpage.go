// Package htmlpage turns a ytb-sum Markdown summary into a self-contained,
// styled HTML page.
package htmlpage

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

//go:embed page.html.tmpl
var pageTmpl string

var tmpl = template.Must(template.New("page").Parse(pageTmpl))

var (
	videoIDRe = regexp.MustCompile(`youtube\.com/watch\?v=([A-Za-z0-9_-]{11})`)
	h2Re      = regexp.MustCompile(`(?m)^## (.+)$`)
	h1Re      = regexp.MustCompile(`(?m)^# .*$`)
	bareURLRe = regexp.MustCompile(`(?m)(:\*\*\s*)(https://www\.youtube\.com/watch\?v=([A-Za-z0-9_-]{11}))\s*$`)
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// No html.WithUnsafe: raw HTML in the Markdown is dropped, because
	// summaries come from an LLM.
)

type section struct {
	ID, Title string
}

type page struct {
	Lang      string
	Title     string
	VideoID   string
	Header    template.HTML
	Body      template.HTML
	Sections  []section
	Generated string
}

// Render converts a summary document (as produced by mdfile.Document) into
// a complete HTML page. lang is the BCP-47 code for the <html lang> attribute.
func Render(markdown, lang string) (string, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	head, body := splitHeader(markdown)
	p := page{
		Lang:      lang,
		Title:     firstH1(markdown),
		Generated: time.Now().Format("2006-01-02 15:04"),
	}
	if m := videoIDRe.FindStringSubmatch(markdown); m != nil {
		p.VideoID = m[1]
	}
	// The H1 is rendered by the template, so drop it from the header block,
	// and show the bare watch URL as a short link.
	head = strings.TrimSpace(h1Re.ReplaceAllString(head, ""))
	head = bareURLRe.ReplaceAllString(head, "$1[youtu.be/$3]($2)")
	var err error
	if p.Header, err = toHTML(head); err != nil {
		return "", err
	}
	if p.Body, err = toHTML(body); err != nil {
		return "", err
	}
	p.Sections = sectionsOf(body)
	if p.Title == "" {
		p.Title = "YouTube summary"
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, p); err != nil {
		return "", err
	}
	return b.String(), nil
}

// splitHeader separates the title/metadata block from the summary body
// (everything from the first "## " heading).
func splitHeader(markdown string) (head, body string) {
	loc := h2Re.FindStringIndex(markdown)
	if loc == nil {
		return markdown, ""
	}
	return markdown[:loc[0]], markdown[loc[0]:]
}

func firstH1(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "# ") {
			t := strings.TrimSpace(line[2:])
			return strings.NewReplacer(`\[`, "[", `\]`, "]").Replace(t)
		}
	}
	return ""
}

func toHTML(src string) (template.HTML, error) {
	var b bytes.Buffer
	ctx := parser.NewContext(parser.WithIDs(newIDs()))
	if err := md.Convert([]byte(src), &b, parser.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	// goldmark escapes raw HTML, so its output is safe to embed.
	return template.HTML(b.String()), nil //nolint:gosec
}

// sectionsOf lists the "## " headings with the ids goldmark assigns them.
func sectionsOf(body string) []section {
	src := []byte(body)
	ctx := parser.NewContext(parser.WithIDs(newIDs()))
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))
	var out []section
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !entering || !ok || h.Level != 2 {
			return ast.WalkContinue, nil
		}
		id, _ := h.AttributeString("id")
		idb, _ := id.([]byte)
		out = append(out, section{ID: string(idb), Title: plainText(h, src)})
		return ast.WalkSkipChildren, nil
	})
	return out
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			b.Write(t.Segment.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// ids generates Unicode-friendly heading anchors ("ключевые-тезисы");
// goldmark's default keeps only ASCII, which empties Cyrillic headings.
type ids struct{ seen map[string]bool }

func newIDs() *ids { return &ids{seen: map[string]bool{}} }

func (s *ids) Generate(value []byte, _ ast.NodeKind) []byte {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(string(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	id := strings.TrimSuffix(b.String(), "-")
	if id == "" {
		id = "section"
	}
	base := id
	for i := 1; s.seen[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	s.seen[id] = true
	return []byte(id)
}

func (s *ids) Put(value []byte) { s.seen[string(value)] = true }

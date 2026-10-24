package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/antalkon/ytb-sum/internal/browser"
	"github.com/antalkon/ytb-sum/internal/htmlpage"
)

func newHTMLCmd() *cobra.Command {
	var out string
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "html <summary.md>",
		Short: "Turn a saved summary into a styled web page and open it",
		Long: `Converts a Markdown summary made by ytb-sum into a self-contained HTML page
next to it (same name, .html) and opens it in the default browser.`,
		Example: `  ytb-sum html "But what is a neural network.md"
  ytb-sum html notes.md -o ~/Desktop/notes.html --no-open`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			text := string(data)
			page, err := htmlpage.Render(text, guessLang(text))
			if err != nil {
				return err
			}
			if out == "" {
				out = strings.TrimSuffix(src, filepath.Ext(src)) + ".html"
			}
			if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
				return err
			}
			fmt.Println("Saved", displayPath(out))
			if noOpen {
				return nil
			}
			return browser.Open(out)
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "output file (default: next to the .md, with .html)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser")
	return cmd
}

// htmlLang maps --lang to an <html lang> value.
func htmlLang(lang string) string {
	l := strings.ToLower(lang)
	if len(l) == 2 || (len(l) == 5 && l[2] == '-') {
		return l
	}
	if l == "russian" {
		return "ru"
	}
	return "en"
}

// guessLang returns "ru" when the text is mostly Cyrillic, otherwise "en".
func guessLang(text string) string {
	var cyr, lat int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case r < 128 && unicode.IsLetter(r):
			lat++
		}
	}
	if cyr > lat {
		return "ru"
	}
	return "en"
}

// Package cli wires the ytb-sum command line.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/antalkon/ytb-sum/internal/config"
	"github.com/antalkon/ytb-sum/internal/llm"
	"github.com/antalkon/ytb-sum/internal/mdfile"
	"github.com/antalkon/ytb-sum/internal/render"
	"github.com/antalkon/ytb-sum/internal/summarize"
	"github.com/antalkon/ytb-sum/internal/transcribe"
	"github.com/antalkon/ytb-sum/internal/youtube"
)

// Version is set at build time with -ldflags "-X github.com/antalkon/ytb-sum/internal/cli.Version=…".
var Version = "dev"

// errInterrupted marks a run cancelled by Ctrl+C.
var errInterrupted = errors.New("interrupted")

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		if errors.Is(err, errInterrupted) {
			fmt.Fprintln(os.Stderr, "\nInterrupted — temporary files removed.")
			return 130
		}
		msg := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("Error:")
		fmt.Fprintln(os.Stderr, msg, err)
		if h := llm.Hint(err); h != "" {
			fmt.Fprintln(os.Stderr, "Hint:", h)
		}
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	var fl config.Config
	cmd := &cobra.Command{
		Use:   "ytb-sum <youtube-url|video-id>",
		Short: "Summarize a YouTube video in the terminal and save it as Markdown",
		Long: `ytb-sum fetches a YouTube video's subtitles and metadata, asks an LLM
(via OpenRouter by default) for a structured summary, prints it in the
terminal and saves "<video title>.md" in the current directory.

Nothing is cached: temporary files are removed when the run ends.

API key lookup: --api-key, $OPENROUTER_API_KEY, macOS Keychain
(service "ytb-sum"), then the config file.`,
		Example: `  ytb-sum https://youtu.be/aircAruvnKk
  ytb-sum -q fast -l en --length short dQw4w9WgXcQ
  ytb-sum -m deepseek/deepseek-v4.1-flash --no-file <url>
  ytb-sum --audio <url-of-video-without-subtitles>`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveConfig(cmd, fl)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cfg, args[0])
		},
	}
	f := cmd.Flags()
	f.StringVar(&fl.APIKey, "api-key", "", "OpenRouter API key (prefer $OPENROUTER_API_KEY)")
	f.StringVar(&fl.BaseURL, "base-url", "", "OpenAI-compatible API base URL (default "+config.DefaultBaseURL+")")
	f.StringVarP(&fl.Quality, "quality", "q", "", "model preset: fast | balanced | best (default balanced)")
	f.StringVarP(&fl.Model, "model", "m", "", "model id, overrides --quality (see openrouter.ai/models)")
	f.StringVar(&fl.FallbackModel, "fallback-model", "", "model used if the main one fails (default "+config.DefaultFallbackModel+")")
	f.StringVar(&fl.AudioModel, "audio-model", "", "model for --audio transcription (default "+config.DefaultAudioModel+")")
	f.StringVarP(&fl.Lang, "lang", "l", "", "summary language (default ru)")
	f.StringVar(&fl.Length, "length", "", "summary length: short | medium | long (default medium)")
	f.StringVar(&fl.SubLang, "sub-lang", "", "preferred subtitle language (default: the video's language)")
	f.StringVarP(&fl.OutputDir, "output", "o", "", "directory for the .md file (default: current directory)")
	f.StringVar(&fl.CookiesFromBrowser, "cookies-from-browser", "", "pass browser cookies to yt-dlp for age-restricted/private videos (e.g. chrome)")
	f.BoolVarP(&fl.AudioFallback, "audio", "a", false, "transcribe the audio when the video has no subtitles")
	f.BoolVar(&fl.NoFile, "no-file", false, "print only, do not write a .md file")
	f.BoolVar(&fl.NoColor, "no-color", false, "disable colors and styling")
	f.DurationVar(&fl.Timeout, "timeout", 0, "overall timeout (default 10m)")
	f.BoolVarP(&fl.Verbose, "verbose", "v", false, "print each stage")
	cmd.SetVersionTemplate("ytb-sum {{.Version}}\n")
	cmd.AddCommand(newConfigCmd(&fl))
	return cmd
}

// resolveConfig layers defaults < file < env < keychain < flags.
func resolveConfig(cmd *cobra.Command, fl config.Config) (config.Config, error) {
	cfg := config.Defaults()
	path, err := config.Path()
	if err != nil {
		return cfg, err
	}
	warn, err := cfg.LoadFile(path)
	if err != nil {
		return cfg, err
	}
	if warn != "" {
		fmt.Fprintln(os.Stderr, "Warning:", warn)
	}
	cfg.LoadEnv(os.Getenv)

	flags := cmd.Flags()
	changed := func(name string) bool { return flags.Lookup(name) != nil && flags.Changed(name) }
	strs := map[string]struct{ dst, src *string }{
		"api-key":              {&cfg.APIKey, &fl.APIKey},
		"base-url":             {&cfg.BaseURL, &fl.BaseURL},
		"quality":              {&cfg.Quality, &fl.Quality},
		"model":                {&cfg.Model, &fl.Model},
		"fallback-model":       {&cfg.FallbackModel, &fl.FallbackModel},
		"audio-model":          {&cfg.AudioModel, &fl.AudioModel},
		"lang":                 {&cfg.Lang, &fl.Lang},
		"length":               {&cfg.Length, &fl.Length},
		"sub-lang":             {&cfg.SubLang, &fl.SubLang},
		"output":               {&cfg.OutputDir, &fl.OutputDir},
		"cookies-from-browser": {&cfg.CookiesFromBrowser, &fl.CookiesFromBrowser},
	}
	for name, p := range strs {
		if changed(name) {
			*p.dst = *p.src
		}
	}
	if changed("api-key") {
		cfg.APIKeySource = "--api-key flag"
	}
	// --quality picks a preset, so it must beat a model set in file/env.
	if changed("quality") && !changed("model") {
		cfg.Model = ""
	}
	bools := map[string]struct{ dst, src *bool }{
		"audio":    {&cfg.AudioFallback, &fl.AudioFallback},
		"no-file":  {&cfg.NoFile, &fl.NoFile},
		"no-color": {&cfg.NoColor, &fl.NoColor},
		"verbose":  {&cfg.Verbose, &fl.Verbose},
	}
	for name, p := range bools {
		if changed(name) {
			*p.dst = *p.src
		}
	}
	if changed("timeout") {
		cfg.Timeout = fl.Timeout
	}
	cfg.LoadKeychain()
	return cfg, cfg.Validate()
}

func run(parent context.Context, cfg config.Config, input string) error {
	start := time.Now()
	id, err := youtube.ParseVideoID(input)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return errors.New("yt-dlp not found in PATH — install it: brew install yt-dlp")
	}
	if cfg.APIKey == "" && strings.Contains(cfg.BaseURL, "openrouter.ai") {
		return errors.New("no API key: export OPENROUTER_API_KEY=sk-or-… (or see `ytb-sum config init`)")
	}
	outDir, err := outputDir(cfg)
	if err != nil {
		return err
	}

	// Ctrl+C / SIGTERM cancel the context: child processes are killed and
	// the deferred cleanup below still runs.
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	removeStaleTemp()
	tmpRoot, err := os.MkdirTemp("", tempPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	spin := render.NewSpinner(cfg.Verbose)
	defer spin.Stop()
	wrap := func(err error) error {
		if parent.Err() == nil && ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errInterrupted
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timed out after %s (raise --timeout)", cfg.Timeout)
		}
		return err
	}

	hc := &http.Client{Timeout: 5 * time.Minute}
	client := llm.New(cfg.BaseURL, cfg.APIKey, hc)
	yopts := youtube.Options{CookiesFromBrowser: cfg.CookiesFromBrowser}

	spin.Set("Fetching video info")
	video, err := youtube.FetchVideo(ctx, id, tmpRoot, yopts)
	if err != nil {
		return wrap(err)
	}

	var (
		segs         []youtube.Segment
		captionLabel string
		usage        llm.Usage
	)
	caption, err := youtube.PickCaption(video, cfg.SubLang)
	switch {
	case err == nil:
		spin.Set("Downloading subtitles")
		segs, err = youtube.FetchTranscript(ctx, hc, video, caption, tmpRoot, yopts)
		if err != nil {
			return wrap(err)
		}
		captionLabel = caption.Source()
	case errors.Is(err, youtube.ErrNoSubtitles) && cfg.AudioFallback:
		if _, lerr := exec.LookPath("ffmpeg"); lerr != nil {
			return errors.New("ffmpeg not found in PATH — install it: brew install ffmpeg")
		}
		tr := &transcribe.Transcriber{Client: client, Model: cfg.AudioModel, Progress: spin.Set}
		var u llm.Usage
		segs, u, err = tr.Transcribe(ctx, id, tmpRoot, yopts)
		usage.Add(u)
		if err != nil {
			return wrap(err)
		}
		captionLabel = "audio → " + cfg.AudioModel
	case errors.Is(err, youtube.ErrNoSubtitles):
		return errors.New("this video has no subtitles — rerun with --audio to transcribe its audio")
	default:
		return wrap(err)
	}

	sz := &summarize.Summarizer{
		Client:         client,
		Model:          cfg.ResolvedModel(),
		Fallback:       cfg.FallbackModel,
		Lang:           cfg.Lang,
		Length:         cfg.Length,
		MaxInputTokens: 300_000,
		ChunkTokens:    100_000,
		Progress:       spin.Set,
	}
	sum, res, err := sz.Summarize(ctx, video, youtube.Group(segs, 20))
	usage.Add(res.Usage)
	if err != nil {
		return wrap(err)
	}
	spin.Finish()

	stats := render.Stats{Model: res.Model, Caption: captionLabel, Usage: usage, Chunks: res.Chunks}
	if !cfg.NoFile {
		path, err := mdfile.Write(outDir, video.Title, mdfile.Document(video, sum, cfg.Lang))
		if err != nil {
			return fmt.Errorf("save markdown: %w", err)
		}
		stats.File = displayPath(path)
	}
	stats.Elapsed = time.Since(start)
	return render.NewPrinter(cfg.NoColor).Summary(video, sum, cfg.Lang, stats)
}

func outputDir(cfg config.Config) (string, error) {
	dir := cfg.OutputDir
	if dir == "" {
		return os.Getwd()
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	st, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("output directory: %w", err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("output %s is not a directory", dir)
	}
	return dir, nil
}

func displayPath(p string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return p
}

const tempPrefix = "ytb-sum-"

// removeStaleTemp deletes temp dirs left by runs that were killed with
// SIGKILL (where deferred cleanup cannot run).
func removeStaleTemp() {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), tempPrefix+"*"))
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && st.IsDir() && time.Since(st.ModTime()) > 6*time.Hour {
			os.RemoveAll(m)
		}
	}
}

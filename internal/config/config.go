// Package config resolves ytb-sum settings from defaults, the YAML config
// file, environment variables and CLI flags (in increasing priority).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL       = "https://openrouter.ai/api/v1"
	DefaultQuality       = "balanced"
	DefaultFallbackModel = "openai/gpt-6-sol"
	DefaultAudioModel    = "google/gemini-3.8-flash"
	DefaultLang          = "ru"
	DefaultLength        = "medium"
	DefaultTimeout       = 10 * time.Minute
)

// QualityPresets maps --quality values to OpenRouter model ids.
var QualityPresets = map[string]string{
	"fast":     "anthropic/claude-haiku-5.5",
	"balanced": "anthropic/claude-sonnet-5.5",
	"best":     "anthropic/claude-opus-5.5",
}

var validLengths = map[string]bool{"short": true, "medium": true, "long": true}

// Config is the fully resolved runtime configuration.
type Config struct {
	APIKey             string        `yaml:"api_key"`
	BaseURL            string        `yaml:"base_url"`
	Quality            string        `yaml:"quality"`
	Model              string        `yaml:"model"`
	FallbackModel      string        `yaml:"fallback_model"`
	AudioModel         string        `yaml:"audio_model"`
	Lang               string        `yaml:"lang"`
	Length             string        `yaml:"length"`
	SubLang            string        `yaml:"sub_lang"`
	OutputDir          string        `yaml:"output_dir"`
	CookiesFromBrowser string        `yaml:"cookies_from_browser"`
	AudioFallback      bool          `yaml:"audio_fallback"`
	NoFile             bool          `yaml:"no_file"`
	HTML               bool          `yaml:"html"`
	NoOpen             bool          `yaml:"no_open"`
	NoColor            bool          `yaml:"no_color"`
	Timeout            time.Duration `yaml:"timeout"`
	Verbose            bool          `yaml:"-"`

	// APIKeySource describes where the key came from, for `config show`.
	APIKeySource string `yaml:"-"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		BaseURL:       DefaultBaseURL,
		Quality:       DefaultQuality,
		FallbackModel: DefaultFallbackModel,
		AudioModel:    DefaultAudioModel,
		Lang:          DefaultLang,
		Length:        DefaultLength,
		Timeout:       DefaultTimeout,
	}
}

// Path returns the config file location: $YTB_SUM_CONFIG or
// $XDG_CONFIG_HOME/ytb-sum/config.yaml (falling back to ~/.config).
func Path() (string, error) {
	if p := os.Getenv("YTB_SUM_CONFIG"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ytb-sum", "config.yaml"), nil
}

// LoadFile overlays values from the YAML file at path onto c. A missing file
// is not an error. It returns a warning when the file holds an API key but is
// readable by other users.
func (c *Config) LoadFile(path string) (warning string, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return "", fmt.Errorf("parse config %s: %w", path, err)
	}
	if c.APIKey != "" {
		c.APIKeySource = "config file"
		if st, err := os.Stat(path); err == nil && st.Mode().Perm()&0o077 != 0 {
			warning = fmt.Sprintf("%s contains an API key and is readable by others; run: chmod 600 %q", path, path)
		}
	}
	return warning, nil
}

// LoadEnv overlays values from environment variables onto c.
func (c *Config) LoadEnv(getenv func(string) string) {
	set := func(dst *string, keys ...string) {
		for _, k := range keys {
			if v := getenv(k); v != "" {
				*dst = v
				return
			}
		}
	}
	if v := getenv("OPENROUTER_API_KEY"); v != "" {
		c.APIKey, c.APIKeySource = v, "env OPENROUTER_API_KEY"
	} else if v := getenv("YTB_SUM_API_KEY"); v != "" {
		c.APIKey, c.APIKeySource = v, "env YTB_SUM_API_KEY"
	}
	set(&c.BaseURL, "YTB_SUM_BASE_URL")
	set(&c.Quality, "YTB_SUM_QUALITY")
	set(&c.Model, "YTB_SUM_MODEL")
	set(&c.AudioModel, "YTB_SUM_AUDIO_MODEL")
	set(&c.Lang, "YTB_SUM_LANG")
	set(&c.OutputDir, "YTB_SUM_OUTPUT_DIR")
	if getenv("NO_COLOR") != "" {
		c.NoColor = true
	}
}

// ResolvedModel returns the explicit model, or the one from the quality preset.
func (c *Config) ResolvedModel() string {
	if c.Model != "" {
		return c.Model
	}
	return QualityPresets[c.Quality]
}

// Validate checks values that do not depend on the network.
func (c *Config) Validate() error {
	if c.Model == "" {
		if _, ok := QualityPresets[c.Quality]; !ok {
			return fmt.Errorf("unknown --quality %q (want fast, balanced or best)", c.Quality)
		}
	}
	if !validLengths[c.Length] {
		return fmt.Errorf("unknown --length %q (want short, medium or long)", c.Length)
	}
	if c.Timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	if strings.TrimSpace(c.Lang) == "" {
		return errors.New("--lang must not be empty")
	}
	return nil
}

// MaskKey hides all but the edges of an API key.
func MaskKey(k string) string {
	if k == "" {
		return "(not set)"
	}
	if len(k) <= 12 {
		return "****"
	}
	return k[:6] + "…" + k[len(k)-4:]
}

// Template is written by `ytb-sum config init`.
const Template = `# ytb-sum configuration. Priority: flags > env > this file > defaults.
# Prefer OPENROUTER_API_KEY or the macOS Keychain over storing the key here.
# api_key: sk-or-...
base_url: https://openrouter.ai/api/v1
quality: balanced          # fast | balanced | best
# model: anthropic/claude-sonnet-5.5   # overrides quality
fallback_model: openai/gpt-6-sol
audio_model: google/gemini-3.8-flash
lang: ru                   # summary language
length: medium             # short | medium | long
# sub_lang: en             # preferred subtitle language
# output_dir: ~/Notes/youtube
# cookies_from_browser: chrome
audio_fallback: false      # transcribe audio when a video has no subtitles
html: false                # also save an HTML page and open it in the browser
no_open: false             # with html: save it but do not open the browser
timeout: 10m
`

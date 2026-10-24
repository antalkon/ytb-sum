<div align="center">

# ytb-sum

**Summarize any YouTube video in your terminal — and keep it as Markdown.**

[![CI](https://github.com/antalkon/ytb-sum/actions/workflows/ci.yml/badge.svg)](https://github.com/antalkon/ytb-sum/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/antalkon/ytb-sum)](https://github.com/antalkon/ytb-sum/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/antalkon/ytb-sum)](https://goreportcard.com/report/github.com/antalkon/ytb-sum)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

English · [Русский](README.ru.md)

</div>

```text
$ ytb-sum https://youtu.be/aircAruvnKk
  ✔ Fetching video info 2.1s
  ✔ Downloading subtitles 0.4s
  ✔ Summarizing with anthropic/claude-sonnet-5.5 19.8s

  ╭──────────────────────────────────────────────────────────────╮
  │ ▶ But what is a neural network? | Deep learning chapter 1    │
  │ 3Blue1Brown  ·  18:40  ·  2017-10-05                         │
  │ https://www.youtube.com/watch?v=aircAruvnKk                  │
  ╰──────────────────────────────────────────────────────────────╯

  ▍TL;DR
  A neural network for digit recognition is a chain of layers …

  ▍Key points
  • A "neuron" just holds a number between 0 and 1 …

  ▍Contents
  › 0:00 · Recognizing a handwritten digit
  › 2:42 · What a neuron is
  …
  anthropic/claude-sonnet-5.5  ·  subs: en  ·  8.1k→2.8k tok  ·  $0.0440  ·  25.4s
  ✔ Saved But what is a neural network Deep learning chapter 1.md
```

See a full generated file: [docs/example-ru.md](docs/example-ru.md) and its web page version [docs/example-ru.html](docs/example-ru.html).

## Features

- **Clear, structured summaries**: TL;DR, key points, chronological sections with clickable timestamps, takeaways and mentioned resources.
- **Pretty terminal output** with colors, adapting to dark/light themes. Plain Markdown when piped or with `--no-color`.
- **Saves `<video title>.md`** in the current directory (never overwrites; adds ` (2)`).
- **Web page view**: `--html` also saves a styled, self-contained `.html` (dark/light theme, section navigation, clickable timestamps) and opens it in your browser; `ytb-sum html <file.md>` does the same for any saved summary.
- **Any model via [OpenRouter](https://openrouter.ai)**, or any OpenAI-compatible API (Ollama, LM Studio…), with automatic fallback to a second model.
- **Any summary language** (`--lang`, default `ru`), whatever the video's language.
- **Videos without subtitles**: `--audio` transcribes the audio track with Gemini.
- **Long videos**: up to ~1M-token context in one pass, with map-reduce condensing beyond that.
- **Leaves no trace**: subtitles are fetched straight into memory, yt-dlp runs with `--no-cache-dir`, temporary audio is deleted even on Ctrl+C, child processes are killed with their whole tree.

## Installation

ytb-sum needs **[yt-dlp](https://github.com/yt-dlp/yt-dlp)** at runtime (and **ffmpeg** only for `--audio`).

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.sh | sh
```

That's all. The script:

- detects your OS/CPU and verifies the SHA-256 checksum;
- installs into a writable directory already on your `PATH` (e.g. `/opt/homebrew/bin`), so `ytb-sum` works right away; otherwise uses `~/.local/bin` and adds it to your shell profile (`.zshrc`, `.bashrc`, fish);
- installs missing `yt-dlp` and `ffmpeg` with Homebrew, apt, dnf, pacman or apk.

Options: `YTB_SUM_VERSION=v0.1.0`, `YTB_SUM_INSTALL_DIR=…`, `YTB_SUM_NO_DEPS=1`, `YTB_SUM_NO_MODIFY_PATH=1`.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.ps1 | iex
```

Adds `ytb-sum` to your user `PATH` (usable in the same window) and installs `yt-dlp` / `ffmpeg` with winget if missing.

### Linux packages

`.deb`, `.rpm` and `.apk` packages are attached to every [release](https://github.com/antalkon/ytb-sum/releases/latest):

```bash
sudo dpkg -i ytb-sum_*_linux_amd64.deb
```

### With Go

```bash
go install github.com/antalkon/ytb-sum/cmd/ytb-sum@latest
```

### Manual

Download the archive for your platform from [Releases](https://github.com/antalkon/ytb-sum/releases/latest), extract it and put `ytb-sum` on your `PATH`. Builds: Linux, macOS, Windows × amd64, arm64.

## Quick start

1. Get an API key at <https://openrouter.ai/keys>.
2. Make it available (pick one):

   ```bash
   export OPENROUTER_API_KEY=sk-or-...                          # e.g. in ~/.zshrc
   security add-generic-password -a "$USER" -s ytb-sum -w       # macOS Keychain
   ```

3. Run:

   ```bash
   ytb-sum https://www.youtube.com/watch?v=aircAruvnKk
   ```

## Usage

```text
ytb-sum <youtube-url|video-id> [flags]
```

Accepts `watch?v=`, `youtu.be/`, `/shorts/`, `/live/`, `/embed/`, `music.youtube.com` links and bare 11-character ids.

| Flag | Default | Description |
|---|---|---|
| `-q, --quality` | `balanced` | Model preset: `fast`, `balanced`, `best` |
| `-m, --model` | — | Any OpenRouter model id; overrides `--quality` |
| `--fallback-model` | `openai/gpt-6-sol` | Used automatically if the main model fails |
| `-l, --lang` | `ru` | Summary language (`en`, `de`, … or a name like `"Brazilian Portuguese"`) |
| `--length` | `medium` | `short`, `medium`, `long` |
| `--sub-lang` | video language | Preferred subtitle track |
| `-a, --audio` | off | Transcribe the audio if the video has no subtitles |
| `--audio-model` | `google/gemini-3.8-flash` | Model for `--audio` |
| `-o, --output` | current dir | Where to save the `.md` file |
| `--no-file` | off | Print only |
| `--html` | off | Also save a styled `.html` page and open it in the browser |
| `--no-open` | off | With `--html`: save the page without opening it |
| `--no-color` | off | Plain output (also respects `NO_COLOR`) |
| `--cookies-from-browser` | — | Pass browser cookies to yt-dlp (age-restricted/members videos), e.g. `chrome` |
| `--base-url` | OpenRouter | Any OpenAI-compatible endpoint |
| `--api-key` | — | Prefer the env var or Keychain |
| `--timeout` | `10m` | Overall timeout |
| `-v, --verbose` | off | Print stages even when not on a terminal |

### Model presets

| Preset | Model | Approx. cost per 1 h video |
|---|---|---|
| `fast` | `anthropic/claude-haiku-5.5` | ~$0.004 |
| `balanced` | `anthropic/claude-sonnet-5.5` | ~$0.07 |
| `best` | `anthropic/claude-opus-5.5` | ~$0.14 |

The actual cost of each run is printed in the footer (as reported by OpenRouter).

### Examples

```bash
ytb-sum -q fast --length short https://youtu.be/dQw4w9WgXcQ   # quick & cheap
ytb-sum -l en -o ~/Notes/youtube <url>                         # English, into a notes folder
ytb-sum -m deepseek/deepseek-v4.1-flash <url>                  # any OpenRouter model
ytb-sum --audio <url>                                          # video without subtitles
ytb-sum --no-file --no-color <url> | less                      # pipe plain Markdown
ytb-sum --html <url>                                           # .md + .html, opened in the browser
ytb-sum html "Video title.md"                                  # web page from a saved summary
ytb-sum --base-url http://localhost:11434/v1 -m qwen3 <url>    # local Ollama
```

## Configuration

Priority: **flags → environment → config file → defaults**.

```bash
ytb-sum html <file.md> [-o page.html] [--no-open]   # Markdown summary → web page
ytb-sum config init   # create ~/.config/ytb-sum/config.yaml (mode 600)
ytb-sum config show   # print resolved settings, key masked
ytb-sum config path
```

| Environment variable | Meaning |
|---|---|
| `OPENROUTER_API_KEY` / `YTB_SUM_API_KEY` | API key |
| `YTB_SUM_MODEL`, `YTB_SUM_QUALITY`, `YTB_SUM_AUDIO_MODEL` | Models |
| `YTB_SUM_LANG`, `YTB_SUM_OUTPUT_DIR`, `YTB_SUM_BASE_URL` | Same as flags |
| `YTB_SUM_CONFIG` | Alternative config file path |
| `NO_COLOR` | Disable colors |

API key lookup order: `--api-key` → `OPENROUTER_API_KEY` → `YTB_SUM_API_KEY` → macOS Keychain (service `ytb-sum`) → config file.

## How it works

```text
URL ─▶ yt-dlp --dump-json ─▶ metadata, chapters, description   (memory)
     └▶ subtitle track (manual > original auto captions) ─▶ HTTP ─▶ transcript (memory)
        └ no subtitles + --audio ─▶ temp audio ─▶ ffmpeg 20-min chunks ─▶ Gemini ─▶ transcript
transcript + metadata ─▶ LLM with strict JSON schema ─▶ Summary
Summary ─▶ terminal (lipgloss + glamour)  and  <title>.md
```

The model answers in a strict JSON schema, so the terminal view and the Markdown file are rendered from the same data, and malformed answers are repaired automatically.

**No leftovers.** Nothing is written to disk except the final `.md`. Temporary directories are created only for the audio fallback (or a rare subtitle fallback), removed with `defer` on success, error and Ctrl+C; stale ones from a `kill -9` are swept on the next run.

## Troubleshooting

| Problem | Fix |
|---|---|
| `yt-dlp not found` | Install yt-dlp and make sure it's on `PATH` |
| `Sign in to confirm you're not a bot` / age-restricted | `--cookies-from-browser chrome`; keep yt-dlp updated (`yt-dlp -U` / `brew upgrade yt-dlp`) |
| `this video has no subtitles` | Re-run with `--audio` |
| `401` | Check the key: `ytb-sum config show` |
| `402` | Top up OpenRouter credits |
| `No endpoints found that can handle the requested parameters` | The model lacks structured-output support; pick another `--model` |

## Development

```bash
make test        # go test -race ./...
make lint        # golangci-lint
make build       # ./bin/ytb-sum
make snapshot    # goreleaser build for all platforms into ./dist
```

Releases are fully automated: push a tag `vX.Y.Z` and the [Release workflow](.github/workflows/release.yml) runs the tests and publishes binaries, packages and checksums with GoReleaser.

## License

[MIT](LICENSE)
